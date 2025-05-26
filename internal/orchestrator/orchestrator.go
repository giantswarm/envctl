package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// Orchestrator manages the overall application state, including:
// - K8s connection health monitoring
// - Service lifecycle based on dependencies
// - Works for both TUI and non-TUI modes
type Orchestrator struct {
	kubeMgr     k8smanager.KubeManagerAPI
	serviceMgr  managers.ServiceManagerAPI
	k8sStateMgr k8smanager.K8sStateManager
	depGraph    *dependency.Graph
	reporter    reporting.ServiceReporter

	// Configuration
	mcName       string
	wcName       string
	portForwards []config.PortForwardDefinition
	mcpServers   []config.MCPServerDefinition

	// Health monitoring
	healthCheckInterval time.Duration
	cancelHealthChecks  context.CancelFunc
	mu                  sync.RWMutex
}

// Config holds the configuration for the orchestrator
type Config struct {
	MCName              string
	WCName              string
	PortForwards        []config.PortForwardDefinition
	MCPServers          []config.MCPServerDefinition
	HealthCheckInterval time.Duration
}

// New creates a new Orchestrator
func New(
	kubeMgr k8smanager.KubeManagerAPI,
	serviceMgr managers.ServiceManagerAPI,
	reporter reporting.ServiceReporter,
	cfg Config,
) *Orchestrator {
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 15 * time.Second
	}

	return &Orchestrator{
		kubeMgr:             kubeMgr,
		serviceMgr:          serviceMgr,
		k8sStateMgr:         k8smanager.NewK8sStateManager(kubeMgr),
		reporter:            reporter,
		mcName:              cfg.MCName,
		wcName:              cfg.WCName,
		portForwards:        cfg.PortForwards,
		mcpServers:          cfg.MCPServers,
		healthCheckInterval: cfg.HealthCheckInterval,
	}
}

// Start begins orchestration - builds dependency graph, starts services, and monitors health
func (o *Orchestrator) Start(ctx context.Context) error {
	// Build dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Start initial services
	if err := o.startServices(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Start health monitoring
	healthCtx, cancel := context.WithCancel(ctx)
	o.cancelHealthChecks = cancel
	go o.monitorHealth(healthCtx)

	return nil
}

// Stop gracefully stops all services and health monitoring
func (o *Orchestrator) Stop() {
	if o.cancelHealthChecks != nil {
		o.cancelHealthChecks()
	}

	if o.serviceMgr != nil {
		o.serviceMgr.StopAllServices()
	}
}

// buildDependencyGraph constructs the dependency graph for services
func (o *Orchestrator) buildDependencyGraph() *dependency.Graph {
	g := dependency.New()

	// Add k8s connection nodes
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("k8s:" + mcContext),
			FriendlyName: "K8s MC Connection (" + o.mcName + ")",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil,
		})
	}

	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("k8s:" + wcContext),
			FriendlyName: "K8s WC Connection (" + o.wcName + ")",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil,
		})
	}

	// Add port forward nodes
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}

		deps := []dependency.NodeID{}
		// Determine which k8s context this port forward uses
		contextName := pf.KubeContextTarget
		if contextName != "" {
			deps = append(deps, dependency.NodeID("k8s:"+contextName))
		}

		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("pf:" + pf.Name),
			FriendlyName: pf.Name,
			Kind:         dependency.KindPortForward,
			DependsOn:    deps,
		})
	}

	// Add MCP server nodes
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}

		deps := []dependency.NodeID{}

		// Special handling for kubernetes MCP - it depends on MC k8s connection
		if mcp.Name == "kubernetes" && o.mcName != "" {
			mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
			deps = append(deps, dependency.NodeID("k8s:"+mcContext))
		}

		// Add port forward dependencies
		for _, requiredPf := range mcp.RequiresPortForwards {
			deps = append(deps, dependency.NodeID("pf:"+requiredPf))
		}

		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("mcp:" + mcp.Name),
			FriendlyName: mcp.Name,
			Kind:         dependency.KindMCP,
			DependsOn:    deps,
		})
	}

	return g
}

// startServices starts all configured services
func (o *Orchestrator) startServices() error {
	var managedConfigs []managers.ManagedServiceConfig

	// Add port forwards
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}
		managedConfigs = append(managedConfigs, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypePortForward,
			Label:  pf.Name,
			Config: pf,
		})
	}

	// Add MCP servers
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}
		managedConfigs = append(managedConfigs, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeMCPServer,
			Label:  mcp.Name,
			Config: mcp,
		})
	}

	if len(managedConfigs) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	_, errs := o.serviceMgr.StartServicesWithDependencyOrder(managedConfigs, o.depGraph, &wg)

	if len(errs) > 0 {
		return fmt.Errorf("service startup errors: %v", errs)
	}

	return nil
}

