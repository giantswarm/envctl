package controller

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"
	"errors"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// testModelCleanup holds channels that need to be closed after test
type testModelCleanup struct {
	tuiChannel chan tea.Msg
}

func (c *testModelCleanup) cleanup() {
	if c.tuiChannel != nil {
		close(c.tuiChannel)
	}
}

func createTestModel() (*model.Model, *testModelCleanup) {
	tuiChan := make(chan tea.Msg, 10)
	cleanup := &testModelCleanup{
		tuiChannel: tuiChan,
	}

	return &model.Model{
		Width:                 80,
		Height:                24,
		CurrentAppMode:        model.ModeInitializing,
		ManagementClusterName: "test-mc",
		WorkloadClusterName:   "test-wc",
		CurrentKubeContext:    "test-context",
		TUIChannel:            tuiChan,
		ActivityLog:           []string{},
		LogViewport:           viewport.New(80, 20),
		MainLogViewport:       viewport.New(80, 10),
		McpConfigViewport:     viewport.New(80, 20),
		McpToolsViewport:      viewport.New(80, 20),
		Spinner:               spinner.New(),
		K8sConnections:        make(map[string]*api.K8sConnectionInfo),
		PortForwards:          make(map[string]*api.PortForwardServiceInfo),
		MCPServers:            make(map[string]*api.MCPServerInfo),
		MCPTools:              make(map[string][]api.MCPTool),
		K8sConnectionOrder:    []string{},
		PortForwardOrder:      []string{},
		MCPServerOrder:        []string{},
		PortForwardingConfig:  []config.PortForwardDefinition{},
		MCPServerConfig:       []config.MCPServerDefinition{},
		// Don't set LogChannel or StateChangeEvents - they're not needed for most tests
	}, cleanup
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 40,
	}

	updatedModel, cmd := Update(msg, m)

	assert.Equal(t, 120, updatedModel.Width)
	assert.Equal(t, 40, updatedModel.Height)
	assert.Equal(t, 120, updatedModel.LogViewport.Width)
	assert.Equal(t, 30, updatedModel.LogViewport.Height) // Height - 10
	assert.Equal(t, 120, updatedModel.MainLogViewport.Width)
	assert.Equal(t, 13, updatedModel.MainLogViewport.Height) // Height / 3
	assert.Nil(t, cmd)
}

func TestUpdate_SpinnerTickMsg(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()

	msg := m.Spinner.Tick()

	updatedModel, cmd := Update(msg, m)

	// Spinner should be updated
	assert.NotNil(t, updatedModel.Spinner)
	assert.NotNil(t, cmd) // Spinner returns a tick command
}

func TestUpdate_ServiceMessages(t *testing.T) {
	tests := []struct {
		name         string
		msg          tea.Msg
		expectedLog  string
		expectedType model.MessageType
	}{
		{
			name:         "service started",
			msg:          model.ServiceStartedMsg{Label: "test-service"},
			expectedLog:  "Service test-service started",
			expectedType: model.StatusBarSuccess,
		},
		{
			name:         "service stopped",
			msg:          model.ServiceStoppedMsg{Label: "test-service"},
			expectedLog:  "Service test-service stopped",
			expectedType: model.StatusBarInfo,
		},
		{
			name:         "service restarted",
			msg:          model.ServiceRestartedMsg{Label: "test-service"},
			expectedLog:  "Service test-service restarted",
			expectedType: model.StatusBarSuccess,
		},
		{
			name:         "service error",
			msg:          model.ServiceErrorMsg{Label: "test-service", Err: errors.New("test error")},
			expectedLog:  "Error with service test-service: test error",
			expectedType: model.StatusBarError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := createTestModel()
			defer cleanup.cleanup()

			updatedModel, cmd := Update(tt.msg, m)

			assert.Equal(t, tt.expectedLog, updatedModel.StatusBarMessage)
			assert.Equal(t, tt.expectedType, updatedModel.StatusBarMessageType)
			assert.NotNil(t, cmd) // Should have commands
		})
	}
}

