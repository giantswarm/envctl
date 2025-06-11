package config

import (
	"fmt"
)

// GenerateGiantSwarmClusters creates cluster definitions based on Giant Swarm naming patterns
// This is still available as a helper function for those who want to use it
func GenerateGiantSwarmClusters(mcName, wcName string) []ClusterDefinition {
	clusters := []ClusterDefinition{}

	// Always add the management cluster for observability
	if mcName != "" {
		clusters = append(clusters, ClusterDefinition{
			Name:        fmt.Sprintf("mc-%s", mcName),
			Context:     fmt.Sprintf("teleport.giantswarm.io-%s", mcName),
			Role:        ClusterRoleObservability,
			DisplayName: fmt.Sprintf("MC: %s", mcName),
			Icon:        "🏢",
		})

		// If no WC, MC can also be the target
		if wcName == "" {
			clusters = append(clusters, ClusterDefinition{
				Name:        fmt.Sprintf("mc-%s-target", mcName),
				Context:     fmt.Sprintf("teleport.giantswarm.io-%s", mcName),
				Role:        ClusterRoleTarget,
				DisplayName: fmt.Sprintf("MC: %s (target)", mcName),
				Icon:        "🎯",
			})
		}
	}

	// Add workload cluster as target if specified
	if wcName != "" && mcName != "" {
		clusters = append(clusters, ClusterDefinition{
			Name:        fmt.Sprintf("wc-%s", wcName),
			Context:     fmt.Sprintf("teleport.giantswarm.io-%s-%s", mcName, wcName),
			Role:        ClusterRoleTarget,
			DisplayName: fmt.Sprintf("WC: %s", wcName),
			Icon:        "⚙️",
		})
	}

	return clusters
}

// GetDefaultConfigWithRoles returns minimal default configuration
// By default: no k8s connection, no MCP servers, no port forwarding
func GetDefaultConfigWithRoles(mcName, wcName string) EnvctlConfig {
	return EnvctlConfig{
		Clusters:       []ClusterDefinition{},
		ActiveClusters: make(map[ClusterRole]string),
		PortForwards:   []PortForwardDefinition{},
		MCPServers:     []MCPServerDefinition{},
		GlobalSettings: GlobalSettings{
			DefaultContainerRuntime: "docker",
		},
		Aggregator: AggregatorConfig{
			Port:    8090,
			Host:    "localhost",
			Enabled: true, // Aggregator is enabled by default
		},
		Workflows: []WorkflowDefinition{},
	}
}
