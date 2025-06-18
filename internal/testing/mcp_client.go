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
}

// NewMCPTestClient creates a new MCP test client
func NewMCPTestClient(debug bool) MCPTestClient {
	return &mcpTestClient{
		debug: debug,
	}
}

// Connect establishes connection to the MCP aggregator
func (c *mcpTestClient) Connect(ctx context.Context, endpoint string) error {
	c.endpoint = endpoint

	if c.debug {
		fmt.Printf("🔗 Connecting to MCP aggregator at %s\n", endpoint)
	}

	// Create streamable HTTP client for envctl aggregator
	httpClient, err := client.NewStreamableHttpClient(endpoint)
	if err != nil {
		return fmt.Errorf("failed to create streamable HTTP client: %w", err)
	}

	// Start the streamable HTTP transport
	if err := httpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start streamable HTTP client: %w", err)
	}

	// Store the client
	c.client = httpClient

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

	_, err = c.client.Initialize(initCtx, initRequest)
	if err != nil {
		c.client.Close()
		return fmt.Errorf("failed to initialize MCP protocol: %w", err)
	}

	if c.debug {
		fmt.Printf("🔗 Successfully connected to MCP aggregator at %s\n", endpoint)
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
		fmt.Printf("🔧 Calling tool: %s\n", toolName)
		fmt.Printf("📋 Parameters: %s\n", string(argsJSON))
	}

	// Create timeout context for the tool call
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create the request using the pattern from the existing codebase
	request := mcp.CallToolRequest{
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name:      toolName,
			Arguments: args,
		},
	}

	// Make the tool call
	result, err := c.client.CallTool(callCtx, request)
	if err != nil {
		if c.debug {
			fmt.Printf("❌ Tool call failed: %v\n", err)
		}
		return nil, fmt.Errorf("tool call %s failed: %w", toolName, err)
	}

	if c.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("✅ Tool call result: %s\n", string(resultJSON))
	}

	return result, nil
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
		fmt.Printf("🛠️  Available tools (%d): %v\n", len(toolNames), toolNames)
	}

	return toolNames, nil
}

// Close closes the MCP connection
func (c *mcpTestClient) Close() error {
	if c.client == nil {
		return nil
	}

	if c.debug {
		fmt.Printf("🔌 Closing MCP client connection to %s\n", c.endpoint)
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
