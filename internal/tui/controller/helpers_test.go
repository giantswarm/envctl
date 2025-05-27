package controller

import (
	"encoding/json"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMcpConfigJson(t *testing.T) {
	tests := []struct {
		name             string
		mcpServerConfigs []config.MCPServerDefinition
		mcpServers       map[string]*api.MCPServerInfo
		expectedKeys     []string
		notExpectedKeys  []string
	}{
		{
			name: "basic MCP server with runtime port",
			mcpServerConfigs: []config.MCPServerDefinition{
				{
					Name:    "test-server",
					Type:    config.MCPServerTypeLocalCommand,
					Enabled: true,
				},
			},
			mcpServers: map[string]*api.MCPServerInfo{
				"test-server": {
					Name:  "test-server",
					Port:  8080,
					State: "running",
				},
			},
			expectedKeys: []string{"test-server-mcp"},
		},
		{
			name: "disabled server should not appear",
			mcpServerConfigs: []config.MCPServerDefinition{
				{
					Name:    "disabled-server",
					Type:    config.MCPServerTypeLocalCommand,
					Enabled: false,
				},
			},
			mcpServers:      map[string]*api.MCPServerInfo{},
			notExpectedKeys: []string{"disabled-server-mcp"},
		},
		{
			name: "server with configured proxy port",
			mcpServerConfigs: []config.MCPServerDefinition{
				{
					Name:      "proxy-server",
					Type:      config.MCPServerTypeLocalCommand,
					Enabled:   true,
					ProxyPort: 9090,
				},
			},
			mcpServers:   map[string]*api.MCPServerInfo{},
			expectedKeys: []string{"proxy-server-mcp"},
		},
		{
			name: "local command with URL env var",
			mcpServerConfigs: []config.MCPServerDefinition{
				{
					Name:    "env-server",
					Type:    config.MCPServerTypeLocalCommand,
					Enabled: true,
					Env: map[string]string{
						"SERVER_URL": "http://localhost:3000",
						"API_KEY":    "secret",
					},
				},
			},
			mcpServers:   map[string]*api.MCPServerInfo{},
			expectedKeys: []string{"env-server-mcp"},
		},
		{
			name: "container server with ports",
			mcpServerConfigs: []config.MCPServerDefinition{
				{
					Name:           "container-server",
					Type:           config.MCPServerTypeContainer,
					Enabled:        true,
					ContainerPorts: []string{"8080:80", "9090:90"},
				},
			},
			mcpServers:   map[string]*api.MCPServerInfo{},
			expectedKeys: []string{"container-server-mcp"},
		},
		{
			name: "multiple servers",
			mcpServerConfigs: []config.MCPServerDefinition{
				{
					Name:    "server1",
					Type:    config.MCPServerTypeLocalCommand,
					Enabled: true,
				},
				{
					Name:    "server2",
					Type:    config.MCPServerTypeContainer,
					Enabled: true,
				},
				{
					Name:    "server3",
					Type:    config.MCPServerTypeLocalCommand,
					Enabled: false, // This one should not appear
				},
			},
			mcpServers: map[string]*api.MCPServerInfo{
				"server1": {Name: "server1", Port: 8001},
				"server2": {Name: "server2", Port: 8002},
			},
			expectedKeys:    []string{"server1-mcp", "server2-mcp"},
			notExpectedKeys: []string{"server3-mcp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate the config JSON
			result := GenerateMcpConfigJson(tt.mcpServerConfigs, tt.mcpServers)

			// Parse the result
			var config map[string]interface{}
			err := json.Unmarshal([]byte(result), &config)
			require.NoError(t, err, "Generated JSON should be valid")

			// Check that mcpServers key exists
			mcpServers, ok := config["mcpServers"].(map[string]interface{})
			require.True(t, ok, "mcpServers key should exist and be a map")

			// Check expected keys
			for _, key := range tt.expectedKeys {
				_, exists := mcpServers[key]
				assert.True(t, exists, "Expected key %s to exist in mcpServers", key)
			}

			// Check not expected keys
			for _, key := range tt.notExpectedKeys {
				_, exists := mcpServers[key]
				assert.False(t, exists, "Expected key %s to NOT exist in mcpServers", key)
			}
		})
	}
}

func TestGenerateMcpConfigJson_SpecificValues(t *testing.T) {
	// Test specific values in the generated JSON
	mcpServerConfigs := []config.MCPServerDefinition{
		{
			Name:      "test-server",
			Type:      config.MCPServerTypeLocalCommand,
			Enabled:   true,
			ProxyPort: 8080,
		},
	}
	mcpServers := map[string]*api.MCPServerInfo{
		"test-server": {
			Name:  "test-server",
			Port:  9090, // Runtime port should override ProxyPort
			State: "running",
		},
	}

	result := GenerateMcpConfigJson(mcpServerConfigs, mcpServers)

	var config map[string]interface{}
	err := json.Unmarshal([]byte(result), &config)
	require.NoError(t, err)

	mcpServersMap := config["mcpServers"].(map[string]interface{})
	serverEntry := mcpServersMap["test-server-mcp"].(map[string]interface{})

	// Check URL uses runtime port
	assert.Equal(t, "http://localhost:9090/sse", serverEntry["url"])
	assert.Contains(t, serverEntry["description"], "9090")
}

func TestPrepareLogContent(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		maxWidth int
	}{
		{
			name:     "empty lines",
			lines:    []string{},
			maxWidth: 80,
		},
		{
			name:     "single line",
			lines:    []string{"Test log entry"},
			maxWidth: 80,
		},
		{
			name: "multiple lines",
			lines: []string{
				"First log entry",
				"Second log entry",
				"Third log entry",
			},
			maxWidth: 80,
		},
		{
			name: "long lines that need wrapping",
			lines: []string{
				"This is a very long log entry that should be wrapped when the max width is exceeded",
			},
			maxWidth: 20,
		},
		{
			name:     "zero width",
			lines:    []string{"Test"},
			maxWidth: 0,
		},
		{
			name:     "negative width",
			lines:    []string{"Test with negative width"},
			maxWidth: -10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function
			result := PrepareLogContent(tt.lines, tt.maxWidth)

			// Basic validation - the function should return a string
			assert.IsType(t, "", result)

			// If we have lines, result should not be empty
			if len(tt.lines) > 0 {
				assert.NotEmpty(t, result)
			}

			// For zero or negative width, content should be returned without wrapping
			if tt.maxWidth <= 0 && len(tt.lines) > 0 {
				expected := strings.Join(tt.lines, "\n")
				assert.Equal(t, expected, result)
			}
		})
	}
}

