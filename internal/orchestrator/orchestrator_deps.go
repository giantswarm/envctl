package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"sync"
	"time"
)

// buildDependencyGraph builds the dependency graph for all services.
// This graph is crucial for:
// 1. Starting services in the correct order (dependencies first)
// 2. Stopping dependent services when a dependency fails
// 3. Auto-restarting services when their dependencies recover
//
// The graph structure reflects the service hierarchy:
// - K8s connections have no dependencies (foundation layer)
// - Port forwards depend on K8s connections
// - MCP servers may depend on port forwards
func (o *Orchestrator) buildDependencyGraph() *dependency.Graph {
	graph := dependency.New()

	// Add K8s cluster connection nodes from cluster state
	for _, cluster := range o.clusters {
		graph.AddNode(dependency.Node{
			ID:           dependency.NodeID(cluster.Name),
			FriendlyName: cluster.DisplayName,
			Kind:         dependency.KindK8sConnection,
			DependsOn:    []dependency.NodeID{}, // No dependencies
		})
	}

	// Add port forward nodes and their K8s connection dependencies
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}

		nodeID := dependency.NodeID("pf:" + pf.Name)
		var deps []dependency.NodeID

		// Determine which cluster this port forward depends on
		clusterName, err := o.resolveClusterForPortForward(pf)
		if err != nil {
			logging.Warn("Orchestrator", "Could not resolve cluster for port forward %s: %v", pf.Name, err)
		} else if clusterName != "" {
			deps = append(deps, dependency.NodeID(clusterName))
		}

		graph.AddNode(dependency.Node{
			ID:           nodeID,
			FriendlyName: pf.Name,
			Kind:         dependency.KindPortForward,
			DependsOn:    deps,
		})
	}

	// Add MCP server nodes and their dependencies
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}

		nodeID := dependency.NodeID("mcp:" + mcp.Name)
		var deps []dependency.NodeID

		// Handle direct cluster dependencies
		clusterName, err := o.resolveClusterForMCPServer(mcp)
		if err != nil {
			logging.Warn("Orchestrator", "Could not resolve cluster for MCP server %s: %v", mcp.Name, err)
		} else if clusterName != "" {
			deps = append(deps, dependency.NodeID(clusterName))
			logging.Debug("Orchestrator", "MCP server %s depends on cluster: %s", mcp.Name, clusterName)
		}

		// MCP servers depend on their configured port forwards
		for _, pfName := range mcp.RequiresPortForwards {
			depNodeID := dependency.NodeID("pf:" + pfName)
			// Verify the port forward exists and is enabled
			for _, pf := range o.portForwards {
				if pf.Name == pfName && pf.Enabled {
					deps = append(deps, depNodeID)
					logging.Debug("Orchestrator", "MCP server %s depends on port forward %s", mcp.Name, pfName)
					break
				}
			}
		}

		logging.Debug("Orchestrator", "Adding MCP server node: %s with dependencies: %v", nodeID, deps)

		graph.AddNode(dependency.Node{
			ID:           nodeID,
			FriendlyName: mcp.Name,
			Kind:         dependency.KindMCP,
			DependsOn:    deps,
		})
	}

	// Only add aggregator if we have MCP servers
	graph.AddNode(dependency.Node{
		ID:           dependency.NodeID("agg:mcp-aggregator"),
		FriendlyName: "MCP Aggregator",
		Kind:         dependency.KindK8sConnection,
		DependsOn:    []dependency.NodeID{},
	})

	return graph
}

// resolveClusterForPortForward determines which cluster a port forward should connect to
func (o *Orchestrator) resolveClusterForPortForward(pf config.PortForwardDefinition) (string, error) {
	// Priority: ClusterName > ClusterRole > KubeContextTarget (deprecated)

	if pf.ClusterName != "" {
		// Specific cluster requested
		if cluster, exists := o.clusterState.GetClusterByName(pf.ClusterName); exists {
			return cluster.Name, nil
		}
		return "", fmt.Errorf("cluster %s not found", pf.ClusterName)
	}

	if pf.ClusterRole != "" {
		// Use active cluster for role
		if clusterName, exists := o.clusterState.GetActiveCluster(pf.ClusterRole); exists {
			return clusterName, nil
		}
		return "", fmt.Errorf("no active cluster for role %s", pf.ClusterRole)
	}

	// Fallback to deprecated KubeContextTarget
	if pf.KubeContextTarget != "" {
		// Try to find a cluster with matching context
		for _, cluster := range o.clusters {
			if cluster.Context == pf.KubeContextTarget {
				return cluster.Name, nil
			}
		}
	}

	return "", fmt.Errorf("no cluster specified for port forward %s", pf.Name)
}

