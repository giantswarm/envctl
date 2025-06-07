package api

import (
	"context"
	"envctl/pkg/logging"
	"fmt"
	"strings"
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
	BlockedTools     int    `json:"blocked_tools"`
	YoloMode         bool   `json:"yolo_mode"`
}

// ToolWithStatus represents a tool with its blocked status
type ToolWithStatus struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Blocked     bool   `json:"blocked"`
}

// MCPPrompt represents a prompt from an MCP server
type MCPPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AggregatorAPI provides access to MCP aggregator information
type AggregatorAPI interface {
	// GetAggregatorInfo returns comprehensive aggregator statistics
	GetAggregatorInfo(ctx context.Context) (*AggregatorInfo, error)

	// GetAllTools returns tools from all connected MCP servers via the aggregator
	GetAllTools(ctx context.Context) ([]MCPTool, error)

	// GetAllToolsWithStatus returns tools with their blocked status
	GetAllToolsWithStatus(ctx context.Context) ([]ToolWithStatus, error)

	// GetAllResources returns resources from all connected MCP servers
	GetAllResources(ctx context.Context) ([]MCPResource, error)

	// GetAllPrompts returns prompts from all connected MCP servers
	GetAllPrompts(ctx context.Context) ([]MCPPrompt, error)
}

// aggregatorAPI implements AggregatorAPI
type aggregatorAPI struct {
	// No fields - uses handlers from registry
}

// NewAggregatorAPI creates a new aggregator API
func NewAggregatorAPI() AggregatorAPI {
	return &aggregatorAPI{}
}

// GetAggregatorInfo returns comprehensive aggregator statistics
func (api *aggregatorAPI) GetAggregatorInfo(ctx context.Context) (*AggregatorInfo, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	// Look for the aggregator service
	aggregatorServices := registry.GetByType(TypeAggregator)
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]

	info := &AggregatorInfo{
		State:  string(aggregator.GetState()),
		Health: string(aggregator.GetHealth()),
	}

	// Get service-specific data if available
	if data := aggregator.GetServiceData(); data != nil {
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
		if blockedTools, ok := data["blocked_tools"].(int); ok {
			info.BlockedTools = blockedTools
		}
		if yolo, ok := data["yolo"].(bool); ok {
			info.YoloMode = yolo
		}

		// Debug logging
		logging.Debug("AggregatorAPI", "GetAggregatorInfo returning: %d tools (%d blocked), %d resources, %d prompts (servers: %d/%d)",
			info.ToolsCount, info.BlockedTools, info.ResourcesCount, info.PromptsCount, info.ServersConnected, info.ServersTotal)
	}

	return info, nil
}

// GetAllTools returns tools from all running MCP servers via the aggregator
func (api *aggregatorAPI) GetAllTools(ctx context.Context) ([]MCPTool, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	// Look for the aggregator service
	aggregatorServices := registry.GetByType(TypeAggregator)
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]
	if aggregator.GetState() != StateRunning {
		return nil, fmt.Errorf("MCP aggregator is not running")
	}

	// Get the aggregator port from service data
	var port int
	if data := aggregator.GetServiceData(); data != nil {
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

// GetAllToolsWithStatus returns tools with their blocked status
func (api *aggregatorAPI) GetAllToolsWithStatus(ctx context.Context) ([]ToolWithStatus, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	// Look for the aggregator service
	aggregatorServices := registry.GetByType(TypeAggregator)
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]

	// Get service-specific data if available
	if data := aggregator.GetServiceData(); data != nil {
		// Extract tools with status
		// Note: For now we'll skip trying to extract from service data
		// due to type conversion complexity across package boundaries
		_ = data["tools_with_status"]
	}

	// For now, fall back to regular tools without status
	// This is a simpler approach until we have a proper way to pass structured data
	tools, err := api.GetAllTools(ctx)
	if err != nil {
		return nil, err
	}

	// Get yolo mode from aggregator info
	yoloMode := false
	if info, err := api.GetAggregatorInfo(ctx); err == nil {
		yoloMode = info.YoloMode
	}

	result := make([]ToolWithStatus, 0, len(tools))
	for _, t := range tools {
		// For now, we'll mark tools as blocked based on their names
		// This is a temporary solution until we have better integration
		blocked := false
		if !yoloMode {
			// Check if tool name matches destructive patterns
			// This is a simplified check - ideally we'd get this from the aggregator
			destructivePatterns := []string{
				"apply", "create", "delete", "patch", "rollout", "scale",
				"install", "uninstall", "upgrade", "cleanup", "reconcile",
				"resume", "suspend", "move", "pause", "remediate", "update",
			}
			for _, pattern := range destructivePatterns {
				if strings.Contains(strings.ToLower(t.Name), pattern) {
					blocked = true
					break
				}
			}
		}

		result = append(result, ToolWithStatus{
			Name:        t.Name,
			Description: t.Description,
			Blocked:     blocked,
		})
	}

	return result, nil
}

// GetAllResources returns resources from all connected MCP servers
func (api *aggregatorAPI) GetAllResources(ctx context.Context) ([]MCPResource, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	// Look for the aggregator service
	aggregatorServices := registry.GetByType(TypeAggregator)
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]
	if aggregator.GetState() != StateRunning {
		return nil, fmt.Errorf("MCP aggregator is not running")
	}

	// Get the aggregator port from service data
	var port int
	if data := aggregator.GetServiceData(); data != nil {
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

// GetAllPrompts returns prompts from all connected MCP servers
func (api *aggregatorAPI) GetAllPrompts(ctx context.Context) ([]MCPPrompt, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	// Look for the aggregator service
	aggregatorServices := registry.GetByType(TypeAggregator)
	if len(aggregatorServices) == 0 {
		return nil, fmt.Errorf("no MCP aggregator found")
	}

	aggregator := aggregatorServices[0]
	if aggregator.GetState() != StateRunning {
		return nil, fmt.Errorf("MCP aggregator is not running")
	}

	// Get the aggregator port from service data
	var port int
	if data := aggregator.GetServiceData(); data != nil {
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

	// List available prompts
	promptsResult, err := mcpClient.ListPrompts(ctx, mcp.ListPromptsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	// Convert to our MCPPrompt type
	prompts := make([]MCPPrompt, 0, len(promptsResult.Prompts))
	for _, prompt := range promptsResult.Prompts {
		prompts = append(prompts, MCPPrompt{
			Name:        prompt.Name,
			Description: prompt.Description,
		})
	}

	return prompts, nil
}
