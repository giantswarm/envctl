package api

import (
	"time"
)

// MCPServer represents a single MCP server definition and runtime state
// This consolidates MCPServerDefinition, MCPServerInfo, and MCPServerConfig into one type
type MCPServer struct {
	// Configuration fields (from YAML)
	Name                string        `yaml:"name" json:"name"`
	Type                MCPServerType `yaml:"type" json:"type"`
	Enabled             bool          `yaml:"enabledByDefault" json:"enabled"`
	HealthCheckInterval time.Duration `yaml:"healthCheckInterval,omitempty" json:"healthCheckInterval,omitempty"`
	ToolPrefix          string        `yaml:"toolPrefix,omitempty" json:"toolPrefix,omitempty"`

	// LocalCommand fields
	Command []string          `yaml:"command,omitempty" json:"command,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Container fields
	Image            string            `yaml:"image,omitempty" json:"image,omitempty"`
	ContainerPorts   []string          `yaml:"containerPorts,omitempty" json:"containerPorts,omitempty"`
	ContainerEnv     map[string]string `yaml:"containerEnv,omitempty" json:"containerEnv,omitempty"`
	ContainerVolumes []string          `yaml:"containerVolumes,omitempty" json:"containerVolumes,omitempty"`
	HealthCheckCmd   []string          `yaml:"healthCheckCmd,omitempty" json:"healthCheckCmd,omitempty"`
	Entrypoint       []string          `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	ContainerUser    string            `yaml:"containerUser,omitempty" json:"containerUser,omitempty"`

	// Runtime state fields (for API responses)
	Label       string       `json:"label,omitempty" yaml:"-"`
	State       ServiceState `json:"state,omitempty" yaml:"-"`
	Health      HealthStatus `json:"health,omitempty" yaml:"-"`
	Available   bool         `json:"available,omitempty" yaml:"-"`
	Error       string       `json:"error,omitempty" yaml:"-"`
	Description string       `json:"description,omitempty" yaml:"-"`
}
