package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestNewAPITools(t *testing.T) {
	at := NewAPITools()
	assert.NotNil(t, at)
	assert.NotNil(t, at.orchestratorAPI)
	assert.NotNil(t, at.mcpServiceAPI)
	assert.NotNil(t, at.k8sServiceAPI)
	assert.NotNil(t, at.portForwardServiceAPI)
}

func TestGetAPITools(t *testing.T) {
	at := NewAPITools()
	tools := at.GetAPITools()

	// Count expected tools
	expectedToolCount := 5 + 3 + 3 + 3 + 2 // service + cluster + mcp + k8s + portforward
	assert.Len(t, tools, expectedToolCount)

	// Check some specific tools exist
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	// Service Management Tools
	assert.True(t, toolNames["service_list"])
	assert.True(t, toolNames["service_start"])
	assert.True(t, toolNames["service_stop"])
	assert.True(t, toolNames["service_restart"])
	assert.True(t, toolNames["service_status"])

	// Cluster Management Tools
	assert.True(t, toolNames["cluster_list"])
	assert.True(t, toolNames["cluster_switch"])
	assert.True(t, toolNames["cluster_active"])

	// MCP Server Tools
	assert.True(t, toolNames["mcp_server_list"])
	assert.True(t, toolNames["mcp_server_info"])
	assert.True(t, toolNames["mcp_server_tools"])

	// K8s Connection Tools
	assert.True(t, toolNames["k8s_connection_list"])
	assert.True(t, toolNames["k8s_connection_info"])
	assert.True(t, toolNames["k8s_connection_by_context"])

	// Port Forward Tools
	assert.True(t, toolNames["portforward_list"])
	assert.True(t, toolNames["portforward_info"])
}

func TestServiceListHandler(t *testing.T) {
	at := NewAPITools()

	// Create a mock request
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "service_list",
			Arguments: map[string]interface{}{},
		},
	}

	// Call the handler
	result, err := at.HandleServiceList(context.Background(), req)

	// Should not error (even if no services are registered)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)

	// Check that it returns text content
	_, ok := result.Content[0].(mcp.TextContent)
	assert.True(t, ok, "Expected TextContent")
}

func TestServiceStartHandler_MissingLabel(t *testing.T) {
	at := NewAPITools()

	// Create a request without label
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "service_start",
			Arguments: map[string]interface{}{},
		},
	}

	// Call the handler
	result, err := at.HandleServiceStart(context.Background(), req)

	// Should return an error result
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestClusterListHandler_ValidRole(t *testing.T) {
	at := NewAPITools()

	// Create a request with valid role
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "cluster_list",
			Arguments: map[string]interface{}{
				"role": "management",
			},
		},
	}

	// Call the handler
	result, err := at.HandleClusterList(context.Background(), req)

	// Should not error
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Content, 1)
}
