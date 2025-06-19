package config

import (
	"time"
)

// EnvctlConfig is the top-level configuration structure for envctl.
type EnvctlConfig struct {
	GlobalSettings GlobalSettings   `yaml:"globalSettings"`
	Aggregator     AggregatorConfig `yaml:"aggregator"`
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

const (
	// MCPTransportStreamableHTTP is the streamable HTTP transport.
	MCPTransportStreamableHTTP = "streamable-http"
	// MCPTransportSSE is the Server-Sent Events transport.
	MCPTransportSSE = "sse"
	// MCPTransportStdio is the standard I/O transport.
	MCPTransportStdio = "stdio"
)

// CapabilityType defines the type of capability that an MCP server can provide.
type CapabilityType string

const (
	CapabilityTypeAuthProvider        CapabilityType = "auth_provider"
	CapabilityTypeDiscoveryProvider   CapabilityType = "discovery_provider"
	CapabilityTypePortforwardProvider CapabilityType = "portforward_provider"
	CapabilityTypeClusterProvider     CapabilityType = "cluster_provider"
)

// CapabilityType is kept for reference but capabilities are now defined in YAML files
// and MCP servers just provide tools without capability awareness

// MCPServerDefinition defines how to run and manage an MCP server.
type MCPServerDefinition struct {
	Name                string        `yaml:"name"`                          // Unique name for this server, e.g., "kubernetes", "prometheus-main"
	Type                MCPServerType `yaml:"type"`                          // "localCommand", "container", or "mock"
	Enabled             bool          `yaml:"enabledByDefault"`              // Whether this server is started by default
	Icon                string        `yaml:"icon,omitempty"`                // Optional: an icon/emoji for display in TUI
	Category            string        `yaml:"category,omitempty"`            // Optional: for grouping in TUI, e.g., "Core", "Monitoring"
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval,omitempty"` // Optional: custom health check interval
	ToolPrefix          string        `yaml:"toolPrefix,omitempty"`          // Custom prefix for tools (defaults to server name with underscore)

	// Fields for Type = "localCommand"
	Command []string          `yaml:"command,omitempty"` // Command and its arguments, e.g., ["npx", "mcp-server-kubernetes"]
	Env     map[string]string `yaml:"env,omitempty"`     // Environment variables

	// Fields for Type = "container"
	Image            string            `yaml:"image,omitempty"`            // Container image, e.g., "giantswarm/mcp-server-prometheus:latest"
	ContainerPorts   []string          `yaml:"containerPorts,omitempty"`   // Port mappings, e.g., ["8080:8080", "9090:9000"] (host:container)
	ContainerEnv     map[string]string `yaml:"containerEnv,omitempty"`     // Environment variables for the container
	ContainerVolumes []string          `yaml:"containerVolumes,omitempty"` // Volume mounts, e.g., ["~/.kube/config:/root/.kube/config"]
	HealthCheckCmd   []string          `yaml:"healthCheckCmd,omitempty"`   // Optional command to run inside container to check health
	Entrypoint       []string          `yaml:"entrypoint,omitempty"`       // Optional container entrypoint override
	ContainerUser    string            `yaml:"containerUser,omitempty"`    // Optional user to run container as
}

// AggregatorConfig defines the configuration for the MCP aggregator service.
type AggregatorConfig struct {
	Port         int    `yaml:"port,omitempty"`         // Port for the aggregator SSE endpoint (default: 8080)
	Host         string `yaml:"host,omitempty"`         // Host to bind to (default: localhost)
	Transport    string `yaml:"transport,omitempty"`    // Transport to use (default: streamable-http)
	Enabled      bool   `yaml:"enabled,omitempty"`      // Whether the aggregator is enabled (default: true if MCP servers exist)
	EnvctlPrefix string `yaml:"envctlPrefix,omitempty"` // Pre-prefix for all tools (default: "x")
}

// GetDefaultConfig returns the default configuration for envctl.
// mcName and wcName are the canonical names provided by the user.
func GetDefaultConfig(mcName, wcName string) EnvctlConfig {
	// Return minimal defaults - no k8s connection, no MCP servers, no port forwarding
	return GetDefaultConfigWithRoles()
}