func TestUpdate_ClearStatusBarMsg(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()
	m.StatusBarMessage = "Test message"
	m.StatusBarMessageType = model.StatusBarError

	msg := model.ClearStatusBarMsg{}

	updatedModel, _ := Update(msg, m)

	assert.Empty(t, updatedModel.StatusBarMessage)
	assert.Equal(t, model.StatusBarInfo, updatedModel.StatusBarMessageType)
}

func TestUpdate_NewLogEntryMsg(t *testing.T) {
	tests := []struct {
		name        string
		debugMode   bool
		logLevel    logging.LogLevel
		expectAdded bool
	}{
		{
			name:        "info log always added",
			debugMode:   false,
			logLevel:    logging.LevelInfo,
			expectAdded: true,
		},
		{
			name:        "debug log in debug mode",
			debugMode:   true,
			logLevel:    logging.LevelDebug,
			expectAdded: true,
		},
		{
			name:        "debug log not in debug mode",
			debugMode:   false,
			logLevel:    logging.LevelDebug,
			expectAdded: false,
		},
		{
			name:        "error log always added",
			debugMode:   false,
			logLevel:    logging.LevelError,
			expectAdded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := createTestModel()
			defer cleanup.cleanup()
			m.DebugMode = tt.debugMode

			// Create closed channels for log listening
			m.LogChannel = make(<-chan logging.LogEntry)

			msg := model.NewLogEntryMsg{
				Entry: logging.LogEntry{
					Timestamp: time.Now(),
					Level:     tt.logLevel,
					Subsystem: "test",
					Message:   "test message",
				},
			}

			initialLogCount := len(m.ActivityLog)
			updatedModel, cmd := Update(msg, m)

			if tt.expectAdded {
				assert.Len(t, updatedModel.ActivityLog, initialLogCount+1)
				assert.True(t, updatedModel.ActivityLogDirty)
			} else {
				assert.Len(t, updatedModel.ActivityLog, initialLogCount)
			}

			assert.NotNil(t, cmd) // Should return ListenForLogs command
		})
	}
}

func TestUpdate_KeyMessages(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		currentMode  model.AppMode
		expectedMode model.AppMode
		focusedPanel string
		expectQuit   bool
	}{
		{
			name:         "quit key",
			key:          "q",
			currentMode:  model.ModeMainDashboard,
			expectedMode: model.ModeQuitting,
			expectQuit:   true,
		},
		{
			name:         "help key",
			key:          "?",
			currentMode:  model.ModeMainDashboard,
			expectedMode: model.ModeHelpOverlay,
		},
		{
			name:         "log overlay key",
			key:          "L",
			currentMode:  model.ModeMainDashboard,
			expectedMode: model.ModeLogOverlay,
		},
		{
			name:         "mcp config overlay key",
			key:          "C",
			currentMode:  model.ModeMainDashboard,
			expectedMode: model.ModeMcpConfigOverlay,
		},
		{
			name:         "mcp tools overlay key",
			key:          "M",
			currentMode:  model.ModeMainDashboard,
			expectedMode: model.ModeMcpToolsOverlay,
		},
		{
			name:         "escape from help overlay",
			key:          "esc",
			currentMode:  model.ModeHelpOverlay,
			expectedMode: model.ModeMainDashboard,
		},
		{
			name:         "toggle debug mode",
			key:          "z",
			currentMode:  model.ModeMainDashboard,
			expectedMode: model.ModeMainDashboard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := createTestModel()
			defer cleanup.cleanup()
			m.CurrentAppMode = tt.currentMode
			m.LastAppMode = model.ModeMainDashboard
			if tt.focusedPanel != "" {
				m.FocusedPanelKey = tt.focusedPanel
			}

			msg := tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune(tt.key),
			}
			if tt.key == "esc" {
				msg = tea.KeyMsg{Type: tea.KeyEscape}
			}

			updatedModel, _ := Update(msg, m)

			assert.Equal(t, tt.expectedMode, updatedModel.CurrentAppMode)
			if tt.expectQuit {
				assert.True(t, updatedModel.QuitApp)
			}
		})
	}
}

