package orchestrator

import (
	"envctl/internal/capability"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
)

// ExtendedOrchestrator adds capability management to the orchestrator
type ExtendedOrchestrator struct {
	*Orchestrator

	// Capability management
	capabilityRegistry *capability.Registry
	capabilityResolver *capability.Resolver
}

// NewExtended creates a new orchestrator with capability support
func NewExtended(cfg Config) *ExtendedOrchestrator {
	// Create base orchestrator
	base := New(cfg)

	// Create capability registry and resolver
	registry := capability.NewRegistry()
	resolver := capability.NewResolver(registry)

	// Set up capability change monitoring
	registry.OnRegister(func(cap *capability.Capability) {
		logging.Info("Orchestrator", "Capability registered: %s (type: %s, provider: %s)",
			cap.Name, cap.Type, cap.Provider)
	})

	registry.OnUnregister(func(capabilityID string) {
		logging.Info("Orchestrator", "Capability unregistered: %s", capabilityID)
		// Check if any services were using this capability
		services := resolver.GetServicesUsingCapability(capabilityID)
		if len(services) > 0 {
			logging.Warn("Orchestrator", "Services affected by capability removal: %v", services)
		}
	})

	registry.OnUpdate(func(cap *capability.Capability) {
		logging.Debug("Orchestrator", "Capability updated: %s (state: %s)", cap.Name, cap.Status.State)
		// Check if capability became unhealthy
		if cap.Status.State == capability.CapabilityStateUnhealthy {
			services := resolver.GetServicesUsingCapability(cap.ID)
			if len(services) > 0 {
				logging.Warn("Orchestrator", "Capability %s became unhealthy, affected services: %v",
					cap.Name, services)
			}
		}
	})

	return &ExtendedOrchestrator{
		Orchestrator:       base,
		capabilityRegistry: registry,
		capabilityResolver: resolver,
	}
}

// GetCapabilityRegistry returns the capability registry
func (eo *ExtendedOrchestrator) GetCapabilityRegistry() *capability.Registry {
	return eo.capabilityRegistry
}

// GetCapabilityResolver returns the capability resolver
func (eo *ExtendedOrchestrator) GetCapabilityResolver() *capability.Resolver {
	return eo.capabilityResolver
}

// StartServiceWithCapabilities starts a service after resolving its capability requirements
func (eo *ExtendedOrchestrator) StartServiceWithCapabilities(label string) error {
	service, exists := eo.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Check if service has capability requirements
	if consumer, ok := service.(capability.CapabilityConsumer); ok {
		requirements := consumer.GetRequiredCapabilities()

		// Resolve each requirement
		for _, req := range requirements {
			cap, err := eo.capabilityResolver.ResolveRequirement(req)
			if err != nil {
				if !req.Optional {
					return fmt.Errorf("failed to resolve required capability %s: %w", req.Type, err)
				}
				logging.Warn("Orchestrator", "Optional capability %s not available for service %s",
					req.Type, label)
				continue
			}

			if cap != nil {
				// Request the capability for this service
				request := capability.CapabilityRequest{
					Type:     req.Type,
					Features: req.Features,
					Config:   req.Config,
				}

				handle, err := eo.capabilityResolver.RequestCapability(label, request)
				if err != nil {
					if !req.Optional {
						return fmt.Errorf("failed to request capability %s: %w", req.Type, err)
					}
					logging.Warn("Orchestrator", "Failed to request optional capability %s for service %s: %v",
						req.Type, label, err)
					continue
				}

				// Notify the service about the capability
				if err := consumer.OnCapabilityProvided(*handle); err != nil {
					// Release the capability if the service can't use it
					eo.capabilityResolver.ReleaseCapability(label, handle.ID)
					if !req.Optional {
						return fmt.Errorf("service %s failed to accept capability %s: %w",
							label, req.Type, err)
					}
					logging.Warn("Orchestrator", "Service %s failed to accept optional capability %s: %v",
						label, req.Type, err)
				}
			}
		}
	}

	// Now start the service normally
	return eo.StartService(label)
}

