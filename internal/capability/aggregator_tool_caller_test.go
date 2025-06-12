package capability

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// mockAggregatorClient implements AggregatorClient for testing
type mockAggregatorClient struct {
	callResponses  map[string]*mcp.CallToolResult
	availableTools map[string]bool
	callCount      int
	lastCalledTool string
	lastCalledArgs map[string]interface{}
}

func newMockAggregatorClient() *mockAggregatorClient {
	return &mockAggregatorClient{
		callResponses:  make(map[string]*mcp.CallToolResult),
		availableTools: make(map[string]bool),
	}
}

func (m *mockAggregatorClient) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	m.callCount++
	m.lastCalledTool = toolName
	m.lastCalledArgs = args

	if result, exists := m.callResponses[toolName]; exists {
		return result, nil
	}

	// Default successful response
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("Mock response"),
		},
		IsError: false,
	}, nil
}

func (m *mockAggregatorClient) IsToolAvailable(toolName string) bool {
	if available, exists := m.availableTools[toolName]; exists {
		return available
	}
	return true // Default to available
}

func (m *mockAggregatorClient) setToolResponse(toolName string, response *mcp.CallToolResult) {
	m.callResponses[toolName] = response
}

func (m *mockAggregatorClient) setToolAvailable(toolName string, available bool) {
	m.availableTools[toolName] = available
}

func TestAggregatorToolCaller_CallTool(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		args           map[string]interface{}
		mockResponse   *mcp.CallToolResult
		toolAvailable  bool
		expectError    bool
		expectedResult map[string]interface{}
	}{
		{
			name:          "successful text response",
			toolName:      "test_tool",
			args:          map[string]interface{}{"param1": "value1"},
			toolAvailable: true,
			mockResponse: &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Success message"),
				},
				IsError: false,
			},
			expectError: false,
			expectedResult: map[string]interface{}{
				"text":    "Success message",
				"success": true,
			},
		},
		{
			name:          "tool not available",
			toolName:      "unavailable_tool",
			args:          map[string]interface{}{},
			toolAvailable: false,
			expectError:   true,
		},
		{
			name:          "multiple text content",
			toolName:      "multi_text_tool",
			args:          map[string]interface{}{},
			toolAvailable: true,
			mockResponse: &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("First message"),
					mcp.NewTextContent("Second message"),
				},
				IsError: false,
			},
			expectError: false,
			expectedResult: map[string]interface{}{
				"text":    "First message",
				"text_1":  "Second message",
				"success": true,
			},
		},
		{
			name:          "error response",
			toolName:      "error_tool",
			args:          map[string]interface{}{},
			toolAvailable: true,
			mockResponse: &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error message"),
				},
				IsError: true,
			},
			expectError: false,
			expectedResult: map[string]interface{}{
				"text":    "Error message",
				"success": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newMockAggregatorClient()
			mockClient.setToolAvailable(tt.toolName, tt.toolAvailable)
			if tt.mockResponse != nil {
				mockClient.setToolResponse(tt.toolName, tt.mockResponse)
			}

			toolCaller := NewAggregatorToolCaller(mockClient)

			result, err := toolCaller.CallTool(context.Background(), tt.toolName, tt.args)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result, got nil")
				return
			}

			// Check expected results
			for key, expectedValue := range tt.expectedResult {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("expected key %s not found in result", key)
				} else if actualValue != expectedValue {
					t.Errorf("expected %s = %v, got %v", key, expectedValue, actualValue)
				}
			}

			// Verify tool was called with correct arguments
			if mockClient.lastCalledTool != tt.toolName {
				t.Errorf("expected tool %s to be called, got %s", tt.toolName, mockClient.lastCalledTool)
			}

			if len(tt.args) > 0 {
				for key, expectedValue := range tt.args {
					if actualValue, exists := mockClient.lastCalledArgs[key]; !exists {
						t.Errorf("expected argument %s not passed to tool", key)
					} else if actualValue != expectedValue {
						t.Errorf("expected argument %s = %v, got %v", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestAggregatorToolCaller_IsToolAvailable(t *testing.T) {
	mockClient := newMockAggregatorClient()
	mockClient.setToolAvailable("available_tool", true)
	mockClient.setToolAvailable("unavailable_tool", false)

	toolCaller := NewAggregatorToolCaller(mockClient)

	if !toolCaller.IsToolAvailable("available_tool") {
		t.Errorf("expected available_tool to be available")
	}

	if toolCaller.IsToolAvailable("unavailable_tool") {
		t.Errorf("expected unavailable_tool to be unavailable")
	}
}

func TestAggregatorToolCaller_NilClient(t *testing.T) {
	toolCaller := NewAggregatorToolCaller(nil)

	_, err := toolCaller.CallTool(context.Background(), "test_tool", map[string]interface{}{})
	if err == nil {
		t.Errorf("expected error with nil client, got nil")
	}

	if toolCaller.IsToolAvailable("test_tool") {
		t.Errorf("expected tool to be unavailable with nil client")
	}
}
