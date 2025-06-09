package capability

import (
	"context"
)

// CapabilityProvider is implemented by services that provide capabilities
type CapabilityProvider interface {
	// GetCapabilityType returns the type of capability this provider offers
	GetCapabilityType() CapabilityType

	// GetCapabilityFeatures returns the list of features this provider supports
	GetCapabilityFeatures() []string

	// ValidateCapabilityRequest validates if this provider can fulfill a request
	ValidateCapabilityRequest(req CapabilityRequest) error

	// ProvideCapability attempts to fulfill a capability request
	ProvideCapability(ctx context.Context, req CapabilityRequest) (CapabilityHandle, error)
}

// CapabilityConsumer is implemented by services that require capabilities
type CapabilityConsumer interface {
	// GetRequiredCapabilities returns the capabilities this service needs
	GetRequiredCapabilities() []CapabilityRequirement

	// OnCapabilityProvided is called when a required capability is fulfilled
	OnCapabilityProvided(cap CapabilityHandle) error

	// OnCapabilityLost is called when a capability is no longer available
	OnCapabilityLost(capabilityID string) error
}

// RegistryObserver can be implemented to receive notifications about registry changes
type RegistryObserver interface {
	// OnCapabilityRegistered is called when a new capability is registered
	OnCapabilityRegistered(cap *Capability)

	// OnCapabilityUnregistered is called when a capability is removed
	OnCapabilityUnregistered(capabilityID string)

	// OnCapabilityUpdated is called when a capability is updated
	OnCapabilityUpdated(cap *Capability)
} 