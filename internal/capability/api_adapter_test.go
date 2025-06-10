package capability

import (
	"context"
	"envctl/internal/api"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockToolCaller is a mock implementation of api.ToolCaller
type MockToolCaller struct {
	mock.Mock
}

func (m *MockToolCaller) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	argList := m.Called(ctx, toolName, args)
	if result := argList.Get(0); result != nil {
		return result.(*mcp.CallToolResult), argList.Error(1)
	}
	return nil, argList.Error(1)
}

// Create test definitions for testing
func createTestDefinitions(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "capability-adapter-test-*")
	require.NoError(t, err)

	// Create test auth capability definition
	authYAML := `
name: test_auth
type: auth_provider
version: "1.0.0"
description: "Test authentication provider"
operations:
  login:
    description: "Test login operation"
    parameters:
      cluster:
        type: string
        required: true
        description: "Target cluster"
      user:
        type: string
        required: false
        description: "Username"
    requires:
      - x_teleport_kube
      - x_teleport_status
    workflow:
      name: test_auth_login
      description: "Test login workflow"
      steps:
        - id: login
          tool: x_teleport_kube
          args:
            cluster: "{{ .cluster }}"
          store: result
  logout:
    description: "Test logout operation"
    requires:
      - x_teleport_logout
    workflow:
      name: test_auth_logout
      steps:
        - id: logout
          tool: x_teleport_logout
          args: {}
          store: result
  status:
    description: "Test status operation"
    requires:
      - x_teleport_status
    workflow:
      name: test_auth_status
      steps:
        - id: status
          tool: x_teleport_status
          args: {}
          store: result
`

	authFile := filepath.Join(tmpDir, "test_auth.yaml")
	err = os.WriteFile(authFile, []byte(authYAML), 0644)
	require.NoError(t, err)

	return tmpDir
}

func TestAdapter_GetTools(t *testing.T) {
	tmpDir := createTestDefinitions(t)
	defer os.RemoveAll(tmpDir)

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_teleport_status": true,
			"x_teleport_kube":   true,
			"x_teleport_logout": true,
		},
	}

	// Create mock tool caller
	mockCaller := &MockToolCaller{}

	// Create test adapter
	adapter := NewAdapter(tmpDir, mockChecker, mockCaller)

	// Load test definitions
	err := adapter.LoadDefinitions()
	assert.NoError(t, err)

	// Get tools
	tools := adapter.GetTools()

	// Should have management tools + capability operation tools
	assert.True(t, len(tools) >= 3) // At least 3 management tools

	// Check management tools exist
	managementTools := map[string]bool{
		"capability_list":  false,
		"capability_info":  false,
		"capability_check": false,
	}

	for _, tool := range tools {
		if _, exists := managementTools[tool.Name]; exists {
			managementTools[tool.Name] = true
		}
	}

	// All management tools should be present
	for name, found := range managementTools {
		assert.True(t, found, "Management tool %s not found", name)
	}

	// Check that capability operations are exposed as tools
	authOps := map[string]bool{
		"auth_provider_login":  false,
		"auth_provider_logout": false,
		"auth_provider_status": false,
	}
	
	for _, tool := range tools {
		if _, exists := authOps[tool.Name]; exists {
			authOps[tool.Name] = true
		}
	}

	// All auth operations should be found
	for op, found := range authOps {
		assert.True(t, found, "Auth operation %s not found", op)
	}

	// Check that parameters are correctly extracted
	for _, tool := range tools {
		if tool.Name == "auth_provider_login" {
			assert.Len(t, tool.Parameters, 2) // cluster and user
			// Check cluster parameter
			var hasCluster, hasUser bool
			for _, param := range tool.Parameters {
				if param.Name == "cluster" {
					hasCluster = true
					assert.Equal(t, "string", param.Type)
					assert.True(t, param.Required)
				}
				if param.Name == "user" {
					hasUser = true
					assert.Equal(t, "string", param.Type)
					assert.False(t, param.Required)
				}
			}
			assert.True(t, hasCluster)
			assert.True(t, hasUser)
		}
	}
}

