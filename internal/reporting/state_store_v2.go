package reporting

import (
	"sync"
	"time"
)

// StateStoreV2 is a simplified state store that only tracks high-level service states
// Service-specific data is now managed by the services themselves
type StateStoreV2 struct {
	mu     sync.RWMutex
	states map[string]ServiceStateV2
}

// ServiceStateV2 represents simplified service state without service-specific details
type ServiceStateV2 struct {
	Label       string
	Type        ServiceType
	State       ServiceState
	Health      HealthStatus
	LastUpdated time.Time
	Error       error
}

// NewStateStoreV2 creates a new simplified state store
func NewStateStoreV2() *StateStoreV2 {
	return &StateStoreV2{
		states: make(map[string]ServiceStateV2),
	}
}

// UpdateState updates the state of a service
func (s *StateStoreV2) UpdateState(label string, serviceType ServiceType, state ServiceState, health HealthStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.states[label] = ServiceStateV2{
		Label:       label,
		Type:        serviceType,
		State:       state,
		Health:      health,
		LastUpdated: time.Now(),
		Error:       err,
	}
}

// GetState returns the state of a service
func (s *StateStoreV2) GetState(label string) (ServiceStateV2, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	state, exists := s.states[label]
	return state, exists
}

// GetAllStates returns all service states
func (s *StateStoreV2) GetAllStates() map[string]ServiceStateV2 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return a copy to prevent external modifications
	copy := make(map[string]ServiceStateV2)
	for k, v := range s.states {
		copy[k] = v
	}
	return copy
}

// GetStatesByType returns all services of a specific type
func (s *StateStoreV2) GetStatesByType(serviceType ServiceType) map[string]ServiceStateV2 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]ServiceStateV2)
	for label, state := range s.states {
		if state.Type == serviceType {
			result[label] = state
		}
	}
	return result
}

// RemoveState removes a service from the state store
func (s *StateStoreV2) RemoveState(label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.states, label)
}

// Clear removes all states
func (s *StateStoreV2) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.states = make(map[string]ServiceStateV2)
}

// HealthStatus represents the health status of a service
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthChecking  HealthStatus = "checking"
) 