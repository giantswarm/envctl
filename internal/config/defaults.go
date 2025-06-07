package config

import (
	"envctl/internal/kube"
	"fmt"
)

// GenerateGiantSwarmClusters creates cluster definitions based on Giant Swarm naming patterns
func GenerateGiantSwarmClusters(mcName, wcName string) []ClusterDefinition {
	clusters := []ClusterDefinition{}

	// Always add the management cluster for observability
	if mcName != "" {
		clusters = append(clusters, ClusterDefinition{
			Name:        fmt.Sprintf("mc-%s", mcName),
			Context:     kube.BuildMcContext(mcName),
			Role:        ClusterRoleObservability,
			DisplayName: fmt.Sprintf("MC: %s", mcName),
			Icon:        "🏢",
		})

		// If no WC, MC can also be the target
		if wcName == "" {
			clusters = append(clusters, ClusterDefinition{
				Name:        fmt.Sprintf("mc-%s-target", mcName),
				Context:     kube.BuildMcContext(mcName),
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
			Context:     kube.BuildWcContext(mcName, wcName),
			Role:        ClusterRoleTarget,
			DisplayName: fmt.Sprintf("WC: %s", wcName),
			Icon:        "⚙️",
		})
	}

	return clusters
}

// GetDefaultConfigWithRoles returns the default configuration using the new cluster role system
func GetDefaultConfigWithRoles(mcName, wcName string) EnvctlConfig {
	clusters := GenerateGiantSwarmClusters(mcName, wcName)

	// Determine initial active clusters
	activeClusters := make(map[ClusterRole]string)
	for _, c := range clusters {
		// Set the first cluster found for each role as active
		if _, exists := activeClusters[c.Role]; !exists {
			activeClusters[c.Role] = c.Name
		}
	}

	return EnvctlConfig{
		Clusters:       clusters,
		ActiveClusters: activeClusters,
		PortForwards: []PortForwardDefinition{
			{
				Name:        "mc-prometheus",
				Enabled:     true,
				ClusterRole: ClusterRoleObservability, // Uses observability cluster
				Namespace:   "mimir",
				TargetType:  "service",
				TargetName:  "mimir-query-frontend",
				LocalPort:   "8080",
				RemotePort:  "8080",
			},
			{
				Name:        "mc-grafana",
				Enabled:     true,
				ClusterRole: ClusterRoleObservability, // Uses observability cluster
				Namespace:   "monitoring",
				TargetType:  "service",
				TargetName:  "grafana",
				LocalPort:   "3000",
				RemotePort:  "3000",
			},
			{
				Name:        "alloy-metrics",
				Enabled:     true,
				ClusterRole: ClusterRoleTarget, // Uses target cluster
				Namespace:   "kube-system",
				TargetType:  "service",
				TargetName:  "alloy-metrics",
				LocalPort:   "12345",
				RemotePort:  "12345",
			},
		},
		MCPServers: []MCPServerDefinition{
			{
				Name:     "teleport",
				Type:     MCPServerTypeLocalCommand,
				Enabled:  true,
				Icon:     "🔌",
				Category: "Core",
				Command:  []string{"node", "/home/teemow/projects/giantswarm/teleport-mcp/dist/index.js"},
			},
			{
				Name:                "k8s",
				Type:                MCPServerTypeLocalCommand,
				Enabled:             true,
				Icon:                "☸",
				Category:            "Core",
				Command:             []string{"npx", "mcp-server-kubernetes"},
				RequiresClusterRole: ClusterRoleTarget, // Uses the target cluster
			},
			{
				Name:                "capi",
				Type:                MCPServerTypeLocalCommand,
				Enabled:             true,
				Icon:                "☸",
				Category:            "Core",
				Command:             []string{"mcp-capi"},
				RequiresClusterRole: ClusterRoleTarget, // Uses the target cluster
			},
			{
				Name:                "app",
				Type:                MCPServerTypeLocalCommand,
				Enabled:             true,
				Icon:                "☸",
				Category:            "Core",
				Command:             []string{"mcp-giantswarm-apps"},
				RequiresClusterRole: ClusterRoleTarget, // Uses the target cluster
			},
			{
				Name:                "flux",
				Type:                MCPServerTypeLocalCommand,
				Enabled:             true,
				Icon:                "☸",
				Category:            "Core",
				Command:             []string{"flux-operator-mcp", "serve"},
				Env:                 map[string]string{"KUBECONFIG": "/home/teemow/.kube/config"},
				RequiresClusterRole: ClusterRoleTarget, // Uses the target cluster
			},
			{
				Name:     "prometheus",
				Type:     MCPServerTypeLocalCommand,
				Enabled:  true,
				Icon:     "🔥",
				Category: "Monitoring",
				Command:  []string{"uv", "--directory", "/home/teemow/projects/prometheus-mcp-server", "run", "src/prometheus_mcp_server/main.py"},
				Env: map[string]string{
					"PROMETHEUS_URL": "http://localhost:8080/prometheus",
					"ORG_ID":         "giantswarm",
				},
				RequiresPortForwards: []string{"mc-prometheus"},
			},
			{
				Name:                 "grafana",
				Type:                 MCPServerTypeLocalCommand,
				Enabled:              true,
				Icon:                 "📊",
				Category:             "Monitoring",
				Command:              []string{"mcp-grafana"},
				Env:                  map[string]string{"GRAFANA_URL": "http://localhost:3000"},
				RequiresPortForwards: []string{"mc-grafana"},
			},
		},
		GlobalSettings: GlobalSettings{
			DefaultContainerRuntime: "docker",
		},
		Aggregator: AggregatorConfig{
			Port:    8090,
			Host:    "localhost",
			Enabled: true,
		},
	}
}