func TestAdapter_ExecuteTool(t *testing.T) {
	tmpDir := createTestDefinitions(t)
	defer os.RemoveAll(tmpDir)

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_teleport_status": true,
			"x_teleport_kube":   true,
			"x_teleport_logout": true,
		},
	}

	// Create mock tool caller
	mockCaller := &MockToolCaller{}

	// Create test adapter
	adapter := NewAdapter(tmpDir, mockChecker, mockCaller)

	// Load test definitions
	err := adapter.LoadDefinitions()
	assert.NoError(t, err)

	ctx := context.Background()

	t.Run("ExecuteTool_CapabilityList", func(t *testing.T) {
		result, err := adapter.ExecuteTool(ctx, "capability_list", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)
		assert.Len(t, result.Content, 1)

		// Check result structure
		content := result.Content[0].(map[string]interface{})
		assert.Contains(t, content, "capabilities")
		assert.Contains(t, content, "total")
		
		capabilities := content["capabilities"].([]api.CapabilityInfo)
		assert.Len(t, capabilities, 1)
		assert.Equal(t, "auth_provider", capabilities[0].Type)
	})

	t.Run("ExecuteTool_CapabilityInfo", func(t *testing.T) {
		// Test with valid capability type
		args := map[string]interface{}{
			"type": "auth_provider",
		}
		result, err := adapter.ExecuteTool(ctx, "capability_info", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)
		
		capInfo := result.Content[0].(api.CapabilityInfo)
		assert.Equal(t, "auth_provider", capInfo.Type)
		assert.Equal(t, "test_auth", capInfo.Name)

		// Test without type parameter
		result, err = adapter.ExecuteTool(ctx, "capability_info", map[string]interface{}{})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "type is required")

		// Test with non-existent type
		args = map[string]interface{}{
			"type": "non_existent",
		}
		result, err = adapter.ExecuteTool(ctx, "capability_info", args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "not found")
	})

	t.Run("ExecuteTool_CapabilityCheck", func(t *testing.T) {
		// Test with valid parameters
		args := map[string]interface{}{
			"type":      "auth_provider",
			"operation": "login",
		}
		result, err := adapter.ExecuteTool(ctx, "capability_check", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)

		content := result.Content[0].(map[string]interface{})
		assert.Equal(t, "auth_provider", content["type"])
		assert.Equal(t, "login", content["operation"])
		assert.Equal(t, true, content["available"])

		// Test without required parameters
		result, err = adapter.ExecuteTool(ctx, "capability_check", map[string]interface{}{})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "type is required")

		args = map[string]interface{}{
			"type": "auth_provider",
		}
		result, err = adapter.ExecuteTool(ctx, "capability_check", args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "operation is required")
	})

	t.Run("ExecuteTool_CapabilityOperation", func(t *testing.T) {
		// Mock the workflow execution
		mockCaller.On("CallToolInternal", ctx, "action_test_auth_login", mock.Anything).Return(
			&mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("login successful"),
				},
				IsError: false,
			}, nil)

		// Test capability operation call
		args := map[string]interface{}{
			"cluster": "test-cluster",
		}
		result, err := adapter.ExecuteTool(ctx, "auth_provider_login", args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)
		assert.Len(t, result.Content, 1)

		// Verify the mock was called with correct arguments
		mockCaller.AssertCalled(t, "CallToolInternal", ctx, "action_test_auth_login", args)
	})

	t.Run("ExecuteTool_UnknownTool", func(t *testing.T) {
		result, err := adapter.ExecuteTool(ctx, "unknown_tool", nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not available")
	})
}