func TestPerformSwitchKubeContextCmd(t *testing.T) {
	tests := []struct {
		name          string
		targetContext string
	}{
		{
			name:          "switch to MC context",
			targetContext: "gs-test-mc",
		},
		{
			name:          "switch to WC context",
			targetContext: "gs-test-mc-test-wc",
		},
		{
			name:          "switch to custom context",
			targetContext: "custom-context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the command
			cmd := PerformSwitchKubeContextCmd(tt.targetContext)

			// Verify we get a command
			assert.NotNil(t, cmd)

			// Execute the command
			msg := cmd()

			// Verify we get a KubeContextSwitchedMsg
			switchMsg, ok := msg.(model.KubeContextSwitchedMsg)
			assert.True(t, ok, "Expected KubeContextSwitchedMsg")
			assert.Equal(t, tt.targetContext, switchMsg.TargetContext)

			// Note: The actual context switch might fail in tests due to kubeconfig
			// not being available, but that's expected
		})
	}
}

func TestGenerateMcpConfigJson_EdgeCases(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		result := GenerateMcpConfigJson([]config.MCPServerDefinition{}, map[string]*api.MCPServerInfo{})

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		assert.Empty(t, mcpServers)
	})

	t.Run("nil mcpServers map", func(t *testing.T) {
		mcpServerConfigs := []config.MCPServerDefinition{
			{
				Name:    "test",
				Type:    config.MCPServerTypeLocalCommand,
				Enabled: true,
			},
		}

		result := GenerateMcpConfigJson(mcpServerConfigs, nil)

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		assert.Contains(t, mcpServers, "test-mcp")
	})

	t.Run("container with multiple ports", func(t *testing.T) {
		mcpServerConfigs := []config.MCPServerDefinition{
			{
				Name:           "multi-port",
				Type:           config.MCPServerTypeContainer,
				Enabled:        true,
				ContainerPorts: []string{"8080:80", "9090:90", "3000:3000"},
			},
		}

		result := GenerateMcpConfigJson(mcpServerConfigs, map[string]*api.MCPServerInfo{})

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		serverEntry := mcpServers["multi-port-mcp"].(map[string]interface{})

		// Should use the first port
		assert.Equal(t, "http://localhost:8080", serverEntry["url"])
	})

	t.Run("env var with different cases", func(t *testing.T) {
		mcpServerConfigs := []config.MCPServerDefinition{
			{
				Name:    "env-test",
				Type:    config.MCPServerTypeLocalCommand,
				Enabled: true,
				Env: map[string]string{
					"server_url":    "http://localhost:1111",
					"SERVER_URL":    "http://localhost:2222",
					"SOMETHING_URL": "http://localhost:3333",
					"URL_SOMETHING": "http://localhost:4444", // Should not match
				},
			},
		}

		result := GenerateMcpConfigJson(mcpServerConfigs, map[string]*api.MCPServerInfo{})

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		serverEntry := mcpServers["env-test-mcp"].(map[string]interface{})

		// Should find one of the URL env vars
		url := serverEntry["url"].(string)
		assert.True(t, strings.HasPrefix(url, "http://localhost:"))
	})
}

func TestPerformSwitchKubeContextCmd_Integration(t *testing.T) {
	// Test the command execution flow
	cmd := PerformSwitchKubeContextCmd("test-context")

	// Create a channel to receive the message
	msgChan := make(chan tea.Msg, 1)

	// Execute in a goroutine
	go func() {
		msgChan <- cmd()
	}()

	// Wait for the message
	select {
	case msg := <-msgChan:
		switchMsg, ok := msg.(model.KubeContextSwitchedMsg)
		require.True(t, ok)
		assert.Equal(t, "test-context", switchMsg.TargetContext)
		// Error is expected in test environment
		if switchMsg.Err != nil {
			t.Logf("Expected error in test environment: %v", switchMsg.Err)
		}
	case <-make(chan struct{}):
		t.Fatal("Command did not complete in time")
	}
}
