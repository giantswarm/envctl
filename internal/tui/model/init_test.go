package model

import (
	"envctl/internal/config"
	"envctl/pkg/logging"
	"testing"
)

func TestInitializeModel(t *testing.T) {
	tests := []struct {
		name    string
		mcName  string
		wcName  string
		wantErr bool
	}{
		{
			name:    "valid initialization",
			mcName:  "test-mc",
			wcName:  "test-wc",
			wantErr: false,
		},
		{
			name:    "empty mc name",
			mcName:  "",
			wcName:  "test-wc",
			wantErr: false,
		},
		{
			name:    "empty wc name",
			mcName:  "test-mc",
			wcName:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logChan := make(chan logging.LogEntry, 100)
			cfg := config.EnvctlConfig{}

			m, err := InitializeModel(tt.mcName, tt.wcName, "test-context", false, cfg, logChan)

			if (err != nil) != tt.wantErr {
				t.Errorf("InitializeModel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify basic initialization
				if m.ManagementClusterName != tt.mcName {
					t.Errorf("ManagementClusterName = %v, want %v", m.ManagementClusterName, tt.mcName)
				}
				if m.WorkloadClusterName != tt.wcName {
					t.Errorf("WorkloadClusterName = %v, want %v", m.WorkloadClusterName, tt.wcName)
				}
				if m.CurrentKubeContext != "test-context" {
					t.Errorf("CurrentKubeContext = %v, want test-context", m.CurrentKubeContext)
				}
				if m.CurrentAppMode != ModeInitializing {
					t.Errorf("CurrentAppMode = %v, want ModeInitializing", m.CurrentAppMode)
				}

				// Verify APIs are initialized
				if m.OrchestratorAPI == nil {
					t.Error("OrchestratorAPI is nil")
				}
				if m.MCPServiceAPI == nil {
					t.Error("MCPServiceAPI is nil")
				}
				if m.PortForwardAPI == nil {
					t.Error("PortForwardAPI is nil")
				}
				if m.K8sServiceAPI == nil {
					t.Error("K8sServiceAPI is nil")
				}

				// Verify data structures are initialized
				if m.K8sConnections == nil {
					t.Error("K8sConnections is nil")
				}
				if m.PortForwards == nil {
					t.Error("PortForwards is nil")
				}
				if m.MCPServers == nil {
					t.Error("MCPServers is nil")
				}
				if m.MCPTools == nil {
					t.Error("MCPTools is nil")
				}
			}
		})
	}
}
