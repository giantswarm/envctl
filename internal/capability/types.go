package capability

import (
	"time"
)

// CapabilityDefinition defines a capability with operations and requirements
type CapabilityDefinition struct {
	Name        string                         `yaml:"name"`
	Type        string                         `yaml:"type"`
	Version     string                         `yaml:"version"`
	Description string                         `yaml:"description"`
	Operations  map[string]OperationDefinition `yaml:"operations"`
}

// OperationDefinition defines an operation within a capability
type OperationDefinition struct {
	Description string               `yaml:"description"`
	Parameters  map[string]Parameter `yaml:"parameters"`
	Requires    []string             `yaml:"requires"`
	Workflow    *WorkflowDefinition  `yaml:"workflow,omitempty"`
}

// Parameter defines a parameter for an operation
type Parameter struct {
	Type        string      `yaml:"type"`
	Required    bool        `yaml:"required"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default,omitempty"`
}

// WorkflowDefinition defines a workflow for an operation
type WorkflowDefinition struct {
	Name            string                 `yaml:"name"`
	Description     string                 `yaml:"description"`
	AgentModifiable bool                   `yaml:"agentModifiable"`
	InputSchema     map[string]interface{} `yaml:"inputSchema"`
	Steps           []WorkflowStep         `yaml:"steps"`
}

// WorkflowStep defines a step in a workflow
type WorkflowStep struct {
	ID    string                 `yaml:"id"`
	Tool  string                 `yaml:"tool"`
	Args  map[string]interface{} `yaml:"args"`
	Store string                 `yaml:"store"`
}

// CapabilityType represents the type of capability
type CapabilityType string

// Capability represents a runtime capability instance (consolidated from multiple types)
type Capability struct {
	ID          string                 `json:"id"`
	Type        CapabilityType         `json:"type"`
	Provider    string                 `json:"provider"` // MCP server name
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Features    []string               `json:"features"`
	Config      map[string]interface{} `json:"config"`
	Status      CapabilityStatus       `json:"status"`
	Operations  []string               `json:"operations"`
}

// CapabilityStatus represents the current status of a capability
type CapabilityStatus struct {
	State     CapabilityState `json:"state"`
	LastCheck time.Time       `json:"last_check"`
	Error     string          `json:"error,omitempty"`
	Health    HealthStatus    `json:"health"`
}

// CapabilityState represents the state of a capability
type CapabilityState string

const (
	// CapabilityStateRegistering is when a capability is being registered
	CapabilityStateRegistering CapabilityState = "registering"
	// CapabilityStateActive is when a capability is active and healthy
	CapabilityStateActive CapabilityState = "active"
	// CapabilityStateUnhealthy is when a capability is not functioning properly
	CapabilityStateUnhealthy CapabilityState = "unhealthy"
	// CapabilityStateInactive is when a capability is not available
	CapabilityStateInactive CapabilityState = "inactive"
)

// HealthStatus represents the health of a capability
type HealthStatus string

const (
	// HealthStatusHealthy indicates the capability is functioning normally
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusUnhealthy indicates the capability has issues
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusUnknown indicates the health status is unknown
	HealthStatusUnknown HealthStatus = "unknown"
)

// CapabilityRequest represents a request for a capability
type CapabilityRequest struct {
	Type     CapabilityType         `json:"type"`
	Features []string               `json:"features"` // Required features
	Config   map[string]interface{} `json:"config"`   // Request-specific config
	Timeout  time.Duration          `json:"timeout"`
}

// CapabilityHandle represents an active capability fulfillment
type CapabilityHandle struct {
	ID         string                 `json:"id"`
	Provider   string                 `json:"provider"`
	Type       CapabilityType         `json:"type"`
	Config     map[string]interface{} `json:"config"` // Fulfillment details
	ValidUntil *time.Time             `json:"valid_until,omitempty"`
}

// CapabilityRequirement represents a capability requirement for a service
type CapabilityRequirement struct {
	Type     CapabilityType         `json:"type"`
	Features []string               `json:"features"`
	Config   map[string]interface{} `json:"config"`
	Optional bool                   `json:"optional"`
}

// CapabilityRegistration represents the data sent when registering a capability
type CapabilityRegistration struct {
	Type        CapabilityType         `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Features    []string               `json:"features"`
	Config      map[string]interface{} `json:"config"`
}

// CapabilityUpdate represents an update to a capability's status
type CapabilityUpdate struct {
	CapabilityID string          `json:"capability_id"`
	State        CapabilityState `json:"state"`
	Error        string          `json:"error,omitempty"`
}

// IsValidCapabilityType checks if a capability type is valid
// A valid capability type is any non-empty string with valid characters
func IsValidCapabilityType(capType string) bool {
	// Allow any non-empty string as a capability type
	// Users can define their own capability types like "database", "monitoring", etc.
	return len(capType) > 0 && capType != ""
}
