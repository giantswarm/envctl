package api

import (
	"context"
	"envctl/internal/services"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
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

	// GetTools returns the list of tools exposed by an MCP server
	GetTools(ctx context.Context, serverName string) ([]MCPTool, error)
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

// GetTools returns the list of tools exposed by an MCP server
func (api *mcpServiceAPI) GetTools(ctx context.Context, serverName string) ([]MCPTool, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get server info to find the port
	serverInfo, err := api.GetServerInfo(ctx, serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	if serverInfo.State != "Running" {
		return nil, fmt.Errorf("MCP server %s is not running (state: %s)", serverName, serverInfo.State)
	}

	if serverInfo.Port == 0 {
		return nil, fmt.Errorf("MCP server %s has no port configured", serverName)
	}

	// Create MCP client using the library
	port := serverInfo.Port
	mcpClient, err := client.NewSSEMCPClient(fmt.Sprintf("http://localhost:%d/sse", port))
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}
	defer mcpClient.Close()

	// Start the SSE transport
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	// Initialize the MCP protocol
	initResult, err := mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "envctl",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP protocol: %w", err)
	}

	// Check if the server supports tools
	if initResult.Capabilities.Tools == nil {
		return []MCPTool{}, nil // No tools available
	}

	// List available tools
	toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Convert to our MCPTool type
	tools := make([]MCPTool, 0, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		tools = append(tools, MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	return tools, nil
}