// StopServiceWithCapabilities stops a service and releases its capabilities
func (eo *ExtendedOrchestrator) StopServiceWithCapabilities(label string) error {
	// Release all capabilities for this service
	eo.capabilityResolver.ReleaseAllForService(label)

	// Check if service implements capability consumer
	if service, exists := eo.registry.Get(label); exists {
		if consumer, ok := service.(capability.CapabilityConsumer); ok {
			// Get all handles for this service
			handles := eo.capabilityResolver.GetServiceHandles(label)
			for _, handle := range handles {
				// Notify the service about capability loss
				consumer.OnCapabilityLost(handle.ID)
			}
		}
	}

	// Now stop the service normally
	return eo.StopService(label)
}

// RegisterCapabilityProvider registers a service as a capability provider
func (eo *ExtendedOrchestrator) RegisterCapabilityProvider(label string, provider capability.CapabilityProvider) error {
	// Create capability registration
	cap := &capability.Capability{
		Type:     provider.GetCapabilityType(),
		Provider: label,
		Name:     fmt.Sprintf("%s Provider", provider.GetCapabilityType()),
		Features: provider.GetCapabilityFeatures(),
	}

	// Register the capability
	return eo.capabilityRegistry.Register(cap)
}

// checkCapabilityHealth checks the health of all capabilities
func (eo *ExtendedOrchestrator) checkCapabilityHealth() {
	capabilities := eo.capabilityRegistry.ListAll()

	for _, cap := range capabilities {
		// Get the service that provides this capability
		service, exists := eo.registry.Get(cap.Provider)
		if !exists {
			// Provider service doesn't exist, mark capability as inactive
			status := capability.CapabilityStatus{
				State:  capability.CapabilityStateInactive,
				Error:  "Provider service not found",
				Health: capability.HealthStatusUnknown,
			}
			eo.capabilityRegistry.Update(cap.ID, status)
			continue
		}

		// Check service state
		state := service.GetState()
		health := service.GetHealth()

		var capState capability.CapabilityState
		var capHealth capability.HealthStatus
		var errorMsg string

		switch state {
		case services.StateRunning:
			if health == services.HealthHealthy {
				capState = capability.CapabilityStateActive
				capHealth = capability.HealthStatusHealthy
			} else {
				capState = capability.CapabilityStateUnhealthy
				capHealth = capability.HealthStatusUnhealthy
				errorMsg = fmt.Sprintf("Provider service unhealthy: %s", health)
			}
		case services.StateStopped:
			capState = capability.CapabilityStateInactive
			capHealth = capability.HealthStatusUnknown
			errorMsg = "Provider service stopped"
		case services.StateFailed:
			capState = capability.CapabilityStateUnhealthy
			capHealth = capability.HealthStatusUnhealthy
			errorMsg = "Provider service failed"
		default:
			capState = capability.CapabilityStateInactive
			capHealth = capability.HealthStatusUnknown
			errorMsg = fmt.Sprintf("Provider service in %s state", state)
		}

		// Update capability status
		status := capability.CapabilityStatus{
			State:  capState,
			Error:  errorMsg,
			Health: capHealth,
		}
		eo.capabilityRegistry.Update(cap.ID, status)
	}
}

// MonitorCapabilities starts monitoring capability health
func (eo *ExtendedOrchestrator) MonitorCapabilities() {
	// Run initial health check
	eo.checkCapabilityHealth()

	// Set up monitoring for service state changes
	eo.setGlobalStateChangeCallback(func(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
		// Check if this service provides any capabilities
		capabilities := eo.capabilityRegistry.ListByProvider(label)
		if len(capabilities) > 0 {
			logging.Debug("Orchestrator", "Service %s state changed, updating %d capabilities",
				label, len(capabilities))
			eo.checkCapabilityHealth()
		}

		// Call the original callback if it exists
		if eo.globalStateChangeCallback != nil {
			eo.globalStateChangeCallback(label, oldState, newState, health, err)
		}
	})
}
