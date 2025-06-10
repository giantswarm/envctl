package capability

import (
	"context"
	"testing"

	"envctl/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkflowExecutor implements workflow.ToolCaller for testing
type mockWorkflowExecutor struct {
	executedWorkflows map[string]map[string]interface{}
}

func (m *mockWorkflowExecutor) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if m.executedWorkflows == nil {
		m.executedWorkflows = make(map[string]map[string]interface{})
	}
	m.executedWorkflows[toolName] = args

	// Return mock success result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("Success"),
		},
		IsError: false,
	}, nil
}

func TestCapabilityIntegration_TeleportAuth(t *testing.T) {
	// Create a test directory with the teleport_auth capability
	definitionsPath := "definitions"

	// Create a mock tool checker that reports required tools as available
	toolChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_teleport_kube":   true,
			"x_teleport_status": true,
		},
	}

	// Create a mock workflow executor
	workflowExecutor := &mockWorkflowExecutor{}

	// Create the capability adapter
	adapter := NewAdapter(definitionsPath, toolChecker, workflowExecutor)

	// Register it with the API
	adapter.Register()
	defer func() { api.RegisterCapability(nil) }() // Clean up

	// Load definitions
	err := adapter.LoadDefinitions()
	// It's ok if loading fails due to missing files in test environment
	if err != nil {
		t.Logf("Skipping integration test - definitions not found: %v", err)
		t.Skip()
	}

	// Get the capability handler through the API
	handler := api.GetCapability()
	require.NotNil(t, handler)

	// List capabilities
	capabilities := handler.ListCapabilities()
	t.Logf("Found %d capabilities", len(capabilities))
	for _, cap := range capabilities {
		t.Logf("Capability: %s (type: %s) with %d operations", cap.Name, cap.Type, len(cap.Operations))
		for _, op := range cap.Operations {
			t.Logf("  Operation: %s (available: %v)", op.Name, op.Available)
		}
	}

	// Find teleport_auth capability
	var teleportCap *api.CapabilityInfo
	for _, cap := range capabilities {
		if cap.Name == "teleport_auth" {
			teleportCap = &cap
			break
		}
	}

	if teleportCap == nil {
		t.Skip("teleport_auth capability not found - skipping integration test")
	}

	// Verify capability properties
	assert.Equal(t, "auth_provider", teleportCap.Type)
	assert.Equal(t, "teleport_auth", teleportCap.Name)
	assert.Contains(t, teleportCap.Description, "Teleport")

	// Find login operation
	var loginOp *api.OperationInfo
	for _, op := range teleportCap.Operations {
		if op.Name == "login" {
			loginOp = &op
			break
		}
	}

	require.NotNil(t, loginOp, "login operation should exist")
	assert.True(t, loginOp.Available, "login operation should be available")

	// Test executing the capability
	ctx := context.Background()
	result, err := handler.ExecuteCapability(ctx, "auth_provider", "login", map[string]interface{}{
		"cluster": "test-cluster",
		"ttl":     "8h",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify the workflow was executed
	assert.NotEmpty(t, workflowExecutor.executedWorkflows)
}
