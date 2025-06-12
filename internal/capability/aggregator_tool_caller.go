package capability

import (
	"context"
	"fmt"

	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
)

// AggregatorClient represents the interface we need from the aggregator
// This allows us to avoid importing the full aggregator package
type AggregatorClient interface {
	CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error)
	IsToolAvailable(toolName string) bool
}

// AggregatorToolCaller implements the ToolCaller interface using an aggregator client
type AggregatorToolCaller struct {
	aggregatorClient AggregatorClient
}

// NewAggregatorToolCaller creates a new AggregatorToolCaller
func NewAggregatorToolCaller(aggregatorClient AggregatorClient) *AggregatorToolCaller {
	return &AggregatorToolCaller{
		aggregatorClient: aggregatorClient,
	}
}

// CallTool implements the ToolCaller interface
func (atc *AggregatorToolCaller) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	if atc.aggregatorClient == nil {
		return nil, fmt.Errorf("aggregator client is nil")
	}

	// Check if tool is available before calling
	if !atc.aggregatorClient.IsToolAvailable(toolName) {
		return nil, fmt.Errorf("tool %s is not available", toolName)
	}

	logging.Debug("AggregatorToolCaller", "Calling tool %s with args: %v", toolName, arguments)

	// Call the tool through the aggregator
	result, err := atc.aggregatorClient.CallToolInternal(ctx, toolName, arguments)
	if err != nil {
		logging.Error("AggregatorToolCaller", err, "Failed to call tool %s", toolName)
		return nil, fmt.Errorf("failed to call tool %s: %w", toolName, err)
	}

	if result == nil {
		return nil, fmt.Errorf("tool %s returned nil result", toolName)
	}

	// Convert MCP result to our expected format
	responseData := make(map[string]interface{})

	// Handle different types of MCP content using helper functions
	for i, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			if i == 0 {
				// First text content goes to a standard field
				responseData["text"] = textContent.Text
			} else {
				// Additional text content gets indexed
				responseData[fmt.Sprintf("text_%d", i)] = textContent.Text
			}
		} else if imageContent, ok := mcp.AsImageContent(content); ok {
			if i == 0 {
				responseData["image"] = imageContent.Data
				responseData["image_mime_type"] = imageContent.MIMEType
			} else {
				responseData[fmt.Sprintf("image_%d", i)] = imageContent.Data
				responseData[fmt.Sprintf("image_%d_mime_type", i)] = imageContent.MIMEType
			}
		} else if audioContent, ok := mcp.AsAudioContent(content); ok {
			if i == 0 {
				responseData["audio"] = audioContent.Data
				responseData["audio_mime_type"] = audioContent.MIMEType
			} else {
				responseData[fmt.Sprintf("audio_%d", i)] = audioContent.Data
				responseData[fmt.Sprintf("audio_%d_mime_type", i)] = audioContent.MIMEType
			}
		} else {
			// Handle any other content types as generic data
			if i == 0 {
				responseData["content"] = content
			} else {
				responseData[fmt.Sprintf("content_%d", i)] = content
			}
		}
	}

	// If we didn't get any content, try to extract from the result metadata
	if len(responseData) == 0 && result.Meta != nil {
		responseData["meta"] = result.Meta
	}

	// Add success indicator
	responseData["success"] = !result.IsError

	logging.Debug("AggregatorToolCaller", "Tool %s call completed successfully", toolName)
	return responseData, nil
}

// IsToolAvailable checks if a tool is available through the aggregator
func (atc *AggregatorToolCaller) IsToolAvailable(toolName string) bool {
	if atc.aggregatorClient == nil {
		return false
	}
	return atc.aggregatorClient.IsToolAvailable(toolName)
}
