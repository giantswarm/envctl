package model_test

import (
	"context" // Required for mockClusterService.Health
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/service" // For service.Services and sub-interfaces
	"envctl/internal/tui/controller"
	"envctl/internal/tui/model"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Mock Service Implementations ---

type mockClusterService struct{}

func (m *mockClusterService) CurrentContext() (string, error)   { return "test-context", nil }
func (m *mockClusterService) SwitchContext(mc, wc string) error { return nil }
func (m *mockClusterService) Health(ctx context.Context, cluster string) (service.ClusterHealthInfo, error) {
	return service.ClusterHealthInfo{IsLoading: false, Error: nil}, nil
}

type mockPFService struct{}

func (m *mockPFService) Start(cfg portforwarding.PortForwardConfig, cb portforwarding.PortForwardUpdateFunc) (stopChan chan struct{}, err error) {
	return make(chan struct{}), nil
}
func (m *mockPFService) Status(id string) portforwarding.PortForwardProcessUpdate {
	return portforwarding.PortForwardProcessUpdate{InstanceKey: id, StatusMsg: "mocked pf status", Running: true}
}

type mockProxyService struct{}

func (m *mockProxyService) Start(cfg mcpserver.PredefinedMcpServer, updateFn func(mcpserver.McpProcessUpdate)) (stopChan chan struct{}, pid int, err error) {
	return make(chan struct{}), 0, nil
}
func (m *mockProxyService) Status(name string) (running bool, err error) { return true, nil }

// --- Tests ---

func TestAppModeTransitions(t *testing.T) {
	tests := []struct {
		name            string
		initialAppMode  model.AppMode
		msg             tea.Msg
		expectedAppMode model.AppMode
		description     string
		initialModelSetup func(*model.Model)
		assertModel       func(*testing.T, *model.Model)
	}{
		{
			name:            "Toggle Help Overlay from MainDashboard via 'h' key",
			initialAppMode:  model.ModeMainDashboard,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: false},
			expectedAppMode: model.ModeHelpOverlay,
			description:     "Pressing 'h' on MainDashboard should show HelpOverlay.",
		},
		{
			name:            "Toggle Help Overlay off from HelpOverlay via 'h' key",
			initialAppMode:  model.ModeHelpOverlay,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}, Alt: false},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Pressing 'h' on HelpOverlay should return to MainDashboard.",
		},
		{
			name:            "Toggle Help Overlay off from HelpOverlay via 'esc' key",
			initialAppMode:  model.ModeHelpOverlay,
			msg:             tea.KeyMsg{Type: tea.KeyEscape, Alt: false},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Pressing 'esc' on HelpOverlay should return to MainDashboard.",
		},
		{
			name:            "Toggle Log Overlay from MainDashboard via 'L' key",
			initialAppMode:  model.ModeMainDashboard,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}, Alt: false},
			expectedAppMode: model.ModeLogOverlay,
			description:     "Pressing 'L' on MainDashboard should show LogOverlay.",
		},
		{
			name:            "Toggle Log Overlay off from LogOverlay via 'L' key",
			initialAppMode:  model.ModeLogOverlay,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}, Alt: false},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Pressing 'L' on LogOverlay should return to MainDashboard.",
		},
		{
			name:            "Toggle Log Overlay off from LogOverlay via 'esc' key",
			initialAppMode:  model.ModeLogOverlay,
			msg:             tea.KeyMsg{Type: tea.KeyEscape, Alt: false},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Pressing 'esc' on LogOverlay should return to MainDashboard.",
		},
		{
			name:            "Toggle McpConfig Overlay from MainDashboard via 'C' key",
			initialAppMode:  model.ModeMainDashboard,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}, Alt: false},
			expectedAppMode: model.ModeMcpConfigOverlay,
			description:     "Pressing 'C' on MainDashboard should show McpConfigOverlay.",
		},
		{
			name:            "Toggle McpConfig Overlay off from McpConfigOverlay via 'C' key",
			initialAppMode:  model.ModeMcpConfigOverlay,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}, Alt: false},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Pressing 'C' on McpConfigOverlay should return to MainDashboard.",
		},
		{
			name:            "Toggle McpConfig Overlay off from McpConfigOverlay via 'esc' key",
			initialAppMode:  model.ModeMcpConfigOverlay,
			msg:             tea.KeyMsg{Type: tea.KeyEscape, Alt: false},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Pressing 'esc' on McpConfigOverlay should return to MainDashboard.",
		},
		{
			name:            "Enter NewConnectionInput from MainDashboard via 'n' key",
			initialAppMode:  model.ModeMainDashboard,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}, Alt: false},
			expectedAppMode: model.ModeNewConnectionInput,
			description:     "Pressing 'n' on MainDashboard should go to NewConnectionInput mode.",
		},
		{
			name:            "Quit from MainDashboard via 'q' key",
			initialAppMode:  model.ModeMainDashboard,
			msg:             tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}, Alt: false},
			expectedAppMode: model.ModeQuitting,
			description:     "Pressing 'q' on MainDashboard should set mode to Quitting.",
		},
		// Test case for WindowSizeMsg changing from Initializing to MainDashboard
		{
			name:            "WindowSizeMsg transitions from Initializing to MainDashboard",
			initialAppMode:  model.ModeInitializing,
			msg:             tea.WindowSizeMsg{Width: 80, Height: 24},
			expectedAppMode: model.ModeMainDashboard,
			description:     "Receiving WindowSizeMsg when Initializing should switch to MainDashboard.",
		},
		// --- Tests for ModeNewConnectionInput flow ---
		{
			name:            "NewConnectionInput: MC name entered, press Enter",
			initialAppMode:  model.ModeNewConnectionInput, // Assume 'n' was pressed before
			msg:             tea.KeyMsg{Type: tea.KeyEnter, Alt: false},
			// Setup initial model for this specific state within NewConnectionInput
			initialModelSetup: func(m *model.Model) {
				m.CurrentInputStep = model.McInputStep
				m.NewConnectionInput.SetValue("my-mc")
				m.NewConnectionInput.Focus() // Ensure input is focused
			},
			expectedAppMode: model.ModeNewConnectionInput, // Stays in this mode, but step changes
			assertModel: func(t *testing.T, m *model.Model) {
				if m.CurrentInputStep != model.WcInputStep {
					t.Errorf("Expected CurrentInputStep to be WcInputStep, got %v", m.CurrentInputStep)
				}
				if m.StashedMcName != "my-mc" {
					t.Errorf("Expected StashedMcName to be 'my-mc', got '%s'", m.StashedMcName)
				}
				if m.NewConnectionInput.Value() != "" {
					t.Errorf("Expected NewConnectionInput value to be empty for WC, got '%s'", m.NewConnectionInput.Value())
				}
			},
			description: "Entering MC name and pressing Enter should move to WC input step.",
		},
		{
			name:            "NewConnectionInput: WC name entered, press Enter",
			initialAppMode:  model.ModeNewConnectionInput,
			msg:             tea.KeyMsg{Type: tea.KeyEnter, Alt: false},
			initialModelSetup: func(m *model.Model) {
				m.CurrentInputStep = model.WcInputStep
				m.StashedMcName = "my-mc"
				m.NewConnectionInput.SetValue("my-wc")
				m.NewConnectionInput.Focus()
			},
			expectedAppMode: model.ModeMainDashboard, // Transitions out of input mode
			assertModel: func(t *testing.T, m *model.Model) {
				// Check if input is blurred and reset (controller.handleKeyMsgInputMode does this)
				if m.NewConnectionInput.Focused() {
					t.Error("Expected NewConnectionInput to be blurred")
				}
				if m.NewConnectionInput.Value() != "" { // Should be reset
					t.Errorf("Expected NewConnectionInput value to be reset, got '%s'", m.NewConnectionInput.Value())
				}
				// FocusedPanelKey might be set by controller if PortForwardOrder is not empty
				// This depends on the state of PortForwardOrder after SetupPortForwards
				// For simplicity, we won't assert FocusedPanelKey rigidly here without knowing exact default PFs
			},
			description: "Entering WC name and pressing Enter should return to MainDashboard.",
		},
		{
			name:            "NewConnectionInput: MC step, press Esc",
			initialAppMode:  model.ModeNewConnectionInput,
			msg:             tea.KeyMsg{Type: tea.KeyEscape, Alt: false},
			initialModelSetup: func(m *model.Model) {
				m.CurrentInputStep = model.McInputStep
				m.NewConnectionInput.SetValue("some-mc-text")
				m.NewConnectionInput.Focus()
			},
			expectedAppMode: model.ModeMainDashboard,
			assertModel: func(t *testing.T, m *model.Model) {
				if m.NewConnectionInput.Value() != "" {
					t.Errorf("Expected NewConnectionInput value to be reset, got '%s'", m.NewConnectionInput.Value())
				}
				if m.CurrentInputStep != model.McInputStep { // Should reset to initial step
					t.Errorf("Expected CurrentInputStep to be reset to McInputStep, got %v", m.CurrentInputStep)
				}
			},
			description: "Pressing Esc during MC input should cancel and return to MainDashboard.",
		},
		{
			name:            "NewConnectionInput: WC step, press Esc",
			initialAppMode:  model.ModeNewConnectionInput,
			msg:             tea.KeyMsg{Type: tea.KeyEscape, Alt: false},
			initialModelSetup: func(m *model.Model) {
				m.CurrentInputStep = model.WcInputStep
				m.StashedMcName = "my-mc"
				m.NewConnectionInput.SetValue("some-wc-text")
				m.NewConnectionInput.Focus()
			},
			expectedAppMode: model.ModeMainDashboard,
			assertModel: func(t *testing.T, m *model.Model) {
				if m.NewConnectionInput.Value() != "" {
					t.Errorf("Expected NewConnectionInput value to be reset, got '%s'", m.NewConnectionInput.Value())
				}
				if m.StashedMcName != "" { // Stashed name should be cleared
					t.Errorf("Expected StashedMcName to be cleared, got '%s'", m.StashedMcName)
				}
				if m.CurrentInputStep != model.McInputStep { // Should reset to initial step
					t.Errorf("Expected CurrentInputStep to be reset to McInputStep, got %v", m.CurrentInputStep)
				}
			},
			description: "Pressing Esc during WC input should cancel, clear stashed MC, and return to MainDashboard.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreModel := model.InitialModel("test-mc", "test-wc", "test-context", false)
			coreModel.CurrentAppMode = tt.initialAppMode
			
			// Apply test-specific initial model setup
			if tt.initialModelSetup != nil {
				tt.initialModelSetup(coreModel)
			}

			coreModel.Services = service.Services{
				Cluster: &mockClusterService{},
				PF:      &mockPFService{},
				Proxy:   &mockProxyService{},
			}

			appModel := controller.NewAppModel(coreModel, coreModel.ManagementClusterName, coreModel.WorkloadClusterName)

			updatedTeaModel, _ := appModel.Update(tt.msg)

			finalControllerApp, ok := updatedTeaModel.(*controller.AppModel)
			if !ok {
				t.Fatalf("Update did not return a *controller.AppModel as expected")
			}

			finalModel := finalControllerApp.GetModel() // Use the new GetModel() method
			if finalModel.CurrentAppMode != tt.expectedAppMode {
				t.Errorf("%s: expected AppMode %s (%d), got %s (%d)", 
					tt.description, 
					tt.expectedAppMode.String(), tt.expectedAppMode, 
					finalModel.CurrentAppMode.String(), finalModel.CurrentAppMode)
			}
			// Apply additional model assertions if provided
			if tt.assertModel != nil {
				tt.assertModel(t, finalModel)
			}
		})
	}
}

