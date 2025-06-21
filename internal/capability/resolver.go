package capability

import (
	"context"
	"fmt"
	"sync"

	"envctl/internal/api"
	"envctl/pkg/logging"
)

// Resolver handles capability resolution and fulfillment
type Resolver struct {
	mu        sync.RWMutex
	registry  *Registry
	providers []Provider
	handles   map[string]*api.CapabilityHandle // service ID -> handles
}

// NewResolver creates a new capability resolver
func NewResolver(registry *Registry) *Resolver {
	return &Resolver{
		registry:  registry,
		providers: []Provider{},
		handles:   make(map[string]*api.CapabilityHandle),
	}
}

// RegisterProvider registers a capability provider
func (r *Resolver) RegisterProvider(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
	logging.Info("Resolver", "Registered capability provider for types: %v", provider.GetCapabilityTypes())
}

// ResolveRequirement attempts to resolve a capability requirement
func (r *Resolver) ResolveRequirement(ctx context.Context, serviceID string, req api.CapabilityRequirement) (*api.CapabilityHandle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	logging.Debug("Resolver", "Resolving requirement for service %s: type=%s, features=%v",
		serviceID, req.Type, req.Features)

	// First, try to find existing capabilities in the registry
	existing := r.registry.ListByType(req.Type)
	for _, cap := range existing {
		if r.matchesRequirement(cap, req) {
			// Create a handle for the existing capability
			handle := &api.CapabilityHandle{
				ID:       fmt.Sprintf("%s-%s", serviceID, cap.ID),
				Provider: cap.Provider,
				Type:     req.Type,
				Config:   req.Config,
			}

			r.handles[serviceID] = handle
			logging.Info("Resolver", "Resolved requirement using existing capability: %s", cap.Name)
			return handle, nil
		}
	}

	// If no existing capability found, try providers
	capReq := api.CapabilityRequest{
		Type:     req.Type,
		Features: req.Features,
		Config:   req.Config,
		Timeout:  0, // Use default timeout
	}

	for _, provider := range r.providers {
		if provider.CanProvide(req.Type, req.Features) {
			handle, err := provider.Request(ctx, capReq)
			if err != nil {
				logging.Warn("Resolver", "Provider failed to fulfill capability: %v", err)
				continue
			}

			r.handles[serviceID] = handle
			logging.Info("Resolver", "Resolved requirement using provider: %s", provider)
			return handle, nil
		}
	}

	return nil, fmt.Errorf("no provider available for capability type %s with features %v", req.Type, req.Features)
}

// ReleaseHandle releases a capability handle for a service
func (r *Resolver) ReleaseHandle(ctx context.Context, serviceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	handle, exists := r.handles[serviceID]
	if !exists {
		return fmt.Errorf("no handle found for service %s", serviceID)
	}

	// Find the provider and release the capability
	for _, provider := range r.providers {
		if err := provider.Release(ctx, handle); err != nil {
			logging.Warn("Resolver", "Provider failed to release capability: %v", err)
			continue
		}

		delete(r.handles, serviceID)
		logging.Info("Resolver", "Released capability handle for service %s", serviceID)
		return nil
	}

	return fmt.Errorf("failed to release capability handle for service %s", serviceID)
}

// GetHandle returns the capability handle for a service
func (r *Resolver) GetHandle(serviceID string) (*api.CapabilityHandle, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handle, exists := r.handles[serviceID]
	return handle, exists
}

// ListHandles returns all active capability handles
func (r *Resolver) ListHandles() map[string]*api.CapabilityHandle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*api.CapabilityHandle)
	for k, v := range r.handles {
		result[k] = v
	}
	return result
}

// matchesRequirement checks if a capability matches a requirement
func (r *Resolver) matchesRequirement(cap *api.Capability, req api.CapabilityRequirement) bool {
	// Check if capability is active
	if cap.State != api.CapabilityStateActive {
		return false
	}

	// Check if all required features are supported
	for _, requiredFeature := range req.Features {
		found := false
		for _, feature := range cap.Features {
			if feature == requiredFeature {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// GetProviders returns all registered providers
func (r *Resolver) GetProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, len(r.providers))
	copy(result, r.providers)
	return result
}
