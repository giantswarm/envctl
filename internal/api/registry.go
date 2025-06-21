package api

import "sync"

// Registry holds references to all APIs that services can use.
// This provides a centralized way for services to access APIs without
// creating circular dependencies or complex factory patterns.
type Registry struct {
	mu              sync.RWMutex
	orchestratorAPI OrchestratorAPI
}

// Global registry instance
var globalRegistry = &Registry{}

// SetOrchestratorAPI sets the orchestrator API in the registry
func SetOrchestratorAPI(api OrchestratorAPI) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.orchestratorAPI = api
}

// GetOrchestratorAPI gets the orchestrator API from the registry
func GetOrchestratorAPI() OrchestratorAPI {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.orchestratorAPI
}

// SetAll sets all APIs at once (convenience method for initialization)
func SetAll(orchestrator OrchestratorAPI) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.orchestratorAPI = orchestrator
}

// Clear clears all APIs from the registry (useful for testing)
func Clear() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.orchestratorAPI = nil
}
