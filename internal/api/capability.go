package api

import (
	"time"
)

// Capability represents a single capability definition and runtime state
// This consolidates CapabilityDefinition, CapabilityInfo, and runtime Capability into one type
type Capability struct {
	// Configuration fields (from YAML)
	Name        string                         `yaml:"name" json:"name"`
	Type        string                         `yaml:"type" json:"type"`
	Version     string                         `yaml:"version" json:"version"`
	Description string                         `yaml:"description" json:"description"`
	Operations  map[string]OperationDefinition `yaml:"operations" json:"operations"`

	// Runtime state fields (for API responses)
	Available bool            `json:"available,omitempty" yaml:"-"`
	State     CapabilityState `json:"state,omitempty" yaml:"-"`
	Health    HealthStatus    `json:"health,omitempty" yaml:"-"`
	Error     string          `json:"error,omitempty" yaml:"-"`
	Provider  string          `json:"provider,omitempty" yaml:"-"`

	// Runtime configuration
	ID        string                 `json:"id,omitempty" yaml:"-"`
	Features  []string               `json:"features,omitempty" yaml:"-"`
	Config    map[string]interface{} `json:"config,omitempty" yaml:"-"`
	LastCheck time.Time              `json:"lastCheck,omitempty" yaml:"-"`
}

// CapabilityRequest represents a request for a capability
type CapabilityRequest struct {
	Type     string                 `json:"type"`
	Features []string               `json:"features"`
	Config   map[string]interface{} `json:"config"`
	Timeout  time.Duration          `json:"timeout"`
}

// CapabilityHandle represents an active capability fulfillment
type CapabilityHandle struct {
	ID         string                 `json:"id"`
	Provider   string                 `json:"provider"`
	Type       string                 `json:"type"`
	Config     map[string]interface{} `json:"config"`
	ValidUntil *time.Time             `json:"valid_until,omitempty"`
}

// CapabilityRequirement represents a capability requirement for a service
type CapabilityRequirement struct {
	Type     string                 `json:"type"`
	Features []string               `json:"features"`
	Config   map[string]interface{} `json:"config"`
	Optional bool                   `json:"optional"`
}

// CapabilityRegistration represents the data sent when registering a capability
type CapabilityRegistration struct {
	Type        string                 `json:"type"`
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
