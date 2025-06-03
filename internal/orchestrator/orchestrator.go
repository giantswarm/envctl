package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/services"
	"envctl/internal/services/k8s"
	"envctl/internal/services/mcpserver"
	"envctl/internal/services/portforward"
	"envctl/pkg/logging"
	"fmt"
	"sync"
)

// StopReason tracks why a service was stopped.
// This is crucial for the auto-recovery mechanism to distinguish between
// user-initiated stops (which should not be auto-restarted) and
// dependency-related stops (which should be auto-restarted when dependencies recover).
type StopReason int

const (
	StopReasonManual     StopReason = iota // User explicitly stopped the service
	StopReasonDependency                   // Service stopped due to dependency failure
)

// Orchestrator manages services using the new service registry architecture.
// It coordinates the lifecycle of all services, handles dependencies, and
// provides automatic recovery capabilities. The orchestrator is the central
// control point for all service operations in envctl.
type Orchestrator struct {
	registry services.ServiceRegistry
	kubeMgr  kube.Manager
	depGraph *dependency.Graph

	// Configuration
	mcName         string
	wcName         string
	portForwards   []config.PortForwardDefinition
	mcpServers     []config.MCPServerDefinition
	aggregatorPort int // Port for the MCP aggregator

	// Service tracking
	stopReasons     map[string]StopReason // Tracks why each service was stopped for auto-recovery decisions
	pendingRestarts map[string]bool       // Services waiting to be restarted after dependency recovery
	healthCheckers  map[string]bool       // Track which services have health checkers running to avoid duplicates

	// Global state change callback
	globalStateChangeCallback services.StateChangeCallback

	// State change event subscribers
	stateChangeSubscribers []chan<- ServiceStateChangedEvent

	// Context for cancellation
	ctx        context.Context
	cancelFunc context.CancelFunc

	mu sync.RWMutex // Protects concurrent access to service tracking maps
}

// Config holds configuration for the new orchestrator.
// This structure is passed during orchestrator creation to define
// which services should be managed and their configurations.
type Config struct {
	MCName         string                         // Management cluster name
	WCName         string                         // Workload cluster name (optional)
	PortForwards   []config.PortForwardDefinition // Port forward configurations
	MCPServers     []config.MCPServerDefinition   // MCP server configurations
	AggregatorPort int                            // Port for the MCP aggregator (default: 8080)
}

// New creates a new orchestrator using the service registry.
// This initializes the orchestrator with the provided configuration but
// does not start any services. Call Start() to begin service management.
func New(cfg Config) *Orchestrator {
	// Create service registry
	registry := services.NewRegistry()

	// Create kube manager
	kubeMgr := kube.NewManager(nil)

	return &Orchestrator{
		registry:               registry,
		kubeMgr:                kubeMgr,
		mcName:                 cfg.MCName,
		wcName:                 cfg.WCName,
		portForwards:           cfg.PortForwards,
		mcpServers:             cfg.MCPServers,
		aggregatorPort:         cfg.AggregatorPort,
		stopReasons:            make(map[string]StopReason),
		pendingRestarts:        make(map[string]bool),
		healthCheckers:         make(map[string]bool),
		stateChangeSubscribers: make([]chan<- ServiceStateChangedEvent, 0),
	}
}

// Start initializes and starts all services.
// This method:
// 1. Builds the dependency graph to understand service relationships
// 2. Registers all configured services with the registry
// 3. Starts services in dependency order (K8s connections → port forwards → MCP servers)
// 4. Begins monitoring services for health and auto-recovery
//
// The method is idempotent and can be called multiple times safely.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Create cancellable context for graceful shutdown
	o.ctx, o.cancelFunc = context.WithCancel(ctx)

	// Initialize the internal state change callback system
	// This ensures all services get proper state change monitoring
	o.setGlobalStateChangeCallback(nil)

	// Build dependency graph to understand service relationships
	o.depGraph = o.buildDependencyGraph()

	// Register all services with the registry
	if err := o.registerServices(); err != nil {
		return fmt.Errorf("failed to register services: %w", err)
	}

	// Start services in dependency order to ensure prerequisites are met
	if err := o.startServicesInOrder(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Start monitoring for service health and restarts
	// This goroutine runs for the lifetime of the orchestrator
	go o.monitorServices()

	return nil
}

// Stop gracefully stops all services.
// Services are stopped in reverse dependency order to ensure
// dependent services are stopped before their dependencies.
// This prevents errors and ensures clean shutdown.
func (o *Orchestrator) Stop() error {
	if o.cancelFunc != nil {
		o.cancelFunc()
	}

	// Stop all services in reverse dependency order
	return o.stopAllServices()
}

