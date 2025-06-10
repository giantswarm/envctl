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

	// Create a test configuration
	testConfig := &config.EnvctlConfig{
		Clusters: []config.ClusterDefinition{
			{Name: "test-cluster", Context: "test-context", Role: config.ClusterRoleTarget},
		},
		ActiveClusters: map[config.ClusterRole]string{
			config.ClusterRoleTarget: "test-cluster",
		},
		MCPServers: []config.MCPServerDefinition{
			{Name: "test-server", Type: config.MCPServerTypeLocalCommand},
		},
		PortForwards: []config.PortForwardDefinition{
			{Name: "test-forward", Namespace: "default"},
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

	// Create adapter
	adapter := NewConfigAdapter(testConfig, "")

	t.Run("GetConfig", func(t *testing.T) {
		cfg, err := adapter.GetConfig(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Len(t, cfg.Clusters, 1)
		assert.Equal(t, "test-cluster", cfg.Clusters[0].Name)
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

	t.Run("DeleteCluster", func(t *testing.T) {
		// First add some clusters
		testConfig.Clusters = []config.ClusterDefinition{
			{Name: "cluster1", Context: "context1", Role: config.ClusterRoleTarget},
			{Name: "cluster2", Context: "context2", Role: config.ClusterRoleObservability},
		}
		testConfig.ActiveClusters = map[config.ClusterRole]string{
			config.ClusterRoleTarget:        "cluster1",
			config.ClusterRoleObservability: "cluster2",
		}

		err := adapter.DeleteCluster(ctx, "cluster1")
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig(ctx)
		assert.Len(t, cfg.Clusters, 1)
		// Should also remove from active clusters
		_, exists := cfg.ActiveClusters[config.ClusterRoleTarget]
		assert.False(t, exists)
	})

	t.Run("SaveConfig", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir, err := os.MkdirTemp("", "envctl-config-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		testPath := filepath.Join(tmpDir, "test-config.yaml")
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
		assert.Len(t, loadedConfig.Clusters, 1)
		assert.Equal(t, "cluster2", loadedConfig.Clusters[0].Name)
	})

	t.Run("GetTools", func(t *testing.T) {
		tools := adapter.GetTools()
		assert.Greater(t, len(tools), 0)

		// Check that all expected tools are present
		expectedTools := []string{
			"config_get",
			"config_get_clusters",
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
		// Create adapter with test config
		adapter := &ConfigAdapter{
			config: &config.EnvctlConfig{
				Clusters:       []config.ClusterDefinition{},
				ActiveClusters: make(map[config.ClusterRole]string),
			},
		}
		api.RegisterConfig(adapter)

		// Mock the config reload to return success
		ctx := context.Background()

		// Call ReloadConfig
		err := adapter.ReloadConfig(ctx)
		assert.NoError(t, err, "ReloadConfig should not error when config is valid")
	})

	t.Run("ExecuteTool_config_reload", func(t *testing.T) {
		// Create adapter
		adapter := &ConfigAdapter{
			config: &config.EnvctlConfig{
				Clusters:       []config.ClusterDefinition{},
				ActiveClusters: make(map[config.ClusterRole]string),
			},
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
