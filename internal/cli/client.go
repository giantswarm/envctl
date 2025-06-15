package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// CLIClient provides a simplified MCP client for CLI commands
type CLIClient struct {
	endpoint string
	client   client.MCPClient
	timeout  time.Duration
}

// NewCLIClient creates a new CLI client with auto-detected endpoint
func NewCLIClient() (*CLIClient, error) {
	endpoint, err := DetectAggregatorEndpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to detect aggregator endpoint: %w", err)
	}

	return &CLIClient{
		endpoint: endpoint,
		timeout:  30 * time.Second,
	}, nil
}

// NewCLIClientWithEndpoint creates a new CLI client with a specific endpoint
func NewCLIClientWithEndpoint(endpoint string) *CLIClient {
	return &CLIClient{
		endpoint: endpoint,
		timeout:  30 * time.Second,
	}
}

// Connect establishes connection to the MCP aggregator
func (c *CLIClient) Connect(ctx context.Context) error {
	// Create streamable-http client
	httpClient, err := client.NewStreamableHttpClient(c.endpoint)
	if err != nil {
		return fmt.Errorf("failed to create streamable-http client: %w", err)
	}
	c.client = httpClient

	// Start the streamable-http transport
	if err := httpClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start streamable-http client: %w", err)
	}

	// Initialize the session
	if err := c.initialize(ctx); err != nil {
		httpClient.Close()
		return fmt.Errorf("initialization failed: %w", err)
	}

	return nil
}

// CallTool executes a tool and returns the result
func (c *CLIClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	req := mcp.CallToolRequest{
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name:      name,
			Arguments: args,
		},
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Send request
	result, err := c.client.CallTool(timeoutCtx, req)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	return result, nil
}

// CallToolSimple executes a tool and returns the text content as a string
func (c *CLIClient) CallToolSimple(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	result, err := c.CallTool(ctx, name, args)
	if err != nil {
		return "", err
	}

	if result.IsError {
		var errorMsgs []string
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				errorMsgs = append(errorMsgs, textContent.Text)
			}
		}
		return "", fmt.Errorf("tool error: %s", fmt.Sprintf("%v", errorMsgs))
	}

	var output []string
	for _, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			output = append(output, textContent.Text)
		}
	}

	if len(output) == 0 {
		return "", nil
	}

	return output[0], nil
}

// CallToolJSON executes a tool and returns the result as parsed JSON
func (c *CLIClient) CallToolJSON(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	textResult, err := c.CallToolSimple(ctx, name, args)
	if err != nil {
		return nil, err
	}

	var jsonResult interface{}
	if err := json.Unmarshal([]byte(textResult), &jsonResult); err != nil {
		// If it's not JSON, return the text as-is
		return textResult, nil
	}

	return jsonResult, nil
}

// Close closes the connection
func (c *CLIClient) Close() error {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
	return nil
}

// initialize performs the MCP protocol handshake
func (c *CLIClient) initialize(ctx context.Context) error {
	req := mcp.InitializeRequest{
		Params: struct {
			ProtocolVersion string                 `json:"protocolVersion"`
			Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      mcp.Implementation     `json:"clientInfo"`
		}{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "envctl-cli",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Send request
	_, err := c.client.Initialize(timeoutCtx, req)
	if err != nil {
		return err
	}

	return nil
} 