// StartService starts a specific service by label.
// This method ensures all dependencies are running before starting the service.
// If dependencies are not running, they will be started automatically.
// The service is removed from the stop reasons tracking to enable auto-recovery.
func (o *Orchestrator) StartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	logging.Debug("Orchestrator", "Starting service %s (type: %s)", label, service.GetType())

	// Check and start dependencies first to ensure prerequisites are met
	if err := o.checkDependencies(label); err != nil {
		logging.Debug("Orchestrator", "Service %s failed dependency check: %v", label, err)

		// Mark the service as stopped due to dependency failure
		// This ensures it will be auto-started when dependencies become available
		o.mu.Lock()
		o.stopReasons[label] = StopReasonDependency
		o.mu.Unlock()

		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Start the service
	if err := service.Start(o.ctx); err != nil {
		logging.Debug("Orchestrator", "Service %s failed to start: %v", label, err)
		return fmt.Errorf("failed to start service %s: %w", label, err)
	}

	// Remove from stop reasons to enable auto-recovery if it fails later
	o.mu.Lock()
	delete(o.stopReasons, label)
	o.mu.Unlock()

	logging.Info("Orchestrator", "Started service: %s", label)
	return nil
}

// StopService stops a specific service by label.
// This method:
// 1. Marks the service as manually stopped (prevents auto-restart)
// 2. Stops the service itself
// 3. Cascades the stop to all dependent services
//
// Manually stopped services will not be auto-restarted even if their
// dependencies recover. This respects user intent.
func (o *Orchestrator) StopService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Mark as manually stopped to prevent auto-restart
	o.mu.Lock()
	o.stopReasons[label] = StopReasonManual
	o.mu.Unlock()

	// Stop the service
	if err := service.Stop(o.ctx); err != nil {
		return fmt.Errorf("failed to stop service %s: %w", label, err)
	}

	// Stop dependent services to maintain consistency
	// We continue even if this fails to ensure the main service is stopped
	if err := o.stopDependentServices(label); err != nil {
		logging.Error("Orchestrator", err, "Failed to stop dependent services for %s", label)
	}

	logging.Info("Orchestrator", "Stopped service: %s", label)
	return nil
}

// RestartService restarts a specific service by label.
// This is a convenience method that stops and starts the service.
// Unlike stop/start separately, this maintains the service's auto-recovery status.
func (o *Orchestrator) RestartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Restart the service (internally handles stop/start sequence)
	if err := service.Restart(o.ctx); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", label, err)
	}

	logging.Info("Orchestrator", "Restarted service: %s", label)
	return nil
}

// GetServiceRegistry returns the service registry for API access.
// This allows external components (like the API server) to query
// service status without going through the orchestrator.
func (o *Orchestrator) GetServiceRegistry() services.ServiceRegistry {
	return o.registry
}

// handleServiceStateChange handles immediate processing of service state changes
func (o *Orchestrator) handleServiceStateChange(label string, oldState, newState services.ServiceState) {
	// If a service just became running, immediately check for dependent services
	if oldState != services.StateRunning && newState == services.StateRunning {
		logging.Debug("Orchestrator", "Service %s became running, immediately checking dependent services", label)

		// Get the service to check if it needs health monitoring
		if service, exists := o.registry.Get(label); exists {
			// Start health checker if needed
			o.mu.RLock()
			hasHealthChecker := o.healthCheckers[label]
			o.mu.RUnlock()

			if !hasHealthChecker {
				if healthChecker, ok := service.(services.HealthChecker); ok {
					logging.Debug("Orchestrator", "Starting health checker for newly running service: %s", label)

					// Mark health checker as running
					o.mu.Lock()
					o.healthCheckers[label] = true
					o.mu.Unlock()

					go o.runHealthChecksForService(service, healthChecker)
				}
			}
		}

		// Start dependent services in a goroutine to avoid blocking
		go o.startDependentServices(label)
	}
}

// registerServices creates and registers all configured services.
// Services are registered in a specific order to ensure proper initialization:
// 1. K8s connections (foundation services)
// 2. Port forwards (depend on K8s connections)
// 3. MCP servers (may depend on port forwards)
// 4. MCP aggregator (depends on MCP servers)
//
// Registration does not start services, it only makes them available
// in the registry for later management.
func (o *Orchestrator) registerServices() error {
	// Register K8s connection services first as they are the foundation
	if err := o.registerK8sServices(); err != nil {
		return fmt.Errorf("failed to register K8s services: %w", err)
	}

	// Register port forward services which depend on K8s connections
	if err := o.registerPortForwardServices(); err != nil {
		return fmt.Errorf("failed to register port forward services: %w", err)
	}

	// Register MCP server services which may depend on port forwards
	if err := o.registerMCPServices(); err != nil {
		return fmt.Errorf("failed to register MCP services: %w", err)
	}

	// Register the aggregator service which depends on MCP servers
	if err := o.registerAggregatorService(); err != nil {
		return fmt.Errorf("failed to register aggregator service: %w", err)
	}

	return nil
}

