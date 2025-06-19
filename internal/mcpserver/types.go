package mcpserver

import (
	"context"
	"io"
	"time"
)

// MCPClient interface defines what mcpserver needs from an MCP client
// This breaks the import cycle with aggregator package
type MCPClient interface {
	Initialize(ctx context.Context) error
	Close() error
	GetStderr() (io.Reader, bool)
}

// McpDiscreteStatusUpdate is used to report discrete status changes from a running MCP process.
// It focuses on the state, not verbose logs.
type McpDiscreteStatusUpdate struct {
	Label         string // The unique label of the MCP server instance
	ProcessStatus string // A string indicating the process status, e.g., "ProcessInitializing", "ProcessStarting", "ProcessRunning", "ProcessExitedWithError"
	ProcessErr    error  // The actual Go error object if the process failed or exited with an error
}

// McpUpdateFunc is a callback function type for receiving McpDiscreteStatusUpdate messages.
type McpUpdateFunc func(update McpDiscreteStatusUpdate)

// MCPServerType defines the type of MCP server.
type MCPServerType string

const (
	MCPServerTypeLocalCommand MCPServerType = "localCommand"
	MCPServerTypeContainer    MCPServerType = "container"
	MCPServerTypeMock         MCPServerType = "mock"
)

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
