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
	"strings"
	"sync"
	"time"
)

// StopReason tracks why a service was stopped
type StopReason int

const (
	StopReasonManual     StopReason = iota // User explicitly stopped the service
	StopReasonDependency                   // Service stopped due to dependency failure
)

// Orchestrator manages the overall application state, including:
// - K8s connection health monitoring
// - Service lifecycle based on dependencies
// - Restart logic
// - Cascade stop logic
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

	// Service state tracking
	stopReasons      map[string]StopReason           // Track why services were stopped
	pendingRestarts  map[string]bool                 // Track services pending restart
	serviceConfigs   map[string]managers.ManagedServiceConfig // Store all service configs
	activeWaitGroup  *sync.WaitGroup                 // Track active services
	
	mu sync.RWMutex
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
		stopReasons:         make(map[string]StopReason),
		pendingRestarts:     make(map[string]bool),
		serviceConfigs:      make(map[string]managers.ManagedServiceConfig),
		activeWaitGroup:     &sync.WaitGroup{},
	}
}

// Start begins orchestration - builds dependency graph, starts services, and monitors health
func (o *Orchestrator) Start(ctx context.Context) error {
	// Build dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Initialize service configs
	o.initializeServiceConfigs()

	// Set up service state monitoring
	o.setupServiceStateMonitoring()

	// Start health monitoring first
	healthCtx, cancel := context.WithCancel(ctx)
	o.cancelHealthChecks = cancel
	go o.monitorHealth(healthCtx)

	// Perform initial health check and wait for results
	o.checkHealth(ctx)

	// Give health check a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Start only services whose dependencies are healthy
	if err := o.startServicesWithHealthCheck(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

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

// initializeServiceConfigs builds the initial service configuration map
func (o *Orchestrator) initializeServiceConfigs() {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Add port forward configs
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}
		o.serviceConfigs[pf.Name] = managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypePortForward,
			Label:  pf.Name,
			Config: pf,
		}
	}

	// Add MCP server configs
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}
		o.serviceConfigs[mcp.Name] = managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeMCPServer,
			Label:  mcp.Name,
			Config: mcp,
		}
	}
}

// setupServiceStateMonitoring sets up monitoring for service state changes
func (o *Orchestrator) setupServiceStateMonitoring() {
	// Create a custom reporter that intercepts updates for restart logic
	interceptor := &serviceStateInterceptor{
		orchestrator:    o,
		originalReporter: o.reporter,
	}
	o.serviceMgr.SetReporter(interceptor)
}

// serviceStateInterceptor intercepts service state updates to handle restarts
type serviceStateInterceptor struct {
	orchestrator     *Orchestrator
	originalReporter reporting.ServiceReporter
}

// Report intercepts service state updates
func (s *serviceStateInterceptor) Report(update reporting.ManagedServiceUpdate) {
	// Forward to original reporter
	if s.originalReporter != nil {
		s.originalReporter.Report(update)
	}

	// Check for restart handling
	s.orchestrator.handleServiceStateUpdate(update)
}

// ReportHealth forwards health reports
func (s *serviceStateInterceptor) ReportHealth(update reporting.HealthStatusUpdate) {
	if s.originalReporter != nil {
		s.originalReporter.ReportHealth(update)
	}
}

