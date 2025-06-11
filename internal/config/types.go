package config

import (
	"time"
)

// EnvctlConfig is the top-level configuration structure for envctl.
type EnvctlConfig struct {
	MCPServers     []MCPServerDefinition `yaml:"mcpServers"`
	GlobalSettings GlobalSettings        `yaml:"globalSettings"`
	Aggregator     AggregatorConfig      `yaml:"aggregator"`
	Workflows      []WorkflowDefinition  `yaml:"workflows"`
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
	Type                MCPServerType `yaml:"type"`                          // "localCommand" or "container"
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
	Enabled      bool   `yaml:"enabled,omitempty"`      // Whether the aggregator is enabled (default: true if MCP servers exist)
	EnvctlPrefix string `yaml:"envctlPrefix,omitempty"` // Pre-prefix for all tools (default: "x")
}

// GetDefaultConfig returns the default configuration for envctl.
// mcName and wcName are the canonical names provided by the user.
func GetDefaultConfig(mcName, wcName string) EnvctlConfig {
	// Return minimal defaults - no k8s connection, no MCP servers, no port forwarding
	return GetDefaultConfigWithRoles()
}

// WorkflowDefinition defines a sequence of MCP tool calls
type WorkflowDefinition struct {
	Name            string              `yaml:"name"`
	Description     string              `yaml:"description"`
	Icon            string              `yaml:"icon,omitempty"`
	AgentModifiable bool                `yaml:"agentModifiable"`
	CreatedBy       string              `yaml:"createdBy,omitempty"`
	CreatedAt       time.Time           `yaml:"createdAt,omitempty"`
	LastModified    time.Time           `yaml:"lastModified,omitempty"`
	Version         int                 `yaml:"version,omitempty"`
	InputSchema     WorkflowInputSchema `yaml:"inputSchema"`
	Steps           []WorkflowStep      `yaml:"steps"`
}

// WorkflowInputSchema defines the input parameters for a workflow
type WorkflowInputSchema struct {
	Type       string                    `yaml:"type"`
	Properties map[string]SchemaProperty `yaml:"properties"`
	Required   []string                  `yaml:"required,omitempty"`
}

// SchemaProperty defines a single property in the schema
type SchemaProperty struct {
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default,omitempty"`
}

// WorkflowStep defines a single step in a workflow
type WorkflowStep struct {
	ID    string                 `yaml:"id"`
	Tool  string                 `yaml:"tool"`
	Args  map[string]interface{} `yaml:"args"`
	Store string                 `yaml:"store,omitempty"`
}

// WorkflowConfig for separate workflow files
type WorkflowConfig struct {
	Workflows []WorkflowDefinition `yaml:"workflows"`
}

// Workflow-related constants
const (
	// Workflow file names
	UserWorkflowsFile  = "workflows.yaml"
	AgentWorkflowsFile = "agent_workflows.yaml"

	// Workflow creator types
	WorkflowCreatorUser  = "user"
	WorkflowCreatorAgent = "agent"
)