// resolveClusterForMCPServer determines which cluster an MCP server should connect to
func (o *Orchestrator) resolveClusterForMCPServer(mcp config.MCPServerDefinition) (string, error) {
	// Priority: RequiresClusterName > RequiresClusterRole

	if mcp.RequiresClusterName != "" {
		// Specific cluster requested
		if cluster, exists := o.clusterState.GetClusterByName(mcp.RequiresClusterName); exists {
			return cluster.Name, nil
		}
		return "", fmt.Errorf("cluster %s not found", mcp.RequiresClusterName)
	}

	if mcp.RequiresClusterRole != "" {
		// Use active cluster for role
		if clusterName, exists := o.clusterState.GetActiveCluster(mcp.RequiresClusterRole); exists {
			return clusterName, nil
		}
		return "", fmt.Errorf("no active cluster for role %s", mcp.RequiresClusterRole)
	}

	// No direct cluster dependency
	return "", nil
}

// startServicesInOrder starts all services in dependency order with parallel execution.
// This ensures that dependencies are running before dependent services start,
// while allowing independent services to start simultaneously for better performance.
// The method respects manual stop decisions - services that were manually
// stopped by the user will not be started automatically.
//
// Parallel startup groups:
// 1. K8s connections (parallel) - foundation services
// 2. Aggregator (independent) - no dependencies, registers MCP servers as they become healthy
// 3. Port forwards (parallel after dependencies) - depend on specific K8s connections
// 4. MCP servers (parallel after dependencies) - may depend on port forwards
func (o *Orchestrator) startServicesInOrder() error {
	// Get all services that should be started
	servicesToStart := o.getServicesToStart()

	// Group 1: Start K8s connections in parallel
	if err := o.startK8sConnectionsInParallel(servicesToStart); err != nil {
		return fmt.Errorf("failed to start K8s connections: %w", err)
	}

	// Group 2: Start aggregator (no dependencies)
	if err := o.startAggregator(servicesToStart); err != nil {
		return fmt.Errorf("failed to start aggregator: %w", err)
	}

	// Group 3: Start port forwards in parallel (wait for their dependencies)
	if err := o.startPortForwardsInParallel(servicesToStart); err != nil {
		return fmt.Errorf("failed to start port forwards: %w", err)
	}

	// Group 4: Start MCP servers in parallel (wait for their dependencies)
	if err := o.startMCPServersInParallel(servicesToStart); err != nil {
		return fmt.Errorf("failed to start MCP servers: %w", err)
	}

	return nil
}

// getServicesToStart extracts the service filtering logic.
// Returns a list of service labels that should be started, filtering out
// manually stopped services to respect user intent.
func (o *Orchestrator) getServicesToStart() []string {
	var servicesToStart []string
	allServices := o.registry.GetAll()

	for _, service := range allServices {
		label := service.GetLabel()

		// Skip manually stopped services to respect user intent
		o.mu.RLock()
		if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
			o.mu.RUnlock()
			continue
		}
		o.mu.RUnlock()

		servicesToStart = append(servicesToStart, label)
	}

	return servicesToStart
}

// startK8sConnectionsInParallel starts all K8s connections simultaneously.
// K8s connections have no dependencies and form the foundation for other services,
// so they can all start in parallel for better performance.
func (o *Orchestrator) startK8sConnectionsInParallel(servicesToStart []string) error {
	var k8sWg sync.WaitGroup
	var k8sServices []string

	// Identify K8s connection services
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypeKubeConnection {
			k8sServices = append(k8sServices, label)
		}
	}

	logging.Debug("Orchestrator", "Starting %d K8s connections in parallel: %v", len(k8sServices), k8sServices)

	// Start all K8s connections in parallel
	for _, label := range k8sServices {
		k8sWg.Add(1)
		go func(svcLabel string) {
			defer k8sWg.Done()
			if err := o.StartService(svcLabel); err != nil {
				logging.Error("Orchestrator", err, "Failed to start K8s connection %s", svcLabel)
			}
		}(label)
	}

	// Wait for all K8s connections to complete starting (or fail)
	k8sWg.Wait()

	return nil
}

