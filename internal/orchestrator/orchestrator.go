package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/internal/state"
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
	serviceMgr  managers.ServiceManagerAPI
	k8sStateMgr state.K8sStateManager
	kubeMgr     kube.Manager
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
	stopReasons     map[string]StopReason                    // Track why services were stopped
	pendingRestarts map[string]bool                          // Track services pending restart
	serviceConfigs  map[string]managers.ManagedServiceConfig // Store all service configs
	activeWaitGroup *sync.WaitGroup                          // Track active services

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
	serviceMgr managers.ServiceManagerAPI,
	reporter reporting.ServiceReporter,
	cfg Config,
) *Orchestrator {
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 15 * time.Second
	}

	// Create kube manager
	kubeMgr := kube.NewManager(reporter)

	return &Orchestrator{
		serviceMgr:          serviceMgr,
		k8sStateMgr:         kubeMgr.GetK8sStateManager(),
		kubeMgr:             kubeMgr,
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

	// Create K8s connection services
	k8sServices := o.createK8sConnectionServices()

	// Add K8s services to the service configs map
	o.mu.Lock()
	for _, svc := range k8sServices {
		o.serviceConfigs[svc.Label] = svc
	}
	o.mu.Unlock()

	// Start service health monitoring
	healthCtx, cancel := context.WithCancel(ctx)
	o.cancelHealthChecks = cancel
	go o.StartServiceHealthMonitoring(healthCtx)

	// Get all enabled services
	var allServices []managers.ManagedServiceConfig
	o.mu.RLock()
	for _, cfg := range o.serviceConfigs {
		// Skip manually stopped services
		if reason, exists := o.stopReasons[cfg.Label]; exists && reason == StopReasonManual {
			continue
		}
		allServices = append(allServices, cfg)
	}
	o.mu.RUnlock()

	// Start all services in dependency order
	if err := o.startServicesInDependencyOrder(allServices); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Start monitoring for services that need to be restarted
	go o.monitorAndStartServices(ctx)

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
		orchestrator:     o,
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

// GetStateStore forwards to the original reporter
func (s *serviceStateInterceptor) GetStateStore() reporting.StateStore {
	if s.originalReporter != nil {
		return s.originalReporter.GetStateStore()
	}
	return nil
}

// handleServiceStateUpdate processes service state changes and triggers restarts if needed
func (o *Orchestrator) handleServiceStateUpdate(update reporting.ManagedServiceUpdate) {
	o.mu.Lock()
	defer o.mu.Unlock()

	label := update.SourceLabel

	// Log the state update with correlation info
	logging.Debug("Orchestrator", "Received service state update: %s -> %s (correlationID: %s, causedBy: %s)",
		label, update.State, update.CorrelationID, update.CausedBy)

	// Check if this service was pending restart
	if wasPendingRestart, exists := o.pendingRestarts[label]; exists && wasPendingRestart {
		if update.State == reporting.StateStopped || update.State == reporting.StateFailed {
			// Service has stopped, now restart it
			delete(o.pendingRestarts, label)

			if _, configExists := o.serviceConfigs[label]; configExists {
				logging.Info("Orchestrator", "Restarting service %s after stop (correlationID: %s)", label, update.CorrelationID)

				// Start the service with correlation tracking
				go func() {
					// Use startServiceWithDependencies to also restart any dependencies
					if err := o.startServiceWithDependencies(label); err != nil {
						logging.Error("Orchestrator", err, "Failed to restart service %s", label)

						// Report restart failure with correlation
						if o.reporter != nil {
							failureUpdate := reporting.NewManagedServiceUpdate(
								update.SourceType,
								label,
								reporting.StateFailed,
							).WithCause("restart_failed", update.CorrelationID).WithError(err)

							o.reporter.Report(failureUpdate)
						}
					}
				}()
			}
		}
	}
}

// buildDependencyGraph constructs the dependency graph for services
func (o *Orchestrator) buildDependencyGraph() *dependency.Graph {
	g := dependency.New()

	// Add k8s connection nodes with service labels
	if o.mcName != "" {
		mcServiceLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID(mcServiceLabel),
			FriendlyName: "K8s MC Connection (" + o.mcName + ")",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil,
		})
	}

	if o.wcName != "" && o.mcName != "" {
		wcServiceLabel := fmt.Sprintf("k8s-wc-%s", o.wcName)
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID(wcServiceLabel),
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
			// Map context to service label
			if contextName == o.kubeMgr.BuildMcContextName(o.mcName) && o.mcName != "" {
				deps = append(deps, dependency.NodeID(fmt.Sprintf("k8s-mc-%s", o.mcName)))
			} else if contextName == o.kubeMgr.BuildWcContextName(o.mcName, o.wcName) && o.wcName != "" {
				deps = append(deps, dependency.NodeID(fmt.Sprintf("k8s-wc-%s", o.wcName)))
			}
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
			deps = append(deps, dependency.NodeID(fmt.Sprintf("k8s-mc-%s", o.mcName)))
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

// GetDependencyGraph returns the current dependency graph
func (o *Orchestrator) GetDependencyGraph() *dependency.Graph {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.depGraph
}

// GetK8sStateManager returns the k8s state manager
func (o *Orchestrator) GetK8sStateManager() state.K8sStateManager {
	return o.k8sStateMgr
}

// StopService stops a specific service through the orchestrator
// This handles cascade stops and dependency tracking
func (o *Orchestrator) StopService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Generate correlation ID for this user action
	correlationID := reporting.GenerateCorrelationID()
	logging.Info("Orchestrator", "User requested stop for service: %s (correlationID: %s)", label, correlationID)

	// Get the node ID for this service
	o.mu.Lock()
	cfg, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}

	nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)

	// Mark as manual stop
	o.stopReasons[label] = StopReasonManual
	o.mu.Unlock()

	// Use cascading stop to properly handle dependencies
	return o.stopServiceWithDependentsCorrelated(nodeID, "user_action", correlationID)
}

