// Package adapters provides adapter implementations that bridge between different
// interfaces in the envctl application.
//
// # Overview
//
// This package contains adapter patterns that allow components with incompatible
// interfaces to work together. The adapters translate between the API layer
// interfaces and the service-specific interfaces required by various components.
//
// # Adapters
//
// Currently, the package provides two main adapters:
//
//  1. **OrchestratorEventAdapter**: Adapts the OrchestratorAPI to the
//     OrchestratorEventProvider interface required by the aggregator service.
//     It subscribes to orchestrator state change events and converts them to
//     the format expected by the aggregator.
//
//  2. **MCPServiceAdapter**: Adapts the MCPServiceAPI to the MCPServiceProvider
//     interface. It provides access to MCP service information and MCP client
//     instances through the service registry.
//
// # Design Pattern
//
// The adapters follow the classic Adapter design pattern:
//
//	// Target interface (what the client expects)
//	type OrchestratorEventProvider interface {
//	    SubscribeToStateChanges() <-chan ServiceStateChangedEvent
//	}
//
//	// Adaptee (what we have)
//	type OrchestratorAPI interface {
//	    SubscribeToStateChanges() <-chan APIStateChangedEvent
//	}
//
//	// Adapter (bridges the gap)
//	type OrchestratorEventAdapter struct {
//	    api OrchestratorAPI
//	}
//
// # Benefits
//
// - **Decoupling**: Services don't need to know about API layer details
// - **Testability**: Adapters can be easily mocked for testing
// - **Flexibility**: Easy to change implementations without affecting clients
// - **Single Responsibility**: Each adapter has one clear purpose
//
// # Usage
//
// Adapters are typically created during service initialization:
//
//	orchestratorAdapter := adapters.NewOrchestratorEventAdapter(orchestratorAPI)
//	mcpAdapter := adapters.NewMCPServiceAdapter(mcpAPI, registry)
//
//	aggregator := NewAggregatorService(config, orchestratorAdapter, mcpAdapter)
package adapters
