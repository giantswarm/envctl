package k8smanager

import (
	"context"
	"sync"
	"time"
)

// K8sConnectionState represents the state of a kubernetes connection
type K8sConnectionState struct {
	Context         string
	IsAuthenticated bool
	IsHealthy       bool
	LastHealthCheck time.Time
	LastLoginTime   time.Time
	Error           error
}

// K8sStateManager manages the state of kubernetes connections
type K8sStateManager interface {
	GetConnectionState(context string) *K8sConnectionState
	UpdateConnectionState(context string, state K8sConnectionState)
	SetAuthenticated(context string, authenticated bool)
	SetHealthy(context string, healthy bool, err error)
	StartHealthMonitor(context string, interval time.Duration) func()
	IsConnectionHealthy(context string) bool
}

// defaultK8sStateManager is the default implementation
type defaultK8sStateManager struct {
	states       map[string]*K8sConnectionState
	mu           sync.RWMutex
	healthChecks map[string]context.CancelFunc
	kubeMgr      KubeManagerAPI
}

// NewK8sStateManager creates a new K8sStateManager
func NewK8sStateManager(kubeMgr KubeManagerAPI) K8sStateManager {
	return &defaultK8sStateManager{
		states:       make(map[string]*K8sConnectionState),
		healthChecks: make(map[string]context.CancelFunc),
		kubeMgr:      kubeMgr,
	}
}

// GetConnectionState returns the state for a given context
func (m *defaultK8sStateManager) GetConnectionState(context string) *K8sConnectionState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[context]
	if !exists {
		return &K8sConnectionState{
			Context:         context,
			IsAuthenticated: false,
			IsHealthy:       false,
		}
	}

	// Return a copy to prevent external modification
	stateCopy := *state
	return &stateCopy
}

// UpdateConnectionState updates the entire state for a context
func (m *defaultK8sStateManager) UpdateConnectionState(context string, state K8sConnectionState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state.Context = context // Ensure context matches
	m.states[context] = &state
}

// SetAuthenticated updates just the authentication status
func (m *defaultK8sStateManager) SetAuthenticated(context string, authenticated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[context]
	if !exists {
		state = &K8sConnectionState{Context: context}
		m.states[context] = state
	}

	state.IsAuthenticated = authenticated
	if authenticated {
		state.LastLoginTime = time.Now()
		state.Error = nil
	}
}

// SetHealthy updates the health status
func (m *defaultK8sStateManager) SetHealthy(context string, healthy bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[context]
	if !exists {
		state = &K8sConnectionState{Context: context}
		m.states[context] = state
	}

	state.IsHealthy = healthy
	state.LastHealthCheck = time.Now()
	state.Error = err
}

// IsConnectionHealthy checks if a connection is both authenticated and healthy
func (m *defaultK8sStateManager) IsConnectionHealthy(contextName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[contextName]
	if !exists {
		return false
	}

	return state.IsAuthenticated && state.IsHealthy
}

// StartHealthMonitor starts monitoring health for a given context
func (m *defaultK8sStateManager) StartHealthMonitor(contextName string, interval time.Duration) func() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop any existing monitor
	if cancel, exists := m.healthChecks[contextName]; exists {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.healthChecks[contextName] = cancel

	// Start the monitor goroutine
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Perform health check
				health, err := m.kubeMgr.GetClusterNodeHealth(ctx, contextName)
				healthy := err == nil && health.Error == nil
				m.SetHealthy(contextName, healthy, err)
			}
		}
	}()

	// Return a stop function
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if c, exists := m.healthChecks[contextName]; exists {
			c()
			delete(m.healthChecks, contextName)
		}
	}
}
