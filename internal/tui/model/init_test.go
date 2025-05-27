package model

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

func TestDefaultKeyMap(t *testing.T) {
	keys := DefaultKeyMap()

	// Test that all key bindings are initialized
	tests := []struct {
		name    string
		binding key.Binding
	}{
		{"Up", keys.Up},
		{"Down", keys.Down},
		{"Tab", keys.Tab},
		{"ShiftTab", keys.ShiftTab},
		{"Enter", keys.Enter},
		{"Esc", keys.Esc},
		{"Quit", keys.Quit},
		{"Help", keys.Help},
		{"NewCollection", keys.NewCollection},
		{"Restart", keys.Restart},
		{"Stop", keys.Stop},
		{"SwitchContext", keys.SwitchContext},
		{"ToggleDark", keys.ToggleDark},
		{"ToggleDebug", keys.ToggleDebug},
		{"CopyLogs", keys.CopyLogs},
		{"ToggleLog", keys.ToggleLog},
		{"ToggleMcpConfig", keys.ToggleMcpConfig},
		{"ToggleMcpTools", keys.ToggleMcpTools},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.binding.Keys()) == 0 {
				t.Errorf("KeyBinding %s has no keys defined", tt.name)
			}
			if tt.binding.Help().Desc == "" {
				t.Errorf("KeyBinding %s has no help description", tt.name)
			}
		})
	}
}

func TestModel_Init(t *testing.T) {
	logChan := make(chan logging.LogEntry, 100)
	cfg := config.EnvctlConfig{}

	m, err := InitializeModel("test-mc", "test-wc", "test-context", false, cfg, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Call Init and verify it returns commands
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil command")
	}

	// Verify the command is a batch command
	// Note: We can't easily test the actual commands without running them
}

func TestModel_Update(t *testing.T) {
	logChan := make(chan logging.LogEntry, 100)
	cfg := config.EnvctlConfig{}

	m, err := InitializeModel("test-mc", "test-wc", "test-context", false, cfg, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Test Update method
	updatedModel, cmd := m.Update(nil)

	// Verify the model is returned unchanged
	if updatedModel != m {
		t.Error("Update() should return the same model")
	}

	// Verify no command is returned
	if cmd != nil {
		t.Error("Update() should return nil command")
	}
}

func TestModel_View(t *testing.T) {
	logChan := make(chan logging.LogEntry, 100)
	cfg := config.EnvctlConfig{}

	m, err := InitializeModel("test-mc", "test-wc", "test-context", false, cfg, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Test View method
	view := m.View()

	// Verify the view contains expected content
	if view == "" {
		t.Error("View() returned empty string")
	}

	// Check for expected content
	expectedContent := "envctl - Services:"
	if !contains(view, expectedContent) {
		t.Errorf("View() does not contain expected content: %s", expectedContent)
	}
}

func TestChannelReaderCmd(t *testing.T) {
	ch := make(chan tea.Msg, 1)

	// Test with a message
	testMsg := NopMsg{}
	ch <- testMsg

	cmd := ChannelReaderCmd(ch)
	if cmd == nil {
		t.Fatal("ChannelReaderCmd() returned nil")
	}

	msg := cmd()
	if _, ok := msg.(NopMsg); !ok {
		t.Errorf("ChannelReaderCmd() returned wrong message type: %T", msg)
	}

	// Test with closed channel
	close(ch)
	msg = cmd()
	if msg != nil {
		t.Error("ChannelReaderCmd() should return nil for closed channel")
	}
}

func TestListenForStateChanges(t *testing.T) {
	logChan := make(chan logging.LogEntry, 100)
	cfg := config.EnvctlConfig{}

	m, err := InitializeModel("test-mc", "test-wc", "test-context", false, cfg, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Create a test event channel
	eventChan := make(chan api.ServiceStateChangedEvent, 1)
	m.StateChangeEvents = eventChan

	// Send a test event
	testEvent := api.ServiceStateChangedEvent{
		Label:    "test-service",
		OldState: "stopped",
		NewState: "running",
	}
	eventChan <- testEvent

	// Call ListenForStateChanges
	cmd := m.ListenForStateChanges()
	if cmd == nil {
		t.Fatal("ListenForStateChanges() returned nil")
	}

	// Execute the command
	msg := cmd()
	event, ok := msg.(api.ServiceStateChangedEvent)
	if !ok {
		t.Fatalf("ListenForStateChanges() returned wrong message type: %T", msg)
	}

	if event.Label != testEvent.Label {
		t.Errorf("Event label = %v, want %v", event.Label, testEvent.Label)
	}

	// Test with closed channel
	close(eventChan)
	msg = cmd()
	if msg != nil {
		t.Error("ListenForStateChanges() should return nil for closed channel")
	}
}

func TestListenForLogs(t *testing.T) {
	logChan := make(chan logging.LogEntry, 1)
	cfg := config.EnvctlConfig{}

	m, err := InitializeModel("test-mc", "test-wc", "test-context", false, cfg, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Send a test log entry
	testEntry := logging.LogEntry{
		Level:   logging.LevelInfo,
		Message: "test log message",
	}
	logChan <- testEntry

	// Call ListenForLogs
	cmd := m.ListenForLogs()
	if cmd == nil {
		t.Fatal("ListenForLogs() returned nil")
	}

	// Execute the command
	msg := cmd()
	logMsg, ok := msg.(NewLogEntryMsg)
	if !ok {
		t.Fatalf("ListenForLogs() returned wrong message type: %T", msg)
	}

	if logMsg.Entry.Message != testEntry.Message {
		t.Errorf("Log message = %v, want %v", logMsg.Entry.Message, testEntry.Message)
	}

	// Test with closed channel
	close(logChan)
	msg = cmd()
	if msg != nil {
		t.Error("ListenForLogs() should return nil for closed channel")
	}
}

func TestInitializeModel_WithConfig(t *testing.T) {
	logChan := make(chan logging.LogEntry, 100)

	// Test with configuration
	cfg := config.EnvctlConfig{
		PortForwards: []config.PortForwardDefinition{
			{
				Name:       "test-pf",
				Namespace:  "default",
				TargetType: "deployment",
				TargetName: "test",
				LocalPort:  "8080",
				RemotePort: "80",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:      "test-mcp",
				Type:      config.MCPServerTypeLocalCommand,
				ProxyPort: 9090,
			},
		},
	}

	m, err := InitializeModel("test-mc", "test-wc", "test-context", true, cfg, logChan)
	if err != nil {
		t.Fatalf("InitializeModel() error = %v", err)
	}

	// Verify configuration is set
	if len(m.PortForwardingConfig) != 1 {
		t.Errorf("PortForwardingConfig length = %v, want 1", len(m.PortForwardingConfig))
	}
	if len(m.MCPServerConfig) != 1 {
		t.Errorf("MCPServerConfig length = %v, want 1", len(m.MCPServerConfig))
	}

	// Verify debug mode
	if !m.DebugMode {
		t.Error("DebugMode should be true")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
