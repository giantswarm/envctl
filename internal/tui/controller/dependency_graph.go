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

	// 1. Port-forward nodes
	// m.PortForwards is map[string]*model.PortForwardProcess
	// The key is pf.Name (formerly pf.Label)
	for pfName := range m.PortForwards {
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("pf:" + pfName),
			FriendlyName: pfName,
			Kind:         dependency.KindPortForward,
			DependsOn:    nil, // Dependencies for PFs are not modeled this way here.
		})
	}

	// 2. MCP proxy nodes with their static dependencies
	// m.MCPServerConfig is []config.MCPServerDefinition
	for _, mcpCfg := range m.MCPServerConfig { // Iterate over the config from the model
		deps := []dependency.NodeID{}
		for _, requiredPfName := range mcpCfg.RequiresPortForwards {
			deps = append(deps, dependency.NodeID("pf:" + requiredPfName))
		}

		node := dependency.Node{
			ID:           dependency.NodeID("mcp:" + mcpCfg.Name),
			FriendlyName: mcpCfg.Name,
			Kind:         dependency.KindMCP,
			DependsOn:    deps, // Use dynamically read dependencies
		}
		g.AddNode(node)
	}

	return g
}
