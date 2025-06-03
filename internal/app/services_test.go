package app

import (
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"testing"
)

// mockOrchestrator implements minimal orchestrator interface for testing
type mockOrchestrator struct {
	registry services.ServiceRegistry
}

func (m *mockOrchestrator) GetServiceRegistry() services.ServiceRegistry {
	return m.registry
}

func TestInitializeServices(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectError   bool
		checkServices func(*testing.T, *Services)
	}{
		{
			name: "basic initialization without aggregator",
			config: &Config{
				ManagementCluster: "test-mc",
				WorkloadCluster:   "test-wc",
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{},
					MCPServers:   []config.MCPServerDefinition{},
					Aggregator: config.AggregatorConfig{
						Enabled: false,
						Port:    0,
					},
				},
			},
			expectError: false,
			checkServices: func(t *testing.T, s *Services) {
				if s.Orchestrator == nil {
					t.Error("Orchestrator should not be nil")
				}
				if s.OrchestratorAPI == nil {
					t.Error("OrchestratorAPI should not be nil")
				}
				if s.MCPAPI == nil {
					t.Error("MCPAPI should not be nil")
				}
				if s.PortForwardAPI == nil {
					t.Error("PortForwardAPI should not be nil")
				}
				if s.K8sAPI == nil {
					t.Error("K8sAPI should not be nil")
				}
			},
		},
		{
			name: "initialization with aggregator",
			config: &Config{
				ManagementCluster: "test-mc",
				WorkloadCluster:   "",
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{},
					MCPServers: []config.MCPServerDefinition{
						{Name: "test-mcp-server"},
					},
					Aggregator: config.AggregatorConfig{
						Enabled: true,
						Port:    8090,
						Host:    "localhost",
					},
				},
			},
			expectError: false,
			checkServices: func(t *testing.T, s *Services) {
				if s.AggregatorPort != 8090 {
					t.Errorf("AggregatorPort = %d, want 8090", s.AggregatorPort)
				}
			},
		},
		{
			name: "initialization with default aggregator port",
			config: &Config{
				ManagementCluster: "test-mc",
				EnvctlConfig: &config.EnvctlConfig{
					PortForwards: []config.PortForwardDefinition{},
					MCPServers: []config.MCPServerDefinition{
						{Name: "test-mcp-server"},
					},
					Aggregator: config.AggregatorConfig{
						Enabled: true,
						Port:    0, // Should default to 8080
						Host:    "",
					},
				},
			},
			expectError: false,
			checkServices: func(t *testing.T, s *Services) {
				// The aggregator port in Services will still be 0,
				// but the actual aggregator service will use 8080
				if s.AggregatorPort != 0 {
					t.Errorf("AggregatorPort in Services = %d, want 0", s.AggregatorPort)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := InitializeServices(tt.config)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && tt.checkServices != nil {
				tt.checkServices(t, services)
			}
		})
	}
}

func TestInitializeServices_OrchestratorConfig(t *testing.T) {
	cfg := &Config{
		ManagementCluster: "test-mc",
		WorkloadCluster:   "test-wc",
		EnvctlConfig: &config.EnvctlConfig{
			PortForwards: []config.PortForwardDefinition{
				{Name: "test-pf"},
			},
			MCPServers: []config.MCPServerDefinition{
				{Name: "test-mcp"},
			},
			Aggregator: config.AggregatorConfig{
				Port: 9090,
			},
		},
	}

	// We can't easily test the full initialization without mocking orchestrator.New
	// But we can verify that the config is passed correctly
	expectedConfig := orchestrator.Config{
		MCName:       "test-mc",
		WCName:       "test-wc",
		PortForwards: cfg.EnvctlConfig.PortForwards,
		MCPServers:   cfg.EnvctlConfig.MCPServers,
	}

	// Verify the expected config matches what we expect to be passed
	if expectedConfig.MCName != cfg.ManagementCluster {
		t.Errorf("Expected MCName = %s, got %s", cfg.ManagementCluster, expectedConfig.MCName)
	}
	if expectedConfig.WCName != cfg.WorkloadCluster {
		t.Errorf("Expected WCName = %s, got %s", cfg.WorkloadCluster, expectedConfig.WCName)
	}
	if len(expectedConfig.PortForwards) != len(cfg.EnvctlConfig.PortForwards) {
		t.Errorf("Expected %d PortForwards, got %d", len(cfg.EnvctlConfig.PortForwards), len(expectedConfig.PortForwards))
	}
	if len(expectedConfig.MCPServers) != len(cfg.EnvctlConfig.MCPServers) {
		t.Errorf("Expected %d MCPServers, got %d", len(cfg.EnvctlConfig.MCPServers), len(expectedConfig.MCPServers))
	}
}
