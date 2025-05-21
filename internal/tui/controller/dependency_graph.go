package controller

import (
	"envctl/internal/dependency"
	"envctl/internal/mcpserver"
	"envctl/internal/tui/model"
)

// BuildDependencyGraph constructs a dependency graph linking MCP proxies and
// port-forward processes so that the TUI can make lifecycle decisions
// (e.g. when an MCP is restarted its port-forwards are restarted first).
// It now accepts the list of MCP server configurations as an argument.
func BuildDependencyGraph(m *model.Model, mcpServerConfig []mcpserver.MCPServerConfig) *dependency.Graph {
	g := dependency.New()

	// 1. Port-forward nodes
	for label := range m.PortForwards {
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("pf:" + label),
			FriendlyName: label,
			Kind:         dependency.KindPortForward,
			DependsOn:    nil,
		})
	}

	// 2. MCP proxy nodes with their static dependencies
	for _, cfg := range mcpServerConfig {
		node := dependency.Node{
			ID:           dependency.NodeID("mcp:" + cfg.Name),
			FriendlyName: cfg.Name,
			Kind:         dependency.KindMCP,
		}
		switch cfg.Name {
		case "prometheus":
			node.DependsOn = []dependency.NodeID{dependency.NodeID("pf:Prometheus (MC)")}
		case "grafana":
			node.DependsOn = []dependency.NodeID{dependency.NodeID("pf:Grafana (MC)")}
		case "kubernetes":
			// No runtime port-forward deps
		}
		g.AddNode(node)
	}

	return g
}