// handleServiceStateUpdate handles service state changes for restart logic
func (o *Orchestrator) handleServiceStateUpdate(update reporting.ManagedServiceUpdate) {
	if update.State == reporting.StateStopped || update.State == reporting.StateFailed {
		o.mu.Lock()
		defer o.mu.Unlock()

		// Check if this service has a pending restart
		if o.pendingRestarts[update.SourceLabel] {
			delete(o.pendingRestarts, update.SourceLabel)
			delete(o.stopReasons, update.SourceLabel) // Clear stop reason for restart

			// Schedule restart
			go func() {
				time.Sleep(100 * time.Millisecond) // Small delay to ensure clean stop
				
				o.mu.RLock()
				cfg, exists := o.serviceConfigs[update.SourceLabel]
				o.mu.RUnlock()
				
				if exists {
					logging.Info("Orchestrator", "Restarting service %s", update.SourceLabel)
					configs := []managers.ManagedServiceConfig{cfg}
					o.startServicesInDependencyOrder(configs)
				}
			}()
		}
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

// startServicesWithHealthCheck starts only services whose K8s dependencies are healthy
func (o *Orchestrator) startServicesWithHealthCheck() error {
	var managedConfigs []managers.ManagedServiceConfig

	// Check which K8s connections are healthy
	healthyContexts := make(map[string]bool)
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		state := o.k8sStateMgr.GetConnectionState(mcContext)
		healthyContexts[mcContext] = state.IsHealthy
	}
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		state := o.k8sStateMgr.GetConnectionState(wcContext)
		healthyContexts[wcContext] = state.IsHealthy
	}

	// Filter services based on K8s health
	o.mu.RLock()
	for label, cfg := range o.serviceConfigs {
		// Skip manually stopped services
		if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
			continue
		}

		// Check if service's K8s dependency is healthy
		shouldStart := true
		
		if pfConfig, ok := cfg.Config.(config.PortForwardDefinition); ok && pfConfig.KubeContextTarget != "" {
			if healthy, exists := healthyContexts[pfConfig.KubeContextTarget]; !exists || !healthy {
				logging.Info("Orchestrator", "Skipping port forward %s - K8s context %s not healthy", pfConfig.Name, pfConfig.KubeContextTarget)
				shouldStart = false
			}
		}

		// Special check for kubernetes MCP
		if mcpConfig, ok := cfg.Config.(config.MCPServerDefinition); ok && mcpConfig.Name == "kubernetes" && o.mcName != "" {
			mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
			if healthy, exists := healthyContexts[mcContext]; !exists || !healthy {
				logging.Info("Orchestrator", "Skipping kubernetes MCP - MC context not healthy")
				shouldStart = false
			}
		}

		if shouldStart {
			managedConfigs = append(managedConfigs, cfg)
		}
	}
	o.mu.RUnlock()

	if len(managedConfigs) == 0 {
		logging.Info("Orchestrator", "No services to start - waiting for K8s connections to become healthy")
		return nil
	}

	return o.startServicesInDependencyOrder(managedConfigs)
}

// startServicesInDependencyOrder starts services in the correct order based on dependencies
func (o *Orchestrator) startServicesInDependencyOrder(configs []managers.ManagedServiceConfig) error {
	if o.depGraph == nil {
		// No dependency graph, start all at once
		_, errs := o.serviceMgr.StartServices(configs, o.activeWaitGroup)
		if len(errs) > 0 {
			return fmt.Errorf("service startup errors: %v", errs)
		}
		return nil
	}

	// Group services by dependency levels
	levels := o.groupServicesByDependencyLevel(configs)

	// Start services level by level
	for levelIndex, levelConfigs := range levels {
		if len(levelConfigs) == 0 {
			continue
		}

		logging.Info("Orchestrator", "Starting dependency level %d with %d services", levelIndex, len(levelConfigs))

		_, errs := o.serviceMgr.StartServices(levelConfigs, o.activeWaitGroup)
		if len(errs) > 0 {
			return fmt.Errorf("errors starting level %d: %v", levelIndex, errs)
		}

		// Wait for services in this level to become running before starting next level
		if levelIndex < len(levels)-1 {
			o.waitForServicesToBeRunning(levelConfigs, 30*time.Second)
		}
	}

	return nil
}

