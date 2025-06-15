package mcpserver

import (
	"testing"
)

// TestErrorHandling tests that errors are properly reported when starting a server with invalid command
func TestErrorHandling(t *testing.T) {
	// Test that errors are properly reported
	serverCfg := MCPServerDefinition{
		Name:    "error-test-server",
		Command: []string{"/non/existent/command"},
		Type:    MCPServerTypeLocalCommand,
	}

	var reportedError error
	updateFn := func(update McpDiscreteStatusUpdate) {
		if update.ProcessErr != nil {
			reportedError = update.ProcessErr
		}
	}

	_, err := StartAndManageIndividualMcpServer(serverCfg, updateFn, nil)

	if err == nil {
		t.Fatal("Expected error for non-existent command")
	}

	// Should have reported an error through the update function
	if reportedError == nil {
		t.Error("Expected error to be reported through update function")
	}
}

// TestStartAndManageIndividualMcpServer tests basic server creation
// Note: This test is limited because the mark3labs/mcp-go library handles process management internally
func TestStartAndManageIndividualMcpServer(t *testing.T) {
	t.Skip("Skipping test that requires real MCP server - mark3labs/mcp-go handles process management internally")
}

// TestPipeFails is no longer applicable since mark3labs/mcp-go handles pipe creation internally
func TestPipeFails(t *testing.T) {
	t.Skip("Skipping test - mark3labs/mcp-go handles pipe creation internally")
}
