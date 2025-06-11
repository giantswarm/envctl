package api

import (
	"context"
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

func (m *mockConfigHandler) GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error) {
	return m.config.MCPServers, nil
}

func (m *mockConfigHandler) GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error) {
	return m.config.Workflows, nil
}

func (m *mockConfigHandler) GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error) {
	return &m.config.Aggregator, nil
}

func (m *mockConfigHandler) GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error) {
	return &m.config.GlobalSettings, nil
}

func (m *mockConfigHandler) UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error {
	// Find and update or add
	found := false
	for i, s := range m.config.MCPServers {
		if s.Name == server.Name {
			m.config.MCPServers[i] = server
			found = true
			break
		}
	}
	if !found {
		m.config.MCPServers = append(m.config.MCPServers, server)
	}
	return nil
}

func (m *mockConfigHandler) UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error {
	// Find and update or add
	found := false
	for i, w := range m.config.Workflows {
		if w.Name == workflow.Name {
			m.config.Workflows[i] = workflow
			found = true
			break
		}
	}
	if !found {
		m.config.Workflows = append(m.config.Workflows, workflow)
	}
	return nil
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
	servers := []config.MCPServerDefinition{}
	for _, s := range m.config.MCPServers {
		if s.Name != name {
			servers = append(servers, s)
		}
	}
	m.config.MCPServers = servers
	return nil
}

func (m *mockConfigHandler) DeleteWorkflow(ctx context.Context, name string) error {
	workflows := []config.WorkflowDefinition{}
	for _, w := range m.config.Workflows {
		if w.Name != name {
			workflows = append(workflows, w)
		}
	}
	m.config.Workflows = workflows
	return nil
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
	// Create a mock config
	mockCfg := &config.EnvctlConfig{
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
		assert.Len(t, servers, 1)
		assert.Equal(t, "test-server", servers[0].Name)
	})

	t.Run("GetWorkflows", func(t *testing.T) {
		workflows, err := api.GetWorkflows(ctx)
		assert.NoError(t, err)
		assert.Len(t, workflows, 1)
		assert.Equal(t, "test-workflow", workflows[0].Name)
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
		newServer := config.MCPServerDefinition{
			Name: "new-server",
			Type: config.MCPServerTypeContainer,
		}
		err := api.UpdateMCPServer(ctx, newServer)
		assert.NoError(t, err)

		servers, _ := api.GetMCPServers(ctx)
		assert.Len(t, servers, 2)
	})

	t.Run("DeleteMCPServer", func(t *testing.T) {
		err := api.DeleteMCPServer(ctx, "test-server")
		assert.NoError(t, err)

		servers, _ := api.GetMCPServers(ctx)
		assert.Len(t, servers, 1)
		assert.Equal(t, "new-server", servers[0].Name)
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