// monitorHealth continuously monitors k8s connection health
func (o *Orchestrator) monitorHealth(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Error("Orchestrator", fmt.Errorf("panic in monitorHealth: %v", r),
				"Panic recovered in health monitoring goroutine")
		}
	}()

	ticker := time.NewTicker(o.healthCheckInterval)
	defer ticker.Stop()

	// Initial health check
	o.checkHealth(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.checkHealth(ctx)
		}
	}
}

// checkHealth performs health checks on k8s connections
func (o *Orchestrator) checkHealth(ctx context.Context) {
	// Check MC health
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		o.checkConnectionHealth(ctx, mcContext, true)
	}

	// Check WC health
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		o.checkConnectionHealth(ctx, wcContext, false)
	}
}

// checkConnectionHealth checks a specific k8s connection and manages dependent services
func (o *Orchestrator) checkConnectionHealth(ctx context.Context, contextName string, isMC bool) {
	// Get previous state
	previousState := o.k8sStateMgr.GetConnectionState(contextName)
	wasHealthy := previousState.IsHealthy
	hadPreviousState := previousState.LastHealthCheck.After(time.Time{}) // Check if we have a previous check

	// Perform health check
	health, err := o.kubeMgr.GetClusterNodeHealth(ctx, contextName)

	isHealthy := err == nil && health.Error == nil

	// Update state
	o.k8sStateMgr.SetHealthy(contextName, isHealthy, err)

	// Log result
	clusterType := "WC"
	if isMC {
		clusterType = "MC"
	}

	if isHealthy {
		logging.Info("Orchestrator", "[HEALTH %s] Nodes: %d/%d", clusterType, health.ReadyNodes, health.TotalNodes)
	} else {
		logging.Error("Orchestrator", err, "[HEALTH %s] Connection unhealthy", clusterType)
	}

	// Report health status to the UI/console via reporter
	if o.reporter != nil {
		healthUpdate := reporting.HealthStatusUpdate{
			Timestamp:        time.Now(),
			ContextName:      contextName,
			ClusterShortName: o.mcName,
			IsMC:             isMC,
			IsHealthy:        isHealthy,
			ReadyNodes:       health.ReadyNodes,
			TotalNodes:       health.TotalNodes,
			Error:            err,
		}
		if !isMC {
			healthUpdate.ClusterShortName = o.wcName
		}

		o.reporter.ReportHealth(healthUpdate)
	}

	// Handle state changes (only if we had a previous state)
	if hadPreviousState && wasHealthy != isHealthy {
		k8sNodeID := "k8s:" + contextName

		if !isHealthy {
			// Connection became unhealthy - stop dependent services
			logging.Info("Orchestrator", "K8s connection %s became unhealthy, stopping dependent services", contextName)
			if err := o.serviceMgr.StopServiceWithDependents(k8sNodeID, o.depGraph); err != nil {
				logging.Error("Orchestrator", err, "Failed to stop services dependent on %s", contextName)
			}
		} else {
			// Connection became healthy - restart dependent services
			logging.Info("Orchestrator", "K8s connection %s is healthy again, restarting dependent services", contextName)
			if err := o.serviceMgr.StartServicesDependingOn(k8sNodeID, o.depGraph); err != nil {
				logging.Error("Orchestrator", err, "Failed to restart services dependent on %s", contextName)
			}
		}
	}
}

// GetDependencyGraph returns the current dependency graph
func (o *Orchestrator) GetDependencyGraph() *dependency.Graph {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.depGraph
}

// GetK8sStateManager returns the k8s state manager
func (o *Orchestrator) GetK8sStateManager() k8smanager.K8sStateManager {
	return o.k8sStateMgr
}

// StopService stops a specific service through the orchestrator
// This allows the TUI to stop services while respecting dependencies
func (o *Orchestrator) StopService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Use cascading stop to properly handle dependencies
	return o.serviceMgr.StopServiceWithDependents(label, o.depGraph)
}

// RestartService restarts a specific service through the orchestrator
func (o *Orchestrator) RestartService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	return o.serviceMgr.RestartService(label)
}

// ReconfigureAndRestart stops all services and restarts with new configuration
// This is used when switching clusters
func (o *Orchestrator) ReconfigureAndRestart(mcName, wcName string, portForwards []config.PortForwardDefinition, mcpServers []config.MCPServerDefinition) error {
	// Stop all current services
	if o.serviceMgr != nil {
		o.serviceMgr.StopAllServices()
		// Give services a moment to stop cleanly
		time.Sleep(250 * time.Millisecond)
	}

	// Update configuration
	o.mcName = mcName
	o.wcName = wcName
	o.portForwards = portForwards
	o.mcpServers = mcpServers

	// Rebuild dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Start services with new configuration
	return o.startServices()
}
