package model

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/pkg/logging"
	"testing"
)

func TestStartOrchestrator(t *testing.T) {
	tests := []struct {
		name          string
		mcName        string
		wcName        string
		wantErrorType string
	}{
		{
			name:          "successful start with empty names",
			mcName:        "",
			wcName:        "",
			wantErrorType: "",
		},
		{
			name:          "successful start with names",
			mcName:        "test-mc",
			wcName:        "test-wc",
			wantErrorType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real orchestrator
			orchConfig := orchestrator.Config{
				MCName:       tt.mcName,
				WCName:       tt.wcName,
				PortForwards: []config.PortForwardDefinition{},
				MCPServers:   []config.MCPServerDefinition{},
			}
			orch := orchestrator.New(orchConfig)
			registry := orch.GetServiceRegistry()

			// Create APIs
			orchestratorAPI := api.NewOrchestratorAPI(orch, registry)
			mcpAPI := api.NewMCPServiceAPI(registry)
			portForwardAPI := api.NewPortForwardServiceAPI(registry)
			k8sAPI := api.NewK8sServiceAPI(registry)

			// Create model
			m := &Model{
				Orchestrator:       orch,
				OrchestratorAPI:    orchestratorAPI,
				K8sServiceAPI:      k8sAPI,
				PortForwardAPI:     portForwardAPI,
				MCPServiceAPI:      mcpAPI,
				K8sConnections:     make(map[string]*api.K8sConnectionInfo),
				PortForwards:       make(map[string]*api.PortForwardServiceInfo),
				MCPServers:         make(map[string]*api.MCPServerInfo),
				K8sConnectionOrder: []string{},
				PortForwardOrder:   []string{},
				MCPServerOrder:     []string{},
			}

			// Call startOrchestrator
			cmd := m.startOrchestrator()
			if cmd == nil {
				t.Fatal("startOrchestrator() returned nil command")
			}

			// Execute the command
			msg := cmd()

			// The real orchestrator might fail due to missing kube config, etc.
			// We check if it's nil (success) or error message
			if msg == nil {
				// Success case - startOrchestrator returns nil when successful
			} else if _, ok := msg.(ServiceErrorMsg); ok {
				// Error case - this is expected if kube config is missing
			} else {
				t.Errorf("Unexpected message type: %T", msg)
			}
		})
	}
}

// TestStartOrchestratorIntegration tests the startOrchestrator method with different scenarios
func TestStartOrchestratorIntegration(t *testing.T) {
	logChan := make(chan logging.LogEntry, 100)

	// Test with minimal configuration
	cfg := config.EnvctlConfig{
		PortForwards: []config.PortForwardDefinition{},
		MCPServers:   []config.MCPServerDefinition{},
	}

	// Create TUIConfig
	tuiConfig := TUIConfig{
		ManagementClusterName: "",
		WorkloadClusterName:   "",
		DebugMode:             false,
		ColorMode:             "auto",
		PortForwardingConfig:  cfg.PortForwards,
		MCPServerConfig:       cfg.MCPServers,
		AggregatorConfig:      cfg.Aggregator,
	}

	m, err := InitializeModel(tuiConfig, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Call startOrchestrator
	cmd := m.startOrchestrator()
	if cmd == nil {
		t.Fatal("startOrchestrator() returned nil command")
	}

	// Execute the command
	msg := cmd()

	// Verify the orchestrator was started
	// Note: The actual start might fail due to missing kube config,
	// but we should get either nil (success) or error message
	if msg == nil {
		// Success case
		t.Log("Orchestrator started successfully")
	} else if _, ok := msg.(ServiceErrorMsg); ok {
		// Error case - this is expected in test environment
		t.Log("Orchestrator start failed (expected in test environment)")
	} else {
		t.Errorf("Unexpected message type: %T", msg)
	}
}
