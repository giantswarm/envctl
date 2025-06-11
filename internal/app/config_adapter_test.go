package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"envctl/internal/api"
	"envctl/internal/config"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestConfigAdapter(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "envctl-config-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test configuration
	testConfig := &config.EnvctlConfig{
		MCPServers: []config.MCPServerDefinition{
			{Name: "test-server", Type: config.MCPServerTypeLocalCommand},
		},
		Workflows: []config.WorkflowDefinition{
			{Name: "test-workflow"},
		},
		Aggregator: config.AggregatorConfig{
			Port: 8080,
		},
		GlobalSettings: config.GlobalSettings{
			DefaultContainerRuntime: "docker",
		},
	}

	// Create adapter with temporary config path to avoid creating files in working directory
	testConfigPath := filepath.Join(tmpDir, "config.yaml")
	adapter := NewConfigAdapter(testConfig, testConfigPath)

	t.Run("GetConfig", func(t *testing.T) {
		cfg, err := adapter.GetConfig(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Len(t, cfg.MCPServers, 1)
		assert.Equal(t, "test-server", cfg.MCPServers[0].Name)
	})

	t.Run("UpdateMCPServer", func(t *testing.T) {
		newServer := config.MCPServerDefinition{
			Name: "new-server",
			Type: config.MCPServerTypeContainer,
		}
		err := adapter.UpdateMCPServer(ctx, newServer)
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig(ctx)
		assert.Len(t, cfg.MCPServers, 2)

		// Update existing
		updatedServer := config.MCPServerDefinition{
			Name:  "test-server",
			Type:  config.MCPServerTypeContainer,
			Image: "test-image",
		}
		err = adapter.UpdateMCPServer(ctx, updatedServer)
		assert.NoError(t, err)

		cfg, _ = adapter.GetConfig(ctx)
		assert.Len(t, cfg.MCPServers, 2)
		for _, s := range cfg.MCPServers {
			if s.Name == "test-server" {
				assert.Equal(t, config.MCPServerTypeContainer, s.Type)
				assert.Equal(t, "test-image", s.Image)
			}
		}
	})

	t.Run("DeleteMCPServer", func(t *testing.T) {
		err := adapter.DeleteMCPServer(ctx, "new-server")
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig(ctx)
		assert.Len(t, cfg.MCPServers, 1)

		// Delete non-existent
		err = adapter.DeleteMCPServer(ctx, "non-existent")
		assert.Error(t, err)
	})

	t.Run("UpdateWorkflow", func(t *testing.T) {
		newWorkflow := config.WorkflowDefinition{
			Name:        "new-workflow",
			Description: "A new workflow",
		}
		err := adapter.UpdateWorkflow(ctx, newWorkflow)
		assert.NoError(t, err)

		workflows, _ := adapter.GetWorkflows(ctx)
		assert.Len(t, workflows, 2)
	})

	t.Run("UpdateGlobalSettings", func(t *testing.T) {
		newSettings := config.GlobalSettings{
			DefaultContainerRuntime: "containerd",
		}
		err := adapter.UpdateGlobalSettings(ctx, newSettings)
		assert.NoError(t, err)

		settings, _ := adapter.GetGlobalSettings(ctx)
		assert.Equal(t, "containerd", settings.DefaultContainerRuntime)
	})

	t.Run("SaveConfig", func(t *testing.T) {
		// Create a separate temporary directory for this test
		saveTestTmpDir, err := os.MkdirTemp("", "envctl-config-save-test")
		assert.NoError(t, err)
		defer os.RemoveAll(saveTestTmpDir)

		testPath := filepath.Join(saveTestTmpDir, "test-config.yaml")
		adapter.configPath = testPath

		// Save configuration
		err = adapter.SaveConfig(ctx)
		assert.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(testPath)
		assert.NoError(t, err)

		// Load and verify content
		data, err := os.ReadFile(testPath)
		assert.NoError(t, err)

		var loadedConfig config.EnvctlConfig
		err = yaml.Unmarshal(data, &loadedConfig)
		assert.NoError(t, err)
		assert.Len(t, loadedConfig.MCPServers, 1)
		assert.Equal(t, "test-server", loadedConfig.MCPServers[0].Name)
	})

	t.Run("GetTools", func(t *testing.T) {
		tools := adapter.GetTools()
		assert.Greater(t, len(tools), 0)

		// Check that all expected tools are present
		expectedTools := []string{
			"config_get",
			"config_update_mcp_server",
			"config_delete_mcp_server",
			"config_save",
		}

		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name] = true
		}

		for _, expected := range expectedTools {
			assert.True(t, toolNames[expected], "Expected tool %s not found", expected)
		}
	})

	t.Run("ExecuteTool", func(t *testing.T) {
		// Test config_get tool
		result, err := adapter.ExecuteTool(ctx, "config_get", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)

		// Test unknown tool
		result, err = adapter.ExecuteTool(ctx, "unknown_tool", nil)
		assert.Error(t, err)
		assert.Nil(t, result)

		// Test config_update_mcp_server with missing args
		result, err = adapter.ExecuteTool(ctx, "config_update_mcp_server", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsError)
	})

	t.Run("ReloadConfig", func(t *testing.T) {
		// Create adapter with test config and temporary path for this subtest
		reloadTestTmpDir, err := os.MkdirTemp("", "envctl-config-reload-test")
		assert.NoError(t, err)
		defer os.RemoveAll(reloadTestTmpDir)

		reloadTestConfigPath := filepath.Join(reloadTestTmpDir, "config.yaml")
		adapter := &ConfigAdapter{
			config: &config.EnvctlConfig{
				MCPServers: []config.MCPServerDefinition{},
			},
			configPath: reloadTestConfigPath,
		}
		api.RegisterConfig(adapter)

		// Mock the config reload to return success
		ctx := context.Background()

		// Call ReloadConfig
		err = adapter.ReloadConfig(ctx)
		assert.NoError(t, err, "ReloadConfig should not error when config is valid")
	})

	t.Run("ExecuteTool_config_reload", func(t *testing.T) {
		// Create adapter with temporary path for this subtest
		reloadToolTestTmpDir, err := os.MkdirTemp("", "envctl-config-reload-tool-test")
		assert.NoError(t, err)
		defer os.RemoveAll(reloadToolTestTmpDir)

		reloadToolTestConfigPath := filepath.Join(reloadToolTestTmpDir, "config.yaml")
		adapter := &ConfigAdapter{
			config: &config.EnvctlConfig{
				MCPServers: []config.MCPServerDefinition{},
			},
			configPath: reloadToolTestConfigPath,
		}
		api.RegisterConfig(adapter)

		// Execute config_reload tool
		ctx := context.Background()
		result, err := adapter.ExecuteTool(ctx, "config_reload", nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Configuration reloaded successfully", result.Content[0])
		assert.False(t, result.IsError)
	})
}
