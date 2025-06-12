package capability

import (
	"time"
)

// CapabilityType represents the type of capability
type CapabilityType string

// Capability represents a capability that can be provided by an MCP server
type Capability struct {
	ID          string                 `json:"id"`
	Type        CapabilityType         `json:"type"`
	Provider    string                 `json:"provider"` // MCP server name
	Name        string                 `json:"name"`     // Human-readable name
	Description string                 `json:"description"`
	Features    []string               `json:"features"` // List of supported features
	Config      map[string]interface{} `json:"config"`   // Provider-specific config
	Status      CapabilityStatus       `json:"status"`
	Metadata    map[string]string      `json:"metadata"`
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
	Metadata    map[string]string      `json:"metadata"`
}

// CapabilityUpdate represents an update to a capability's status
type CapabilityUpdate struct {
	CapabilityID string            `json:"capability_id"`
	State        CapabilityState   `json:"state"`
	Error        string            `json:"error,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// IsValidCapabilityType checks if a capability type is valid
// A valid capability type is any non-empty string with valid characters
func IsValidCapabilityType(capType string) bool {
	// Allow any non-empty string as a capability type
	// Users can define their own capability types like "database", "monitoring", etc.
	return len(capType) > 0 && capType != ""
}
