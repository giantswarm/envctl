package api

import (
	"envctl/internal/aggregator"
	"envctl/internal/services"
)

// orchestratorEventAdapter adapts the OrchestratorAPI to the OrchestratorEventProvider interface
type orchestratorEventAdapter struct {
	api OrchestratorAPI
}

// NewOrchestratorEventAdapter creates a new adapter
func NewOrchestratorEventAdapter(api OrchestratorAPI) aggregator.OrchestratorEventProvider {
	return &orchestratorEventAdapter{api: api}
}

// SubscribeToStateChanges returns a channel for service state change events
func (a *orchestratorEventAdapter) SubscribeToStateChanges() <-chan aggregator.ServiceStateChangedEvent {
	// Create a channel to forward events
	eventChan := make(chan aggregator.ServiceStateChangedEvent, 100)

	// Subscribe to API events
	apiEvents := a.api.SubscribeToStateChanges()

	// Forward events in a goroutine
	go func() {
		for apiEvent := range apiEvents {
			// Convert API event to aggregator event
			aggEvent := aggregator.ServiceStateChangedEvent{
				Label:    apiEvent.Label,
				OldState: apiEvent.OldState,
				NewState: apiEvent.NewState,
				Health:   apiEvent.Health,
				Error:    apiEvent.Error,
			}

			select {
			case eventChan <- aggEvent:
				// Event forwarded successfully
			default:
				// Channel full, drop event
			}
		}
		close(eventChan)
	}()

	return eventChan
}

// mcpServiceAdapter adapts the MCPServiceAPI to the MCPServiceProvider interface
type mcpServiceAdapter struct {
	api      MCPServiceAPI
	registry services.ServiceRegistry
}

// NewMCPServiceAdapter creates a new adapter
func NewMCPServiceAdapter(api MCPServiceAPI, registry services.ServiceRegistry) aggregator.MCPServiceProvider {
	return &mcpServiceAdapter{
		api:      api,
		registry: registry,
	}
}

// GetAllMCPServices returns all MCP services
func (a *mcpServiceAdapter) GetAllMCPServices() []aggregator.MCPServiceInfo {
	// Get all services from registry
	allServices := a.registry.GetAll()

	var mcpServices []aggregator.MCPServiceInfo
	for _, service := range allServices {
		// Filter for MCP server services
		if service.GetType() == services.TypeMCPServer {
			mcpServices = append(mcpServices, aggregator.MCPServiceInfo{
				Name:   service.GetLabel(),
				State:  string(service.GetState()),
				Health: string(service.GetHealth()),
			})
		}
	}

	return mcpServices
}

// GetMCPClient returns the MCP client for a specific service
func (a *mcpServiceAdapter) GetMCPClient(name string) interface{} {
	// Get the service from registry
	service, exists := a.registry.Get(name)
	if !exists {
		return nil
	}

	// Check if it's an MCP server service
	if service.GetType() != services.TypeMCPServer {
		return nil
	}

	// Try to get the MCP client from the service using the provider interface
	type mcpClientProvider interface {
		GetMCPClient() interface{}
	}

	provider, ok := service.(mcpClientProvider)
	if !ok {
		return nil
	}

	return provider.GetMCPClient()
}
