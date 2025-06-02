package aggregator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPClient implements MCPClient for testing
type mockMCPClient struct {
	initialized bool
	tools       []mcp.Tool
	resources   []mcp.Resource
	prompts     []mcp.Prompt
	pingErr     error
	closed      bool
}

func (m *mockMCPClient) Initialize(ctx context.Context) error {
	if m.initialized {
		return errors.New("already initialized")
	}
	m.initialized = true
	return nil
}

func (m *mockMCPClient) Close() error {
	if m.closed {
		return errors.New("already closed")
	}
	m.closed = true
	return nil
}

func (m *mockMCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	if !m.initialized {
		return nil, errors.New("not initialized")
	}
	return m.tools, nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if !m.initialized {
		return nil, errors.New("not initialized")
	}
	// Find the tool
	for _, tool := range m.tools {
		if tool.Name == name {
			// Return a minimal result - the actual structure will be filled by the mcp-go library
			return &mcp.CallToolResult{}, nil
		}
	}
	return nil, errors.New("tool not found")
}

func (m *mockMCPClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	if !m.initialized {
		return nil, errors.New("not initialized")
	}
	return m.resources, nil
}

func (m *mockMCPClient) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	if !m.initialized {
		return nil, errors.New("not initialized")
	}
	// Find the resource
	for _, res := range m.resources {
		if res.URI == uri {
			// Return a minimal result
			return &mcp.ReadResourceResult{
				Contents: []mcp.ResourceContents{},
			}, nil
		}
	}
	return nil, errors.New("resource not found")
}

func (m *mockMCPClient) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	if !m.initialized {
		return nil, errors.New("not initialized")
	}
	return m.prompts, nil
}

func (m *mockMCPClient) GetPrompt(ctx context.Context, name string, args map[string]interface{}) (*mcp.GetPromptResult, error) {
	if !m.initialized {
		return nil, errors.New("not initialized")
	}
	// Find the prompt
	for _, prompt := range m.prompts {
		if prompt.Name == name {
			// Return a minimal result
			return &mcp.GetPromptResult{}, nil
		}
	}
	return nil, errors.New("prompt not found")
}

func (m *mockMCPClient) Ping(ctx context.Context) error {
	if !m.initialized {
		return errors.New("not initialized")
	}
	return m.pingErr
}

func TestAggregatorServer_HandlerTracking(t *testing.T) {
	ctx := context.Background()
	config := AggregatorConfig{
		Host: "localhost",
		Port: 0, // Use any available port
	}

	server := NewAggregatorServer(config)
	require.NotNil(t, server)

	// Start the server
	err := server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Create mock clients with tools
	client1 := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "tool1", Description: "Tool 1"},
			{Name: "shared-tool", Description: "Shared tool"},
		},
		resources: []mcp.Resource{
			{URI: "resource1", Name: "Resource 1"},
		},
		prompts: []mcp.Prompt{
			{Name: "prompt1", Description: "Prompt 1"},
		},
	}

	client2 := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "tool2", Description: "Tool 2"},
			{Name: "shared-tool", Description: "Shared tool from server2"},
		},
	}

	// Register servers
	err = server.RegisterServer(ctx, "server1", client1)
	assert.NoError(t, err)

	err = server.RegisterServer(ctx, "server2", client2)
	assert.NoError(t, err)

	// Give the registry update a moment to process
	// This is needed because updateCapabilities runs in a goroutine
	time.Sleep(50 * time.Millisecond)

	// Verify tools are available
	tools := server.GetTools()
	assert.Len(t, tools, 4) // tool1, tool2, and shared-tool from each server (prefixed)

	// Verify active tools are tracked
	server.mu.RLock()
	assert.True(t, server.activeTools["tool1"])
	assert.True(t, server.activeTools["tool2"])
	assert.True(t, server.activeTools["server1.shared-tool"])
	assert.True(t, server.activeTools["server2.shared-tool"])
	server.mu.RUnlock()

	// Deregister server1
	err = server.DeregisterServer("server1")
	assert.NoError(t, err)

	// Give the registry update a moment to process
	time.Sleep(50 * time.Millisecond)

	// Verify tools from server1 are no longer active
	server.mu.RLock()
	assert.False(t, server.activeTools["tool1"])
	assert.False(t, server.activeTools["server1.shared-tool"])
	assert.True(t, server.activeTools["tool2"])
	// After deregistering server1, shared-tool should no longer be prefixed for server2
	assert.True(t, server.activeTools["shared-tool"])
	server.mu.RUnlock()

	// Verify only server2 tools are available
	tools = server.GetTools()
	assert.Len(t, tools, 2) // tool2 and shared-tool (no longer prefixed since only one server has it)

	// Verify the shared tool is now unprefixed
	hasUnprefixedSharedTool := false
	for _, tool := range tools {
		if tool.Name == "shared-tool" {
			hasUnprefixedSharedTool = true
			break
		}
	}
	assert.True(t, hasUnprefixedSharedTool, "shared-tool should be unprefixed after server1 is removed")
}