func TestUpdate_ServiceStateChangedEvent(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()

	// Create closed channel for state changes
	m.StateChangeEvents = make(<-chan api.ServiceStateChangedEvent)

	event := api.ServiceStateChangedEvent{
		Label:       "test-service",
		ServiceType: "PortForward",
		OldState:    "stopped",
		NewState:    "running",
		Error:       nil,
	}

	initialLogCount := len(m.ActivityLog)
	updatedModel, cmd := Update(event, m)

	// Should add a log entry
	assert.Len(t, updatedModel.ActivityLog, initialLogCount+1)
	assert.True(t, updatedModel.ActivityLogDirty)
	assert.Contains(t, updatedModel.ActivityLog[len(updatedModel.ActivityLog)-1], "test-service")
	assert.Contains(t, updatedModel.ActivityLog[len(updatedModel.ActivityLog)-1], "stopped → running")

	assert.NotNil(t, cmd) // Should have refresh command
}

func TestUpdate_KubeContextSwitchedMsg(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedType model.MessageType
	}{
		{
			name:         "successful switch",
			err:          nil,
			expectedType: model.StatusBarSuccess,
		},
		{
			name:         "failed switch",
			err:          errors.New("switch failed"),
			expectedType: model.StatusBarError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := createTestModel()
			defer cleanup.cleanup()

			msg := model.KubeContextSwitchedMsg{
				TargetContext: "new-context",
				Err:           tt.err,
			}

			updatedModel, _ := Update(msg, m)

			if tt.err == nil {
				assert.Equal(t, "new-context", updatedModel.CurrentKubeContext)
				assert.Contains(t, updatedModel.StatusBarMessage, "Switched to context")
			} else {
				assert.Contains(t, updatedModel.StatusBarMessage, "Failed to switch")
			}
			assert.Equal(t, tt.expectedType, updatedModel.StatusBarMessageType)
		})
	}
}

func TestHandleMainDashboardKeys(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		focusedPanelKey  string
		setupModel       func(*model.Model)
		expectedBehavior string
	}{
		{
			name:             "restart focused service",
			key:              "r",
			focusedPanelKey:  "test-service",
			expectedBehavior: "should restart service",
		},
		{
			name:             "stop focused service",
			key:              "x",
			focusedPanelKey:  "test-service",
			expectedBehavior: "should stop service",
		},
		{
			name:            "start stopped service",
			key:             "enter",
			focusedPanelKey: "test-mcp",
			setupModel: func(m *model.Model) {
				m.MCPServers["test-mcp"] = &api.MCPServerInfo{
					Name:  "test-mcp",
					State: "stopped",
				}
			},
			expectedBehavior: "should start service",
		},
		{
			name:             "switch to MC context",
			key:              "s",
			focusedPanelKey:  model.McPaneFocusKey,
			expectedBehavior: "should switch context",
		},
		{
			name:             "switch to WC context",
			key:              "s",
			focusedPanelKey:  model.WcPaneFocusKey,
			expectedBehavior: "should switch context",
		},
		{
			name:             "tab navigation",
			key:              "tab",
			expectedBehavior: "should cycle focus",
		},
		{
			name:             "dark mode toggle",
			key:              "D",
			expectedBehavior: "should toggle dark mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := createTestModel()
			defer cleanup.cleanup()
			m.CurrentAppMode = model.ModeMainDashboard
			m.FocusedPanelKey = tt.focusedPanelKey

			if tt.setupModel != nil {
				tt.setupModel(m)
			}

			msg := tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune(tt.key),
			}
			if tt.key == "tab" {
				msg = tea.KeyMsg{Type: tea.KeyTab}
			} else if tt.key == "enter" {
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			}

			cmd := handleMainDashboardKeys(m, msg)

			// Verify appropriate command is returned based on the key
			switch tt.key {
			case "r", "x", "enter", "s":
				assert.NotNil(t, cmd, "Expected command for key %s", tt.key)
			case "q":
				assert.True(t, m.QuitApp)
			}
		})
	}
}

