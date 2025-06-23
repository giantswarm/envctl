package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// mcpTestClient implements the MCPTestClient interface
type mcpTestClient struct {
	client   client.MCPClient
	endpoint string
	debug    bool
	logger   TestLogger
}

// NewMCPTestClient creates a new MCP test client
func NewMCPTestClient(debug bool) MCPTestClient {
	return &mcpTestClient{
		debug:  debug,
		logger: NewStdoutLogger(false, debug), // Default to stdout logger
	}
}

// NewMCPTestClientWithLogger creates a new MCP test client with custom logger
func NewMCPTestClientWithLogger(debug bool, logger TestLogger) MCPTestClient {
	return &mcpTestClient{
		debug:  debug,
		logger: logger,
	}
}

// Connect establishes connection to the MCP aggregator
func (c *mcpTestClient) Connect(ctx context.Context, endpoint string) error {
	c.endpoint = endpoint

	if c.debug {
		c.logger.Debug("🔗 Connecting to MCP aggregator at %s\n", endpoint)
	}

	// Create streamable HTTP client for envctl aggregator
	httpClient, err := client.NewStreamableHttpClient(endpoint)
	if err != nil {
		return fmt.Errorf("failed to create streamable HTTP client: %w", err)
	}

	// Start the streamable HTTP transport
	if err := httpClient.Start(ctx); err != nil {
		httpClient.Close() // Clean up failed client
		return fmt.Errorf("failed to start streamable HTTP client: %w", err)
	}

	// Initialize the MCP protocol
	initRequest := mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "envctl-test-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	// Initialize with timeout
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// CRITICAL: Only store the client AFTER successful initialization
	_, err = httpClient.Initialize(initCtx, initRequest)
	if err != nil {
		httpClient.Close() // Clean up failed client
		return fmt.Errorf("failed to initialize MCP protocol: %w", err)
	}

	// SUCCESS: Store the client only after full initialization
	c.client = httpClient

	if c.debug {
		c.logger.Debug("✅ Successfully connected to MCP aggregator at %s\n", endpoint)
	}

	return nil
}

// CallTool invokes an MCP tool with the given parameters
func (c *mcpTestClient) CallTool(ctx context.Context, toolName string, parameters map[string]interface{}) (interface{}, error) {
	if c.client == nil {
		return nil, fmt.Errorf("MCP client not connected")
	}

	// Convert parameters to the format expected by the MCP client
	var args interface{}
	if parameters != nil {
		args = parameters
	}

	if c.debug {
		argsJSON, _ := json.MarshalIndent(parameters, "", "  ")
		c.logger.Debug("🔧 Calling tool: %s\n", toolName)
		c.logger.Debug("📋 Parameters: %s\n", string(argsJSON))
	}

	// Create timeout context for the tool call
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// First, try the tool name as provided
	resolvedToolName, err := c.resolveToolName(callCtx, toolName)
	if err != nil {
		if c.debug {
			c.logger.Debug("❌ Tool resolution failed: %v\n", err)
		}
		return nil, fmt.Errorf("tool call %s failed: %w", toolName, err)
	}

	if c.debug && resolvedToolName != toolName {
		c.logger.Debug("🔄 Resolved tool name: %s -> %s\n", toolName, resolvedToolName)
	}

	// Create the request using the pattern from the existing codebase
	request := mcp.CallToolRequest{
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name:      resolvedToolName,
			Arguments: args,
		},
	}

	// Make the tool call
	result, err := c.client.CallTool(callCtx, request)
	if err != nil {
		if c.debug {
			c.logger.Debug("❌ Tool call failed: %v\n", err)
		}
		return nil, fmt.Errorf("tool call %s failed: %w", toolName, err)
	}

	if c.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		c.logger.Debug("✅ Tool call result: %s\n", string(resultJSON))
	}

	return result, nil
}