func TestAggregatorServer_InitialRegistration(t *testing.T) {
	ctx := context.Background()
	config := AggregatorConfig{
		Host: "localhost",
		Port: 0,
	}

	server := NewAggregatorServer(config)
	require.NotNil(t, server)

	// Create a mock client with tools before starting the server
	client := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "test-tool", Description: "Test tool"},
		},
	}

	// Start the server
	err := server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Register a server - this should immediately update capabilities
	err = server.RegisterServer(ctx, "test-server", client)
	assert.NoError(t, err)

	// Wait for the asynchronous update to complete
	time.Sleep(50 * time.Millisecond)

	// The tools should be available
	tools := server.GetTools()
	assert.Len(t, tools, 1)
	assert.Equal(t, "test-tool", tools[0].Name)

	// Verify the tool is marked as active
	server.mu.RLock()
	assert.True(t, server.activeTools["test-tool"])
	server.mu.RUnlock()
}

func TestAggregatorServer_EmptyStart(t *testing.T) {
	ctx := context.Background()
	config := AggregatorConfig{
		Host: "localhost",
		Port: 0,
	}

	server := NewAggregatorServer(config)
	require.NotNil(t, server)

	// Start the server with no registered servers
	err := server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Should have no tools initially
	tools := server.GetTools()
	assert.Len(t, tools, 0)

	// Active tools map should be empty
	server.mu.RLock()
	assert.Len(t, server.activeTools, 0)
	server.mu.RUnlock()

	// Now register a server
	client := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "late-tool", Description: "Tool added after start"},
		},
	}

	err = server.RegisterServer(ctx, "late-server", client)
	assert.NoError(t, err)

	// Tool should now be available
	tools = server.GetTools()
	assert.Len(t, tools, 1)
	assert.Equal(t, "late-tool", tools[0].Name)
}

func TestAggregatorServer_HandlerExecution(t *testing.T) {
	ctx := context.Background()
	config := AggregatorConfig{
		Host: "localhost",
		Port: 0,
	}

	server := NewAggregatorServer(config)
	require.NotNil(t, server)

	// Start the server
	err := server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Create and register a mock client
	client := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "exec-tool", Description: "Tool for execution test"},
		},
	}

	err = server.RegisterServer(ctx, "exec-server", client)
	assert.NoError(t, err)

	// Wait for the asynchronous update to complete
	time.Sleep(50 * time.Millisecond)

	// Get the tool handler (we can't directly test it, but we can verify it's set up)
	tools := server.GetTools()
	require.Len(t, tools, 1)
	assert.Equal(t, "exec-tool", tools[0].Name)

	// Verify the tool is active
	server.mu.RLock()
	assert.True(t, server.activeTools["exec-tool"])
	server.mu.RUnlock()

	// Deregister the server
	err = server.DeregisterServer("exec-server")
	assert.NoError(t, err)

	// Wait for the asynchronous update to complete
	time.Sleep(50 * time.Millisecond)

	// Tool should no longer be active
	server.mu.RLock()
	assert.False(t, server.activeTools["exec-tool"])
	server.mu.RUnlock()
}

func TestAggregatorServer_ToolsRemovedOnServerStop(t *testing.T) {
	// This test specifically verifies that tools are removed when an MCP server stops
	ctx := context.Background()
	config := AggregatorConfig{
		Host: "localhost",
		Port: 0,
	}

	server := NewAggregatorServer(config)
	require.NotNil(t, server)

	// Start the server
	err := server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	// Create and register two MCP servers
	client1 := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "tool1", Description: "Tool from server 1"},
			{Name: "tool2", Description: "Another tool from server 1"},
		},
	}

	client2 := &mockMCPClient{
		tools: []mcp.Tool{
			{Name: "tool3", Description: "Tool from server 2"},
		},
	}

	// Register both servers
	err = server.RegisterServer(ctx, "server1", client1)
	assert.NoError(t, err)

	err = server.RegisterServer(ctx, "server2", client2)
	assert.NoError(t, err)

	// Wait for updates
	time.Sleep(50 * time.Millisecond)

	// Verify all tools are available
	tools := server.GetTools()
	assert.Len(t, tools, 3, "Should have 3 tools total")

	// Find tool names
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}
	assert.True(t, toolNames["tool1"])
	assert.True(t, toolNames["tool2"])
	assert.True(t, toolNames["tool3"])

	// Now stop server1 by deregistering it
	err = server.DeregisterServer("server1")
	assert.NoError(t, err)

	// Wait for updates
	time.Sleep(50 * time.Millisecond)

	// Verify only server2's tools remain
	tools = server.GetTools()
	assert.Len(t, tools, 1, "Should have only 1 tool after server1 is removed")
	assert.Equal(t, "tool3", tools[0].Name)

	// Verify server1's tools are no longer active
	server.mu.RLock()
	assert.False(t, server.activeTools["tool1"], "tool1 should not be active")
	assert.False(t, server.activeTools["tool2"], "tool2 should not be active")
	assert.True(t, server.activeTools["tool3"], "tool3 should still be active")
	server.mu.RUnlock()
}
