package capability

import (
	"fmt"
	"sync"
	"time"

	"envctl/internal/api"
	"envctl/pkg/logging"

	"github.com/google/uuid"
)

// Registry manages registered capabilities
type Registry struct {
	mu           sync.RWMutex
	capabilities map[string]*api.Capability   // capability ID -> capability
	byType       map[string][]*api.Capability // type -> capabilities
	byProvider   map[string][]*api.Capability // provider -> capabilities

	// Callbacks
	onRegister   []func(cap *api.Capability)
	onUnregister []func(capabilityID string)
	onUpdate     []func(cap *api.Capability)
}

// NewRegistry creates a new capability registry
func NewRegistry() *Registry {
	return &Registry{
		capabilities: make(map[string]*api.Capability),
		byType:       make(map[string][]*api.Capability),
		byProvider:   make(map[string][]*api.Capability),
		onRegister:   []func(cap *api.Capability){},
		onUnregister: []func(capabilityID string){},
		onUpdate:     []func(cap *api.Capability){},
	}
}

// Register registers a new capability
func (r *Registry) Register(cap *api.Capability) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not provided
	if cap.ID == "" {
		cap.ID = uuid.New().String()
	}

	// Check if already registered
	if _, exists := r.capabilities[cap.ID]; exists {
		return fmt.Errorf("capability %s already registered", cap.ID)
	}

	// Set initial status
	cap.State = api.CapabilityStateRegistering
	cap.LastCheck = time.Now()
	cap.Health = api.HealthUnknown

	// Add to registry
	r.capabilities[cap.ID] = cap
	r.byType[cap.Type] = append(r.byType[cap.Type], cap)
	r.byProvider[cap.Provider] = append(r.byProvider[cap.Provider], cap)

	// Update status to active
	cap.State = api.CapabilityStateActive
	cap.Health = api.HealthHealthy

	logging.Info("Registry", "Registered capability %s (type: %s, provider: %s)",
		cap.Name, cap.Type, cap.Provider)

	// Notify observers
	for _, callback := range r.onRegister {
		callback(cap)
	}

	return nil
}

// Unregister removes a capability
func (r *Registry) Unregister(capabilityID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cap, exists := r.capabilities[capabilityID]
	if !exists {
		return api.NewCapabilityNotFoundError(capabilityID)
	}

	// Remove from registry
	delete(r.capabilities, capabilityID)

	// Remove from type index
	typeSlice := r.byType[cap.Type]
	r.byType[cap.Type] = r.removeCapabilityFromSlice(typeSlice, cap)

	// Remove from provider index
	providerSlice := r.byProvider[cap.Provider]
	r.byProvider[cap.Provider] = r.removeCapabilityFromSlice(providerSlice, cap)

	logging.Info("Registry", "Unregistered capability %s", cap.Name)

	// Notify observers
	for _, callback := range r.onUnregister {
		callback(capabilityID)
	}

	return nil
}

// Update updates a capability's status
func (r *Registry) Update(capabilityID string, state api.CapabilityState, health api.HealthStatus, error string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cap, exists := r.capabilities[capabilityID]
	if !exists {
		return api.NewCapabilityNotFoundError(capabilityID)
	}

	// Update status
	cap.State = state
	cap.Health = health
	cap.Error = error
	cap.LastCheck = time.Now()

	logging.Debug("Registry", "Updated capability %s status to %s", cap.Name, state)

	// Notify observers
	for _, callback := range r.onUpdate {
		callback(cap)
	}

	return nil
}

// Get retrieves a capability by ID
func (r *Registry) Get(capabilityID string) (*api.Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, exists := r.capabilities[capabilityID]
	return cap, exists
}

// ListByType returns all capabilities of a given type
func (r *Registry) ListByType(capType string) []*api.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	caps := r.byType[capType]
	result := make([]*api.Capability, len(caps))
	copy(result, caps)
	return result
}

// ListByProvider returns all capabilities from a provider
func (r *Registry) ListByProvider(provider string) []*api.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	caps := r.byProvider[provider]
	result := make([]*api.Capability, len(caps))
	copy(result, caps)
	return result
}

// ListAll returns all registered capabilities
func (r *Registry) ListAll() []*api.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*api.Capability, 0, len(r.capabilities))
	for _, cap := range r.capabilities {
		result = append(result, cap)
	}
	return result
}

// FindMatching finds capabilities matching a request
func (r *Registry) FindMatching(req api.CapabilityRequest) []*api.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	candidates := r.byType[req.Type]
	var matching []*api.Capability

	for _, cap := range candidates {
		if r.matchesRequest(cap, req) {
			matching = append(matching, cap)
		}
	}

	return matching
}

// OnRegister adds a callback for capability registration
func (r *Registry) OnRegister(callback func(cap *api.Capability)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onRegister = append(r.onRegister, callback)
}

// OnUnregister adds a callback for capability removal
func (r *Registry) OnUnregister(callback func(capabilityID string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onUnregister = append(r.onUnregister, callback)
}

// OnUpdate adds a callback for capability updates
func (r *Registry) OnUpdate(callback func(cap *api.Capability)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onUpdate = append(r.onUpdate, callback)
}

// matchesRequest checks if a capability matches a request
func (r *Registry) matchesRequest(cap *api.Capability, req api.CapabilityRequest) bool {
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

	// Additional matching logic can be added here based on config

	return true
}

// removeCapabilityFromSlice removes a capability from a slice and returns the new slice
func (r *Registry) removeCapabilityFromSlice(slice []*api.Capability, cap *api.Capability) []*api.Capability {
	for i, c := range slice {
		if c.ID == cap.ID {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// GetProvider retrieves a capability by provider name
func (r *Registry) GetProvider(providerName string) (*api.Capability, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cap := range r.capabilities {
		if cap.Provider == providerName {
			return cap, nil
		}
	}

	return nil, fmt.Errorf("no capability found for provider %s", providerName)
}
