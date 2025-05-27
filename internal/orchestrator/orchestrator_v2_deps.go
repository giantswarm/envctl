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

// buildDependencyGraph builds the dependency graph for all services
func (o *OrchestratorV2) buildDependencyGraph() *dependency.Graph {
	graph := dependency.New()

	// Add K8s connection nodes
	if o.mcName != "" {
		mcLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)
		graph.AddNode(dependency.Node{
			ID:           dependency.NodeID(mcLabel),
			FriendlyName: fmt.Sprintf("K8s MC: %s", o.mcName),
			Kind:         dependency.KindK8sConnection,
			DependsOn:    []dependency.NodeID{},
		})
	}

	if o.wcName != "" && o.mcName != "" {
		wcLabel := fmt.Sprintf("k8s-wc-%s", o.wcName)
		mcLabel := fmt.Sprintf("k8s-mc-%s", o.mcName)
		graph.AddNode(dependency.Node{
			ID:           dependency.NodeID(wcLabel),
			FriendlyName: fmt.Sprintf("K8s WC: %s", o.wcName),
			Kind:         dependency.KindK8sConnection,
			DependsOn:    []dependency.NodeID{dependency.NodeID(mcLabel)},
		})
	}

	// Add port forward nodes and dependencies
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}

		nodeID := dependency.NodeID("pf:" + pf.Name)
		var deps []dependency.NodeID

		// Port forwards depend on their target K8s connection
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

	// Add MCP server nodes and dependencies
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}

		nodeID := dependency.NodeID("mcp:" + mcp.Name)
		var deps []dependency.NodeID

		// MCP servers depend on their configured port forwards
		for _, pfName := range mcp.RequiresPortForwards {
			depNodeID := dependency.NodeID("pf:" + pfName)
			// Check if the port forward exists
			for _, pf := range o.portForwards {
				if pf.Name == pfName && pf.Enabled {
					deps = append(deps, depNodeID)
					break
				}
			}
		}

		graph.AddNode(dependency.Node{
			ID:           nodeID,
			FriendlyName: mcp.Name,
			Kind:         dependency.KindMCP,
			DependsOn:    deps,
		})
	}

	return graph
}

// startServicesInOrder starts all services in dependency order
func (o *OrchestratorV2) startServicesInOrder() error {
	// Get all services that should be started
	var servicesToStart []string
	allServices := o.registry.GetAll()

	for _, service := range allServices {
		label := service.GetLabel()

		// Skip manually stopped services
		o.mu.RLock()
		if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
			o.mu.RUnlock()
			continue
		}
		o.mu.RUnlock()

		servicesToStart = append(servicesToStart, label)
	}

	// For now, use a simple approach: start K8s connections first, then port forwards, then MCP servers
	// This is a temporary solution until we implement proper topological sorting

	// Start K8s connections first
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypeKubeConnection {
			if err := o.StartService(label); err != nil {
				logging.Error("OrchestratorV2", err, "Failed to start K8s connection %s", label)
			}
		}
	}

	// Wait a bit for K8s connections to be ready
	time.Sleep(500 * time.Millisecond)

	// Start port forwards
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypePortForward {
			if err := o.StartService(label); err != nil {
				logging.Error("OrchestratorV2", err, "Failed to start port forward %s", label)
			}
		}
	}

	// Wait a bit for port forwards to be ready
	time.Sleep(500 * time.Millisecond)

	// Start MCP servers
	for _, label := range servicesToStart {
		service, _ := o.registry.Get(label)
		if service.GetType() == services.TypeMCPServer {
			if err := o.StartService(label); err != nil {
				logging.Error("OrchestratorV2", err, "Failed to start MCP server %s", label)
			}
		}
	}

	return nil
}

// checkDependencies checks if all dependencies of a service are running
func (o *OrchestratorV2) checkDependencies(label string) error {
	nodeID := o.getNodeIDForService(label)
	node := o.depGraph.Get(dependency.NodeID(nodeID))
	if node == nil {
		return nil // No node in graph, no dependencies
	}

	for _, depNodeID := range node.DependsOn {
		depLabel := o.getLabelFromNodeID(string(depNodeID))
		depService, exists := o.registry.Get(depLabel)

		if !exists {
			return fmt.Errorf("dependency %s not found", depLabel)
		}

		if depService.GetState() != services.StateRunning {
			return fmt.Errorf("dependency %s is not running (state: %s)", depLabel, depService.GetState())
		}
	}

	return nil
}

