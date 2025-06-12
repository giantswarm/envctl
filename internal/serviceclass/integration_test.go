package serviceclass

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockToolChecker implements ToolChecker for testing
type mockToolChecker struct {
	availableTools map[string]bool
}

func (m *mockToolChecker) IsToolAvailable(toolName string) bool {
	if m.availableTools == nil {
		return false
	}
	return m.availableTools[toolName]
}

// mockToolCaller implements api.ToolCaller for testing
type mockToolCaller struct {
	calls []toolCall
}

type toolCall struct {
	toolName string
	args     map[string]interface{}
}

func (m *mockToolCaller) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	m.calls = append(m.calls, toolCall{toolName: toolName, args: args})

	// Return different responses based on tool name
	switch toolName {
	case "x_kubernetes_connect":
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(`{
					"connectionId": "k8s-test-connection-123",
					"status": "connected",
					"connected": true,
					"clusterName": "test-cluster",
					"context": "test-context"
				}`),
			},
			IsError: false,
		}, nil
	default:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(`{"status": "success", "tool": "` + toolName + `"}`),
			},
			IsError: false,
		}, nil
	}
}

// TestServiceClassManagerIntegration tests the complete integration of ServiceClass loading
func TestServiceClassManagerIntegration(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write a test ServiceClass definition
	testYAML := `name: test_k8s_connection
type: service_k8s_connection_provider
version: "1.0.0"
description: "Test Kubernetes connection service class"

serviceConfig:
  serviceType: "DynamicK8sConnection"
  defaultLabel: "k8s-{{ .cluster_name }}"
  dependencies: []
  
  lifecycleTools:
    create:
      tool: "x_kubernetes_connect"
      arguments:
        clusterName: "{{ .cluster_name }}"
        context: "{{ .context | default .cluster_name }}"
      responseMapping:
        serviceId: "$.connectionId"
        status: "$.status"
        health: "$.connected"
    delete:
      tool: "x_kubernetes_disconnect"
      arguments:
        connectionId: "{{ .service_id }}"
      responseMapping:
        status: "$.status"
  
  createParameters:
    cluster_name:
      toolParameter: "clusterName"
      required: true
    context:
      toolParameter: "context"
      required: false

operations:
  create_connection:
    description: "Create Kubernetes connection"
    parameters:
      cluster_name:
        type: string
        required: true
    requires:
      - x_kubernetes_connect

metadata:
  provider: "kubernetes"
  category: "connection"
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service_k8s_connection.yaml"), []byte(testYAML), 0644))

	// Create mock tool checker with required tools available
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_kubernetes_connect":    true,
			"x_kubernetes_disconnect": true,
		},
	}

	// Create ServiceClass manager
	manager := NewServiceClassManager(tmpDir, mockChecker)

	// Load definitions
	require.NoError(t, manager.LoadServiceDefinitions())

	// Test that ServiceClass is loaded and available
	assert.True(t, manager.IsServiceClassAvailable("test_k8s_connection"))

	// Test retrieving ServiceClass definition
	def, exists := manager.GetServiceClassDefinition("test_k8s_connection")
	require.True(t, exists)
	assert.Equal(t, "test_k8s_connection", def.Name)
	assert.Equal(t, "service_k8s_connection_provider", def.Type)
	assert.Equal(t, "Test Kubernetes connection service class", def.Description)

	// Test listing ServiceClass definitions
	definitions := manager.ListServiceClasses()
	assert.Len(t, definitions, 1)
	assert.Equal(t, "test_k8s_connection", definitions[0].Name)
	assert.True(t, definitions[0].Available)
	assert.True(t, definitions[0].CreateToolAvailable)
	assert.True(t, definitions[0].DeleteToolAvailable)
}

// TestServiceClassMissingTools tests behavior when required tools are not available
func TestServiceClassMissingTools(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write a ServiceClass definition that requires missing tools
	portForwardYAML := `name: test_portforward
type: service_portforward_provider
version: "1.0.0"
description: "Test port forward service class"

serviceConfig:
  serviceType: "DynamicPortForward"
  defaultLabel: "pf-{{ .service_name }}"
  
  lifecycleTools:
    create:
      tool: "x_k8s_port_forward"
      arguments:
        namespace: "{{ .namespace }}"
        service: "{{ .service_name }}"
      responseMapping:
        serviceId: "$.forwardId"
        status: "$.status"
    delete:
      tool: "x_k8s_port_forward_stop"
      arguments:
        forwardId: "{{ .service_id }}"
      responseMapping:
        status: "$.status"

operations:
  create_portforward:
    description: "Create port forward"
    parameters:
      namespace:
        type: string
        required: true
    requires:
      - x_k8s_port_forward

metadata:
  provider: "kubectl"
  category: "networking"
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service_portforward.yaml"), []byte(portForwardYAML), 0644))

	// Create mock tool checker with no tools available
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{},
	}

	// Create ServiceClass manager
	manager := NewServiceClassManager(tmpDir, mockChecker)

	// Load definitions
	require.NoError(t, manager.LoadServiceDefinitions())

	// Test that ServiceClass is loaded but not available
	assert.False(t, manager.IsServiceClassAvailable("test_portforward"))

	// Test listing should show unavailable service class
	definitions := manager.ListServiceClasses()
	assert.Len(t, definitions, 1)
	assert.Equal(t, "test_portforward", definitions[0].Name)
	assert.False(t, definitions[0].Available)
	assert.False(t, definitions[0].CreateToolAvailable)
	assert.False(t, definitions[0].DeleteToolAvailable)
}