// resolveToolName attempts to resolve a tool name to its actual prefixed name in the aggregator
func (c *mcpTestClient) resolveToolName(ctx context.Context, toolName string) (string, error) {
	// First, try to get the list of available tools
	availableTools, err := c.ListTools(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list available tools: %w", err)
	}

	if c.debug {
		c.logger.Debug("🔍 Resolving tool name '%s' from %d available tools\n", toolName, len(availableTools))
		c.logger.Debug("🛠️  Available tools: %v\n", availableTools)
	}

	// Check if the exact tool name exists
	for _, availableTool := range availableTools {
		if availableTool == toolName {
			if c.debug {
				c.logger.Debug("✅ Found exact match: %s\n", toolName)
			}
			return toolName, nil
		}
	}

	// If exact name doesn't exist, try to find a prefixed version
	// Look for tools that end with the requested tool name
	var candidates []string
	for _, availableTool := range availableTools {
		// Check if this tool ends with our desired tool name
		if c.isToolMatch(availableTool, toolName) {
			candidates = append(candidates, availableTool)
			if c.debug {
				c.logger.Debug("🎯 Found candidate match: %s for %s\n", availableTool, toolName)
			}
		}
	}

	if len(candidates) == 0 {
		if c.debug {
			c.logger.Debug("❌ No candidates found for tool '%s'\n", toolName)
		}
		return "", fmt.Errorf("tool '%s' not found: tool not found", toolName)
	}

	if len(candidates) == 1 {
		if c.debug {
			c.logger.Debug("✅ Single candidate found: %s\n", candidates[0])
		}
		return candidates[0], nil
	}

	// If multiple candidates, prefer the shortest one (likely the most direct match)
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if len(candidate) < len(best) {
			best = candidate
		}
	}

	if c.debug {
		c.logger.Debug("🔍 Multiple tool candidates found for '%s': %v, choosing: %s\n", toolName, candidates, best)
	}

	return best, nil
}

// isToolMatch checks if a prefixed tool name matches the requested tool name
func (c *mcpTestClient) isToolMatch(prefixedTool, requestedTool string) bool {
	// Check if the prefixed tool ends with the requested tool name
	if prefixedTool == requestedTool {
		return true
	}

	// Check for exact suffix match with underscore separator
	suffix := "_" + requestedTool
	if len(prefixedTool) > len(suffix) && prefixedTool[len(prefixedTool)-len(suffix):] == suffix {
		if c.debug {
			c.logger.Debug("🎯 Suffix match: %s matches %s (suffix: %s)\n", prefixedTool, requestedTool, suffix)
		}
		return true
	}

	// Check for dash separator as well (for names like storage-mock)
	dashSuffix := "-" + requestedTool
	if len(prefixedTool) > len(dashSuffix) && prefixedTool[len(prefixedTool)-len(dashSuffix):] == dashSuffix {
		if c.debug {
			c.logger.Debug("🎯 Dash suffix match: %s matches %s (suffix: %s)\n", prefixedTool, requestedTool, dashSuffix)
		}
		return true
	}

	return false
}

// ListTools returns available MCP tools
func (c *mcpTestClient) ListTools(ctx context.Context) ([]string, error) {
	if c.client == nil {
		return nil, fmt.Errorf("MCP client not connected")
	}

	// Create timeout context for the tools list request
	listCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Get the list of available tools
	result, err := c.client.ListTools(listCtx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Extract tool names
	var toolNames []string
	for _, tool := range result.Tools {
		toolNames = append(toolNames, tool.Name)
	}

	if c.debug {
		c.logger.Debug("🛠️  Available tools (%d): %v\n", len(toolNames), toolNames)
	}

	return toolNames, nil
}

// ListToolsWithSchemas returns available MCP tools with their full schemas
func (c *mcpTestClient) ListToolsWithSchemas(ctx context.Context) ([]mcp.Tool, error) {
	if c.client == nil {
		return nil, fmt.Errorf("MCP client not connected")
	}

	// Create timeout context for the tools list request
	listCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Get the list of available tools
	result, err := c.client.ListTools(listCtx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if c.debug {
		c.logger.Debug("🛠️  Available tools with schemas (%d): %v\n", len(result.Tools), result.Tools)
	}

	return result.Tools, nil
}

// Close closes the MCP connection
func (c *mcpTestClient) Close() error {
	if c.client == nil {
		return nil
	}

	if c.debug {
		c.logger.Debug("🔌 Closing MCP client connection to %s\n", c.endpoint)
	}

	err := c.client.Close()
	c.client = nil
	return err
}

// IsConnected returns whether the client is connected
func (c *mcpTestClient) IsConnected() bool {
	return c.client != nil
}

// GetEndpoint returns the current endpoint
func (c *mcpTestClient) GetEndpoint() string {
	return c.endpoint
}