// stopDependentServices stops all services that depend on the given service
func (o *OrchestratorV2) stopDependentServices(label string) error {
	nodeID := o.getNodeIDForService(label)
	dependents := o.depGraph.Dependents(dependency.NodeID(nodeID))

	var errors []error
	for _, depNodeID := range dependents {
		depLabel := o.getLabelFromNodeID(string(depNodeID))

		// Mark as stopped due to dependency
		o.mu.Lock()
		o.stopReasons[depLabel] = StopReasonDependency
		o.mu.Unlock()

		// Stop the dependent service
		if depService, exists := o.registry.Get(depLabel); exists {
			if err := depService.Stop(o.ctx); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop dependent %s: %w", depLabel, err))
			} else {
				logging.Info("OrchestratorV2", "Stopped dependent service: %s", depLabel)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping dependent services: %v", errors)
	}

	return nil
}

// stopAllServices stops all services in reverse dependency order
func (o *OrchestratorV2) stopAllServices() error {
	allServices := o.registry.GetAll()

	// Stop in reverse order: MCP servers first, then port forwards, then K8s connections

	// Stop MCP servers with 3-second timeout
	mcpCtx, mcpCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer mcpCancel()
	
	var wg sync.WaitGroup
	for _, service := range allServices {
		if service.GetType() == services.TypeMCPServer {
			wg.Add(1)
			go func(svc services.Service) {
				defer wg.Done()
				if err := svc.Stop(mcpCtx); err != nil && err != context.DeadlineExceeded {
					logging.Error("OrchestratorV2", err, "Failed to stop MCP server %s", svc.GetLabel())
				} else {
					logging.Debug("OrchestratorV2", "Stopped MCP server %s", svc.GetLabel())
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
					logging.Error("OrchestratorV2", err, "Failed to stop port forward %s", svc.GetLabel())
				} else {
					logging.Debug("OrchestratorV2", "Stopped port forward %s", svc.GetLabel())
				}
			}(service)
		}
	}
	wg.Wait()

	// Stop K8s connections with 1-second timeout (they should stop quickly)
	k8sCtx, k8sCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer k8sCancel()
	
	for _, service := range allServices {
		if service.GetType() == services.TypeKubeConnection {
			wg.Add(1)
			go func(svc services.Service) {
				defer wg.Done()
				if err := svc.Stop(k8sCtx); err != nil && err != context.DeadlineExceeded {
					logging.Error("OrchestratorV2", err, "Failed to stop K8s connection %s", svc.GetLabel())
				} else {
					logging.Debug("OrchestratorV2", "Stopped K8s connection %s", svc.GetLabel())
				}
			}(service)
		}
	}
	wg.Wait()

	logging.Info("OrchestratorV2", "All services stopped")
	return nil
}

// getNodeIDForService converts a service label to a dependency graph node ID
func (o *OrchestratorV2) getNodeIDForService(label string) string {
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

// getLabelFromNodeID extracts the service label from a node ID
func (o *OrchestratorV2) getLabelFromNodeID(nodeID string) string {
	if strings.HasPrefix(nodeID, "pf:") {
		return strings.TrimPrefix(nodeID, "pf:")
	} else if strings.HasPrefix(nodeID, "mcp:") {
		return strings.TrimPrefix(nodeID, "mcp:")
	}
	return nodeID
}

// monitorServices monitors service health and handles restarts
func (o *OrchestratorV2) monitorServices() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.checkAndRestartFailedServices()
		}
	}
}

// checkAndRestartFailedServices checks for failed services and restarts them if needed
func (o *OrchestratorV2) checkAndRestartFailedServices() {
	allServices := o.registry.GetAll()

	for _, service := range allServices {
		label := service.GetLabel()

		// Skip manually stopped services
		o.mu.RLock()
		if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
			o.mu.RUnlock()
			continue
		}
		o.mu.RUnlock()

		// Check if service has failed
		if service.GetState() == services.StateFailed {
			// Check if dependencies are still satisfied
			if err := o.checkDependencies(label); err != nil {
				logging.Debug("OrchestratorV2", "Skipping restart of %s: %v", label, err)
				continue
			}

			// Attempt to restart
			logging.Info("OrchestratorV2", "Attempting to restart failed service: %s", label)
			if err := service.Restart(o.ctx); err != nil {
				logging.Error("OrchestratorV2", err, "Failed to restart service %s", label)
			}
		}
	}
}