// startPortForwardsInParallel starts port forwards in parallel after their dependencies are ready.
// Each port forward waits for its specific K8s connection dependency before starting,
// allowing independent port forwards to start simultaneously.
func (o *Orchestrator) startPortForwardsInParallel(servicesToStart []string) error {
	var pfWg sync.WaitGroup
	var pfServices []string
	skippedServices := make([]string, 0)
	var mu sync.Mutex

	// Identify port forward services
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypePortForward {
			pfServices = append(pfServices, label)
		}
	}

	logging.Debug("Orchestrator", "Starting %d port forwards in parallel: %v", len(pfServices), pfServices)

	// Start all port forwards in parallel, each waiting for its dependencies
	for _, label := range pfServices {
		pfWg.Add(1)
		go func(svcLabel string) {
			defer pfWg.Done()

			// Check dependencies first with a shorter timeout for failed deps
			if err := o.waitForDependencies(svcLabel, 5*time.Second); err != nil {
				logging.Warn("Orchestrator", "Skipping port forward %s: %v", svcLabel, err)

				// Update the service state to Waiting
				if service, exists := o.registry.Get(svcLabel); exists {
					if updater, ok := service.(services.StateUpdater); ok {
						updater.UpdateState(services.StateWaiting, services.HealthUnknown, fmt.Errorf("waiting for dependencies: %w", err))
					}
				}

				// Mark as stopped due to dependency so it can be auto-started later
				o.mu.Lock()
				o.stopReasons[svcLabel] = StopReasonDependency
				o.mu.Unlock()

				mu.Lock()
				skippedServices = append(skippedServices, svcLabel)
				mu.Unlock()
				return
			}

			if err := o.StartService(svcLabel); err != nil {
				logging.Error("Orchestrator", err, "Failed to start port forward %s", svcLabel)
			}
		}(label)
	}

	// Wait for all port forwards to complete starting (or be skipped)
	pfWg.Wait()

	if len(skippedServices) > 0 {
		logging.Info("Orchestrator", "Skipped %d port forwards due to missing dependencies: %v",
			len(skippedServices), skippedServices)
	}

	return nil
}

// startMCPServersInParallel starts MCP servers in parallel after their dependencies are ready.
// This preserves the existing parallel MCP server startup logic while adding dependency waiting.
func (o *Orchestrator) startMCPServersInParallel(servicesToStart []string) error {
	var mcpWg sync.WaitGroup
	var mcpServices []string
	skippedServices := make([]string, 0)
	var mu sync.Mutex

	// Identify MCP server services
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypeMCPServer {
			mcpServices = append(mcpServices, label)
		}
	}

	logging.Debug("Orchestrator", "Starting %d MCP servers in parallel: %v", len(mcpServices), mcpServices)

	// Start all MCP servers in parallel, each waiting for its dependencies
	for _, label := range mcpServices {
		mcpWg.Add(1)
		go func(svcLabel string) {
			defer mcpWg.Done()

			// Check dependencies first with a shorter timeout for failed deps
			if err := o.waitForDependencies(svcLabel, 5*time.Second); err != nil {
				logging.Warn("Orchestrator", "Skipping MCP server %s: %v", svcLabel, err)

				// Update the service state to Waiting
				if service, exists := o.registry.Get(svcLabel); exists {
					if updater, ok := service.(services.StateUpdater); ok {
						updater.UpdateState(services.StateWaiting, services.HealthUnknown, fmt.Errorf("waiting for dependencies: %w", err))
					}
				}

				// Mark as stopped due to dependency so it can be auto-started later
				o.mu.Lock()
				o.stopReasons[svcLabel] = StopReasonDependency
				o.mu.Unlock()

				mu.Lock()
				skippedServices = append(skippedServices, svcLabel)
				mu.Unlock()
				return
			}

			if err := o.StartService(svcLabel); err != nil {
				logging.Error("Orchestrator", err, "Failed to start MCP server %s", svcLabel)
			}
		}(label)
	}

	// Wait for all MCP servers to finish starting (or be skipped)
	mcpWg.Wait()

	if len(skippedServices) > 0 {
		logging.Info("Orchestrator", "Skipped %d MCP servers due to missing dependencies: %v",
			len(skippedServices), skippedServices)
	}

	return nil
}

