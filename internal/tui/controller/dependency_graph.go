package controller

import (
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/tui/model"
)

// BuildDependencyGraph constructs a dependency graph linking MCP proxies and
// port-forward processes so that the TUI can make lifecycle decisions
// (e.g. when an MCP is restarted its port-forwards are restarted first).
// It now uses m.MCPServerConfig which is []config.MCPServerDefinition.
func BuildDependencyGraph(m *model.Model) *dependency.Graph {
	g := dependency.New()

	// 1. K8s connection nodes (no dependencies)
	if m.ManagementClusterName != "" {
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("k8s-mc"),
			FriendlyName: "K8s MC Connection",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil,
		})
	}

	if m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("k8s-wc"),
			FriendlyName: "K8s WC Connection",
			Kind:         dependency.KindK8sConnection,
			DependsOn:    nil,
		})
	}

	// 2. Port-forward nodes - now depend on their k8s connection
	// m.PortForwards is map[string]*model.PortForwardProcess
	// The key is pf.Name (formerly pf.Label)
	for _, pf := range m.PortForwardingConfig {
		if !pf.Enabled {
			continue
		}

		deps := []dependency.NodeID{}

		// Determine which k8s context this port forward uses
		contextName := pf.KubeContextTarget
		if contextName != "" {
			// Map context to k8s connection node
			if contextName == kube.BuildMcContext(m.ManagementClusterName) && m.ManagementClusterName != "" {
				deps = append(deps, dependency.NodeID("k8s-mc"))
			} else if contextName == kube.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName) && m.WorkloadClusterName != "" {
				deps = append(deps, dependency.NodeID("k8s-wc"))
			}
		}

		g.AddNode(dependency.Node{
			ID:           dependency.NodeID("pf:" + pf.Name),
			FriendlyName: pf.Name,
			Kind:         dependency.KindPortForward,
			DependsOn:    deps, // Now depends on k8s connection
		})
	}

	// 3. MCP proxy nodes with their dependencies
	// m.MCPServerConfig is []config.MCPServerDefinition
	for _, mcp := range m.MCPServerConfig { // Iterate over the config from the model
		if !mcp.Enabled {
			continue
		}

		deps := []dependency.NodeID{}

		// Special handling for kubernetes MCP - it depends on k8s connection
		if mcp.Name == "kubernetes" && m.ManagementClusterName != "" {
			deps = append(deps, dependency.NodeID("k8s-mc"))
		}

		// Add port forward dependencies
		for _, requiredPf := range mcp.RequiresPortForwards {
			deps = append(deps, dependency.NodeID("pf:"+requiredPf))
		}

		node := dependency.Node{
			ID:           dependency.NodeID("mcp:" + mcp.Name),
			FriendlyName: mcp.Name,
			Kind:         dependency.KindMCP,
			DependsOn:    deps, // Combined dependencies
		}
		g.AddNode(node)
	}

	return g
}
