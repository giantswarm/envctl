package orchestrator

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/services"
	agg "envctl/internal/services/aggregator"
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
	mcName       string
	wcName       string
	portForwards []config.PortForwardDefinition
	mcpServers   []config.MCPServerDefinition
	aggregator   config.AggregatorConfig
	yolo         bool

	// Cluster management
	clusterState *ClusterState
	clusters     []config.ClusterDefinition

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

// Config holds the configuration for the orchestrator.
// It includes both legacy fields (MCName, WCName) and new cluster-based configuration.
type Config struct {
	MCName         string                         // Management cluster name (DEPRECATED: use Clusters)
	WCName         string                         // Workload cluster name (DEPRECATED: use Clusters)
	Clusters       []config.ClusterDefinition     // Available clusters with roles
	ActiveClusters map[config.ClusterRole]string  // Initial active cluster for each role
	PortForwards   []config.PortForwardDefinition // Port forward configurations
	MCPServers     []config.MCPServerDefinition   // MCP server configurations
	Aggregator     config.AggregatorConfig        // Aggregator configuration
	Yolo           bool                           // Yolo mode for aggregator
}

// New creates a new orchestrator using the service registry.
// This initializes the orchestrator with the provided configuration but
// does not start any services. Call Start() to begin service management.
func New(cfg Config) *Orchestrator {
	// Create service registry
	registry := services.NewRegistry()

	// Create kube manager
	kubeMgr := kube.NewManager(nil)

	// If using legacy MC/WC config, convert to clusters
	clusters := cfg.Clusters
	activeClusters := cfg.ActiveClusters
	if len(clusters) == 0 && (cfg.MCName != "" || cfg.WCName != "") {
		// Legacy mode: generate clusters from MC/WC names
		clusters = config.GenerateGiantSwarmClusters(cfg.MCName, cfg.WCName)
		activeClusters = make(map[config.ClusterRole]string)
		for _, c := range clusters {
			if _, exists := activeClusters[c.Role]; !exists {
				activeClusters[c.Role] = c.Name
			}
		}
	}

	// Create cluster state
	clusterState := NewClusterState(clusters, activeClusters)

	return &Orchestrator{
		registry:               registry,
		kubeMgr:                kubeMgr,
		mcName:                 cfg.MCName,
		wcName:                 cfg.WCName,
		clusters:               clusters,
		clusterState:           clusterState,
		portForwards:           cfg.PortForwards,
		mcpServers:             cfg.MCPServers,
		aggregator:             cfg.Aggregator,
		yolo:                   cfg.Yolo,
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
// 4. Aggregator (depends on MCP servers)
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

	// Register aggregator service which depends on MCP servers
	if err := o.registerAggregatorService(); err != nil {
		return fmt.Errorf("failed to register aggregator service: %w", err)
	}

	return nil
}

// registerK8sServices registers Kubernetes connection services.
// These are the foundation services that establish connections to
// Giant Swarm clusters via Teleport. All other services depend on these.
func (o *Orchestrator) registerK8sServices() error {
	// Register all configured clusters
	for _, cluster := range o.clusters {
		k8sService := k8s.NewK8sConnectionService(
			cluster.Name,
			cluster.Context,
			cluster.Role == config.ClusterRoleObservability, // isMC for health check purposes
			o.kubeMgr,
		)

		// Set the global state change callback if configured
		o.mu.RLock()
		if o.globalStateChangeCallback != nil {
			k8sService.SetStateChangeCallback(o.globalStateChangeCallback)
		}
		o.mu.RUnlock()

		o.registry.Register(k8sService)

		// Register the API adapter for this service
		k8sAdapter := k8s.NewAPIAdapter(k8sService)
		k8sAdapter.Register()

		logging.Debug("Orchestrator", "Registered K8s service: %s (role: %s, context: %s)",
			cluster.Name, cluster.Role, cluster.Context)
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

		// Resolve the actual Kubernetes context for this port forward
		var context string
		var resolveError error
		if pf.ClusterName != "" {
			// Specific cluster requested
			if cluster, exists := o.clusterState.GetClusterByName(pf.ClusterName); exists {
				context = cluster.Context
			} else {
				resolveError = fmt.Errorf("cluster %s not found", pf.ClusterName)
				logging.Warn("Orchestrator", "Port forward %s: %v", pf.Name, resolveError)
			}
		} else if pf.ClusterRole != "" {
			// Use active cluster for role
			ctx, err := o.clusterState.GetActiveClusterContext(pf.ClusterRole)
			if err != nil {
				resolveError = err
				logging.Warn("Orchestrator", "Port forward %s: %v", pf.Name, err)
			} else {
				context = ctx
			}
		} else if pf.KubeContextTarget != "" {
			// Fallback to deprecated field
			context = pf.KubeContextTarget
		} else {
			resolveError = fmt.Errorf("no cluster specified")
			logging.Warn("Orchestrator", "Port forward %s: %v", pf.Name, resolveError)
		}

		// Update the port forward configuration with resolved context
		pfConfig := pf
		pfConfig.KubeContextTarget = context

		pfService := portforward.NewPortForwardService(pfConfig, o.kubeMgr)

		// Set the global state change callback if configured
		o.mu.RLock()
		if o.globalStateChangeCallback != nil {
			pfService.SetStateChangeCallback(o.globalStateChangeCallback)
		}
		o.mu.RUnlock()

		o.registry.Register(pfService)

		// Register the API adapter for this service
		pfAdapter := portforward.NewAPIAdapter(pfService)
		pfAdapter.Register()

		// If we couldn't resolve the cluster, mark it as stopped due to dependency
		if resolveError != nil {
			o.mu.Lock()
			o.stopReasons[pf.Name] = StopReasonDependency
			o.mu.Unlock()
			logging.Debug("Orchestrator", "Registered port forward service: %s (marked as dependency-stopped: %v)", pf.Name, resolveError)
		} else {
			logging.Debug("Orchestrator", "Registered port forward service: %s (context: %s)", pf.Name, context)
		}
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

		// Register the API adapter for this service
		mcpAdapter := mcpserver.NewAPIAdapter(mcpService)
		mcpAdapter.Register()

		// Check if this MCP server requires a cluster role that might not be available
		if mcp.RequiresClusterRole != "" {
			_, err := o.clusterState.GetActiveClusterContext(mcp.RequiresClusterRole)
			if err != nil {
				// Mark as stopped due to dependency
				o.mu.Lock()
				o.stopReasons[mcp.Name] = StopReasonDependency
				o.mu.Unlock()
				logging.Debug("Orchestrator", "Registered MCP server service: %s (marked as dependency-stopped: %v)", mcp.Name, err)
			} else {
				logging.Debug("Orchestrator", "Registered MCP server service: %s", mcp.Name)
			}
		} else {
			logging.Debug("Orchestrator", "Registered MCP server service: %s", mcp.Name)
		}
	}

	return nil
}

// registerAggregatorService registers the aggregator service.
// The aggregator service aggregates data from MCP servers and provides
// a unified view of service health and status.
func (o *Orchestrator) registerAggregatorService() error {
	// Only register if aggregator is enabled
	if !o.aggregator.Enabled {
		logging.Debug("Orchestrator", "Aggregator service is disabled, skipping registration")
		return nil
	}

	// Get APIs that the aggregator needs
	orchestratorAPI := api.NewOrchestratorAPI()
	mcpAPI := api.NewMCPServiceAPI()
	registryHandler := api.GetServiceRegistry()

	// Get config directory for workflows
	configDir, err := config.GetUserConfigDir()
	if err != nil {
		// Fall back to a default if we can't get the user config dir
		logging.Warn("Orchestrator", "Failed to get user config directory, using default: %v", err)
		configDir = ".config/envctl"
	}

	// Create aggregator configuration
	aggConfig := aggregator.AggregatorConfig{
		Host:      o.aggregator.Host,
		Port:      o.aggregator.Port,
		Yolo:      o.yolo,
		ConfigDir: configDir,
	}

	// Set defaults if not configured
	if aggConfig.Host == "" {
		aggConfig.Host = "localhost"
	}
	if aggConfig.Port == 0 {
		aggConfig.Port = 8080
	}

	// Create aggregator service
	aggService := agg.NewAggregatorService(aggConfig, orchestratorAPI, mcpAPI, registryHandler)

	// Set the global state change callback if configured
	o.mu.RLock()
	if o.globalStateChangeCallback != nil {
		aggService.SetStateChangeCallback(o.globalStateChangeCallback)
	}
	o.mu.RUnlock()

	o.registry.Register(aggService)

	// Register the API adapter for this service
	aggAdapter := agg.NewAPIAdapter(aggService)
	aggAdapter.Register()

	logging.Debug("Orchestrator", "Registered aggregator service on port %d", o.aggregator.Port)
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

		// Get the service to retrieve its type
		var serviceType string
		if service, exists := o.registry.Get(label); exists {
			serviceType = string(service.GetType())
		}

		// Broadcast to all subscribers
		event := ServiceStateChangedEvent{
			Label:       label,
			ServiceType: serviceType,
			OldState:    string(oldState),
			NewState:    string(newState),
			Health:      string(health),
			Error:       err,
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
	Label       string
	ServiceType string
	OldState    string
	NewState    string
	Health      string
	Error       error
}

// GetAvailableClusters returns all clusters configured for a specific role
func (o *Orchestrator) GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition {
	return o.clusterState.GetAvailableClusters(role)
}

// GetActiveCluster returns the currently active cluster for a role
func (o *Orchestrator) GetActiveCluster(role config.ClusterRole) (string, bool) {
	return o.clusterState.GetActiveCluster(role)
}

// SwitchCluster changes the active cluster for a role and restarts affected services
func (o *Orchestrator) SwitchCluster(role config.ClusterRole, clusterName string) error {
	// Get the old cluster name before switching
	oldCluster, _ := o.clusterState.GetActiveCluster(role)

	// Update cluster state
	if err := o.clusterState.SetActiveCluster(role, clusterName); err != nil {
		return err
	}

	logging.Info("Orchestrator", "Switching %s cluster from %s to %s", role, oldCluster, clusterName)

	// Find affected services that need to be restarted
	affectedServices := o.findServicesUsingClusterRole(role)

	// Stop affected services
	for _, svcLabel := range affectedServices {
		logging.Debug("Orchestrator", "Stopping service %s for cluster switch", svcLabel)
		if err := o.StopService(svcLabel); err != nil {
			logging.Error("Orchestrator", err, "Failed to stop service %s for cluster switch", svcLabel)
		}
	}

	// Clear the manual stop reason so services can be restarted
	o.mu.Lock()
	for _, svcLabel := range affectedServices {
		delete(o.stopReasons, svcLabel)
	}
	o.mu.Unlock()

	// Restart affected services with new cluster
	for _, svcLabel := range affectedServices {
		logging.Debug("Orchestrator", "Starting service %s with new cluster", svcLabel)
		if err := o.StartService(svcLabel); err != nil {
			logging.Error("Orchestrator", err, "Failed to restart service %s after cluster switch", svcLabel)
		}
	}

	return nil
}

// findServicesUsingClusterRole finds all services that depend on a specific cluster role
func (o *Orchestrator) findServicesUsingClusterRole(role config.ClusterRole) []string {
	var affected []string

	// Check K8s connection services
	activeClusterName, exists := o.clusterState.GetActiveCluster(role)
	if exists {
		// The K8s connection service itself
		if _, exists := o.registry.Get(activeClusterName); exists {
			affected = append(affected, activeClusterName)
		}
	}

	// Check port forwards
	for _, pf := range o.portForwards {
		if pf.ClusterRole == role {
			affected = append(affected, pf.Name)
		}
	}

	// Check MCP servers
	for _, mcp := range o.mcpServers {
		if mcp.RequiresClusterRole == role {
			affected = append(affected, mcp.Name)
		}
	}

	return affected
}
