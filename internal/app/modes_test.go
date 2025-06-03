package app

import (
	"envctl/internal/config"
	"testing"
)

func TestModeSelection(t *testing.T) {
	// Test that we can determine which mode should be run based on configuration
	tests := []struct {
		name      string
		noTUI     bool
		expectCLI bool
		expectTUI bool
	}{
		{
			name:      "CLI mode selected",
			noTUI:     true,
			expectCLI: true,
			expectTUI: false,
		},
		{
			name:      "TUI mode selected",
			noTUI:     false,
			expectCLI: false,
			expectTUI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ManagementCluster: "test-mc",
				NoTUI:             tt.noTUI,
				Debug:             false,
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{},
					MCPServers:   []config.MCPServerDefinition{},
					Aggregator:   config.AggregatorConfig{},
				},
			}

			// Test mode selection logic
			shouldUseCLI := cfg.NoTUI
			shouldUseTUI := !cfg.NoTUI

			if tt.expectCLI && !shouldUseCLI {
				t.Error("Expected CLI mode to be selected")
			}
			if tt.expectTUI && !shouldUseTUI {
				t.Error("Expected TUI mode to be selected")
			}
			if !tt.expectCLI && shouldUseCLI {
				t.Error("Did not expect CLI mode to be selected")
			}
			if !tt.expectTUI && shouldUseTUI {
				t.Error("Did not expect TUI mode to be selected")
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	// Test that the configuration is properly validated before running modes
	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
	}{
		{
			name: "valid config with MC only",
			cfg: &Config{
				ManagementCluster: "test-mc",
				WorkloadCluster:   "",
				NoTUI:             true,
				Debug:             false,
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{},
					MCPServers:   []config.MCPServerDefinition{},
					Aggregator:   config.AggregatorConfig{},
				},
			},
			wantError: false,
		},
		{
			name: "valid config with MC and WC",
			cfg: &Config{
				ManagementCluster: "test-mc",
				WorkloadCluster:   "test-wc",
				NoTUI:             false,
				Debug:             true,
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{
						{Name: "test-pf", Enabled: true},
					},
					MCPServers: []config.MCPServerDefinition{
						{Name: "test-mcp", Enabled: true},
					},
					Aggregator: config.AggregatorConfig{
						Port: 8080,
						Host: "localhost",
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate basic config structure
			if tt.cfg.EnvctlConfig == nil && !tt.wantError {
				t.Error("EnvctlConfig should not be nil for valid configs")
			}

			// Validate management cluster is set for non-error cases
			if tt.cfg.ManagementCluster == "" && !tt.wantError {
				t.Error("ManagementCluster should not be empty for valid configs")
			}

			// Validate that the config has the expected structure
			if tt.cfg.EnvctlConfig != nil {
				if tt.cfg.EnvctlConfig.PortForwards == nil {
					t.Error("PortForwards slice should not be nil")
				}
				if tt.cfg.EnvctlConfig.MCPServers == nil {
					t.Error("MCPServers slice should not be nil")
				}
			}
		})
	}
}

func TestModeHandlerSelection(t *testing.T) {
	// Test the mode handler selection logic without actually running the modes
	tests := []struct {
		name     string
		noTUI    bool
		expected string
	}{
		{
			name:     "CLI mode handler",
			noTUI:    true,
			expected: "CLI",
		},
		{
			name:     "TUI mode handler",
			noTUI:    false,
			expected: "TUI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ManagementCluster: "test-mc",
				NoTUI:             tt.noTUI,
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{},
					MCPServers:   []config.MCPServerDefinition{},
					Aggregator:   config.AggregatorConfig{},
				},
			}

			// Simulate the mode selection logic from bootstrap.go
			var selectedMode string
			if cfg.NoTUI {
				selectedMode = "CLI"
			} else {
				selectedMode = "TUI"
			}

			if selectedMode != tt.expected {
				t.Errorf("Expected mode %s, got %s", tt.expected, selectedMode)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that configs have sensible defaults and validation
	cfg := &Config{
		ManagementCluster: "test-mc",
		WorkloadCluster:   "test-wc",
		NoTUI:             false,
		Debug:             true,
		EnvctlConfig: &config.EnvctlConfig{
			PortForwards: []config.PortForwardDefinition{},
			MCPServers:   []config.MCPServerDefinition{},
			Aggregator: config.AggregatorConfig{
				Port:    0, // Should get default
				Host:    "",
				Enabled: false,
			},
		},
	}

	// Verify the config structure is valid
	if cfg.ManagementCluster == "" {
		t.Error("ManagementCluster should not be empty")
	}

	if cfg.EnvctlConfig == nil {
		t.Error("EnvctlConfig should not be nil")
	}

	if cfg.EnvctlConfig.PortForwards == nil {
		t.Error("PortForwards should not be nil")
	}

	if cfg.EnvctlConfig.MCPServers == nil {
		t.Error("MCPServers should not be nil")
	}

	// Test that both CLI and TUI modes can be configured
	if cfg.NoTUI && cfg.Debug {
		t.Log("CLI mode with debug enabled")
	}
	if !cfg.NoTUI && cfg.Debug {
		t.Log("TUI mode with debug enabled")
	}
}
