package mcpserver

import (
	"context"
	"fmt"

	"envctl/internal/api"
)

// ServiceAdapter adapts the MCP service functionality to implement api.MCPServiceHandler
type ServiceAdapter struct{}

// NewServiceAdapter creates a new MCP service adapter
func NewServiceAdapter() *ServiceAdapter {
	return &ServiceAdapter{}
}

// Register registers the adapter with the API
func (a *ServiceAdapter) Register() {
	api.RegisterMCPServiceHandler(a)
}

// MCPServiceHandler implementation
func (a *ServiceAdapter) GetModelID() string {
	// This is a global handler, individual services implement this
	return ""
}

func (a *ServiceAdapter) GetProvider() string {
	return ""
}

func (a *ServiceAdapter) GetURL() string {
	return ""
}

func (a *ServiceAdapter) GetClusterLabel() string {
	return ""
}

func (a *ServiceAdapter) GetMCPTools() []api.MCPTool {
	return nil
}

func (a *ServiceAdapter) GetResources() []api.MCPResource {
	return nil
}

// ListServers lists all MCP servers
func (a *ServiceAdapter) ListServers(ctx context.Context) ([]*api.MCPServerInfo, error) {
	serviceAPI := api.GetMCPServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("MCP service API not available")
	}
	return serviceAPI.ListServers(ctx)
}

// GetServerInfo gets info about a specific MCP server
func (a *ServiceAdapter) GetServerInfo(ctx context.Context, label string) (*api.MCPServerInfo, error) {
	serviceAPI := api.GetMCPServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("MCP service API not available")
	}
	return serviceAPI.GetServerInfo(ctx, label)
}

// GetServerTools gets tools exposed by an MCP server
func (a *ServiceAdapter) GetServerTools(ctx context.Context, serverName string) ([]api.MCPTool, error) {
	serviceAPI := api.GetMCPServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("MCP service API not available")
	}
	return serviceAPI.GetTools(ctx, serverName)
}

// GetTools returns all tools this provider offers
func (a *ServiceAdapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		{
			Name:        "mcp_server_list",
			Description: "List all MCP servers",
		},
		{
			Name:        "mcp_server_info",
			Description: "Get detailed information about an MCP server",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "MCP server label",
				},
			},
		},
		{
			Name:        "mcp_server_tools",
			Description: "List tools exposed by an MCP server",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "server_name",
					Type:        "string",
					Required:    true,
					Description: "MCP server name",
				},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *ServiceAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "mcp_server_list":
		return a.handleMCPServerList(ctx)
	case "mcp_server_info":
		return a.handleMCPServerInfo(ctx, args)
	case "mcp_server_tools":
		return a.handleMCPServerTools(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (a *ServiceAdapter) handleMCPServerList(ctx context.Context) (*api.CallToolResult, error) {
	servers, err := a.ListServers(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to list MCP servers: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"servers": servers,
		"total":   len(servers),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleMCPServerInfo(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	info, err := a.GetServerInfo(ctx, label)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get MCP server info: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{info},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleMCPServerTools(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	serverName, ok := args["server_name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"server_name is required"},
			IsError: true,
		}, nil
	}

	tools, err := a.GetServerTools(ctx, serverName)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get MCP server tools: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"server": serverName,
		"tools":  tools,
		"total":  len(tools),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
} 