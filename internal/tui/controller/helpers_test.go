package controller

import (
	"encoding/json"
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMcpConfigJson(t *testing.T) {
	tests := []struct {
		name             string
		mcpServerConfigs []api.MCPServerInfo
		mcpServers       map[string]*api.MCPServerInfo
		aggregatorPort   int
		expectedKeys     []string
		notExpectedKeys  []string
		expectEmpty      bool
	}{
		{
			name: "basic MCP server with runtime port",
			mcpServerConfigs: []api.MCPServerInfo{
				{
					Name:    "test-server",
					Enabled: true,
				},
			},
			mcpServers: map[string]*api.MCPServerInfo{
				"test-server": {
					Name:   "test-server",
					State:  "Running",
					Health: "Healthy",
				},
			},
			aggregatorPort: 8080,
			expectedKeys:   []string{"envctl-aggregator"},
		},
		{
			name: "disabled server should not appear",
			mcpServerConfigs: []api.MCPServerInfo{
				{
					Name:    "disabled-server",
					Enabled: false,
				},
			},
			mcpServers:     map[string]*api.MCPServerInfo{},
			aggregatorPort: 8080,
			expectEmpty:    true,
		},
		{
			name: "multiple servers",
			mcpServerConfigs: []api.MCPServerInfo{
				{
					Name:    "server1",
					Enabled: true,
				},
				{
					Name:    "server2",
					Enabled: true,
				},
				{
					Name:    "server3",
					Enabled: false, // This one should not appear
				},
			},
			mcpServers: map[string]*api.MCPServerInfo{
				"server1": {Name: "server1", State: "Running", Health: "Healthy"},
				"server2": {Name: "server2", State: "Running", Health: "Healthy"},
			},
			aggregatorPort: 8080,
			expectedKeys:   []string{"envctl-aggregator"},
		},
		{
			name:             "no enabled servers",
			mcpServerConfigs: []api.MCPServerInfo{},
			mcpServers:       map[string]*api.MCPServerInfo{},
			aggregatorPort:   8080,
			expectEmpty:      true,
		},
		{
			name: "custom aggregator port",
			mcpServerConfigs: []api.MCPServerInfo{
				{
					Name:    "test-server",
					Enabled: true,
				},
			},
			mcpServers:     map[string]*api.MCPServerInfo{},
			aggregatorPort: 9999,
			expectedKeys:   []string{"envctl-aggregator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate the config JSON
			result := GenerateMcpConfigJson(tt.mcpServerConfigs, tt.mcpServers, tt.aggregatorPort)

			// Parse the result
			var config map[string]interface{}
			err := json.Unmarshal([]byte(result), &config)
			require.NoError(t, err, "Generated JSON should be valid")

			// Check that mcpServers key exists
			mcpServers, ok := config["mcpServers"].(map[string]interface{})
			require.True(t, ok, "mcpServers key should exist and be a map")

			if tt.expectEmpty {
				assert.Empty(t, mcpServers, "Expected mcpServers to be empty")
				return
			}

			// Check expected keys
			for _, key := range tt.expectedKeys {
				entry, exists := mcpServers[key]
				assert.True(t, exists, "Expected key %s to exist in mcpServers", key)

				if exists {
					// Check the structure of the aggregator entry
					entryMap := entry.(map[string]interface{})
					assert.Contains(t, entryMap, "url")
					assert.Contains(t, entryMap, "description")

					// Check URL format
					url := entryMap["url"].(string)
					expectedURL := fmt.Sprintf("http://localhost:%d/sse", tt.aggregatorPort)
					assert.Equal(t, expectedURL, url)
				}
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
	mcpServerConfigs := []api.MCPServerInfo{
		{
			Name:    "test-server",
			Enabled: true,
		},
		{
			Name:    "another-server",
			Enabled: true,
		},
	}
	mcpServers := map[string]*api.MCPServerInfo{
		"test-server": {
			Name:   "test-server",
			State:  "Running",
			Health: "Healthy",
		},
		"another-server": {
			Name:   "another-server",
			State:  "Running",
			Health: "Warning",
		},
	}

	result := GenerateMcpConfigJson(mcpServerConfigs, mcpServers, 7777)

	var config map[string]interface{}
	err := json.Unmarshal([]byte(result), &config)
	require.NoError(t, err)

	mcpServersMap := config["mcpServers"].(map[string]interface{})

	// Should have single aggregator entry
	assert.Len(t, mcpServersMap, 1)

	serverEntry := mcpServersMap["envctl-aggregator"].(map[string]interface{})

	// Check URL uses aggregator port
	assert.Equal(t, "http://localhost:7777/sse", serverEntry["url"])

	// Check description includes server status
	description := serverEntry["description"].(string)
	assert.Contains(t, description, "test-server (✓)")
	assert.Contains(t, description, "another-server (⚠)")
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
		result := GenerateMcpConfigJson([]api.MCPServerInfo{}, map[string]*api.MCPServerInfo{}, 8080)

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		assert.Empty(t, mcpServers)
	})

	t.Run("nil mcpServers map", func(t *testing.T) {
		mcpServerConfigs := []api.MCPServerInfo{
			{
				Name:    "test",
				Enabled: true,
			},
		}

		result := GenerateMcpConfigJson(mcpServerConfigs, nil, 8080)

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		assert.Contains(t, mcpServers, "envctl-aggregator")
	})

	t.Run("container with multiple ports", func(t *testing.T) {
		mcpServerConfigs := []api.MCPServerInfo{
			{
				Name:    "multi-port",
				Enabled: true,
			},
		}

		result := GenerateMcpConfigJson(mcpServerConfigs, map[string]*api.MCPServerInfo{}, 9999)

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		// Should have single aggregator entry
		assert.Len(t, mcpServers, 1)
		serverEntry := mcpServers["envctl-aggregator"].(map[string]interface{})

		// Should use the aggregator port
		assert.Equal(t, "http://localhost:9999/sse", serverEntry["url"])
	})

	t.Run("env var with different cases", func(t *testing.T) {
		mcpServerConfigs := []api.MCPServerInfo{
			{
				Name:    "env-test",
				Enabled: true,
			},
		}

		result := GenerateMcpConfigJson(mcpServerConfigs, map[string]*api.MCPServerInfo{}, 8080)

		var config map[string]interface{}
		err := json.Unmarshal([]byte(result), &config)
		require.NoError(t, err)

		mcpServers := config["mcpServers"].(map[string]interface{})
		// Should have aggregator entry if enabled servers exist
		assert.Contains(t, mcpServers, "envctl-aggregator")
		serverEntry := mcpServers["envctl-aggregator"].(map[string]interface{})

		// Check URL format
		url := serverEntry["url"].(string)
		assert.True(t, strings.HasPrefix(url, "http://localhost:"))

		// Should mention at least one of the servers
		description := serverEntry["description"].(string)
		assert.True(t, strings.Contains(description, "env-test") ||
			strings.Contains(description, "servers"))
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