// TestServiceClassAPIAdapter tests the API adapter integration
func TestServiceClassAPIAdapter(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write a simple ServiceClass definition
	simpleYAML := `name: test_simple
type: simple_service
version: "1.0.0"
description: "Simple test service class"

serviceConfig:
  serviceType: "TestService"
  defaultLabel: "test-{{ .name }}"
  
  lifecycleTools:
    create:
      tool: "test_create_tool"
      arguments:
        name: "{{ .name }}"
      responseMapping:
        serviceId: "$.id"
        status: "$.status"
    delete:
      tool: "test_delete_tool"
      arguments:
        serviceId: "{{ .service_id }}"
      responseMapping:
        status: "$.status"

metadata:
  provider: "test"
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service_simple.yaml"), []byte(simpleYAML), 0644))

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"test_create_tool": true,
			"test_delete_tool": true,
		},
	}

	// Create ServiceClass manager and adapter
	manager := NewServiceClassManager(tmpDir, mockChecker)
	adapter := NewAdapter(manager)

	// Load definitions through adapter
	require.NoError(t, adapter.LoadServiceDefinitions())

	// Test API methods
	serviceClass, err := adapter.GetServiceClass("test_simple")
	require.NoError(t, err)
	assert.Equal(t, "test_simple", serviceClass.Name)
	assert.Equal(t, "simple_service", serviceClass.Type)

	// Test availability check
	assert.True(t, adapter.IsServiceClassAvailable("test_simple"))

	// Test listing
	classes := adapter.ListServiceClasses()
	assert.Len(t, classes, 1)
	assert.Equal(t, "test_simple", classes[0].Name)
	assert.True(t, classes[0].Available)

	// Test create and delete tool info through adapter
	createTool, createArgs, createMapping, err := adapter.GetCreateTool("test_simple")
	require.NoError(t, err)
	assert.Equal(t, "test_create_tool", createTool)
	assert.NotNil(t, createArgs)
	assert.Equal(t, "$.id", createMapping["serviceId"])

	deleteTool, deleteArgs, deleteMapping, err := adapter.GetDeleteTool("test_simple")
	require.NoError(t, err)
	assert.Equal(t, "test_delete_tool", deleteTool)
	assert.NotNil(t, deleteArgs)
	assert.Equal(t, "$.status", deleteMapping["status"])
}

// TestServiceClassErrorHandling tests various error scenarios
func TestServiceClassErrorHandling(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write an invalid ServiceClass definition (missing required fields)
	invalidYAML := `name: test_invalid
# Missing type and other required fields
description: "Invalid service class for testing"
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service_invalid.yaml"), []byte(invalidYAML), 0644))

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{},
	}

	// Create ServiceClass manager
	manager := NewServiceClassManager(tmpDir, mockChecker)

	// Loading should succeed but invalid definitions should be skipped
	require.NoError(t, manager.LoadServiceDefinitions())

	// Invalid ServiceClass should not be available
	assert.False(t, manager.IsServiceClassAvailable("test_invalid"))

	// Test getting non-existent ServiceClass
	_, exists := manager.GetServiceClassDefinition("non_existent")
	assert.False(t, exists)

	// Test API adapter error handling
	adapter := NewAdapter(manager)

	// Test getting non-existent ServiceClass through API
	_, err := adapter.GetServiceClass("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test getting create tool for non-existent ServiceClass
	_, _, _, err = adapter.GetCreateTool("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestServiceClassToolProviderIntegration tests the ToolProvider functionality
func TestServiceClassToolProviderIntegration(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write a ServiceClass definition
	testYAML := `name: test_tool_provider
type: tool_provider_test
version: "1.0.0"
description: "Test tool provider service class"

serviceConfig:
  serviceType: "ToolProviderTest"
  defaultLabel: "test"
  
  lifecycleTools:
    create:
      tool: "test_tool"
      arguments:
        param: "value"
      responseMapping:
        serviceId: "$.id"
        status: "$.status"
    delete:
      tool: "test_delete"
      arguments:
        id: "{{ .service_id }}"
      responseMapping:
        status: "$.status"

metadata:
  provider: "test"
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "service_tool_provider.yaml"), []byte(testYAML), 0644))

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"test_tool":   true,
			"test_delete": true,
		},
	}

	// Create ServiceClass manager and adapter
	manager := NewServiceClassManager(tmpDir, mockChecker)
	adapter := NewAdapter(manager)

	// Load definitions
	require.NoError(t, adapter.LoadServiceDefinitions())

	// Test ToolProvider interface
	tools := adapter.GetTools()
	assert.Greater(t, len(tools), 0)

	// Should have serviceclass_list tool
	hasListTool := false
	for _, tool := range tools {
		if tool.Name == "serviceclass_list" {
			hasListTool = true
			break
		}
	}
	assert.True(t, hasListTool, "Should have serviceclass_list tool")

	// Test executing a tool
	ctx := context.Background()
	result, err := adapter.ExecuteTool(ctx, "serviceclass_list", map[string]interface{}{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.NotNil(t, result.Content)
}