func TestMessageHandling(t *testing.T) {
	tests := []struct {
		name         string
		initialModel func() *model.Model // Function to get a fresh model
		msg          tea.Msg
		assert       func(t *testing.T, m *model.Model)
		description  string
	}{
		{
			name: "ClearStatusBarMsg clears status bar",
			initialModel: func() *model.Model {
				m := model.InitialModel("mc", "wc", "ctx", false)
				m.StatusBarMessage = "Initial message"
				m.StatusBarMessageType = model.StatusBarInfo
				// Setup mock services
				m.Services = service.Services{
					Cluster: &mockClusterService{},
					PF:      &mockPFService{},
					Proxy:   &mockProxyService{},
				}
				return m
			},
			msg: model.ClearStatusBarMsg{},
			assert: func(t *testing.T, m *model.Model) {
				if m.StatusBarMessage != "" {
					t.Errorf("expected StatusBarMessage to be empty, got '%s'", m.StatusBarMessage)
				}
			},
			description: "ClearStatusBarMsg should reset the StatusBarMessage.",
		},
		{
			name: "PortForwardCoreUpdateMsg updates existing port-forward process",
			initialModel: func() *model.Model {
				m := model.InitialModel("mc", "wc", "ctx", false)
				m.Services = service.Services{
					Cluster: &mockClusterService{},
					PF:      &mockPFService{},
					Proxy:   &mockProxyService{},
				}
				return m
			},
			msg: model.PortForwardCoreUpdateMsg{
				Update: portforwarding.PortForwardProcessUpdate{
					InstanceKey: "Prometheus (MC)",
					StatusMsg:   "Updated Prometheus Status",
					OutputLog:   "New Prometheus log line",
					Running:     true,
					Error:       nil,
				},
			},
			assert: func(t *testing.T, m *model.Model) {
				pfKey := "Prometheus (MC)"
				updatedPf, ok := m.PortForwards[pfKey]
				if !ok {
					t.Fatalf("PortForward process '%s' not found in model. Available: %v", pfKey, getMapKeys(m.PortForwards))
				}
				if updatedPf.StatusMsg != "Updated Prometheus Status" {
					t.Errorf("expected StatusMsg 'Updated Prometheus Status', got '%s'", updatedPf.StatusMsg)
				}
				if len(updatedPf.Log) != 1 || updatedPf.Log[0] != "New Prometheus log line" {
					t.Errorf("expected log to contain 'New Prometheus log line', got %v", updatedPf.Log)
				}
				if !updatedPf.Running {
					t.Errorf("expected port-forward '%s' to be running", pfKey)
				}
			},
			description: "PortForwardCoreUpdateMsg should update a pre-defined PortForwardProcess.",
		},
		{
			name: "SetStatusMessage updates status bar and handles cancellation channel",
			initialModel: func() *model.Model {
				m := model.InitialModel("mc", "wc", "ctx", false)
				m.Services = service.Services{
					Cluster: &mockClusterService{},
					PF:      &mockPFService{},
					Proxy:   &mockProxyService{},
				}
				return m
			},
			// No specific message, we will call the method directly on the model in assert.
			// However, the test structure expects a message to dispatch through AppModel.Update().
			// To test SetStatusMessage directly on the model and its cancellation logic properly,
			// it might be better to have a dedicated test that doesn't go through AppModel.Update()
			// or to use a custom message that triggers this if AppModel.Update() is required.
			// For now, let's use a dummy msg and perform assertions directly on coreModel after setup.
			msg: model.NopMsg{}, // A dummy message that won't affect state much
			assert: func(t *testing.T, m *model.Model) {
				// First call to SetStatusMessage
				cmd1 := m.SetStatusMessage("First message", model.StatusBarSuccess, 1*time.Second)
				if m.StatusBarMessage != "First message" {
					t.Errorf("Expected StatusBarMessage 'First message', got '%s'", m.StatusBarMessage)
				}
				if m.StatusBarMessageType != model.StatusBarSuccess {
					t.Errorf("Expected StatusBarMessageType Success, got %v", m.StatusBarMessageType)
				}
				if m.StatusBarClearCancel == nil {
					t.Error("Expected StatusBarClearCancel to be non-nil after first call")
				}
				if cmd1 == nil {
					t.Error("Expected a non-nil tea.Cmd from SetStatusMessage")
				}
				cancelChan1 := m.StatusBarClearCancel

				// Second call to SetStatusMessage
				cmd2 := m.SetStatusMessage("Second message", model.StatusBarError, 1*time.Second)
				if m.StatusBarMessage != "Second message" {
					t.Errorf("Expected StatusBarMessage 'Second message', got '%s'", m.StatusBarMessage)
				}
				if m.StatusBarMessageType != model.StatusBarError {
					t.Errorf("Expected StatusBarMessageType Error, got %v", m.StatusBarMessageType)
				}
				if m.StatusBarClearCancel == nil {
					t.Error("Expected StatusBarClearCancel to be non-nil after second call")
				}
				if m.StatusBarClearCancel == cancelChan1 {
					t.Error("Expected StatusBarClearCancel to be a new channel after second call")
				}
				// Check if the first channel was closed
				select {
				case <-cancelChan1:
					// Expected: channel is closed
				default:
					t.Error("Expected first StatusBarClearCancel channel to be closed")
				}
				if cmd2 == nil {
					t.Error("Expected a non-nil tea.Cmd from second SetStatusMessage call")
				}
			},
			description: "SetStatusMessage should update message, type, and manage cancellation channel.",
		},
		// TODO: Add test for other relevant messages mentioned in the issue
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coreModel := tt.initialModel()
			// Ensure services are set up if not done in initialModel func
			if coreModel.Services.Cluster == nil { // Basic check, assumes partial setup if one is nil
				coreModel.Services = service.Services{
					Cluster: &mockClusterService{},
					PF:      &mockPFService{},
					Proxy:   &mockProxyService{},
				}
			}

			appModel := controller.NewAppModel(coreModel, coreModel.ManagementClusterName, coreModel.WorkloadClusterName)
			updatedTeaModel, _ := appModel.Update(tt.msg)

			finalControllerApp, ok := updatedTeaModel.(*controller.AppModel)
			if !ok {
				t.Fatalf("Update did not return a *controller.AppModel as expected")
			}

			tt.assert(t, finalControllerApp.GetModel())
		})
	}
}

