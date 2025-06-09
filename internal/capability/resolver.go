package capability

import (
	"fmt"
	"sync"
)

// Resolver resolves capability requirements to providers
type Resolver struct {
	registry *Registry
	mu       sync.RWMutex
	
	// Track which services are using which capabilities
	serviceHandles map[string][]CapabilityHandle // service -> handles
}

// NewResolver creates a new capability resolver
func NewResolver(registry *Registry) *Resolver {
	return &Resolver{
		registry:       registry,
		serviceHandles: make(map[string][]CapabilityHandle),
	}
}

// ResolveRequirement finds a provider for a capability requirement
func (r *Resolver) ResolveRequirement(req CapabilityRequirement) (*Capability, error) {
	request := CapabilityRequest{
		Type:     req.Type,
		Features: req.Features,
		Config:   req.Config,
	}
	
	matching := r.registry.FindMatching(request)
	
	if len(matching) == 0 && !req.Optional {
		return nil, fmt.Errorf("no provider found for required capability %s", req.Type)
	}
	
	if len(matching) == 0 {
		// Optional requirement with no provider
		return nil, nil
	}
	
	// For now, return the first matching provider
	// In the future, we could add selection logic based on:
	// - Provider health/status
	// - Load balancing
	// - User preferences
	return matching[0], nil
}

// RequestCapability requests a capability for a service
func (r *Resolver) RequestCapability(serviceLabel string, req CapabilityRequest) (*CapabilityHandle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Find matching capability
	matching := r.registry.FindMatching(req)
	if len(matching) == 0 {
		return nil, fmt.Errorf("no provider found for capability %s", req.Type)
	}
	
	// Use first matching provider
	provider := matching[0]
	
	// Create handle
	handle := &CapabilityHandle{
		ID:       fmt.Sprintf("%s-%s-%s", serviceLabel, provider.Provider, provider.ID),
		Provider: provider.Provider,
		Type:     provider.Type,
		Config:   provider.Config,
	}
	
	// Track the handle
	r.serviceHandles[serviceLabel] = append(r.serviceHandles[serviceLabel], *handle)
	
	return handle, nil
}

// ReleaseCapability releases a capability handle
func (r *Resolver) ReleaseCapability(serviceLabel string, handleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	handles, exists := r.serviceHandles[serviceLabel]
	if !exists {
		return fmt.Errorf("no capabilities registered for service %s", serviceLabel)
	}
	
	// Remove the handle
	var newHandles []CapabilityHandle
	found := false
	for _, h := range handles {
		if h.ID != handleID {
			newHandles = append(newHandles, h)
		} else {
			found = true
		}
	}
	
	if !found {
		return fmt.Errorf("handle %s not found for service %s", handleID, serviceLabel)
	}
	
	if len(newHandles) == 0 {
		delete(r.serviceHandles, serviceLabel)
	} else {
		r.serviceHandles[serviceLabel] = newHandles
	}
	
	return nil
}

// GetServiceHandles returns all capability handles for a service
func (r *Resolver) GetServiceHandles(serviceLabel string) []CapabilityHandle {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	handles := r.serviceHandles[serviceLabel]
	result := make([]CapabilityHandle, len(handles))
	copy(result, handles)
	return result
}

// ReleaseAllForService releases all capabilities for a service
func (r *Resolver) ReleaseAllForService(serviceLabel string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.serviceHandles, serviceLabel)
}

// GetServicesUsingCapability returns all services using a specific capability
func (r *Resolver) GetServicesUsingCapability(capabilityID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var services []string
	for service, handles := range r.serviceHandles {
		for _, handle := range handles {
			// Check if this handle is for the specified capability
			// The handle ID format is: serviceLabel-provider-capabilityID
			if len(handle.ID) > len(capabilityID) && 
			   handle.ID[len(handle.ID)-len(capabilityID):] == capabilityID {
				services = append(services, service)
				break
			}
		}
	}
	
	return services
}

// GetServicesUsingProvider returns all services using a specific provider
func (r *Resolver) GetServicesUsingProvider(provider string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var services []string
	for service, handles := range r.serviceHandles {
		for _, handle := range handles {
			if handle.Provider == provider {
				services = append(services, service)
				break
			}
		}
	}
	
	return services
} 