package controller

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestHandleMCPToolsLoadedMsg(t *testing.T) {
	// Create a test model
	m := &model.Model{
		MCPTools:       make(map[string][]api.MCPTool),
		CurrentAppMode: model.ModeMcpToolsOverlay,
		ActivityLog:    []string{},
	}

	// Create test tools
	tools := []api.MCPTool{
		{Name: "tool1", Description: "Tool 1 description"},
		{Name: "tool2", Description: "Tool 2 description"},
	}

	// Create message
	msg := model.MCPToolsLoadedMsg{
		ServerName: "test-server",
		Tools:      tools,
	}

	// Handle the message
	updatedModel, cmd := Update(msg, m)

	// Verify the tools were stored
	assert.Equal(t, tools, updatedModel.MCPTools["test-server"])

	// Verify no command was returned
	assert.Nil(t, cmd)
}

func TestHandleMCPToolsErrorMsg(t *testing.T) {
	// Create a test model
	m := &model.Model{
		MCPTools:       make(map[string][]api.MCPTool),
		CurrentAppMode: model.ModeMcpToolsOverlay,
		ActivityLog:    []string{},
	}

	// Create error message
	msg := model.MCPToolsErrorMsg{
		ServerName: "test-server",
		Error:      errors.New("connection failed"),
	}

	// Handle the message
	updatedModel, cmd := Update(msg, m)

	// Verify empty tools list was stored
	assert.Equal(t, []api.MCPTool{}, updatedModel.MCPTools["test-server"])

	// Verify error was logged
	assert.Len(t, updatedModel.ActivityLog, 1)
	assert.Contains(t, updatedModel.ActivityLog[0], "Failed to fetch tools for test-server")
	assert.Contains(t, updatedModel.ActivityLog[0], "connection failed")

	// Verify activity log is marked dirty
	assert.True(t, updatedModel.ActivityLogDirty)

	// Verify no command was returned
	assert.Nil(t, cmd)
}

func TestMKeyPressTriggersToolsFetch(t *testing.T) {
	// Create a test model with MCP servers
	m := &model.Model{
		CurrentAppMode: model.ModeMainDashboard,
		MCPTools:       make(map[string][]api.MCPTool),
		MCPServers: map[string]*api.MCPServerInfo{
			"server1": {
				Name:  "server1",
				State: "Running",
				Port:  8080,
			},
			"server2": {
				Name:  "server2",
				State: "Stopped",
				Port:  8081,
			},
			"server3": {
				Name:  "server3",
				State: "Running",
				Port:  8082,
			},
		},
		MCPServiceAPI: &mockMCPServiceAPI{},
	}

	// Create key message for "M"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}}

	// Handle the key press
	updatedModel, cmd := Update(msg, m)

	// Verify mode changed to tools overlay
	assert.Equal(t, model.ModeMcpToolsOverlay, updatedModel.CurrentAppMode)

	// Verify MCPTools was cleared
	assert.Empty(t, updatedModel.MCPTools)

	// Verify a batch command was returned
	assert.NotNil(t, cmd)

	// Execute the command to verify it creates the right messages
	// Note: In a real test, we'd need to mock the MCPServiceAPI
	// and verify that GetTools is called for running servers only
}

// mockMCPServiceAPI is a simple mock for testing
type mockMCPServiceAPI struct {
	tools       []api.MCPTool
	getToolsErr error
}

func (m *mockMCPServiceAPI) GetServerInfo(ctx context.Context, label string) (*api.MCPServerInfo, error) {
	return nil, nil
}

func (m *mockMCPServiceAPI) ListServers(ctx context.Context) ([]*api.MCPServerInfo, error) {
	return nil, nil
}

func (m *mockMCPServiceAPI) GetTools(ctx context.Context, serverName string) ([]api.MCPTool, error) {
	if m.getToolsErr != nil {
		return nil, m.getToolsErr
	}
	return m.tools, nil
}

func (m *mockMCPServiceAPI) GetAllTools(ctx context.Context) ([]api.MCPTool, error) {
	// For tests, just return the same tools
	if m.getToolsErr != nil {
		return nil, m.getToolsErr
	}
	return m.tools, nil
}
