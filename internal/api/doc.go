// Package api provides public interfaces for interacting with envctl services.
//
// This package contains the API layer that exposes controlled access to service
// functionality without exposing internal implementation details. It serves as
// the primary interface for external consumers like the TUI, HTTP endpoints,
// or other components that need to interact with services.
//
// Architecture:
//
// The API package follows a handler-based architecture that maintains clean
// separation between interfaces and implementations:
//
//  1. **API Interfaces** - Define the contracts for each service type
//     (OrchestratorAPI, MCPServiceAPI, etc.)
//
//  2. **Handler Interfaces** - Define what capabilities the implementations
//     must provide (ServiceRegistryHandler, OrchestratorHandler, etc.)
//
//  3. **Handler Registry** - Manages registration of handler implementations
//     at runtime
//
//  4. **API Implementations** - Thin wrappers that delegate to registered
//     handlers
//
// This design ensures:
// - No circular dependencies (API doesn't import internal packages)
// - Clean separation of concerns
// - Easy testing through handler mocking
// - Runtime flexibility in handler registration
//
// Service Types:
//
//   - OrchestratorAPI: Manages service lifecycle and cluster switching
//   - MCPServiceAPI: Provides MCP server information and tool access
//   - PortForwardServiceAPI: Manages kubectl port-forward tunnels
//   - K8sServiceAPI: Handles Kubernetes cluster connections
//   - AggregatorAPI: Aggregates MCP servers into a single endpoint
//
// Example Usage:
//
//	// At startup, services register their handlers
//	registryAdapter := services.NewRegistryAdapter(registry)
//	registryAdapter.Register()
//
//	orchAdapter := orchestrator.NewAPIAdapter(orch)
//	orchAdapter.Register()
//
//	// Create API instances
//	orchestratorAPI := api.NewOrchestratorAPI()
//	mcpAPI := api.NewMCPServiceAPI()
//
//	// Use the APIs
//	err := orchestratorAPI.StartService("my-service")
//	status, err := orchestratorAPI.GetServiceStatus("my-service")
//
// Handler Registration:
//
// Services must register their handlers before APIs can be used:
//
//	// Service adapters implement handler interfaces
//	type RegistryAdapter struct {
//	    registry ServiceRegistry
//	}
//
//	func (r *RegistryAdapter) Get(label string) (api.ServiceInfo, bool) {
//	    // Implementation
//	}
//
//	func (r *RegistryAdapter) Register() {
//	    api.RegisterServiceRegistry(r)
//	}
//
// Testing:
//
// APIs can be easily tested by registering mock handlers:
//
//	mockRegistry := &mockServiceRegistryHandler{
//	    services: make(map[string]ServiceInfo),
//	}
//	api.RegisterServiceRegistry(mockRegistry)
//	defer api.RegisterServiceRegistry(nil)
//
//	// Test API calls
//	api := api.NewOrchestratorAPI()
//	status, err := api.GetServiceStatus("test")
//
// Thread Safety:
//
// All API methods are thread-safe and can be called concurrently from
// multiple goroutines. The handler registry uses mutex protection for
// safe concurrent access.
//
// The api package defines the public API for envctl's runtime services.
//
// These APIs are exposed as MCP tools through the aggregator and provide programmatic
// access to envctl's functionality.
//
// API Components:
//
// 1. OrchestratorAPI - Service lifecycle and cluster management
// 2. MCPServiceAPI - MCP server information and tools
// 3. K8sServiceAPI - Kubernetes connection management
// 4. PortForwardServiceAPI - Port forwarding management
// 5. ConfigServiceAPI - Configuration management
//
// # ConfigServiceAPI
//
// The ConfigServiceAPI provides runtime access to envctl's configuration. This API allows:
//
// - Reading the entire configuration or specific sections
// - Updating configuration sections dynamically
// - Deleting configuration items
// - Persisting configuration changes to disk
//
// Configuration updates made through this API affect the running system immediately
// for most settings. Some changes may require service restarts to take full effect.
//
// # Service Architecture
package api