// RestartService restarts a specific service through the orchestrator
func (o *Orchestrator) RestartService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Generate correlation ID for this user action
	correlationID := reporting.GenerateCorrelationID()

	o.mu.Lock()
	_, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}

	// Clear manual stop reason if it exists
	delete(o.stopReasons, label)

	// Mark for restart
	o.pendingRestarts[label] = true
	o.mu.Unlock()

	logging.Info("Orchestrator", "User requested restart for service: %s (correlationID: %s)", label, correlationID)

	// Check if service is active
	if !o.serviceMgr.IsServiceActive(label) {
		// Service not active, start it directly with its dependencies
		o.mu.Lock()
		delete(o.pendingRestarts, label) // Clear pending restart
		o.mu.Unlock()

		// Start the service and any dependencies that were stopped due to cascade
		return o.startServiceWithDependencies(label)
	}

	// Stop the service - restart will be triggered by state update handler
	return o.serviceMgr.StopService(label)
}

// startServiceWithDependencies starts a service and any of its dependencies that were stopped due to cascade
func (o *Orchestrator) startServiceWithDependencies(label string) error {
	o.mu.RLock()
	cfg, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.RUnlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}

	// Get the node ID for this service
	nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)
	o.mu.RUnlock()

	logging.Debug("Orchestrator", "startServiceWithDependencies for %s (nodeID: %s)", label, nodeID)

	// Collect all services to start: the requested service plus its dependencies
	var configsToStart []managers.ManagedServiceConfig

	// Add the requested service
	configsToStart = append(configsToStart, cfg)

	// Find and add dependencies that should be restarted
	if o.depGraph != nil {
		node := o.depGraph.Get(dependency.NodeID(nodeID))
		if node != nil {
			logging.Debug("Orchestrator", "Service %s depends on: %v", label, node.DependsOn)
			// Check each dependency
			for _, depNodeID := range node.DependsOn {
				depLabel := o.getLabelFromNodeID(string(depNodeID))

				// Skip k8s nodes
				if strings.HasPrefix(string(depNodeID), "k8s-") {
					continue
				}

				o.mu.RLock()
				// Check if this dependency was stopped due to cascade (not manual)
				reason, hasReason := o.stopReasons[depLabel]
				depCfg, hasConfig := o.serviceConfigs[depLabel]
				isActive := o.serviceMgr.IsServiceActive(depLabel)
				o.mu.RUnlock()

				logging.Debug("Orchestrator", "Checking dependency %s: hasReason=%v, reason=%v, hasConfig=%v, isActive=%v",
					depLabel, hasReason, reason, hasConfig, isActive)

				if hasConfig && !isActive {
					// If the dependency is not active, we should start it regardless of stop reason
					// This ensures dependencies are satisfied
					configsToStart = append(configsToStart, depCfg)
					logging.Info("Orchestrator", "Including dependency %s for restart", depLabel)
					// Clear the stop reason since we're restarting it
					o.mu.Lock()
					delete(o.stopReasons, depLabel)
					o.mu.Unlock()
				}
			}
		}
	}

	logging.Debug("Orchestrator", "Starting %d services in dependency order", len(configsToStart))

	// Start all services in dependency order
	return o.startServicesInDependencyOrder(configsToStart)
}

