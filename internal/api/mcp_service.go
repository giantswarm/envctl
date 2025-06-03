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

	// GetAllTools returns tools from all running MCP servers via the aggregator
	GetAllTools(ctx context.Context) ([]MCPTool, error)
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

	// Get the MCP server service
	service, exists := api.registry.Get(serverName)
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found", serverName)
	}

	if service.GetType() != services.TypeMCPServer {
		return nil, fmt.Errorf("service %s is not an MCP server", serverName)
	}

	if service.GetState() != services.StateRunning {
		return nil, fmt.Errorf("MCP server %s is not running (state: %s)", serverName, service.GetState())
	}

	// Try to get the MCP client from the service
	type mcpClientProvider interface {
		GetMCPClient() interface{}
	}

	provider, ok := service.(mcpClientProvider)
	if !ok {
		return nil, fmt.Errorf("MCP server %s does not provide client access", serverName)
	}

	mcpClient := provider.GetMCPClient()
	if mcpClient == nil {
		return nil, fmt.Errorf("MCP server %s has no client available", serverName)
	}

	// Use the client to list tools
	type toolLister interface {
		ListTools(ctx context.Context) ([]mcp.Tool, error)
	}

	lister, ok := mcpClient.(toolLister)
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

// GetAllTools returns tools from all running MCP servers via the aggregator
func (api *mcpServiceAPI) GetAllTools(ctx context.Context) ([]MCPTool, error) {
	// Look for the aggregator service
	aggregatorServices := api.registry.GetByType(services.ServiceType("Aggregator"))
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]
	if aggregator.GetState() != services.StateRunning {
		return nil, fmt.Errorf("MCP aggregator is not running")
	}

	// Get the aggregator port from service data
	var port int
	if provider, ok := aggregator.(services.ServiceDataProvider); ok {
		data := provider.GetServiceData()
		if p, ok := data["port"].(int); ok {
			port = p
		}
	}

	if port == 0 {
		return nil, fmt.Errorf("MCP aggregator has no port configured")
	}

	// Connect to the aggregator
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
	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "envctl-api",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP protocol: %w", err)
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