func TestCycleFocus(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()
	m.ManagementClusterName = "test-mc"
	m.WorkloadClusterName = "test-wc"

	// Set up config instead of order arrays
	m.PortForwardingConfig = []config.PortForwardDefinition{
		{Name: "pf1", Enabled: true},
		{Name: "pf2", Enabled: true},
	}
	m.MCPServerConfig = []config.MCPServerDefinition{
		{Name: "mcp1", Enabled: true},
		{Name: "mcp2", Enabled: true},
	}

	// Test forward cycling
	m.FocusedPanelKey = model.McPaneFocusKey
	cycleFocus(m, 1)
	assert.Equal(t, model.WcPaneFocusKey, m.FocusedPanelKey)

	cycleFocus(m, 1)
	assert.Equal(t, "pf1", m.FocusedPanelKey)

	cycleFocus(m, 1)
	assert.Equal(t, "pf2", m.FocusedPanelKey)

	cycleFocus(m, 1)
	assert.Equal(t, "mcp1", m.FocusedPanelKey)

	cycleFocus(m, 1)
	assert.Equal(t, "mcp2", m.FocusedPanelKey)

	// Should wrap around
	cycleFocus(m, 1)
	assert.Equal(t, model.McPaneFocusKey, m.FocusedPanelKey)

	// Test backward cycling
	cycleFocus(m, -1)
	assert.Equal(t, "mcp2", m.FocusedPanelKey)

	cycleFocus(m, -1)
	assert.Equal(t, "mcp1", m.FocusedPanelKey)
}

func TestUpdate_MaxActivityLogLines(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()

	// Create closed log channel
	m.LogChannel = make(<-chan logging.LogEntry)

	// Fill activity log beyond max
	for i := 0; i < model.MaxActivityLogLines+10; i++ {
		msg := model.NewLogEntryMsg{
			Entry: logging.LogEntry{
				Timestamp: time.Now(),
				Level:     logging.LevelInfo,
				Subsystem: "test",
				Message:   "test message",
			},
		}
		m, _ = Update(msg, m)
	}

	// Should be capped at MaxActivityLogLines
	assert.LessOrEqual(t, len(m.ActivityLog), model.MaxActivityLogLines)
}

func TestHandleServiceStateChange(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()

	tests := []struct {
		name     string
		event    api.ServiceStateChangedEvent
		checkLog string
	}{
		{
			name: "state change without error",
			event: api.ServiceStateChangedEvent{
				Label:       "test-service",
				ServiceType: "PortForward",
				OldState:    "running",
				NewState:    "stopped",
			},
			checkLog: "running → stopped",
		},
		{
			name: "state change with error",
			event: api.ServiceStateChangedEvent{
				Label:       "test-service",
				ServiceType: "PortForward",
				OldState:    "running",
				NewState:    "error",
				Error:       errors.New("connection failed"),
			},
			checkLog: "error: connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := handleServiceStateChange(m, tt.event)

			assert.NotNil(t, cmd)
			assert.True(t, m.ActivityLogDirty)
			assert.Contains(t, m.ActivityLog[len(m.ActivityLog)-1], tt.checkLog)
		})
	}
}

func TestUpdate_ChannelReaderCmd(t *testing.T) {
	m, cleanup := createTestModel()
	defer cleanup.cleanup()

	// Test that channel reader is re-queued for various messages
	messages := []tea.Msg{
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
		model.ServiceStartedMsg{Label: "test"},
		model.NewLogEntryMsg{Entry: logging.LogEntry{Level: logging.LevelInfo}},
	}

	for _, msg := range messages {
		_, cmd := Update(msg, m)
		assert.NotNil(t, cmd, "Expected channel reader command to be re-queued")
	}
}
