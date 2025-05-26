package state

import (
	"sync"
	"time"
)

// K8sConnectionState represents the state of a K8s connection
type K8sConnectionState struct {
	ContextName      string
	IsHealthy        bool
	Error            error
	LastHealthCheck  time.Time
	ReadyNodes       int
	TotalNodes       int
	ClusterShortName string
	IsMC             bool
}

// K8sStateManager manages the state of K8s connections
type K8sStateManager interface {
	UpdateConnectionState(contextName string, state K8sConnectionState)
	GetConnectionState(contextName string) K8sConnectionState
	GetAllConnectionStates() map[string]K8sConnectionState
}

// k8sStateManager is the concrete implementation
type k8sStateManager struct {
	states map[string]K8sConnectionState
	mu     sync.RWMutex
}

// NewK8sStateManager creates a new K8s state manager
func NewK8sStateManager() K8sStateManager {
	return &k8sStateManager{
		states: make(map[string]K8sConnectionState),
	}
}

// UpdateConnectionState updates the state of a K8s connection
func (m *k8sStateManager) UpdateConnectionState(contextName string, state K8sConnectionState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[contextName] = state
}

// GetConnectionState retrieves the state of a K8s connection
func (m *k8sStateManager) GetConnectionState(contextName string) K8sConnectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[contextName]
}

// GetAllConnectionStates returns all connection states
func (m *k8sStateManager) GetAllConnectionStates() map[string]K8sConnectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	copy := make(map[string]K8sConnectionState)
	for k, v := range m.states {
		copy[k] = v
	}
	return copy
}
