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
			name:        "valid configuration",
			mcName:      "test-mc",
			wcName:      "test-wc",
			debugMode:   false,
			cfg:         config.EnvctlConfig{},
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
			name:      "with aggregator config",
			mcName:    "test-mc",
			wcName:    "test-wc",
			debugMode: false,
			cfg: config.EnvctlConfig{
				Aggregator: config.AggregatorConfig{
					Port: 8080,
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
				DebugMode:        tt.debugMode,
				ColorMode:        "auto",
				MCPServerConfig:  nil, // MCPServers removed
				AggregatorConfig: tt.cfg.Aggregator,
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
	debugMode := true
	cfg := config.EnvctlConfig{}

	// Create a closed log channel
	logChannel := make(chan logging.LogEntry)
	close(logChannel)

	// Create TUIConfig
	tuiConfig := model.TUIConfig{
		DebugMode:        debugMode,
		ColorMode:        "auto",
		MCPServerConfig:  nil, // MCPServers removed
		AggregatorConfig: cfg.Aggregator,
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
			name: "config with disabled MCP servers",
			cfg:  config.EnvctlConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a closed log channel
			logChannel := make(chan logging.LogEntry)
			close(logChannel)

			// Create TUIConfig
			tuiConfig := model.TUIConfig{
				DebugMode:        false,
				ColorMode:        "auto",
				MCPServerConfig:  nil, // MCPServers removed
				AggregatorConfig: tt.cfg.Aggregator,
			}

			program, err := NewProgram(tuiConfig, logChannel)

			// NewProgram should handle these cases gracefully
			require.NoError(t, err)
			assert.NotNil(t, program)
		})
	}
}
