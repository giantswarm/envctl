package model

import (
	"envctl/internal/config"
	"envctl/pkg/logging"
	"testing"
)

func TestInitializeModelV2(t *testing.T) {
	// Create a test config
	cfg := config.EnvctlConfig{
		PortForwards: []config.PortForwardDefinition{
			{
				Name:       "prometheus",
				Enabled:    true,
				LocalPort:  "8080",
				RemotePort: "9090",
				TargetType: "service",
				TargetName: "prometheus",
				Namespace:  "monitoring",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:      "k8s-api",
				Enabled:   true,
				ProxyPort: 8001,
				Type:      config.MCPServerTypeLocalCommand,
				Command:   []string{"npx", "mcp-server-kubernetes"},
			},
		},
	}
	
	// Create a test log channel
	logChan := make(chan logging.LogEntry)
	defer close(logChan)
	
	// Initialize ModelV2
	m, err := InitializeModelV2("test-mc", "test-wc", cfg, logChan)
	if err != nil {
		t.Fatalf("Failed to initialize ModelV2: %v", err)
	}
	
	// Verify basic fields
	if m.ManagementClusterName != "test-mc" {
		t.Errorf("Expected MC name 'test-mc', got '%s'", m.ManagementClusterName)
	}
	
	if m.WorkloadClusterName != "test-wc" {
		t.Errorf("Expected WC name 'test-wc', got '%s'", m.WorkloadClusterName)
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
	
	// Verify orchestrator is created
	if m.Orchestrator == nil {
		t.Error("Orchestrator is nil")
	}
	
	// Verify UI components are initialized
	// Spinner is a value type, not a pointer, so we can't check for nil
	
	if m.LogViewport.Width != 80 || m.LogViewport.Height != 20 {
		t.Errorf("LogViewport dimensions incorrect: %dx%d", m.LogViewport.Width, m.LogViewport.Height)
	}
	
	// Verify channels are created
	if m.TUIChannel == nil {
		t.Error("TUIChannel is nil")
	}
	
	if m.LogChannel == nil {
		t.Error("LogChannel is nil")
	}
	
	if m.StateChangeEvents == nil {
		t.Error("StateChangeEvents channel is nil")
	}
} 