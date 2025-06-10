package capability

import (
	"context"
	"envctl/internal/api"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	
	// Return a successful result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(`{"status": "success", "tool": "` + toolName + `"}`),
		},
		IsError: false,
	}, nil
}

// TestPortForwardCapabilityIntegration tests the complete integration of port forwarding capability
func TestPortForwardCapabilityIntegration(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write the port forward capability definition
	portForwardYAML := `name: portforward_provider
type: portforward_provider
version: "1.0.0"
description: "Port forwarding capability for Kubernetes services and pods"
metadata:
  provider: "kubectl"
  category: "networking"
operations:
  create:
    description: "Create a new port forward to a Kubernetes resource"
    parameters:
      namespace:
        type: string
        required: true
        description: "Kubernetes namespace"
      resource_type:
        type: string
        required: true
        description: "Type of resource"
      resource_name:
        type: string
        required: true
        description: "Name of the resource"
      local_port:
        type: string
        required: true
        description: "Local port"
      remote_port:
        type: string
        required: true
        description: "Remote port"
    requires:
      - x_k8s_port_forward
    workflow:
      name: portforward_create
      description: "Create a port forward"
      steps:
        - id: create_forward
          tool: x_k8s_port_forward
          args:
            namespace: "{{ .input.namespace }}"
  stop:
    description: "Stop a port forward"
    requires:
      - x_k8s_port_forward_stop
    workflow:
      name: portforward_stop
  list:
    description: "List port forwards"
    requires:
      - x_k8s_port_forward_list
    workflow:
      name: portforward_list
  info:
    description: "Get port forward info"
    requires:
      - x_k8s_port_forward_info
    workflow:
      name: portforward_info
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "portforward_provider.yaml"), []byte(portForwardYAML), 0644))

	// Create mock tool checker with kubectl tools available
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_k8s_port_forward":      true,
			"x_k8s_port_forward_stop": true,
			"x_k8s_port_forward_list": true,
			"x_k8s_port_forward_info": true,
		},
	}

	// Create mock tool caller
	mockCaller := &mockToolCaller{}

	// Create capability adapter
	adapter := NewAdapter(tmpDir, mockChecker, mockCaller)

	// Load definitions
	require.NoError(t, adapter.LoadDefinitions())

	// Test that port forwarding capability is available
	assert.True(t, adapter.IsCapabilityAvailable("portforward_provider", "create"))
	assert.True(t, adapter.IsCapabilityAvailable("portforward_provider", "stop"))
	assert.True(t, adapter.IsCapabilityAvailable("portforward_provider", "list"))
	assert.True(t, adapter.IsCapabilityAvailable("portforward_provider", "info"))

	// Test listing capabilities includes port forwarding
	capabilities := adapter.ListCapabilities()
	var portForwardCap *api.CapabilityInfo
	for i := range capabilities {
		if capabilities[i].Type == "portforward_provider" {
			portForwardCap = &capabilities[i]
			break
		}
	}
	require.NotNil(t, portForwardCap, "Port forward capability should be listed")
	assert.Equal(t, "Port forwarding capability for Kubernetes services and pods", portForwardCap.Description)
	assert.Len(t, portForwardCap.Operations, 4) // create, stop, list, info

	// Test executing port forward create operation
	ctx := context.Background()
	result, err := adapter.ExecuteCapability(ctx, "portforward_provider", "create", map[string]interface{}{
		"namespace":     "default",
		"resource_type": "service",
		"resource_name": "my-service",
		"local_port":    "8080",
		"remote_port":   "80",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify the workflow was executed
	assert.Len(t, mockCaller.calls, 1)
	assert.Equal(t, "action_portforward_create", mockCaller.calls[0].toolName)

	// Test that tools are exposed correctly
	tools := adapter.GetTools()
	
	// Should have management tools
	hasCapabilityList := false
	hasPortForwardCreate := false
	
	for _, tool := range tools {
		if tool.Name == "capability_list" {
			hasCapabilityList = true
		}
		if tool.Name == "portforward_provider_create" {
			hasPortForwardCreate = true
		}
	}
	
	assert.True(t, hasCapabilityList, "Should have capability_list tool")
	assert.True(t, hasPortForwardCreate, "Should have portforward_provider_create tool")
}

// TestPortForwardCapabilityWithMissingTools tests behavior when kubectl tools are not available
func TestPortForwardCapabilityWithMissingTools(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir := t.TempDir()

	// Write the port forward capability definition
	portForwardYAML := `name: portforward_provider
type: portforward_provider
version: "1.0.0"
description: "Port forwarding capability"
operations:
  create:
    description: "Create port forward"
    requires:
      - x_k8s_port_forward
    workflow:
      name: portforward_create
`

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "portforward_provider.yaml"), []byte(portForwardYAML), 0644))

	// Create mock tool checker with no tools available
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{},
	}

	// Create mock tool caller
	mockCaller := &mockToolCaller{}

	// Create capability adapter
	adapter := NewAdapter(tmpDir, mockChecker, mockCaller)

	// Load definitions
	require.NoError(t, adapter.LoadDefinitions())

	// Test that port forwarding capability is NOT available
	assert.False(t, adapter.IsCapabilityAvailable("portforward_provider", "create"))

	// Test listing capabilities does not include port forwarding operations
	capabilities := adapter.ListCapabilities()
	for _, cap := range capabilities {
		if cap.Type == "portforward_provider" {
			// The capability might be listed but operations should not be available
			for _, op := range cap.Operations {
				assert.False(t, op.Available, "Operation %s should not be available without tools", op.Name)
			}
		}
	}

	// Test executing port forward create operation fails
	ctx := context.Background()
	_, err := adapter.ExecuteCapability(ctx, "portforward_provider", "create", map[string]interface{}{
		"namespace":     "default",
		"resource_type": "service",
		"resource_name": "my-service",
		"local_port":    "8080",
		"remote_port":   "80",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}