// startAggregator starts the aggregator service independently.
// The aggregator has no dependencies and will dynamically register/deregister
// MCP servers as they become healthy or unhealthy.
func (o *Orchestrator) startAggregator(servicesToStart []string) error {
	// Start the aggregator service
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if string(service.GetType()) == "Aggregator" {
			logging.Debug("Orchestrator", "Starting aggregator: %s", label)
			if err := o.StartService(label); err != nil {
				logging.Error("Orchestrator", err, "Failed to start aggregator %s", label)
				return fmt.Errorf("failed to start aggregator %s: %w", label, err)
			}
		}
	}

	return nil
}

// waitForDependencies waits for all dependencies of a service to be running.
// This is used by parallel startup groups to ensure dependencies are satisfied
// before starting a service, with a configurable timeout for robustness.
func (o *Orchestrator) waitForDependencies(label string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		allReady, missingDep, err := o.checkDependencyStatus(label)
		if err != nil {
			return err
		}
		if allReady {
			logging.Debug("Orchestrator", "All dependencies ready for %s", label)
			return nil
		}

		// Check if dependency is in a failed state - fail fast
		if missingDep != "" {
			if depService, exists := o.registry.Get(missingDep); exists {
				if depService.GetState() == services.StateFailed {
					return fmt.Errorf("dependency %s is in failed state", missingDep)
				}
			}
		}

		select {
		case <-ticker.C:
			// Continue checking
		case <-o.ctx.Done():
			return fmt.Errorf("context cancelled while waiting for dependencies")
		}
	}

	return fmt.Errorf("timeout waiting for dependencies of %s", label)
}

// checkDependencyStatus checks the status of all dependencies for a service.
// Returns (allReady, missingDependency, error).
// This shared logic is used by both checkDependencies and waitForDependencies.
func (o *Orchestrator) checkDependencyStatus(label string) (bool, string, error) {
	nodeID := o.getNodeIDForService(label)
	node := o.depGraph.Get(dependency.NodeID(nodeID))
	if node == nil {
		return true, "", nil // No dependencies
	}

	// Check each dependency
	for _, depNodeID := range node.DependsOn {
		depLabel := o.getLabelFromNodeID(string(depNodeID))
		depService, exists := o.registry.Get(depLabel)

		if !exists {
			return false, depLabel, fmt.Errorf("dependency %s not found", depLabel)
		}

		if depService.GetState() != services.StateRunning {
			return false, depLabel, nil
		}
	}

	return true, "", nil
}

// checkDependencies checks if all dependencies of a service are running.
// This is called before starting a service to ensure its prerequisites are met.
// If any dependency is not running, an error is returned with details about
// which dependency is missing.
func (o *Orchestrator) checkDependencies(label string) error {
	allReady, missingDep, err := o.checkDependencyStatus(label)
	if err != nil {
		return err
	}
	if !allReady {
		// Get the service state for a more detailed error message
		if depService, exists := o.registry.Get(missingDep); exists {
			return fmt.Errorf("dependency %s is not running (state: %s)", missingDep, depService.GetState())
		}
		return fmt.Errorf("dependency %s is not running", missingDep)
	}
	return nil
}

