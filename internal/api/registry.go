package api

import "sync"

// Registry holds references to all APIs that services can use.
// This provides a centralized way for services to access APIs without
// creating circular dependencies or complex factory patterns.
type Registry struct {
	mu              sync.RWMutex
	orchestratorAPI OrchestratorAPI
	mcpServiceAPI   MCPServiceAPI
	portForwardAPI  PortForwardServiceAPI
	k8sServiceAPI   K8sServiceAPI
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

// SetMCPServiceAPI sets the MCP service API in the registry
func SetMCPServiceAPI(api MCPServiceAPI) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.mcpServiceAPI = api
}

// GetMCPServiceAPI gets the MCP service API from the registry
func GetMCPServiceAPI() MCPServiceAPI {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.mcpServiceAPI
}

// SetPortForwardServiceAPI sets the port forward service API in the registry
func SetPortForwardServiceAPI(api PortForwardServiceAPI) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.portForwardAPI = api
}

// GetPortForwardServiceAPI gets the port forward service API from the registry
func GetPortForwardServiceAPI() PortForwardServiceAPI {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.portForwardAPI
}

// SetK8sServiceAPI sets the K8s service API in the registry
func SetK8sServiceAPI(api K8sServiceAPI) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.k8sServiceAPI = api
}

// GetK8sServiceAPI gets the K8s service API from the registry
func GetK8sServiceAPI() K8sServiceAPI {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	return globalRegistry.k8sServiceAPI
}

// SetAll sets all APIs at once (convenience method for initialization)
func SetAll(orchestrator OrchestratorAPI, mcp MCPServiceAPI, pf PortForwardServiceAPI, k8s K8sServiceAPI) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.orchestratorAPI = orchestrator
	globalRegistry.mcpServiceAPI = mcp
	globalRegistry.portForwardAPI = pf
	globalRegistry.k8sServiceAPI = k8s
}

// Clear clears all APIs from the registry (useful for testing)
func Clear() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.orchestratorAPI = nil
	globalRegistry.mcpServiceAPI = nil
	globalRegistry.portForwardAPI = nil
	globalRegistry.k8sServiceAPI = nil
}
