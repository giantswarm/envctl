package config

import (
	"envctl/internal/utils"
)

// EnvctlConfig is the top-level configuration structure for envctl.
type EnvctlConfig struct {
	MCPServers     []MCPServerDefinition   `yaml:"mcpServers"`
	PortForwards   []PortForwardDefinition `yaml:"portForwards"`
	GlobalSettings GlobalSettings          `yaml:"globalSettings"`
}

// GlobalSettings might include things like default log levels, container runtime preferences, etc.
type GlobalSettings struct {
	DefaultContainerRuntime string `yaml:"defaultContainerRuntime,omitempty"` // e.g., "docker", "podman"
	// Add other global settings here
}

// MCPServerType defines the type of MCP server.
type MCPServerType string

const (
	MCPServerTypeLocalCommand MCPServerType = "localCommand"
	MCPServerTypeContainer    MCPServerType = "container"
)

// MCPServerDefinition defines how to run and manage an MCP server.
type MCPServerDefinition struct {
	Name     string        `yaml:"name"`               // Unique name for this server, e.g., "kubernetes", "prometheus-main"
	Type     MCPServerType `yaml:"type"`               // "localCommand" or "container"
	Enabled  bool          `yaml:"enabledByDefault"`   // Whether this server is started by default
	Icon     string        `yaml:"icon,omitempty"`     // Optional: an icon/emoji for display in TUI
	Category string        `yaml:"category,omitempty"` // Optional: for grouping in TUI, e.g., "Core", "Monitoring"

	// Fields for Type = "localCommand"
	Command []string          `yaml:"command,omitempty"` // Command and its arguments, e.g., ["npx", "mcp-server-kubernetes"]
	Env     map[string]string `yaml:"env,omitempty"`     // Environment variables for the command

	// MCP Proxy Configuration
	ProxyPort int `yaml:"proxyPort,omitempty"` // Port for mcp-proxy to bind to (0 = random port)

	// Fields for Type = "container"
	Image            string            `yaml:"image,omitempty"`            // Container image, e.g., "giantswarm/mcp-server-prometheus:latest"
	ContainerPorts   []string          `yaml:"containerPorts,omitempty"`   // Port mappings, e.g., ["8080:8080", "9090:9000"] (host:container)
	ContainerEnv     map[string]string `yaml:"containerEnv,omitempty"`     // Environment variables for the container
	ContainerVolumes []string          `yaml:"containerVolumes,omitempty"` // Volume mounts, e.g., ["~/.kube/config:/root/.kube/config"]
	HealthCheckCmd   []string          `yaml:"healthCheckCmd,omitempty"`   // Optional command to run inside container to check health
	Entrypoint       []string          `yaml:"entrypoint,omitempty"`       // Optional container entrypoint override
	ContainerUser    string            `yaml:"containerUser,omitempty"`    // Optional user to run container as

	// Dependencies
	RequiresPortForwards []string `yaml:"requiresPortForwards,omitempty"` // Names of PortForwardDefinition(s) needed by this server
	DependsOnServices    []string `yaml:"dependsOnServices,omitempty"`    // Names of other MCPServerDefinition(s) that must be healthy first
}

// PortForwardDefinition defines a Kubernetes port-forwarding configuration.
type PortForwardDefinition struct {
	Name     string `yaml:"name"`               // Unique name, e.g., "mc-prometheus", "wc-alloy"
	Enabled  bool   `yaml:"enabledByDefault"`   // Whether this port-forward is started by default
	Icon     string `yaml:"icon,omitempty"`     // Optional: an icon/emoji for display in TUI
	Category string `yaml:"category,omitempty"` // Optional: for grouping

	// KubeContextSelector helps determine which Kube context to use.
	// Examples: "mc", "wc", "explicit:<context-name>"
	// "mc" means use the current MC context.
	// "wc" means use the current WC context (if specified, otherwise fallback or error).
	KubeContextTarget   string `yaml:"kubeContextTarget"`
	Namespace           string `yaml:"namespace"`
	TargetType          string `yaml:"targetType"`                    // "service", "pod", "deployment", "statefulset"
	TargetName          string `yaml:"targetName"`                    // Name of the service, pod, etc.
	TargetLabelSelector string `yaml:"targetLabelSelector,omitempty"` // e.g., "app=prometheus,component=server" (used if TargetName is not specific enough or for pods)
	LocalPort           string `yaml:"localPort"`
	RemotePort          string `yaml:"remotePort"`
	BindAddress         string `yaml:"bindAddress,omitempty"` // Default "127.0.0.1"
}

