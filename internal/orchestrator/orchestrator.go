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

// StopReason tracks why a service was stopped
type StopReason int

const (
	StopReasonManual     StopReason = iota // User explicitly stopped the service
	StopReasonDependency                   // Service stopped due to dependency failure
)

// Orchestrator manages services using the new service registry architecture
type Orchestrator struct {
	registry services.ServiceRegistry
	kubeMgr  kube.Manager
	depGraph *dependency.Graph

	// Configuration
	mcName       string
	wcName       string
	portForwards []config.PortForwardDefinition
	mcpServers   []config.MCPServerDefinition

	// Service tracking
	stopReasons     map[string]StopReason
	pendingRestarts map[string]bool
	healthCheckers  map[string]bool // Track which services have health checkers running

	// Context for cancellation
	ctx        context.Context
	cancelFunc context.CancelFunc

	mu sync.RWMutex
}

// Config holds configuration for the new orchestrator
type Config struct {
	MCName       string
	WCName       string
	PortForwards []config.PortForwardDefinition
	MCPServers   []config.MCPServerDefinition
}

// New creates a new orchestrator using the service registry
func New(cfg Config) *Orchestrator {
	// Create service registry
	registry := services.NewRegistry()

	// Create kube manager
	kubeMgr := kube.NewManager(nil)

	return &Orchestrator{
		registry:        registry,
		kubeMgr:         kubeMgr,
		mcName:          cfg.MCName,
		wcName:          cfg.WCName,
		portForwards:    cfg.PortForwards,
		mcpServers:      cfg.MCPServers,
		stopReasons:     make(map[string]StopReason),
		pendingRestarts: make(map[string]bool),
		healthCheckers:  make(map[string]bool),
	}
}

// Start initializes and starts all services
func (o *Orchestrator) Start(ctx context.Context) error {
	// Create cancellable context
	o.ctx, o.cancelFunc = context.WithCancel(ctx)

	// Build dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Register all services
	if err := o.registerServices(); err != nil {
		return fmt.Errorf("failed to register services: %w", err)
	}

	// Start services in dependency order
	if err := o.startServicesInOrder(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Start monitoring for service health and restarts
	go o.monitorServices()

	return nil
}

// Stop gracefully stops all services
func (o *Orchestrator) Stop() error {
	if o.cancelFunc != nil {
		o.cancelFunc()
	}

	// Stop all services in reverse dependency order
	return o.stopAllServices()
}

// StartService starts a specific service by label
func (o *Orchestrator) StartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Check dependencies first
	if err := o.checkDependencies(label); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Start the service
	if err := service.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start service %s: %w", label, err)
	}

	// Remove from stop reasons if it was there
	o.mu.Lock()
	delete(o.stopReasons, label)
	o.mu.Unlock()

	logging.Info("Orchestrator", "Started service: %s", label)
	return nil
}

// StopService stops a specific service by label
func (o *Orchestrator) StopService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Mark as manually stopped
	o.mu.Lock()
	o.stopReasons[label] = StopReasonManual
	o.mu.Unlock()

	// Stop the service
	if err := service.Stop(o.ctx); err != nil {
		return fmt.Errorf("failed to stop service %s: %w", label, err)
	}

	// Stop dependent services
	if err := o.stopDependentServices(label); err != nil {
		logging.Error("Orchestrator", err, "Failed to stop dependent services for %s", label)
	}

	logging.Info("Orchestrator", "Stopped service: %s", label)
	return nil
}

// RestartService restarts a specific service by label
func (o *Orchestrator) RestartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Restart the service
	if err := service.Restart(o.ctx); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", label, err)
	}

	logging.Info("Orchestrator", "Restarted service: %s", label)
	return nil
}

// GetServiceRegistry returns the service registry for API access
func (o *Orchestrator) GetServiceRegistry() services.ServiceRegistry {
	return o.registry
}

// registerServices creates and registers all configured services
func (o *Orchestrator) registerServices() error {
	// Register K8s connection services
	if err := o.registerK8sServices(); err != nil {
		return fmt.Errorf("failed to register K8s services: %w", err)
	}

	// Register port forward services
	if err := o.registerPortForwardServices(); err != nil {
		return fmt.Errorf("failed to register port forward services: %w", err)
	}

	// Register MCP server services
	if err := o.registerMCPServices(); err != nil {
		return fmt.Errorf("failed to register MCP services: %w", err)
	}

	return nil
}

// registerK8sServices registers Kubernetes connection services
func (o *Orchestrator) registerK8sServices() error {
	// Register MC connection if configured
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		mcLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)

		mcService := k8s.NewK8sConnectionService(mcLabel, mcContext, true, o.kubeMgr)
		o.registry.Register(mcService)

		logging.Debug("Orchestrator", "Registered K8s MC service: %s", mcLabel)
	}

	// Register WC connection if configured
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		wcLabel := fmt.Sprintf("k8s-wc-%s", o.wcName)

		wcService := k8s.NewK8sConnectionService(wcLabel, wcContext, false, o.kubeMgr)
		o.registry.Register(wcService)

		logging.Debug("Orchestrator", "Registered K8s WC service: %s", wcLabel)
	}

	return nil
}

// registerPortForwardServices registers port forward services
func (o *Orchestrator) registerPortForwardServices() error {
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}

		pfService := portforward.NewPortForwardService(pf, o.kubeMgr)
		o.registry.Register(pfService)

		logging.Debug("Orchestrator", "Registered port forward service: %s", pf.Name)
	}

	return nil
}

// registerMCPServices registers MCP server services
func (o *Orchestrator) registerMCPServices() error {
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}

		mcpService := mcpserver.NewMCPServerService(mcp)
		o.registry.Register(mcpService)

		logging.Debug("Orchestrator", "Registered MCP server service: %s", mcp.Name)
	}

	return nil
}