// groupServicesByDependencyLevel groups services into levels based on their dependencies
func (o *Orchestrator) groupServicesByDependencyLevel(configs []managers.ManagedServiceConfig) [][]managers.ManagedServiceConfig {
	// Build a map of configs by node ID
	configsByNodeID := make(map[string]managers.ManagedServiceConfig)
	for _, cfg := range configs {
		nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)
		configsByNodeID[nodeID] = cfg
	}

	// Calculate dependency depth for each service
	depths := make(map[string]int)
	visited := make(map[string]bool)

	var calculateDepth func(nodeID string) int
	calculateDepth = func(nodeID string) int {
		if depth, exists := depths[nodeID]; exists {
			return depth
		}

		if visited[nodeID] {
			return 0 // Circular dependency
		}
		visited[nodeID] = true

		maxDepth := -1
		node := o.depGraph.Get(dependency.NodeID(nodeID))
		if node != nil {
			for _, dep := range node.DependsOn {
				depStr := string(dep)
				// Only consider dependencies that we're actually starting
				if _, exists := configsByNodeID[depStr]; exists {
					depDepth := calculateDepth(depStr)
					if depDepth > maxDepth {
						maxDepth = depDepth
					}
				}
			}
		}

		depth := maxDepth + 1
		depths[nodeID] = depth
		return depth
	}

	// Calculate depths for all services
	maxLevel := 0
	for nodeID := range configsByNodeID {
		depth := calculateDepth(nodeID)
		if depth > maxLevel {
			maxLevel = depth
		}
	}

	// Group services by level
	levels := make([][]managers.ManagedServiceConfig, maxLevel+1)
	for nodeID, cfg := range configsByNodeID {
		level := depths[nodeID]
		levels[level] = append(levels[level], cfg)
	}

	return levels
}

// waitForServicesToBeRunning waits for services to reach running state
func (o *Orchestrator) waitForServicesToBeRunning(configs []managers.ManagedServiceConfig, timeout time.Duration) {
	// For now, use a simple time-based wait
	// In a more sophisticated implementation, we could monitor service states
	time.Sleep(500 * time.Millisecond)
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
			o.stopServiceWithDependents(k8sNodeID)
		} else {
			// Connection became healthy - restart dependent services
			logging.Info("Orchestrator", "K8s connection %s is healthy again, restarting dependent services", contextName)
			o.startServicesDependingOn(k8sNodeID)
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
// This handles cascade stops and dependency tracking
func (o *Orchestrator) StopService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Mark as manual stop
	o.mu.Lock()
	o.stopReasons[label] = StopReasonManual
	o.mu.Unlock()

	// Use cascading stop to properly handle dependencies
	return o.stopServiceWithDependents(label)
}

// RestartService restarts a specific service through the orchestrator
func (o *Orchestrator) RestartService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	o.mu.Lock()
	_, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}
	
	// Mark for restart
	o.pendingRestarts[label] = true
	o.mu.Unlock()

	logging.Info("Orchestrator", "User requested restart for service: %s", label)

	// Check if service is active
	if !o.serviceMgr.IsServiceActive(label) {
		// Service not active, start it directly
		o.mu.Lock()
		delete(o.pendingRestarts, label) // Clear pending restart
		cfg := o.serviceConfigs[label]
		o.mu.Unlock()
		
		configs := []managers.ManagedServiceConfig{cfg}
		return o.startServicesInDependencyOrder(configs)
	}

	// Stop the service - restart will be triggered by state update handler
	return o.serviceMgr.StopService(label)
}

// stopServiceWithDependents stops a service and all services that depend on it
func (o *Orchestrator) stopServiceWithDependents(label string) error {
	logging.Debug("Orchestrator", "stopServiceWithDependents called for: %s", label)
	
	if o.depGraph == nil {
		// No dependency graph, just stop the service
		return o.serviceMgr.StopService(label)
	}

	// Get node ID for the service
	o.mu.RLock()
	cfg, exists := o.serviceConfigs[label]
	o.mu.RUnlock()
	
	if !exists && !strings.HasPrefix(label, "k8s:") {
		return fmt.Errorf("service %s not found", label)
	}

	nodeID := label
	if exists {
		nodeID = o.getNodeIDForService(cfg.Label, cfg.Type)
	}

	logging.Debug("Orchestrator", "NodeID for %s is %s", label, nodeID)

	// Find all dependent services
	dependents := o.findAllDependents(nodeID)

	logging.Debug("Orchestrator", "Found %d dependents to stop", len(dependents))

	// Stop dependent services first
	for _, depNodeID := range dependents {
		if strings.HasPrefix(depNodeID, "k8s:") {
			continue // Skip k8s nodes
		}

		depLabel := o.getLabelFromNodeID(depNodeID)
		
		if o.serviceMgr.IsServiceActive(depLabel) {
			// Mark as dependency stop
			o.mu.Lock()
			o.stopReasons[depLabel] = StopReasonDependency
			o.mu.Unlock()

			if err := o.serviceMgr.StopService(depLabel); err != nil {
				logging.Error("Orchestrator", err, "Failed to stop dependent service %s", depLabel)
			} else {
				logging.Info("Orchestrator", "Stopped service %s as part of cascade from %s", depLabel, label)
			}
		}
	}

	// Stop the initiating service itself (unless it's a k8s node)
	if !strings.HasPrefix(label, "k8s:") {
		if err := o.serviceMgr.StopService(label); err != nil {
			return err
		}
		logging.Info("Orchestrator", "Stopped initiating service %s", label)
	}

	return nil
}

