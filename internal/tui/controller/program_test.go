package controller

import (
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProgram(t *testing.T) {
	tests := []struct {
		name        string
		mcName      string
		wcName      string
		debugMode   bool
		cfg         config.EnvctlConfig
		expectError bool
	}{
		{
			name:      "valid configuration",
			mcName:    "test-mc",
			wcName:    "test-wc",
			debugMode: false,
			cfg: config.EnvctlConfig{
				PortForwards: []config.PortForwardDefinition{
					{
						Name:       "test-forward",
						LocalPort:  "8080",
						RemotePort: "80",
					},
				},
				MCPServers: []config.MCPServerDefinition{
					{
						Name:    "test-mcp",
						Type:    config.MCPServerTypeLocalCommand,
						Enabled: true,
					},
				},
			},
			expectError: false,
		},
		{
			name:        "empty cluster names",
			mcName:      "",
			wcName:      "",
			debugMode:   false,
			cfg:         config.EnvctlConfig{},
			expectError: false,
		},
		{
			name:        "debug mode enabled",
			mcName:      "test-mc",
			wcName:      "test-wc",
			debugMode:   true,
			cfg:         config.EnvctlConfig{},
			expectError: false,
		},
		{
			name:      "with MCP servers and port forwards",
			mcName:    "test-mc",
			wcName:    "test-wc",
			debugMode: false,
			cfg: config.EnvctlConfig{
				PortForwards: []config.PortForwardDefinition{
					{
						Name:       "forward1",
						LocalPort:  "8080",
						RemotePort: "80",
					},
					{
						Name:       "forward2",
						LocalPort:  "9090",
						RemotePort: "90",
					},
				},
				MCPServers: []config.MCPServerDefinition{
					{
						Name:    "mcp1",
						Type:    config.MCPServerTypeLocalCommand,
						Enabled: true,
					},
					{
						Name:    "mcp2",
						Type:    config.MCPServerTypeContainer,
						Enabled: false,
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a closed log channel to prevent goroutine leaks
			logChannel := make(chan logging.LogEntry)
			close(logChannel)

			// Create TUIConfig
			tuiConfig := model.TUIConfig{
				ManagementClusterName: tt.mcName,
				WorkloadClusterName:   tt.wcName,
				DebugMode:             tt.debugMode,
				ColorMode:             "auto",
				PortForwardingConfig:  tt.cfg.PortForwards,
				MCPServerConfig:       tt.cfg.MCPServers,
				AggregatorConfig:      tt.cfg.Aggregator,
			}

			// Create the program
			// Note: This creates the orchestrator but doesn't start it
			// The actual starting happens when the program runs
			program, err := NewProgram(tuiConfig, logChannel)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, program)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, program)
			}
		})
	}
}

func TestNewProgram_Parameters(t *testing.T) {
	// Test that parameters are properly passed through
	mcName := "test-mc"
	wcName := "test-wc"
	debugMode := true
	cfg := config.EnvctlConfig{
		PortForwards: []config.PortForwardDefinition{
			{Name: "test-pf"},
		},
		MCPServers: []config.MCPServerDefinition{
			{Name: "test-mcp"},
		},
	}

	// Create a closed log channel
	logChannel := make(chan logging.LogEntry)
	close(logChannel)

	// Create TUIConfig
	tuiConfig := model.TUIConfig{
		ManagementClusterName: mcName,
		WorkloadClusterName:   wcName,
		DebugMode:             debugMode,
		ColorMode:             "auto",
		PortForwardingConfig:  cfg.PortForwards,
		MCPServerConfig:       cfg.MCPServers,
		AggregatorConfig:      cfg.Aggregator,
	}

	// Create the program
	program, err := NewProgram(tuiConfig, logChannel)

	require.NoError(t, err)
	assert.NotNil(t, program)

	// We can't easily verify the internal state without running the program
	// which would start goroutines, so we just verify it was created successfully
}

func TestNewProgram_ConfigValidation(t *testing.T) {
	// Test various configuration edge cases
	tests := []struct {
		name string
		cfg  config.EnvctlConfig
	}{
		{
			name: "empty config",
			cfg:  config.EnvctlConfig{},
		},
		{
			name: "config with invalid port forward",
			cfg: config.EnvctlConfig{
				PortForwards: []config.PortForwardDefinition{
					{
						Name:       "invalid",
						LocalPort:  "0", // Invalid port
						RemotePort: "80",
					},
				},
			},
		},
		{
			name: "config with disabled MCP servers",
			cfg: config.EnvctlConfig{
				MCPServers: []config.MCPServerDefinition{
					{
						Name:    "disabled-mcp",
						Type:    config.MCPServerTypeLocalCommand,
						Enabled: false,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a closed log channel
			logChannel := make(chan logging.LogEntry)
			close(logChannel)

			// Create TUIConfig
			tuiConfig := model.TUIConfig{
				ManagementClusterName: "test-mc",
				WorkloadClusterName:   "test-wc",
				DebugMode:             false,
				ColorMode:             "auto",
				PortForwardingConfig:  tt.cfg.PortForwards,
				MCPServerConfig:       tt.cfg.MCPServers,
				AggregatorConfig:      tt.cfg.Aggregator,
			}

			program, err := NewProgram(tuiConfig, logChannel)

			// NewProgram should handle these cases gracefully
			require.NoError(t, err)
			assert.NotNil(t, program)
		})
	}
}
