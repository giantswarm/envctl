package services

import (
	"context"
	"envctl/internal/capability"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// CapabilityService extends BaseService with capability management
type CapabilityService struct {
	*BaseService
	
	// Capability requirements
	requirements []capability.CapabilityRequirement
	
	// Active capability handles
	handles map[string]capability.CapabilityHandle
	handlesMu sync.RWMutex
	
	// Capability callbacks
	onCapabilityProvided func(handle capability.CapabilityHandle) error
	onCapabilityLost     func(handleID string) error
}

// NewCapabilityService creates a new capability-aware service
func NewCapabilityService(label string, serviceType ServiceType, dependencies []string, requirements []capability.CapabilityRequirement) *CapabilityService {
	return &CapabilityService{
		BaseService:  NewBaseService(label, serviceType, dependencies),
		requirements: requirements,
		handles:      make(map[string]capability.CapabilityHandle),
	}
}

// GetRequiredCapabilities returns the capabilities this service needs
func (cs *CapabilityService) GetRequiredCapabilities() []capability.CapabilityRequirement {
	return cs.requirements
}

// OnCapabilityProvided is called when a required capability is fulfilled
func (cs *CapabilityService) OnCapabilityProvided(handle capability.CapabilityHandle) error {
	cs.handlesMu.Lock()
	cs.handles[handle.ID] = handle
	cs.handlesMu.Unlock()
	
	logging.Debug(cs.GetLabel(), "Capability provided: %s (provider: %s)", handle.Type, handle.Provider)
	
	// Call custom callback if set
	if cs.onCapabilityProvided != nil {
		return cs.onCapabilityProvided(handle)
	}
	
	return nil
}

// OnCapabilityLost is called when a capability is no longer available
func (cs *CapabilityService) OnCapabilityLost(handleID string) error {
	cs.handlesMu.Lock()
	handle, exists := cs.handles[handleID]
	if exists {
		delete(cs.handles, handleID)
	}
	cs.handlesMu.Unlock()
	
	if exists {
		logging.Warn(cs.GetLabel(), "Capability lost: %s", handle.Type)
	}
	
	// Call custom callback if set
	if cs.onCapabilityLost != nil {
		return cs.onCapabilityLost(handleID)
	}
	
	// Check if this was a required capability
	for _, req := range cs.requirements {
		if !req.Optional && exists && req.Type == handle.Type {
			// A required capability was lost, fail the service
			err := fmt.Errorf("required capability %s lost", req.Type)
			cs.UpdateState(StateFailed, HealthUnhealthy, err)
			return err
		}
	}
	
	return nil
}

// SetCapabilityCallbacks sets custom callbacks for capability events
func (cs *CapabilityService) SetCapabilityCallbacks(
	onProvided func(handle capability.CapabilityHandle) error,
	onLost func(handleID string) error,
) {
	cs.onCapabilityProvided = onProvided
	cs.onCapabilityLost = onLost
}

// GetCapabilityHandle returns a capability handle by ID
func (cs *CapabilityService) GetCapabilityHandle(handleID string) (capability.CapabilityHandle, bool) {
	cs.handlesMu.RLock()
	defer cs.handlesMu.RUnlock()
	
	handle, exists := cs.handles[handleID]
	return handle, exists
}

// GetCapabilityHandleByType returns the first capability handle of a given type
func (cs *CapabilityService) GetCapabilityHandleByType(capType capability.CapabilityType) (capability.CapabilityHandle, bool) {
	cs.handlesMu.RLock()
	defer cs.handlesMu.RUnlock()
	
	for _, handle := range cs.handles {
		if handle.Type == capType {
			return handle, true
		}
	}
	
	return capability.CapabilityHandle{}, false
}

// GetAllCapabilityHandles returns all active capability handles
func (cs *CapabilityService) GetAllCapabilityHandles() []capability.CapabilityHandle {
	cs.handlesMu.RLock()
	defer cs.handlesMu.RUnlock()
	
	handles := make([]capability.CapabilityHandle, 0, len(cs.handles))
	for _, handle := range cs.handles {
		handles = append(handles, handle)
	}
	
	return handles
}

// HasRequiredCapabilities checks if all required capabilities are provided
func (cs *CapabilityService) HasRequiredCapabilities() bool {
	cs.handlesMu.RLock()
	defer cs.handlesMu.RUnlock()
	
	for _, req := range cs.requirements {
		if req.Optional {
			continue
		}
		
		// Check if we have a handle for this capability type
		found := false
		for _, handle := range cs.handles {
			if handle.Type == req.Type {
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

// WaitForCapabilities waits until all required capabilities are available
func (cs *CapabilityService) WaitForCapabilities(ctx context.Context) error {
	// Check if we already have all required capabilities
	if cs.HasRequiredCapabilities() {
		return nil
	}
	
	// Wait for capabilities with a timeout check
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if cs.HasRequiredCapabilities() {
				return nil
			}
		}
	}
} 