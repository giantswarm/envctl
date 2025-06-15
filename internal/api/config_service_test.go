package api

import (
	"context"
	"fmt"
	"testing"

	"envctl/internal/config"

	"github.com/stretchr/testify/assert"
)

// mockConfigHandler implements ConfigHandler for testing
type mockConfigHandler struct {
	config        *config.EnvctlConfig
	saveCount     int
	called        map[string]bool
	saveConfigErr error
}

// ToolProvider methods
func (m *mockConfigHandler) GetTools() []ToolMetadata {
	return []ToolMetadata{
		{
			Name:        "test_tool",
			Description: "Test tool for mock",
			Parameters:  []ParameterMetadata{},
		},
	}
}

func (m *mockConfigHandler) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*CallToolResult, error) {
	return &CallToolResult{
		Content: []interface{}{"test result"},
	}, nil
}

func (m *mockConfigHandler) GetConfig(ctx context.Context) (*config.EnvctlConfig, error) {
	return m.config, nil
}

func (m *mockConfigHandler) GetMCPServers(ctx context.Context) ([]MCPServerDefinition, error) {
	// MCPServers are now managed by MCPServerManager, return empty slice for backward compatibility
	return []MCPServerDefinition{}, nil
}

func (m *mockConfigHandler) GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error) {
	return &m.config.Aggregator, nil
}

func (m *mockConfigHandler) GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error) {
	return &m.config.GlobalSettings, nil
}

func (m *mockConfigHandler) UpdateMCPServer(ctx context.Context, server MCPServerDefinition) error {
	// MCPServers are now managed by MCPServerManager, return error
	return fmt.Errorf("MCP server configuration has moved to directory-based management via MCPServerManager")
}

func (m *mockConfigHandler) UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error {
	m.config.Aggregator = aggregator
	return nil
}

func (m *mockConfigHandler) UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error {
	m.config.GlobalSettings = settings
	return nil
}

func (m *mockConfigHandler) DeleteMCPServer(ctx context.Context, name string) error {
	// MCPServers are now managed by MCPServerManager, return error
	return fmt.Errorf("MCP server configuration has moved to directory-based management via MCPServerManager")
}

func (m *mockConfigHandler) SaveConfig(ctx context.Context) error {
	m.called["SaveConfig"] = true
	return m.saveConfigErr
}

func (m *mockConfigHandler) ReloadConfig(ctx context.Context) error {
	m.called["ReloadConfig"] = true
	return nil
}

func TestConfigServiceAPI(t *testing.T) {
	// Create a mock config - MCPServers field removed as it no longer exists
	mockCfg := &config.EnvctlConfig{
		Aggregator: config.AggregatorConfig{
			Port: 8080,
		},
		GlobalSettings: config.GlobalSettings{
			DefaultContainerRuntime: "docker",
		},
	}

	// Create mock handler
	mockHandler := &mockConfigHandler{
		config: mockCfg,
		called: make(map[string]bool),
	}

	// Register the mock handler
	RegisterConfigHandler(mockHandler)

	// Create the API
	api := NewConfigServiceAPI()
	ctx := context.Background()

	t.Run("GetConfig", func(t *testing.T) {
		cfg, err := api.GetConfig(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("GetMCPServers", func(t *testing.T) {
		servers, err := api.GetMCPServers(ctx)
		assert.NoError(t, err)
		// MCPServers now managed by MCPServerManager, expect empty result
		assert.Len(t, servers, 0)
	})

	t.Run("GetAggregatorConfig", func(t *testing.T) {
		aggConfig, err := api.GetAggregatorConfig(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 8080, aggConfig.Port)
	})

	t.Run("GetGlobalSettings", func(t *testing.T) {
		settings, err := api.GetGlobalSettings(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "docker", settings.DefaultContainerRuntime)
	})

	t.Run("UpdateMCPServer", func(t *testing.T) {
		newServer := MCPServerDefinition{
			Name: "new-server",
			Type: string(config.MCPServerTypeContainer),
		}
		err := api.UpdateMCPServer(ctx, newServer)
		// Expect error since MCP servers moved to MCPServerManager
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP server configuration has moved to directory-based management via MCPServerManager")

		// Verify servers list still empty
		servers, _ := api.GetMCPServers(ctx)
		assert.Len(t, servers, 0)
	})

	t.Run("DeleteMCPServer", func(t *testing.T) {
		err := api.DeleteMCPServer(ctx, "test-server")
		// Expect error since MCP servers moved to MCPServerManager
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MCP server configuration has moved to directory-based management via MCPServerManager")

		// Verify servers list still empty
		servers, _ := api.GetMCPServers(ctx)
		assert.Len(t, servers, 0)
	})

	t.Run("SaveConfig", func(t *testing.T) {
		err := api.SaveConfig(ctx)
		assert.NoError(t, err)
		assert.True(t, mockHandler.called["SaveConfig"])
	})

	t.Run("ReloadConfig", func(t *testing.T) {
		err := api.ReloadConfig(ctx)
		assert.NoError(t, err)
		assert.True(t, mockHandler.called["ReloadConfig"])
	})
}

func TestConfigServiceAPINoHandler(t *testing.T) {
	// Unregister any existing handler
	RegisterConfigHandler(nil)

	api := NewConfigServiceAPI()
	ctx := context.Background()

	_, err := api.GetConfig(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config handler not registered")
}