// Helper function to get keys from a map for debugging
func getMapKeys(m map[string]*model.PortForwardProcess) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestAppMode_String(t *testing.T) {
	tests := []struct {
		mode model.AppMode
		want string
	}{
		{model.ModeInitializing, "Initializing"},
		{model.ModeMainDashboard, "MainDashboard"},
		{model.ModeNewConnectionInput, "NewConnectionInput"},
		{model.ModeHelpOverlay, "HelpOverlay"},
		{model.ModeLogOverlay, "LogOverlay"},
		{model.ModeMcpConfigOverlay, "McpConfigOverlay"},
		{model.ModeQuitting, "Quitting"},
		{model.ModeError, "Error"},
		{model.AppMode(99), "Unknown"}, // Test unknown value
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("AppMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOverallAppStatus_String(t *testing.T) {
	tests := []struct {
		status model.OverallAppStatus
		want   string
	}{
		{model.AppStatusUnknown, "Initializing"}, // Note: As per current String() impl, AppStatusUnknown (0) maps to "Initializing"
		{model.AppStatusUp, "Up"},
		{model.AppStatusConnecting, "Connecting"},
		{model.AppStatusDegraded, "Degraded"},
		{model.AppStatusFailed, "Failed"},
		{model.OverallAppStatus(99), "Unknown"}, // Test out-of-bounds value
		{model.OverallAppStatus(-1), "Unknown"}, // Test out-of-bounds negative value
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("OverallAppStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyMap_HelpMethods(t *testing.T) {
	keyMap := model.DefaultKeyMap() // Assuming this provides a standard KeyMap

	t.Run("FullHelp", func(t *testing.T) {
		fullHelp := keyMap.FullHelp()
		// Basic checks: not nil, expected number of groups, groups not empty
		if fullHelp == nil {
			t.Fatal("FullHelp() returned nil")
		}
		// Based on DefaultKeyMap, expecting 3 groups of keys
		if len(fullHelp) != 3 {
			t.Errorf("FullHelp() expected 3 groups of keybindings, got %d", len(fullHelp))
		}
		for i, group := range fullHelp {
			if len(group) == 0 {
				t.Errorf("FullHelp() group %d is empty", i)
			}
		}
		// Optionally, check for specific keys if their presence is critical
		// e.g., ensure Quit key is in one of the groups
		foundQuit := false
		for _, group := range fullHelp {
			for _, kb := range group {
				if kb.Help().Key == keyMap.Quit.Help().Key { // Comparing by displayed key string
					foundQuit = true
					break
				}
			}
			if foundQuit { break }
		}
		if !foundQuit {
			t.Error("FullHelp() did not contain the Quit keybinding")
		}
	})

	t.Run("ShortHelp", func(t *testing.T) {
		shortHelp := keyMap.ShortHelp()
		if shortHelp == nil {
			t.Fatal("ShortHelp() returned nil")
		}
		// Based on DefaultKeyMap, expecting Help and Quit keys
		if len(shortHelp) != 2 {
			t.Errorf("ShortHelp() expected 2 keybindings, got %d", len(shortHelp))
		}
		// Check for specific keys
		foundHelp := false
		foundQuit := false
		for _, kb := range shortHelp {
			if kb.Help().Key == keyMap.Help.Help().Key {
				foundHelp = true
			}
			if kb.Help().Key == keyMap.Quit.Help().Key {
				foundQuit = true
			}
		}
		if !foundHelp || !foundQuit {
			t.Errorf("ShortHelp() did not find both Help and Quit keybindings. FoundHelp: %v, FoundQuit: %v", foundHelp, foundQuit)
		}
	})

	// Test InputModeHelp as well, as it's part of the KeyMap methods
	t.Run("InputModeHelp", func(t *testing.T) {
		inputHelp := keyMap.InputModeHelp()
		if inputHelp == nil {
			t.Fatal("InputModeHelp() returned nil")
		}
		// Based on current implementation, expecting 1 group with 4 bindings
		if len(inputHelp) != 1 {
			t.Errorf("InputModeHelp() expected 1 group, got %d", len(inputHelp))
		}
		if len(inputHelp) > 0 && len(inputHelp[0]) != 4 {
			t.Errorf("InputModeHelp() group 0 expected 4 bindings, got %d", len(inputHelp[0]))
		}
		// Could add checks for specific keys like "enter" or "esc" if desired
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