// stopServiceWithDependentsCorrelated stops a service and all its dependents with correlation tracking
func (o *Orchestrator) stopServiceWithDependentsCorrelated(nodeID, causedBy, correlationID string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Find all dependents
	dependents := o.findAllDependents(nodeID)

	// Record cascade operation if there are dependents
	if len(dependents) > 0 && o.reporter != nil && o.reporter.GetStateStore() != nil {
		cascade := reporting.CascadeInfo{
			InitiatingService: nodeID,
			AffectedServices:  dependents,
			Reason:            causedBy,
			CorrelationID:     correlationID,
			Timestamp:         time.Now(),
			CascadeType:       reporting.CascadeTypeStop,
		}
		o.reporter.GetStateStore().RecordCascadeOperation(cascade)
	}

	// Stop dependents first (reverse dependency order)
	for i := len(dependents) - 1; i >= 0; i-- {
		dependentNodeID := dependents[i]
		if strings.HasPrefix(dependentNodeID, "k8s-") {
			// Skip K8s connections - they are managed separately
			continue
		}

		dependentLabel := o.getLabelFromNodeID(dependentNodeID)

		o.mu.Lock()
		o.stopReasons[dependentLabel] = StopReasonDependency
		o.mu.Unlock()

		logging.Info("Orchestrator", "Stopping dependent service %s due to %s (correlationID: %s)", dependentLabel, causedBy, correlationID)
		if err := o.serviceMgr.StopService(dependentLabel); err != nil {
			logging.Error("Orchestrator", err, "Failed to stop dependent service %s", dependentLabel)
		}
	}

	// Stop the main service if it's not a K8s connection
	if !strings.HasPrefix(nodeID, "k8s-") {
		mainLabel := o.getLabelFromNodeID(nodeID)
		logging.Info("Orchestrator", "Stopping service %s due to %s (correlationID: %s)", mainLabel, causedBy, correlationID)
		return o.serviceMgr.StopService(mainLabel)
	}

	return nil
}

// startServicesDependingOnCorrelated starts services that depend on the given node with correlation tracking
func (o *Orchestrator) startServicesDependingOnCorrelated(nodeID, causedBy, correlationID string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Find all services that were stopped due to dependency failure
	var servicesToRestart []managers.ManagedServiceConfig

	o.mu.RLock()
	for label, reason := range o.stopReasons {
		if reason == StopReasonDependency {
			if cfg, exists := o.serviceConfigs[label]; exists {
				// Check if this service depends on the recovered node
				serviceNodeID := o.getNodeIDForService(cfg.Label, cfg.Type)
				if o.depGraph != nil {
					dependencies := o.depGraph.Dependencies(dependency.NodeID(serviceNodeID))
					for _, dep := range dependencies {
						if string(dep) == nodeID {
							servicesToRestart = append(servicesToRestart, cfg)
							break
						}
					}
				}
			}
		}
	}
	o.mu.RUnlock()

	if len(servicesToRestart) == 0 {
		return nil
	}

	// Record cascade operation
	if o.reporter != nil && o.reporter.GetStateStore() != nil {
		var affectedServices []string
		for _, cfg := range servicesToRestart {
			affectedServices = append(affectedServices, cfg.Label)
		}

		cascade := reporting.CascadeInfo{
			InitiatingService: nodeID,
			AffectedServices:  affectedServices,
			Reason:            causedBy,
			CorrelationID:     correlationID,
			Timestamp:         time.Now(),
			CascadeType:       reporting.CascadeTypeRestart,
		}
		o.reporter.GetStateStore().RecordCascadeOperation(cascade)
	}

	// Clear stop reasons for services being restarted
	o.mu.Lock()
	for _, cfg := range servicesToRestart {
		delete(o.stopReasons, cfg.Label)
	}
	o.mu.Unlock()

	logging.Info("Orchestrator", "Restarting %d services that depend on %s (correlationID: %s)", len(servicesToRestart), nodeID, correlationID)

	// Start services in dependency order
	return o.startServicesInDependencyOrder(servicesToRestart)
}

