package api

import (
	"context"
	"envctl/internal/services"
	"fmt"
)

// MCPServerInfo contains information about an MCP server
type MCPServerInfo struct {
	Label   string `json:"label"`
	Name    string `json:"name"`
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	State   string `json:"state"`
	Health  string `json:"health"`
	Icon    string `json:"icon"`
	Enabled bool   `json:"enabled"`
	Error   string `json:"error,omitempty"`
}

// MCPServiceAPI provides access to MCP server information
type MCPServiceAPI interface {
	// GetServerInfo returns information about a specific MCP server
	GetServerInfo(ctx context.Context, label string) (*MCPServerInfo, error)

	// ListServers returns information about all MCP servers
	ListServers(ctx context.Context) ([]*MCPServerInfo, error)
}

// mcpServiceAPI implements MCPServiceAPI
type mcpServiceAPI struct {
	registry services.ServiceRegistry
}

// NewMCPServiceAPI creates a new MCP service API
func NewMCPServiceAPI(registry services.ServiceRegistry) MCPServiceAPI {
	return &mcpServiceAPI{
		registry: registry,
	}
}

// GetServerInfo returns information about a specific MCP server
func (api *mcpServiceAPI) GetServerInfo(ctx context.Context, label string) (*MCPServerInfo, error) {
	service, exists := api.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found", label)
	}

	if service.GetType() != services.TypeMCPServer {
		return nil, fmt.Errorf("service %s is not an MCP server", label)
	}

	info := &MCPServerInfo{
		Label:  service.GetLabel(),
		State:  string(service.GetState()),
		Health: string(service.GetHealth()),
	}

	// Get error if any
	if err := service.GetLastError(); err != nil {
		info.Error = err.Error()
	}

	// Get service-specific data if available
	if provider, ok := service.(services.ServiceDataProvider); ok {
		data := provider.GetServiceData()

		if name, ok := data["name"].(string); ok {
			info.Name = name
		}
		if port, ok := data["port"].(int); ok {
			info.Port = port
		}
		if pid, ok := data["pid"].(int); ok {
			info.PID = pid
		}
		if icon, ok := data["icon"].(string); ok {
			info.Icon = icon
		}
		if enabled, ok := data["enabled"].(bool); ok {
			info.Enabled = enabled
		}
	}

	return info, nil
}

// ListServers returns information about all MCP servers
func (api *mcpServiceAPI) ListServers(ctx context.Context) ([]*MCPServerInfo, error) {
	services := api.registry.GetByType(services.TypeMCPServer)

	servers := make([]*MCPServerInfo, 0, len(services))
	for _, service := range services {
		info, err := api.GetServerInfo(ctx, service.GetLabel())
		if err != nil {
			// Log error but continue with other servers
			continue
		}
		servers = append(servers, info)
	}

	return servers, nil
}
