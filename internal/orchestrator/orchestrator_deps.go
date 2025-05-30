package orchestrator

import (
	"context"
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

	// Add K8s connection nodes - these are the foundation services
	if o.mcName != "" {
		mcLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)
		graph.AddNode(dependency.Node{
			ID:           dependency.NodeID(mcLabel),
			FriendlyName: fmt.Sprintf("K8s MC: %s", o.mcName),
			Kind:         dependency.KindK8sConnection,
			DependsOn:    []dependency.NodeID{}, // No dependencies
		})
	}

	if o.wcName != "" && o.mcName != "" {
		wcLabel := fmt.Sprintf("k8s-wc-%s", o.wcName)
		graph.AddNode(dependency.Node{
			ID:           dependency.NodeID(wcLabel),
			FriendlyName: fmt.Sprintf("K8s WC: %s", o.wcName),
			Kind:         dependency.KindK8sConnection,
			DependsOn:    []dependency.NodeID{},
		})
	}

	// Add port forward nodes and their K8s connection dependencies
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}

		nodeID := dependency.NodeID("pf:" + pf.Name)
		var deps []dependency.NodeID

		// Port forwards depend on their target K8s connection
		// We determine this by checking if the context name contains the cluster name
		if pf.KubeContextTarget != "" {
			var k8sNodeID string
			if strings.Contains(pf.KubeContextTarget, o.wcName) && o.wcName != "" {
				k8sNodeID = fmt.Sprintf("k8s-wc-%s", o.wcName)
			} else if strings.Contains(pf.KubeContextTarget, o.mcName) && o.mcName != "" {
				k8sNodeID = fmt.Sprintf("k8s-mc-%s", o.mcName)
			}

			if k8sNodeID != "" {
				deps = append(deps, dependency.NodeID(k8sNodeID))
			}
		}

		graph.AddNode(dependency.Node{
			ID:           nodeID,
			FriendlyName: pf.Name,
			Kind:         dependency.KindPortForward,
			DependsOn:    deps,
		})
	}

	// Add MCP server nodes and their port forward dependencies
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}

		nodeID := dependency.NodeID("mcp:" + mcp.Name)
		var deps []dependency.NodeID

		// MCP servers depend on their configured port forwards
		// This ensures required services are accessible before starting the MCP server
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

// startServicesInOrder starts all services in dependency order.
// This ensures that dependencies are running before dependent services start.
// The method respects manual stop decisions - services that were manually
// stopped by the user will not be started automatically.
//
// Current implementation uses a simple ordering approach:
// 1. K8s connections first (foundation)
// 2. Port forwards second (depend on K8s)
// 3. MCP servers last (may depend on port forwards)
//
// TODO: Implement proper topological sorting for more complex dependency graphs
func (o *Orchestrator) startServicesInOrder() error {
	// Get all services that should be started
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

	// Start K8s connections first as they are the foundation
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypeKubeConnection {
			if err := o.StartService(label); err != nil {
				logging.Error("Orchestrator", err, "Failed to start K8s connection %s", label)
			}
		}
	}

	// Start port forwards which depend on K8s connections
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypePortForward {
			if err := o.StartService(label); err != nil {
				logging.Error("Orchestrator", err, "Failed to start port forward %s", label)
			}
		}
	}

	// Start MCP servers which may depend on port forwards
	var mcpWg sync.WaitGroup
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypeMCPServer {
			mcpWg.Add(1)
			go func(svcLabel string) {
				defer mcpWg.Done()
				if err := o.StartService(svcLabel); err != nil {
					logging.Error("Orchestrator", err, "Failed to start MCP server %s", svcLabel)
				}
			}(label)
		}
	}
	// Wait for all MCP servers to finish starting (or fail)
	mcpWg.Wait()

	// Start the aggregator which depends on MCP servers
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if string(service.GetType()) == "Aggregator" {
			if err := o.StartService(label); err != nil {
				logging.Error("Orchestrator", err, "Failed to start aggregator %s", label)
			}
		}
	}

	return nil
}

// checkDependencies checks if all dependencies of a service are running.
// This is called before starting a service to ensure its prerequisites are met.
// If any dependency is not running, an error is returned with details about
// which dependency is missing.
func (o *Orchestrator) checkDependencies(label string) error {
	nodeID := o.getNodeIDForService(label)
	node := o.depGraph.Get(dependency.NodeID(nodeID))
	if node == nil {
		return nil // No node in graph means no dependencies
	}

	// Check each dependency
	for _, depNodeID := range node.DependsOn {
		depLabel := o.getLabelFromNodeID(string(depNodeID))
		depService, exists := o.registry.Get(depLabel)

		if !exists {
			return fmt.Errorf("dependency %s not found", depLabel)
		}

		// Dependency must be in running state
		if depService.GetState() != services.StateRunning {
			return fmt.Errorf("dependency %s is not running (state: %s)", depLabel, depService.GetState())
		}
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
// 1. Aggregator (2 seconds) - depends on MCP servers
// 2. MCP servers (3 seconds) - may need time to clean up
// 3. Port forwards (2 seconds) - kubectl processes to terminate
// 4. K8s connections (1 second) - should stop quickly
func (o *Orchestrator) stopAllServices() error {
	allServices := o.registry.GetAll()

	// Stop aggregator first with 2-second timeout
	aggCtx, aggCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer aggCancel()

	var wg sync.WaitGroup
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

	// Stop MCP servers with 3-second timeout
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer mcpCancel()

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