func TestAdapter_HandleMethods(t *testing.T) {
	tmpDir := createTestDefinitions(t)
	defer os.RemoveAll(tmpDir)

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_teleport_status": true,
			"x_teleport_kube":   true,
			"x_teleport_logout": true,
		},
	}

	// Create mock tool caller
	mockCaller := &MockToolCaller{}

	// Create test adapter
	adapter := NewAdapter(tmpDir, mockChecker, mockCaller)

	// Load test definitions
	err := adapter.LoadDefinitions()
	assert.NoError(t, err)

	t.Run("handleList", func(t *testing.T) {
		result, err := adapter.handleList()
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)
		assert.Len(t, result.Content, 1)

		content := result.Content[0].(map[string]interface{})
		capabilities := content["capabilities"].([]api.CapabilityInfo)
		assert.Len(t, capabilities, 1)
		assert.Equal(t, 1, content["total"])
	})

	t.Run("handleInfo", func(t *testing.T) {
		// Test with valid type
		args := map[string]interface{}{
			"type": "auth_provider",
		}
		result, err := adapter.handleInfo(args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)

		capInfo := result.Content[0].(api.CapabilityInfo)
		assert.Equal(t, "auth_provider", capInfo.Type)
		assert.Equal(t, "test_auth", capInfo.Name)

		// Test with missing type
		result, err = adapter.handleInfo(map[string]interface{}{})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "type is required")

		// Test with invalid type conversion
		args = map[string]interface{}{
			"type": 123, // not a string
		}
		result, err = adapter.handleInfo(args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "type is required")

		// Test with non-existent type
		args = map[string]interface{}{
			"type": "non_existent",
		}
		result, err = adapter.handleInfo(args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "not found")
	})

	t.Run("handleCheck", func(t *testing.T) {
		// Test with valid parameters
		args := map[string]interface{}{
			"type":      "auth_provider",
			"operation": "login",
		}
		result, err := adapter.handleCheck(args)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)

		content := result.Content[0].(map[string]interface{})
		assert.Equal(t, "auth_provider", content["type"])
		assert.Equal(t, "login", content["operation"])
		assert.Equal(t, true, content["available"])

		// Test with missing parameters
		result, err = adapter.handleCheck(map[string]interface{}{})
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "type is required")

		args = map[string]interface{}{
			"type": "auth_provider",
		}
		result, err = adapter.handleCheck(args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0], "operation is required")

		// Test with invalid type conversion
		args = map[string]interface{}{
			"type":      123, // not a string
			"operation": "login",
		}
		result, err = adapter.handleCheck(args)
		assert.NoError(t, err)
		assert.True(t, result.IsError)

		// Test unavailable operation
		args = map[string]interface{}{
			"type":      "auth_provider",
			"operation": "unknown_op",
		}
		result, err = adapter.handleCheck(args)
		assert.NoError(t, err)
		assert.False(t, result.IsError)
		content = result.Content[0].(map[string]interface{})
		assert.Equal(t, false, content["available"])
	})
}

func TestAdapter_GetLoader(t *testing.T) {
	// Create mock tool checker
	mockChecker := &mockToolChecker{}

	// Create mock tool caller
	mockCaller := &MockToolCaller{}

	// Create test adapter
	adapter := NewAdapter("testpath", mockChecker, mockCaller)

	// Get loader should return the internal loader
	loader := adapter.GetLoader()
	assert.NotNil(t, loader)
}

func TestAdapter_ExecuteCapability_EdgeCases(t *testing.T) {
	tmpDir := createTestDefinitions(t)
	defer os.RemoveAll(tmpDir)

	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_teleport_status": true,
			"x_teleport_kube":   true,
		},
	}

	mockCaller := &MockToolCaller{}
	adapter := NewAdapter(tmpDir, mockChecker, mockCaller)
	err := adapter.LoadDefinitions()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("ExecuteCapability_OperationNotAvailable", func(t *testing.T) {
		// Make logout unavailable by removing its required tool
		mockChecker.availableTools["x_teleport_logout"] = false
		adapter.loader.RefreshAvailability()

		result, err := adapter.ExecuteCapability(ctx, "auth_provider", "logout", nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not available")
	})

	t.Run("ExecuteCapability_WorkflowExecutionError", func(t *testing.T) {
		// Mock workflow execution error
		mockCaller.On("CallToolInternal", ctx, "action_test_auth_login", mock.Anything).Return(
			nil, assert.AnError)

		result, err := adapter.ExecuteCapability(ctx, "auth_provider", "login", map[string]interface{}{
			"cluster": "test",
		})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "workflow execution failed")
	})
} 