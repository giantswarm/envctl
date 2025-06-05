package agent

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// CallTool executes a tool and returns the result
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
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

	// Log request
	c.logger.Request(fmt.Sprintf("tools/call (%s)", name), req.Params)

	// Send request
	result, err := c.client.CallTool(ctx, req)
	if err != nil {
		c.logger.Error("CallTool failed: %v", err)
		return nil, err
	}

	// Log response
	c.logger.Response(fmt.Sprintf("tools/call (%s)", name), result)

	return result, nil
}

// GetResource reads a resource and returns its content
func (c *Client) GetResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	req := mcp.ReadResourceRequest{
		Params: struct {
			URI       string         `json:"uri"`
			Arguments map[string]any `json:"arguments,omitempty"`
		}{
			URI: uri,
		},
	}

	// Log request
	c.logger.Request("resources/read", req.Params)

	// Send request
	result, err := c.client.ReadResource(ctx, req)
	if err != nil {
		c.logger.Error("ReadResource failed: %v", err)
		return nil, err
	}

	// Log response
	c.logger.Response("resources/read", result)

	return result, nil
}

// GetPrompt retrieves a prompt with the given arguments
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*mcp.GetPromptResult, error) {
	req := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name:      name,
			Arguments: args,
		},
	}

	// Log request
	c.logger.Request(fmt.Sprintf("prompts/get (%s)", name), req.Params)

	// Send request
	result, err := c.client.GetPrompt(ctx, req)
	if err != nil {
		c.logger.Error("GetPrompt failed: %v", err)
		return nil, err
	}

	// Log response
	c.logger.Response(fmt.Sprintf("prompts/get (%s)", name), result)

	return result, nil
}