// Backwards compatibility methods that use the new correlated versions
func (o *Orchestrator) stopServiceWithDependents(label string) error {
	return o.stopServiceWithDependentsCorrelated(label, "unknown", reporting.GenerateCorrelationID())
}

func (o *Orchestrator) startServicesDependingOn(nodeID string) error {
	return o.startServicesDependingOnCorrelated(nodeID, "unknown", reporting.GenerateCorrelationID())
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
			if !strings.HasPrefix(depStr, "k8s-") {
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

// getNodeIDForService converts a service label to a dependency graph node ID
func (o *Orchestrator) getNodeIDForService(label string, serviceType reporting.ServiceType) string {
	switch serviceType {
	case reporting.ServiceTypePortForward:
		return "pf:" + label
	case reporting.ServiceTypeMCPServer:
		return "mcp:" + label
	case reporting.ServiceTypeKube:
		// K8s services use their label directly as the node ID
		return label
	default:
		return label
	}
}

// getLabelFromNodeID extracts the service label from a node ID
func (o *Orchestrator) getLabelFromNodeID(nodeID string) string {
	if strings.HasPrefix(nodeID, "pf:") {
		return strings.TrimPrefix(nodeID, "pf:")
	} else if strings.HasPrefix(nodeID, "mcp:") {
		return strings.TrimPrefix(nodeID, "mcp:")
	} else if strings.HasPrefix(nodeID, "k8s-") {
		// K8s services use their label directly as the node ID
		return nodeID
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

	// Get all enabled services
	var allServices []managers.ManagedServiceConfig
	o.mu.RLock()
	for _, cfg := range o.serviceConfigs {
		allServices = append(allServices, cfg)
	}
	o.mu.RUnlock()

	// Start all services in dependency order
	return o.startServicesInDependencyOrder(allServices)
}

// createK8sConnectionServices creates K8s connection services for MC and WC
func (o *Orchestrator) createK8sConnectionServices() []managers.ManagedServiceConfig {
	var services []managers.ManagedServiceConfig

	// Create MC service if configured
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		mcConfig := managers.K8sConnectionConfig{
			Name:                fmt.Sprintf("k8s-mc-%s", o.mcName),
			ContextName:         mcContext,
			IsMC:                true,
			HealthCheckInterval: 15 * time.Second,
		}

		services = append(services, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeKube,
			Label:  mcConfig.Name,
			Config: mcConfig,
		})
	}

	// Create WC service if configured
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		wcConfig := managers.K8sConnectionConfig{
			Name:                fmt.Sprintf("k8s-wc-%s", o.wcName),
			ContextName:         wcContext,
			IsMC:                false,
			HealthCheckInterval: 15 * time.Second,
		}

		services = append(services, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeKube,
			Label:  wcConfig.Name,
			Config: wcConfig,
		})
	}

	return services
}

// monitorAndStartServices monitors for services that need to be restarted after failures
func (o *Orchestrator) monitorAndStartServices(ctx context.Context) {
	// This goroutine is now primarily for handling services that fail and need to be restarted
	// Initial startup is handled by Start() method directly

	// Check every 5 seconds for services that might need restarting
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if any services that should be running are not active
			var servicesToRestart []managers.ManagedServiceConfig

			o.mu.RLock()
			for label, cfg := range o.serviceConfigs {
				// Skip manually stopped services
				if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
					continue
				}

				// Skip already active services
				if o.serviceMgr.IsServiceActive(label) {
					continue
				}

				// Skip if pending restart (will be handled by state update handler)
				if o.pendingRestarts[label] {
					continue
				}

				// This service should be running but isn't - add to restart list
				servicesToRestart = append(servicesToRestart, cfg)
			}
			o.mu.RUnlock()

			// If we found services that need restarting, attempt to restart them
			if len(servicesToRestart) > 0 {
				logging.Info("Orchestrator", "Found %d services that need restarting", len(servicesToRestart))

				// Start services in dependency order
				if err := o.startServicesInDependencyOrder(servicesToRestart); err != nil {
					logging.Error("Orchestrator", err, "Failed to restart services")
				}
			}
		}
	}
}
