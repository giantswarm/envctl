package controller

import (
	"envctl/internal/dependency"
	"envctl/internal/tui/model"
)

// BuildDependencyGraph constructs a dependency graph linking MCP proxies and
// port-forward processes so that the TUI can make lifecycle decisions
// (e.g. when an MCP is restarted its port-forwards are restarted first).
// It now uses m.MCPServerConfig which is []config.MCPServerDefinition.
func BuildDependencyGraph(m *model.Model) *dependency.Graph {
	g := dependency.New()

	// 1. K8s connection nodes - these are fundamental dependencies
	// Add MC k8s connection node
	mcContext := ""
	if m.ManagementClusterName != "" {
		mcContext = m.KubeMgr.BuildMcContextName(m.ManagementClusterName)
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("k8s:" + mcContext),
			FriendlyName: "K8s MC Connection (" + m.ManagementClusterName + ")",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil, // K8s connections don't depend on anything
		})
	}

	// Add WC k8s connection node if applicable
	wcContext := ""
	if m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
		wcContext = m.KubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName)
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("k8s:" + wcContext),
			FriendlyName: "K8s WC Connection (" + m.WorkloadClusterName + ")",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil,
		})
	}

	// 2. Port-forward nodes - now depend on their k8s connection
	// m.PortForwards is map[string]*model.PortForwardProcess
	// The key is pf.Name (formerly pf.Label)
	for pfName, pf := range m.PortForwards {
		deps := []dependency.NodeID{}

		// Determine which k8s context this port forward uses
		if pf.ContextName != "" {
			deps = append(deps, dependency.NodeID("k8s:"+pf.ContextName))
		}

		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("pf:" + pfName),
			FriendlyName: pfName,
			Kind:         dependency.KindPortForward,
			DependsOn:    deps, // Now depends on k8s connection
		})
	}

	// 3. MCP proxy nodes with their dependencies
	// m.MCPServerConfig is []config.MCPServerDefinition
	for _, mcpCfg := range m.MCPServerConfig { // Iterate over the config from the model
		deps := []dependency.NodeID{}

		// Special handling for kubernetes MCP - it depends on k8s connection
		if mcpCfg.Name == "kubernetes" && mcContext != "" {
			deps = append(deps, dependency.NodeID("k8s:"+mcContext))
		}

		// Add port forward dependencies
		for _, requiredPfName := range mcpCfg.RequiresPortForwards {
			deps = append(deps, dependency.NodeID("pf:"+requiredPfName))
		}

		node := dependency.Node{
			ID:           dependency.NodeID("mcp:" + mcpCfg.Name),
			FriendlyName: mcpCfg.Name,
			Kind:         dependency.KindMCP,
			DependsOn:    deps, // Combined dependencies
		}
		g.AddNode(node)
	}

	return g
}
