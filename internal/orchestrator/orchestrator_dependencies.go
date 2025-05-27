package orchestrator

import (
	"envctl/internal/dependency"
	"envctl/internal/managers"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"time"
)

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
