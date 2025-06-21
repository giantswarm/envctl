package api

import (
	"time"
)

// Common enums and types used across all envctl concepts

// ServiceState represents the current state of a service
type ServiceState string

const (
	StateStopped  ServiceState = "stopped"
	StateStarting ServiceState = "starting"
	StateRunning  ServiceState = "running"
	StateStopping ServiceState = "stopping"
	StateError    ServiceState = "error"
	StateFailed   ServiceState = "failed"
	StateUnknown  ServiceState = "unknown"
	StateWaiting  ServiceState = "waiting"
	StateRetrying ServiceState = "retrying"
)

// HealthStatus represents the health status of a service or capability
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthChecking  HealthStatus = "checking"
)

// CapabilityState represents the state of a capability
type CapabilityState string

const (
	CapabilityStateRegistering CapabilityState = "registering"
	CapabilityStateActive      CapabilityState = "active"
	CapabilityStateUnhealthy   CapabilityState = "unhealthy"
	CapabilityStateInactive    CapabilityState = "inactive"
)

// MCPServerType defines the type of MCP server
type MCPServerType string

const (
	MCPServerTypeLocalCommand MCPServerType = "localCommand"
	MCPServerTypeContainer    MCPServerType = "container"
)

// Parameter defines a parameter for operations and workflows
type Parameter struct {
	Type        string      `yaml:"type" json:"type"`
	Required    bool        `yaml:"required" json:"required"`
	Description string      `yaml:"description" json:"description"`
	Default     interface{} `yaml:"default,omitempty" json:"default,omitempty"`
}

// OperationDefinition defines an operation that can be performed
type OperationDefinition struct {
	Description string               `yaml:"description" json:"description"`
	Parameters  map[string]Parameter `yaml:"parameters" json:"parameters"`
	Requires    []string             `yaml:"requires" json:"requires"`
	Workflow    *WorkflowReference   `yaml:"workflow,omitempty" json:"workflow,omitempty"`
}

// WorkflowReference references a workflow for an operation (simplified to avoid circular deps)
type WorkflowReference struct {
	Name            string                 `yaml:"name" json:"name"`
	Description     string                 `yaml:"description" json:"description"`
	AgentModifiable bool                   `yaml:"agentModifiable" json:"agentModifiable"`
	InputSchema     map[string]interface{} `yaml:"inputSchema" json:"inputSchema"`
	Steps           []WorkflowStep         `yaml:"steps" json:"steps"`
}

// WorkflowStep defines a step in a workflow
type WorkflowStep struct {
	ID          string                 `yaml:"id" json:"id"`
	Tool        string                 `yaml:"tool" json:"tool"`
	Args        map[string]interface{} `yaml:"args,omitempty" json:"args,omitempty"`
	Store       string                 `yaml:"store,omitempty" json:"store,omitempty"`
	Condition   string                 `yaml:"condition,omitempty" json:"condition,omitempty"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
}

// WorkflowInputSchema defines the input parameters for a workflow
type WorkflowInputSchema struct {
	Type       string                    `yaml:"type" json:"type"`
	Properties map[string]SchemaProperty `yaml:"properties" json:"properties"`
	Required   []string                  `yaml:"required,omitempty" json:"required,omitempty"`
}

// SchemaProperty defines a single property in the schema
type SchemaProperty struct {
	Type        string      `yaml:"type" json:"type"`
	Description string      `yaml:"description" json:"description"`
	Default     interface{} `yaml:"default,omitempty" json:"default,omitempty"`
}

// TimeoutConfig defines timeout behavior for operations
type TimeoutConfig struct {
	Create      time.Duration `yaml:"create" json:"create"`
	Delete      time.Duration `yaml:"delete" json:"delete"`
	HealthCheck time.Duration `yaml:"healthCheck" json:"healthCheck"`
}

// HealthCheckConfig defines health checking behavior
type HealthCheckConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	Interval         time.Duration `yaml:"interval" json:"interval"`
	FailureThreshold int           `yaml:"failureThreshold" json:"failureThreshold"`
	SuccessThreshold int           `yaml:"successThreshold" json:"successThreshold"`
}

// ParameterMapping defines how service creation parameters map to tool arguments
type ParameterMapping struct {
	ToolParameter string      `yaml:"toolParameter" json:"toolParameter"`
	Default       interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	Required      bool        `yaml:"required" json:"required"`
	Transform     string      `yaml:"transform,omitempty" json:"transform,omitempty"`
}

// ResponseMapping defines how to extract information from tool responses
type ResponseMapping struct {
	ServiceID string            `yaml:"serviceId,omitempty" json:"serviceId,omitempty"`
	Status    string            `yaml:"status,omitempty" json:"status,omitempty"`
	Health    string            `yaml:"health,omitempty" json:"health,omitempty"`
	Error     string            `yaml:"error,omitempty" json:"error,omitempty"`
	Metadata  map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ToolCall defines how to call an aggregator tool for a lifecycle event
type ToolCall struct {
	Tool            string                 `yaml:"tool" json:"tool"`
	Arguments       map[string]interface{} `yaml:"arguments" json:"arguments"`
	ResponseMapping ResponseMapping        `yaml:"responseMapping" json:"responseMapping"`
}

// IsValidCapabilityType checks if a capability type is valid
// A valid capability type is any non-empty string with valid characters
func IsValidCapabilityType(capType string) bool {
	// Allow any non-empty string as a capability type
	// Users can define their own capability types like "database", "monitoring", etc.
	return len(capType) > 0 && capType != ""
}