// registerK8sServices registers Kubernetes connection services.
// These are the foundation services that establish connections to
// Giant Swarm clusters via Teleport. All other services depend on these.
func (o *Orchestrator) registerK8sServices() error {
	// Register MC connection if configured
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		mcLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)

		mcService := k8s.NewK8sConnectionService(mcLabel, mcContext, true, o.kubeMgr)

		// Set the global state change callback if configured
		o.mu.RLock()
		if o.globalStateChangeCallback != nil {
			mcService.SetStateChangeCallback(o.globalStateChangeCallback)
		}
		o.mu.RUnlock()

		o.registry.Register(mcService)

		logging.Debug("Orchestrator", "Registered K8s MC service: %s", mcLabel)
	}

	// Register WC connection if configured
	// WC connections require an MC name to build the full context name
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		wcLabel := fmt.Sprintf("k8s-wc-%s", o.wcName)

		wcService := k8s.NewK8sConnectionService(wcLabel, wcContext, false, o.kubeMgr)

		// Set the global state change callback if configured
		o.mu.RLock()
		if o.globalStateChangeCallback != nil {
			wcService.SetStateChangeCallback(o.globalStateChangeCallback)
		}
		o.mu.RUnlock()

		o.registry.Register(wcService)

		logging.Debug("Orchestrator", "Registered K8s WC service: %s", wcLabel)
	}

	return nil
}

// registerPortForwardServices registers port forward services.
// These services create kubectl port-forward tunnels to expose
// cluster services locally. They depend on K8s connections.
func (o *Orchestrator) registerPortForwardServices() error {
	for _, pf := range o.portForwards {
		// Skip disabled port forwards
		if !pf.Enabled {
			continue
		}

		pfService := portforward.NewPortForwardService(pf, o.kubeMgr)

		// Set the global state change callback if configured
		o.mu.RLock()
		if o.globalStateChangeCallback != nil {
			pfService.SetStateChangeCallback(o.globalStateChangeCallback)
		}
		o.mu.RUnlock()

		o.registry.Register(pfService)

		logging.Debug("Orchestrator", "Registered port forward service: %s", pf.Name)
	}

	return nil
}

// registerMCPServices registers MCP server services.
// These services run Model Context Protocol servers that provide
// AI assistants with access to Kubernetes and monitoring data.
// They may depend on port forwards for accessing cluster services.
func (o *Orchestrator) registerMCPServices() error {
	for _, mcp := range o.mcpServers {
		// Skip disabled MCP servers
		if !mcp.Enabled {
			continue
		}

		mcpService := mcpserver.NewMCPServerService(mcp)

		// Set the global state change callback if configured
		o.mu.RLock()
		if o.globalStateChangeCallback != nil {
			mcpService.SetStateChangeCallback(o.globalStateChangeCallback)
		}
		o.mu.RUnlock()

		o.registry.Register(mcpService)

		logging.Debug("Orchestrator", "Registered MCP server service: %s", mcp.Name)
	}

	return nil
}

// registerAggregatorService registers the MCP aggregator service.
// The aggregator provides a single SSE endpoint that aggregates all
// MCP servers, making it easier for AI assistants to discover and use tools.
func (o *Orchestrator) registerAggregatorService() error {
	// The aggregator is now registered externally in connect.go
	// after the APIs are created, to avoid circular dependencies
	return nil
}

// setGlobalStateChangeCallback sets a callback that will be called for all service state changes.
// This is now private and should only be used internally by the orchestrator.
func (o *Orchestrator) setGlobalStateChangeCallback(callback services.StateChangeCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create a callback that handles both internal logic and broadcasts to subscribers
	wrappedCallback := func(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
		// Handle orchestrator-specific logic for state changes
		o.handleServiceStateChange(label, oldState, newState)

		// Broadcast to all subscribers
		event := ServiceStateChangedEvent{
			Label:    label,
			OldState: string(oldState),
			NewState: string(newState),
			Health:   string(health),
			Error:    err,
		}

		// Send to all subscribers (don't hold the lock while sending)
		o.mu.RLock()
		subscribers := make([]chan<- ServiceStateChangedEvent, len(o.stateChangeSubscribers))
		copy(subscribers, o.stateChangeSubscribers)
		o.mu.RUnlock()

		for _, ch := range subscribers {
			select {
			case ch <- event:
				// Event sent successfully
			default:
				// Channel full, drop event
				logging.Warn("Orchestrator", "Dropped state change event for %s (subscriber channel full)", label)
			}
		}
	}

	o.globalStateChangeCallback = wrappedCallback

	// Apply the callback to all already-registered services
	// This is crucial because services might be registered before the callback is set
	allServices := o.registry.GetAll()
	for _, service := range allServices {
		service.SetStateChangeCallback(wrappedCallback)
	}
}

// SubscribeToStateChanges returns a channel for receiving service state change events.
// This is the public interface for external components to monitor state changes.
func (o *Orchestrator) SubscribeToStateChanges() <-chan ServiceStateChangedEvent {
	eventChan := make(chan ServiceStateChangedEvent, 100)

	// Add this channel to the list of subscribers
	o.mu.Lock()
	o.stateChangeSubscribers = append(o.stateChangeSubscribers, eventChan)
	o.mu.Unlock()

	return eventChan
}

// ServiceStateChangedEvent represents a service state change event
type ServiceStateChangedEvent struct {
	Label    string
	OldState string
	NewState string
	Health   string
	Error    error
}
