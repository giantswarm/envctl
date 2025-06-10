package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"envctl/internal/api"
	"envctl/internal/config"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestConfigReloadIntegration(t *testing.T) {
	t.Skip("Skipping integration test - needs proper mocking of config paths")

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "envctl-config-reload-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	capabilityDir := filepath.Join(tmpDir, "capability", "definitions")
	err = os.MkdirAll(capabilityDir, 0755)
	assert.NoError(t, err)

	// Create initial configuration
	initialConfig := &config.EnvctlConfig{
		Clusters: []config.ClusterDefinition{
			{Name: "test-cluster", Context: "test-context", Role: config.ClusterRoleTarget},
		},
		ActiveClusters: map[config.ClusterRole]string{
			config.ClusterRoleTarget: "test-cluster",
		},
		MCPServers: []config.MCPServerDefinition{
			{Name: "test-server", Type: config.MCPServerTypeLocalCommand},
		},
	}

	// Write initial config
	data, err := yaml.Marshal(initialConfig)
	assert.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	assert.NoError(t, err)

	// Create adapter
	adapter := NewConfigAdapter(initialConfig, configPath)
	api.RegisterConfig(adapter)

	ctx := context.Background()

	// Verify initial config
	cfg, err := adapter.GetConfig(ctx)
	assert.NoError(t, err)
	assert.Len(t, cfg.MCPServers, 1)
	assert.Equal(t, "test-server", cfg.MCPServers[0].Name)

	// Modify the config file directly
	modifiedConfig := &config.EnvctlConfig{
		Clusters: []config.ClusterDefinition{
			{Name: "test-cluster", Context: "test-context", Role: config.ClusterRoleTarget},
			{Name: "new-cluster", Context: "new-context", Role: config.ClusterRoleObservability},
		},
		ActiveClusters: map[config.ClusterRole]string{
			config.ClusterRoleTarget:        "test-cluster",
			config.ClusterRoleObservability: "new-cluster",
		},
		MCPServers: []config.MCPServerDefinition{
			{Name: "test-server", Type: config.MCPServerTypeLocalCommand},
			{Name: "new-server", Type: config.MCPServerTypeContainer},
		},
		PortForwards: []config.PortForwardDefinition{
			{Name: "new-forward", Namespace: "test"},
		},
	}

	// Write modified config
	data, err = yaml.Marshal(modifiedConfig)
	assert.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	assert.NoError(t, err)

	// Ensure file is written (some filesystems have delays)
	time.Sleep(10 * time.Millisecond)

	// Reload configuration
	err = adapter.ReloadConfig(ctx)
	assert.NoError(t, err)

	// Verify config has been reloaded
	cfg, err = adapter.GetConfig(ctx)
	assert.NoError(t, err)
	assert.Len(t, cfg.Clusters, 2)
	assert.Len(t, cfg.MCPServers, 2)
	assert.Len(t, cfg.PortForwards, 1)
	assert.Equal(t, "new-cluster", cfg.Clusters[1].Name)
	assert.Equal(t, "new-server", cfg.MCPServers[1].Name)
	assert.Equal(t, "new-forward", cfg.PortForwards[0].Name)
}

func TestConfigReloadTool(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "envctl-config-reload-tool-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create initial configuration
	initialConfig := &config.EnvctlConfig{
		Clusters:       []config.ClusterDefinition{},
		ActiveClusters: make(map[config.ClusterRole]string),
	}

	// Write initial config
	data, err := yaml.Marshal(initialConfig)
	assert.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	assert.NoError(t, err)

	// Create adapter
	adapter := NewConfigAdapter(initialConfig, configPath)
	api.RegisterConfig(adapter)

	// Test that config_reload tool exists
	tools := adapter.GetTools()
	found := false
	for _, tool := range tools {
		if tool.Name == "config_reload" {
			found = true
			assert.Equal(t, "Reload configuration from disk including capability definitions", tool.Description)
			break
		}
	}
	assert.True(t, found, "config_reload tool should exist")

	// Execute the tool
	ctx := context.Background()
	result, err := adapter.ExecuteTool(ctx, "config_reload", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Equal(t, "Configuration reloaded successfully", result.Content[0])
}
