package api

import (
	"envctl/internal/aggregator"
	"envctl/internal/services"
	"fmt"
)

// mcpClientProvider provides MCP clients from the service registry
type mcpClientProvider struct {
	registry services.ServiceRegistry
}

// NewMCPClientProvider creates a new MCP client provider
func NewMCPClientProvider(registry services.ServiceRegistry) aggregator.MCPClientProvider {
	return &mcpClientProvider{
		registry: registry,
	}
}

// GetMCPClient returns the MCP client for a specific service
func (p *mcpClientProvider) GetMCPClient(label string) (aggregator.MCPClient, error) {
	service, exists := p.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("service %s not found", label)
	}

	if service.GetType() != services.TypeMCPServer {
		return nil, fmt.Errorf("service %s is not an MCP server", label)
	}

	if service.GetState() != services.StateRunning {
		return nil, fmt.Errorf("service %s is not running", label)
	}

	// Use interface to get the client
	type mcpClientGetter interface {
		GetMCPClient() interface{}
	}

	getter, ok := service.(mcpClientGetter)
	if !ok {
		return nil, fmt.Errorf("service does not provide MCP client access")
	}

	clientInterface := getter.GetMCPClient()
	if clientInterface == nil {
		return nil, fmt.Errorf("MCP client not available for %s", label)
	}

	// Type assert to MCPClient
	client, ok := clientInterface.(aggregator.MCPClient)
	if !ok {
		return nil, fmt.Errorf("client is not an MCP client")
	}

	return client, nil
}

// GetAllMCPClients returns all available MCP clients
func (p *mcpClientProvider) GetAllMCPClients() map[string]aggregator.MCPClient {
	clients := make(map[string]aggregator.MCPClient)

	// Get all MCP server services from registry
	mcpServers := p.registry.GetByType(services.TypeMCPServer)

	for _, svc := range mcpServers {
		if svc.GetState() == services.StateRunning {
			if client, err := p.GetMCPClient(svc.GetLabel()); err == nil {
				clients[svc.GetLabel()] = client
			}
		}
	}

	return clients
}