// GetDefaultConfig returns the default, embedded configuration for envctl.
func GetDefaultConfig(mcName, wcName string) EnvctlConfig {
	// Placeholder for where default configs will be constructed
	// This will replicate the current hardcoded logic from
	// mcpserver.GetMCPServerConfig() and portforwarding.GetPortForwardConfig()
	// but map it to the new structures.

	mcKubeContext := ""
	if mcName != "" {
		mcKubeContext = utils.BuildMcContext(mcName)
	}

	wcKubeContext := ""
	alloyMetricsTargetContext := mcKubeContext
	if wcName != "" && mcName != "" {
		wcKubeContext = utils.BuildWcContext(mcName, wcName)
		alloyMetricsTargetContext = wcKubeContext
	}

	defaultPortForwards := []PortForwardDefinition{}
	if mcName != "" {
		defaultPortForwards = append(defaultPortForwards,
			PortForwardDefinition{
				Name:              "mc-prometheus",
				Enabled:           true,
				Icon:              "🔥",
				Category:          "Monitoring (MC)",
				KubeContextTarget: mcKubeContext, // Will need a way to resolve this to actual context name
				Namespace:         "mimir",
				TargetType:        "service",
				TargetName:        "mimir-query-frontend",
				LocalPort:         "8080",
				RemotePort:        "8080",
				BindAddress:       "127.0.0.1",
			},
			PortForwardDefinition{
				Name:              "mc-grafana",
				Enabled:           true,
				Icon:              "📊",
				Category:          "Monitoring (MC)",
				KubeContextTarget: mcKubeContext,
				Namespace:         "monitoring",
				TargetType:        "service",
				TargetName:        "grafana",
				LocalPort:         "3000",
				RemotePort:        "3000",
				BindAddress:       "127.0.0.1",
			},
		)
	}

	if alloyMetricsTargetContext != "" {
		alloyLabel := "alloy-metrics-mc"
		alloyCategory := "Metrics (MC)"
		if wcName != "" {
			alloyLabel = "alloy-metrics-wc"
			alloyCategory = "Metrics (WC)"
		}
		defaultPortForwards = append(defaultPortForwards, PortForwardDefinition{
			Name:              alloyLabel,
			Enabled:           true,
			Icon:              "✨",
			Category:          alloyCategory,
			KubeContextTarget: alloyMetricsTargetContext,
			Namespace:         "kube-system",
			TargetType:        "service",
			TargetName:        "alloy-metrics-cluster",
			LocalPort:         "12345",
			RemotePort:        "12345",
			BindAddress:       "127.0.0.1",
		})
	}

	defaultMCPServers := []MCPServerDefinition{
		{
			Name:      "kubernetes",
			Type:      MCPServerTypeLocalCommand,
			Enabled:   true,
			Icon:      "☸",
			Category:  "Core",
			Command:   []string{"npx", "mcp-server-kubernetes"},
			ProxyPort: 8001, // 0 means random port assignment
		},
		{
			Name:      "prometheus",
			Type:      MCPServerTypeLocalCommand,
			Enabled:   true,
			Icon:      "🔥",
			Category:  "Monitoring",
			Command:   []string{"uv", "--directory", "/home/teemow/projects/prometheus-mcp-server", "run", "src/prometheus_mcp_server/main.py"},
			ProxyPort: 8002, // 0 means random port assignment
			Env: map[string]string{
				"PROMETHEUS_URL": "http://localhost:8080/prometheus", // Assumes mc-prometheus port-forward
				"ORG_ID":         "giantswarm",
			},
			RequiresPortForwards: []string{"mc-prometheus"}, // Link to the port-forward by name
		},
		{
			Name:                 "grafana",
			Type:                 MCPServerTypeLocalCommand,
			Enabled:              true,
			Icon:                 "📊",
			Category:             "Monitoring",
			Command:              []string{"mcp-grafana"},
			ProxyPort:            8003,                                                      // 0 means random port assignment
			Env:                  map[string]string{"GRAFANA_URL": "http://localhost:3000"}, // Assumes mc-grafana port-forward
			RequiresPortForwards: []string{"mc-grafana"},
		},
	}

	return EnvctlConfig{
		MCPServers:   defaultMCPServers,
		PortForwards: defaultPortForwards,
		GlobalSettings: GlobalSettings{
			DefaultContainerRuntime: "docker", // A sensible default
		},
	}
}
