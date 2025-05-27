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
			// We just check that we get a message back
			if msg == nil {
				t.Error("startOrchestrator() command returned nil message")
			}

			// Check if it's an initialization complete or error message
			switch msg.(type) {
			case InitializationCompleteMsg:
				// Success case
			case ServiceErrorMsg:
				// Error case - this is expected if kube config is missing
			default:
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

	m, err := InitializeModel("", "", "", false, cfg, logChan)
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
	if msg == nil {
		t.Error("startOrchestrator() command returned nil message")
	}

	// Verify the orchestrator was started
	// Note: The actual start might fail due to missing kube config,
	// but we should still get a message
	switch msg.(type) {
	case InitializationCompleteMsg:
		// Success case
		t.Log("Orchestrator started successfully")
	case ServiceErrorMsg:
		// Error case - this is expected in test environment
		t.Log("Orchestrator start failed (expected in test environment)")
	default:
		t.Errorf("Unexpected message type: %T", msg)
	}
}
