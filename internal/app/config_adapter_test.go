package app

import (
	"os"
	"path/filepath"
	"testing"

	"envctl/internal/config"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestConfigAdapter(t *testing.T) {
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
		cfg, err := adapter.GetConfig()
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
		err := adapter.UpdateMCPServer(newServer)
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig()
		assert.Len(t, cfg.MCPServers, 2)

		// Update existing
		updatedServer := config.MCPServerDefinition{
			Name:  "test-server",
			Type:  config.MCPServerTypeContainer,
			Image: "test-image",
		}
		err = adapter.UpdateMCPServer(updatedServer)
		assert.NoError(t, err)

		cfg, _ = adapter.GetConfig()
		assert.Len(t, cfg.MCPServers, 2)
		for _, s := range cfg.MCPServers {
			if s.Name == "test-server" {
				assert.Equal(t, config.MCPServerTypeContainer, s.Type)
				assert.Equal(t, "test-image", s.Image)
			}
		}
	})

	t.Run("DeleteMCPServer", func(t *testing.T) {
		err := adapter.DeleteMCPServer("new-server")
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig()
		assert.Len(t, cfg.MCPServers, 1)

		// Delete non-existent
		err = adapter.DeleteMCPServer("non-existent")
		assert.Error(t, err)
	})

	t.Run("UpdateClusters", func(t *testing.T) {
		newClusters := []config.ClusterDefinition{
			{Name: "cluster1", Context: "context1", Role: config.ClusterRoleTarget},
			{Name: "cluster2", Context: "context2", Role: config.ClusterRoleObservability},
		}
		err := adapter.UpdateClusters(newClusters)
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig()
		assert.Len(t, cfg.Clusters, 2)
	})

	t.Run("UpdateActiveClusters", func(t *testing.T) {
		activeClusters := map[config.ClusterRole]string{
			config.ClusterRoleTarget:        "cluster1",
			config.ClusterRoleObservability: "cluster2",
		}
		err := adapter.UpdateActiveClusters(activeClusters)
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig()
		assert.Equal(t, "cluster1", cfg.ActiveClusters[config.ClusterRoleTarget])
		assert.Equal(t, "cluster2", cfg.ActiveClusters[config.ClusterRoleObservability])
	})

	t.Run("DeleteCluster", func(t *testing.T) {
		err := adapter.DeleteCluster("cluster1")
		assert.NoError(t, err)

		cfg, _ := adapter.GetConfig()
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
		err = adapter.SaveConfig()
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
}