// stopDependentServices stops all services that depend on the given service.
// This is called when a service is stopped (either manually or due to failure)
// to maintain system consistency. Services stopped this way are marked with
// StopReasonDependency so they can be auto-restarted when the dependency recovers.
func (o *Orchestrator) stopDependentServices(label string) error {
	nodeID := o.getNodeIDForService(label)
	dependents := o.depGraph.Dependents(dependency.NodeID(nodeID))

	var errors []error
	for _, depNodeID := range dependents {
		depLabel := o.getLabelFromNodeID(string(depNodeID))

		// Mark as stopped due to dependency (enables auto-restart)
		o.mu.Lock()
		o.stopReasons[depLabel] = StopReasonDependency
		o.mu.Unlock()

		// Stop the dependent service
		if depService, exists := o.registry.Get(depLabel); exists {
			if err := depService.Stop(o.ctx); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop dependent %s: %w", depLabel, err))
			} else {
				logging.Info("Orchestrator", "Stopped dependent service: %s", depLabel)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping dependent services: %v", errors)
	}

	return nil
}

// stopAllServices stops all services in reverse dependency order.
// This ensures dependent services are stopped before their dependencies,
// preventing errors during shutdown. The method uses timeouts to ensure
// shutdown completes even if some services hang.
//
// Stop order with timeouts:
// 1. MCP servers (3 seconds) - may need time to clean up
// 2. Aggregator (2 seconds) - independent but should stop after MCP servers
// 3. Port forwards (2 seconds) - kubectl processes to terminate
// 4. K8s connections (1 second) - should stop quickly
func (o *Orchestrator) stopAllServices() error {
	allServices := o.registry.GetAll()

	// Stop MCP servers first with 3-second timeout
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer mcpCancel()

	var wg sync.WaitGroup

	for _, service := range allServices {
		if service.GetType() == services.TypeMCPServer {
			wg.Add(1)
			go func(svc services.Service) {
				defer wg.Done()
				if err := svc.Stop(mcpCtx); err != nil && err != context.DeadlineExceeded {
					logging.Error("Orchestrator", err, "Failed to stop MCP server %s", svc.GetLabel())
				} else {
					logging.Debug("Orchestrator", "Stopped MCP server %s", svc.GetLabel())
				}
			}(service)
		}
	}
	wg.Wait()

	// Stop aggregator with 2-second timeout
	aggCtx, aggCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer aggCancel()

	for _, service := range allServices {
		if string(service.GetType()) == "Aggregator" {
			wg.Add(1)
			go func(svc services.Service) {
				defer wg.Done()
				if err := svc.Stop(aggCtx); err != nil && err != context.DeadlineExceeded {
					logging.Error("Orchestrator", err, "Failed to stop aggregator %s", svc.GetLabel())
				} else {
					logging.Debug("Orchestrator", "Stopped aggregator %s", svc.GetLabel())
				}
			}(service)
		}
	}
	wg.Wait()

	// Stop port forwards with 2-second timeout
	pfCtx, pfCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pfCancel()

	for _, service := range allServices {
		if service.GetType() == services.TypePortForward {
			wg.Add(1)
			go func(svc services.Service) {
				defer wg.Done()
				if err := svc.Stop(pfCtx); err != nil && err != context.DeadlineExceeded {
					logging.Error("Orchestrator", err, "Failed to stop port forward %s", svc.GetLabel())
				} else {
					logging.Debug("Orchestrator", "Stopped port forward %s", svc.GetLabel())
				}
			}(service)
		}
	}
	wg.Wait()

	// Stop K8s connections last with 1-second timeout
	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer k8sCancel()

	for _, service := range allServices {
		if service.GetType() == services.TypeKubeConnection {
			wg.Add(1)
			go func(svc services.Service) {
				defer wg.Done()
				if err := svc.Stop(k8sCtx); err != nil && err != context.DeadlineExceeded {
					logging.Error("Orchestrator", err, "Failed to stop K8s connection %s", svc.GetLabel())
				} else {
					logging.Debug("Orchestrator", "Stopped K8s connection %s", svc.GetLabel())
				}
			}(service)
		}
	}
	wg.Wait()

	logging.Info("Orchestrator", "All services stopped")
	return nil
}

// getNodeIDForService converts a service label to a dependency graph node ID.
// The node ID format depends on the service type:
// - Port forwards: "pf:{label}"
// - MCP servers: "mcp:{label}"
// - K8s connections: use label directly
//
// This mapping allows us to correlate services with their dependency graph nodes.
func (o *Orchestrator) getNodeIDForService(label string) string {
	// Get service to determine type
	service, exists := o.registry.Get(label)
	if !exists {
		return label
	}

	switch service.GetType() {
	case services.TypePortForward:
		return "pf:" + label
	case services.TypeMCPServer:
		return "mcp:" + label
	case services.TypeKubeConnection:
		// K8s services use their label directly
		return label
	default:
		return label
	}
}

// getLabelFromNodeID extracts the service label from a node ID.
// This reverses the mapping done by getNodeIDForService.
func (o *Orchestrator) getLabelFromNodeID(nodeID string) string {
	if strings.HasPrefix(nodeID, "pf:") {
		return strings.TrimPrefix(nodeID, "pf:")
	} else if strings.HasPrefix(nodeID, "mcp:") {
		return strings.TrimPrefix(nodeID, "mcp:")
	}
	return nodeID
}

// monitorServices is the main monitoring loop that handles:
// 1. Periodic health checks for all services
// 2. Auto-restart of failed services (if not manually stopped)
// 3. Detection of state changes to trigger dependent service restarts
//
// This method runs in a separate goroutine for the lifetime of the orchestrator.
func (o *Orchestrator) monitorServices() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track previous states to detect state changes
	previousStates := make(map[string]services.ServiceState)

	// Start health check goroutines for services that support it
	o.startHealthCheckers()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			// Check for failed services that should be restarted
			o.checkAndRestartFailedServices()
			// Check for state changes that might trigger dependent restarts
			o.checkForStateChanges(previousStates)
		}
	}
}

