package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleListTools handles the list_tools MCP tool
func (m *MCPServer) handleListTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	m.client.mu.RLock()
	tools := m.client.toolCache
	m.client.mu.RUnlock()

	if len(tools) == 0 {
		return mcp.NewToolResultText("No tools available"), nil
	}

	// Format tools as JSON for structured output
	type ToolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	toolList := make([]ToolInfo, len(tools))
	for i, tool := range tools {
		toolList[i] = ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
		}
	}

	jsonData, err := json.MarshalIndent(toolList, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format tools: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleListResources handles the list_resources MCP tool
func (m *MCPServer) handleListResources(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	m.client.mu.RLock()
	resources := m.client.resourceCache
	m.client.mu.RUnlock()

	if len(resources) == 0 {
		return mcp.NewToolResultText("No resources available"), nil
	}

	// Format resources as JSON
	type ResourceInfo struct {
		URI         string `json:"uri"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		MIMEType    string `json:"mimeType,omitempty"`
	}

	resourceList := make([]ResourceInfo, len(resources))
	for i, resource := range resources {
		desc := resource.Description
		if desc == "" {
			desc = resource.Name
		}
		resourceList[i] = ResourceInfo{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: desc,
			MIMEType:    resource.MIMEType,
		}
	}

	jsonData, err := json.MarshalIndent(resourceList, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format resources: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleListPrompts handles the list_prompts MCP tool
func (m *MCPServer) handleListPrompts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	m.client.mu.RLock()
	prompts := m.client.promptCache
	m.client.mu.RUnlock()

	if len(prompts) == 0 {
		return mcp.NewToolResultText("No prompts available"), nil
	}

	// Format prompts as JSON
	type PromptInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	promptList := make([]PromptInfo, len(prompts))
	for i, prompt := range prompts {
		promptList[i] = PromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
		}
	}

	jsonData, err := json.MarshalIndent(promptList, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format prompts: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleDescribeTool handles the describe_tool MCP tool
func (m *MCPServer) handleDescribeTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	m.client.mu.RLock()
	defer m.client.mu.RUnlock()

	for _, tool := range m.client.toolCache {
		if tool.Name == name {
			// Format tool info as JSON
			toolInfo := map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			}

			jsonData, err := json.MarshalIndent(toolInfo, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to format tool info: %v", err)), nil
			}

			return mcp.NewToolResultText(string(jsonData)), nil
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("Tool not found: %s", name)), nil
}

// handleDescribeResource handles the describe_resource MCP tool
func (m *MCPServer) handleDescribeResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uri, err := request.RequireString("uri")
	if err != nil {
		return mcp.NewToolResultError("uri parameter is required"), nil
	}

	m.client.mu.RLock()
	defer m.client.mu.RUnlock()

	for _, resource := range m.client.resourceCache {
		if resource.URI == uri {
			// Format resource info as JSON
			resourceInfo := map[string]interface{}{
				"uri":         resource.URI,
				"name":        resource.Name,
				"description": resource.Description,
				"mimeType":    resource.MIMEType,
			}

			jsonData, err := json.MarshalIndent(resourceInfo, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to format resource info: %v", err)), nil
			}

			return mcp.NewToolResultText(string(jsonData)), nil
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("Resource not found: %s", uri)), nil
}

// handleDescribePrompt handles the describe_prompt MCP tool
func (m *MCPServer) handleDescribePrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	m.client.mu.RLock()
	defer m.client.mu.RUnlock()

	for _, prompt := range m.client.promptCache {
		if prompt.Name == name {
			// Format prompt info as JSON
			promptInfo := map[string]interface{}{
				"name":        prompt.Name,
				"description": prompt.Description,
			}

			if len(prompt.Arguments) > 0 {
				args := make([]map[string]interface{}, len(prompt.Arguments))
				for i, arg := range prompt.Arguments {
					args[i] = map[string]interface{}{
						"name":        arg.Name,
						"description": arg.Description,
						"required":    arg.Required,
					}
				}
				promptInfo["arguments"] = args
			}

			jsonData, err := json.MarshalIndent(promptInfo, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to format prompt info: %v", err)), nil
			}

			return mcp.NewToolResultText(string(jsonData)), nil
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("Prompt not found: %s", name)), nil
}

// handleCallTool handles the call_tool MCP tool
func (m *MCPServer) handleCallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	// Get arguments if provided
	var args map[string]interface{}
	if argsRaw := request.GetArguments()["arguments"]; argsRaw != nil {
		var ok bool
		args, ok = argsRaw.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a JSON object"), nil
		}
	}

	// Execute the tool
	result, err := m.client.CallTool(ctx, name, args)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %v", err)), nil
	}

	// Format result
	if result.IsError {
		var errorMessages []string
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				errorMessages = append(errorMessages, textContent.Text)
			}
		}
		return mcp.NewToolResultError(strings.Join(errorMessages, "\n")), nil
	}

	// Format successful result
	var resultTexts []string
	for _, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			resultTexts = append(resultTexts, textContent.Text)
		} else if imageContent, ok := mcp.AsImageContent(content); ok {
			resultTexts = append(resultTexts, fmt.Sprintf("[Image: MIME type %s, %d bytes]", imageContent.MIMEType, len(imageContent.Data)))
		} else if audioContent, ok := mcp.AsAudioContent(content); ok {
			resultTexts = append(resultTexts, fmt.Sprintf("[Audio: MIME type %s, %d bytes]", audioContent.MIMEType, len(audioContent.Data)))
		}
	}

	return mcp.NewToolResultText(strings.Join(resultTexts, "\n")), nil
}

// handleGetResource handles the get_resource MCP tool
func (m *MCPServer) handleGetResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uri, err := request.RequireString("uri")
	if err != nil {
		return mcp.NewToolResultError("uri parameter is required"), nil
	}

	// Retrieve the resource
	result, err := m.client.GetResource(ctx, uri)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Resource retrieval failed: %v", err)), nil
	}

	// Format contents
	var contentTexts []string
	for _, content := range result.Contents {
		if textContent, ok := mcp.AsTextResourceContents(content); ok {
			contentTexts = append(contentTexts, textContent.Text)
		} else if blobContent, ok := mcp.AsBlobResourceContents(content); ok {
			contentTexts = append(contentTexts, fmt.Sprintf("[Binary data: %d bytes]", len(blobContent.Blob)))
		}
	}

	return mcp.NewToolResultText(strings.Join(contentTexts, "\n")), nil
}

// handleGetPrompt handles the get_prompt MCP tool
func (m *MCPServer) handleGetPrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	// Get arguments if provided
	args := make(map[string]string)
	if argsRaw := request.GetArguments()["arguments"]; argsRaw != nil {
		argsMap, ok := argsRaw.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("arguments must be a JSON object"), nil
		}

		// Convert to string map
		for k, v := range argsMap {
			args[k] = fmt.Sprintf("%v", v)
		}
	}

	// Get the prompt
	result, err := m.client.GetPrompt(ctx, name, args)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Prompt retrieval failed: %v", err)), nil
	}

	// Format messages as JSON
	type Message struct {
		Role    mcp.Role        `json:"role"`
		Content json.RawMessage `json:"content"`
	}

	messages := make([]Message, len(result.Messages))
	for i, msg := range result.Messages {
		var content json.RawMessage

		if textContent, ok := mcp.AsTextContent(msg.Content); ok {
			contentMap := map[string]interface{}{
				"type": "text",
				"text": textContent.Text,
			}
			content, _ = json.Marshal(contentMap)
		} else if imageContent, ok := mcp.AsImageContent(msg.Content); ok {
			contentMap := map[string]interface{}{
				"type":     "image",
				"mimeType": imageContent.MIMEType,
				"dataSize": len(imageContent.Data),
			}
			content, _ = json.Marshal(contentMap)
		} else if audioContent, ok := mcp.AsAudioContent(msg.Content); ok {
			contentMap := map[string]interface{}{
				"type":     "audio",
				"mimeType": audioContent.MIMEType,
				"dataSize": len(audioContent.Data),
			}
			content, _ = json.Marshal(contentMap)
		} else if resource, ok := mcp.AsEmbeddedResource(msg.Content); ok {
			contentMap := map[string]interface{}{
				"type":     "embeddedResource",
				"resource": resource.Resource,
			}
			content, _ = json.Marshal(contentMap)
		} else {
			// Fallback
			content, _ = json.Marshal(msg.Content)
		}

		messages[i] = Message{
			Role:    msg.Role,
			Content: content,
		}
	}

	jsonData, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format messages: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
