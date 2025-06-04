package api

import (
	"context"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// AggregatorInfo contains statistics about the MCP aggregator
type AggregatorInfo struct {
	Endpoint         string `json:"endpoint"`
	Port             int    `json:"port"`
	Host             string `json:"host"`
	State            string `json:"state"`
	Health           string `json:"health"`
	ServersTotal     int    `json:"servers_total"`
	ServersConnected int    `json:"servers_connected"`
	ToolsCount       int    `json:"tools_count"`
	ResourcesCount   int    `json:"resources_count"`
	PromptsCount     int    `json:"prompts_count"`
}

// AggregatorAPI provides access to MCP aggregator information
type AggregatorAPI interface {
	// GetAggregatorInfo returns comprehensive aggregator statistics
	GetAggregatorInfo(ctx context.Context) (*AggregatorInfo, error)

	// GetAllTools returns tools from all connected MCP servers via the aggregator
	GetAllTools(ctx context.Context) ([]MCPTool, error)

	// GetAllResources returns resources from all connected MCP servers
	GetAllResources(ctx context.Context) ([]MCPResource, error)
}

// aggregatorAPI implements AggregatorAPI
type aggregatorAPI struct {
	registry services.ServiceRegistry
}

// NewAggregatorAPI creates a new aggregator API
func NewAggregatorAPI(registry services.ServiceRegistry) AggregatorAPI {
	return &aggregatorAPI{
		registry: registry,
	}
}

// GetAggregatorInfo returns comprehensive aggregator statistics
func (api *aggregatorAPI) GetAggregatorInfo(ctx context.Context) (*AggregatorInfo, error) {
	// Look for the aggregator service
	aggregatorServices := api.registry.GetByType(services.ServiceType("Aggregator"))
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]

	info := &AggregatorInfo{
		State:  string(aggregator.GetState()),
		Health: string(aggregator.GetHealth()),
	}

	// Get service-specific data if available
	if provider, ok := aggregator.(services.ServiceDataProvider); ok {
		data := provider.GetServiceData()

		// Extract all the statistics from service data
		if endpoint, ok := data["endpoint"].(string); ok {
			info.Endpoint = endpoint
		}
		if port, ok := data["port"].(int); ok {
			info.Port = port
		}
		if host, ok := data["host"].(string); ok {
			info.Host = host
		}
		if serversTotal, ok := data["servers_total"].(int); ok {
			info.ServersTotal = serversTotal
		}
		if serversConnected, ok := data["servers_connected"].(int); ok {
			info.ServersConnected = serversConnected
		}
		if tools, ok := data["tools"].(int); ok {
			info.ToolsCount = tools
		}
		if resources, ok := data["resources"].(int); ok {
			info.ResourcesCount = resources
		}
		if prompts, ok := data["prompts"].(int); ok {
			info.PromptsCount = prompts
		}

		// Debug logging
		logging.Debug("AggregatorAPI", "GetAggregatorInfo returning: %d tools, %d resources, %d prompts (servers: %d/%d)",
			info.ToolsCount, info.ResourcesCount, info.PromptsCount, info.ServersConnected, info.ServersTotal)
	}

	return info, nil
}

// GetAllTools returns tools from all running MCP servers via the aggregator
func (api *aggregatorAPI) GetAllTools(ctx context.Context) ([]MCPTool, error) {
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

// GetAllResources returns resources from all connected MCP servers
func (api *aggregatorAPI) GetAllResources(ctx context.Context) ([]MCPResource, error) {
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

	// Connect to the aggregator with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

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

	// List available resources
	resourcesResult, err := mcpClient.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Convert to our MCPResource type
	resources := make([]MCPResource, 0, len(resourcesResult.Resources))
	for _, resource := range resourcesResult.Resources {
		resources = append(resources, MCPResource{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MimeType:    resource.MIMEType,
		})
	}

	return resources, nil
}