// checkForStateChanges monitors for service state changes and handles dependent service restarts.
// When a service transitions from non-running to running state, this method:
// 1. Starts a health checker for the newly running service (if applicable)
// 2. Triggers restart of dependent services that were stopped due to dependency failure
//
// This enables the auto-recovery cascade: when a failed dependency recovers,
// all services that depended on it are automatically restarted.
func (o *Orchestrator) checkForStateChanges(previousStates map[string]services.ServiceState) {
	allServices := o.registry.GetAll()

	for _, service := range allServices {
		label := service.GetLabel()
		currentState := service.GetState()
		previousState, hadPrevious := previousStates[label]

		// Update the state tracking
		previousStates[label] = currentState

		// Check if service just became running
		if hadPrevious && previousState != services.StateRunning && currentState == services.StateRunning {
			logging.Info("Orchestrator", "Service %s became running, checking for dependent services to restart", label)

			// Start health checker for this service if needed
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

			// Start dependent services that were stopped due to dependency failure
			// This is done in a goroutine to avoid blocking the monitor loop
			go o.startDependentServices(label)
		}
	}
}

// startDependentServices starts services that depend on the given service and were stopped due to dependency failure.
// This is the auto-recovery mechanism: when a dependency recovers, services that were
// automatically stopped (not manually) are restarted. This maintains the service
// dependency invariant while respecting user intent for manual stops.
func (o *Orchestrator) startDependentServices(label string) {
	nodeID := o.getNodeIDForService(label)
	logging.Debug("Orchestrator", "Checking for dependent services of %s (nodeID: %s)", label, nodeID)

	// Find all services that depend on this one
	allServices := o.registry.GetAll()
	var servicesToStart []string

	for _, service := range allServices {
		depLabel := service.GetLabel()

		// Skip if service is already running
		if service.GetState() == services.StateRunning {
			continue
		}

		// Check if this service was stopped due to dependency failure
		o.mu.RLock()
		reason, hasReason := o.stopReasons[depLabel]
		o.mu.RUnlock()

		logging.Debug("Orchestrator", "Service %s: state=%s, hasStopReason=%v, stopReason=%v",
			depLabel, service.GetState(), hasReason, reason)

		if hasReason && reason == StopReasonDependency {
			// Check if this service depends on the recovered service
			depNodeID := o.getNodeIDForService(depLabel)
			node := o.depGraph.Get(dependency.NodeID(depNodeID))

			if node != nil {
				logging.Debug("Orchestrator", "Service %s (nodeID: %s) has dependencies: %v",
					depLabel, depNodeID, node.DependsOn)

				for _, dep := range node.DependsOn {
					if string(dep) == nodeID {
						logging.Debug("Orchestrator", "Service %s depends on %s, adding to restart list",
							depLabel, label)
						servicesToStart = append(servicesToStart, depLabel)
						break
					}
				}
			}
		}
	}

	logging.Debug("Orchestrator", "Found %d services to restart after %s became running: %v",
		len(servicesToStart), label, servicesToStart)

	// Start the dependent services
	for _, depLabel := range servicesToStart {
		logging.Info("Orchestrator", "Restarting dependent service %s after %s became running", depLabel, label)

		// Clear the stop reason
		o.mu.Lock()
		delete(o.stopReasons, depLabel)
		o.mu.Unlock()

		// Start the service
		if err := o.StartService(depLabel); err != nil {
			logging.Error("Orchestrator", err, "Failed to restart dependent service %s", depLabel)
		}
	}
}
