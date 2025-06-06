package adapters

import (
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/services"
)

// MCPServiceAdapter adapts the MCPServiceAPI to the MCPServiceProvider interface
type MCPServiceAdapter struct {
	api      api.MCPServiceAPI
	registry services.ServiceRegistry
}

// NewMCPServiceAdapter creates a new MCP service adapter
func NewMCPServiceAdapter(api api.MCPServiceAPI, registry services.ServiceRegistry) *MCPServiceAdapter {
	return &MCPServiceAdapter{
		api:      api,
		registry: registry,
	}
}

// GetAllMCPServices returns all MCP services
func (a *MCPServiceAdapter) GetAllMCPServices() []aggregator.MCPServiceInfo {
	// Get all services from registry
	allServices := a.registry.GetAll()

	var mcpServices []aggregator.MCPServiceInfo
	for _, service := range allServices {
		// Filter for MCP server services
		if service.GetType() == services.TypeMCPServer {
			info := aggregator.MCPServiceInfo{
				Name:   service.GetLabel(),
				State:  string(service.GetState()),
				Health: string(service.GetHealth()),
			}

			// Get tool prefix from service data if available
			if provider, ok := service.(services.ServiceDataProvider); ok {
				if data := provider.GetServiceData(); data != nil {
					if toolPrefix, ok := data["toolPrefix"].(string); ok {
						info.ToolPrefix = toolPrefix
					}
				}
			}

			mcpServices = append(mcpServices, info)
		}
	}

	return mcpServices
}

// GetMCPClient returns the MCP client for a specific service
func (a *MCPServiceAdapter) GetMCPClient(name string) interface{} {
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
