package api

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPServerInfo contains information about an MCP server
type MCPServerInfo struct {
	Label   string `json:"label"`
	Name    string `json:"name"`
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

	// GetTools returns the list of tools exposed by an MCP server
	GetTools(ctx context.Context, serverName string) ([]MCPTool, error)
}

// mcpServiceAPI implements MCPServiceAPI
type mcpServiceAPI struct {
	// No fields - uses handlers from registry
}

// NewMCPServiceAPI creates a new MCP service API
func NewMCPServiceAPI() MCPServiceAPI {
	return &mcpServiceAPI{}
}

// GetServerInfo returns information about a specific MCP server
func (api *mcpServiceAPI) GetServerInfo(ctx context.Context, label string) (*MCPServerInfo, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	service, exists := registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found", label)
	}

	if service.GetType() != TypeMCPServer {
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
	if data := service.GetServiceData(); data != nil {
		if name, ok := data["name"].(string); ok {
			info.Name = name
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
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	services := registry.GetByType(TypeMCPServer)

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

// GetTools returns the list of tools exposed by an MCP server
func (api *mcpServiceAPI) GetTools(ctx context.Context, serverName string) ([]MCPTool, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	// Get the MCP server service
	service, exists := registry.Get(serverName)
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found", serverName)
	}

	if service.GetType() != TypeMCPServer {
		return nil, fmt.Errorf("service %s is not an MCP server", serverName)
	}

	if service.GetState() != StateRunning {
		return nil, fmt.Errorf("MCP server %s is not running (state: %s)", serverName, service.GetState())
	}

	// Get the MCP-specific handler for this service
	mcpHandler, ok := GetMCPService(serverName)
	if !ok {
		return nil, fmt.Errorf("MCP server %s does not have a registered handler", serverName)
	}

	// Get tools from the handler
	tools := mcpHandler.GetMCPTools()

	return tools, nil
}

// GetToolsFromClient is a helper function that can be used by MCP handlers to convert tools
func GetToolsFromClient(ctx context.Context, client interface{}) ([]MCPTool, error) {
	// Use the client to list tools
	type toolLister interface {
		ListTools(ctx context.Context) ([]mcp.Tool, error)
	}

	lister, ok := client.(toolLister)
	if !ok {
		return nil, fmt.Errorf("MCP client does not support listing tools")
	}

	// List available tools
	tools, err := lister.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Convert to our MCPTool type
	result := make([]MCPTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	return result, nil
}