// startServicesDependingOn starts all services that depend on the given node
func (o *Orchestrator) startServicesDependingOn(nodeID string) error {
	if o.depGraph == nil {
		return nil
	}

	// Find all services that depend on this node
	dependents := o.findAllDependents(nodeID)
	
	var configsToStart []managers.ManagedServiceConfig

	o.mu.RLock()
	for _, depNodeID := range dependents {
		if strings.HasPrefix(depNodeID, "k8s:") {
			continue // Skip k8s nodes
		}

		label := o.getLabelFromNodeID(depNodeID)
		
		// Skip if already active
		if o.serviceMgr.IsServiceActive(label) {
			continue
		}

		// Skip if manually stopped
		if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
			logging.Info("Orchestrator", "Service %s was manually stopped, not restarting", label)
			continue
		}

		// Get config
		if cfg, exists := o.serviceConfigs[label]; exists {
			configsToStart = append(configsToStart, cfg)
		}
	}
	o.mu.RUnlock()

	if len(configsToStart) == 0 {
		return nil
	}

	logging.Info("Orchestrator", "Starting %d services that depend on %s", len(configsToStart), nodeID)
	return o.startServicesInDependencyOrder(configsToStart)
}

// findAllDependents finds all services that depend on the given node (direct and transitive)
func (o *Orchestrator) findAllDependents(nodeID string) []string {
	allDependents := make(map[string]bool)
	visited := make(map[string]bool)

	var findDependents func(currentID string)
	findDependents = func(currentID string) {
		if visited[currentID] {
			return
		}
		visited[currentID] = true

		// Get direct dependents
		directDependents := o.depGraph.Dependents(dependency.NodeID(currentID))
		logging.Debug("Orchestrator", "Direct dependents of %s: %v", currentID, directDependents)
		
		for _, dep := range directDependents {
			depStr := string(dep)
			if !strings.HasPrefix(depStr, "k8s:") {
				allDependents[depStr] = true
			}
			// Recursively find dependents
			findDependents(depStr)
		}
	}

	findDependents(nodeID)

	// Convert to slice
	result := make([]string, 0, len(allDependents))
	for dep := range allDependents {
		result = append(result, dep)
	}
	
	logging.Debug("Orchestrator", "All dependents of %s: %v", nodeID, result)
	return result
}

// getNodeIDForService converts a service label and type to a node ID
func (o *Orchestrator) getNodeIDForService(label string, serviceType reporting.ServiceType) string {
	switch serviceType {
	case reporting.ServiceTypePortForward:
		return "pf:" + label
	case reporting.ServiceTypeMCPServer:
		return "mcp:" + label
	default:
		return label
	}
}

// getLabelFromNodeID extracts the service label from a node ID
func (o *Orchestrator) getLabelFromNodeID(nodeID string) string {
	parts := strings.SplitN(nodeID, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return nodeID
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

	// Clear all state
	o.mu.Lock()
	o.stopReasons = make(map[string]StopReason)
	o.pendingRestarts = make(map[string]bool)
	o.serviceConfigs = make(map[string]managers.ManagedServiceConfig)
	o.mu.Unlock()

	// Update configuration
	o.mcName = mcName
	o.wcName = wcName
	o.portForwards = portForwards
	o.mcpServers = mcpServers

	// Rebuild dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Reinitialize service configs
	o.initializeServiceConfigs()

	// Start services with new configuration
	return o.startServicesWithHealthCheck()
}
