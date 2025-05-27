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

// OrchestratorV2 manages services using the new service registry architecture
type OrchestratorV2 struct {
	registry   services.ServiceRegistry
	kubeMgr    kube.Manager
	depGraph   *dependency.Graph
	
	// Configuration
	mcName       string
	wcName       string
	portForwards []config.PortForwardDefinition
	mcpServers   []config.MCPServerDefinition
	
	// Service tracking
	stopReasons     map[string]StopReason
	pendingRestarts map[string]bool
	
	// Context for cancellation
	ctx        context.Context
	cancelFunc context.CancelFunc
	
	mu sync.RWMutex
}

// ConfigV2 holds configuration for the new orchestrator
type ConfigV2 struct {
	MCName       string
	WCName       string
	PortForwards []config.PortForwardDefinition
	MCPServers   []config.MCPServerDefinition
}

// NewV2 creates a new orchestrator using the service registry
func NewV2(cfg ConfigV2) *OrchestratorV2 {
	// Create service registry
	registry := services.NewRegistry()
	
	// Create kube manager
	kubeMgr := kube.NewManager(nil)
	
	return &OrchestratorV2{
		registry:        registry,
		kubeMgr:         kubeMgr,
		mcName:          cfg.MCName,
		wcName:          cfg.WCName,
		portForwards:    cfg.PortForwards,
		mcpServers:      cfg.MCPServers,
		stopReasons:     make(map[string]StopReason),
		pendingRestarts: make(map[string]bool),
	}
}

// Start initializes and starts all services
func (o *OrchestratorV2) Start(ctx context.Context) error {
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
func (o *OrchestratorV2) Stop() error {
	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	
	// Stop all services in reverse dependency order
	return o.stopAllServices()
}

// StartService starts a specific service by label
func (o *OrchestratorV2) StartService(label string) error {
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
	
	logging.Info("OrchestratorV2", "Started service: %s", label)
	return nil
}

// StopService stops a specific service by label
func (o *OrchestratorV2) StopService(label string) error {
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
		logging.Error("OrchestratorV2", err, "Failed to stop dependent services for %s", label)
	}
	
	logging.Info("OrchestratorV2", "Stopped service: %s", label)
	return nil
}

// RestartService restarts a specific service by label
func (o *OrchestratorV2) RestartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}
	
	// Restart the service
	if err := service.Restart(o.ctx); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", label, err)
	}
	
	logging.Info("OrchestratorV2", "Restarted service: %s", label)
	return nil
}

// GetServiceRegistry returns the service registry for API access
func (o *OrchestratorV2) GetServiceRegistry() services.ServiceRegistry {
	return o.registry
}

// registerServices creates and registers all configured services
func (o *OrchestratorV2) registerServices() error {
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
func (o *OrchestratorV2) registerK8sServices() error {
	// Register MC connection if configured
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		mcLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)
		
		mcService := k8s.NewK8sConnectionService(mcLabel, mcContext, true, o.kubeMgr)
		o.registry.Register(mcService)
		
		logging.Debug("OrchestratorV2", "Registered K8s MC service: %s", mcLabel)
	}
	
	// Register WC connection if configured
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		wcLabel := fmt.Sprintf("k8s-wc-%s", o.wcName)
		
		wcService := k8s.NewK8sConnectionService(wcLabel, wcContext, false, o.kubeMgr)
		o.registry.Register(wcService)
		
		logging.Debug("OrchestratorV2", "Registered K8s WC service: %s", wcLabel)
	}
	
	return nil
}

// registerPortForwardServices registers port forward services
func (o *OrchestratorV2) registerPortForwardServices() error {
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}
		
		pfService := portforward.NewPortForwardService(pf, o.kubeMgr)
		o.registry.Register(pfService)
		
		logging.Debug("OrchestratorV2", "Registered port forward service: %s", pf.Name)
	}
	
	return nil
}

// registerMCPServices registers MCP server services
func (o *OrchestratorV2) registerMCPServices() error {
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}
		
		mcpService := mcpserver.NewMCPServerService(mcp)
		o.registry.Register(mcpService)
		
		logging.Debug("OrchestratorV2", "Registered MCP server service: %s", mcp.Name)
	}
	
	return nil
} 