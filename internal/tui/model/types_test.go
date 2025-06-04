package model

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"errors"
	"testing"
	"time"
)

func TestSetStatusMessage(t *testing.T) {
	// Create a model with some width
	m := &Model{
		Width:                100,
		StatusBarMessage:     "",
		StatusBarMessageType: StatusBarInfo,
	}

	// Test 1: First call to SetStatusMessage
	cmd1 := m.SetStatusMessage("First message", StatusBarSuccess, 1*time.Second)
	if m.StatusBarMessage != "First message" {
		t.Errorf("Expected StatusBarMessage 'First message', got '%s'", m.StatusBarMessage)
	}
	if m.StatusBarMessageType != StatusBarSuccess {
		t.Errorf("Expected StatusBarMessageType Success, got %v", m.StatusBarMessageType)
	}
	if m.StatusBarClearCancel == nil {
		t.Error("Expected StatusBarClearCancel to be non-nil after first call")
	}
	if cmd1 == nil {
		t.Error("Expected a non-nil tea.Cmd from SetStatusMessage")
	}
	cancelChan1 := m.StatusBarClearCancel

	// Test 2: Second call to SetStatusMessage (should cancel the first)
	cmd2 := m.SetStatusMessage("Second message", StatusBarError, 1*time.Second)
	if m.StatusBarMessage != "Second message" {
		t.Errorf("Expected StatusBarMessage 'Second message', got '%s'", m.StatusBarMessage)
	}
	if m.StatusBarMessageType != StatusBarError {
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
}

func TestOverallAppStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status OverallAppStatus
		want   string
	}{
		{
			name:   "AppStatusUp",
			status: AppStatusUp,
			want:   "Up",
		},
		{
			name:   "AppStatusConnecting",
			status: AppStatusConnecting,
			want:   "Connecting",
		},
		{
			name:   "AppStatusDegraded",
			status: AppStatusDegraded,
			want:   "Degraded",
		},
		{
			name:   "AppStatusFailed",
			status: AppStatusFailed,
			want:   "Failed",
		},
		{
			name:   "AppStatusUnknown",
			status: AppStatusUnknown,
			want:   "Unknown",
		},
		{
			name:   "Invalid status",
			status: OverallAppStatus(999),
			want:   "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("OverallAppStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppMode_String(t *testing.T) {
	tests := []struct {
		name string
		mode AppMode
		want string
	}{
		{
			name: "ModeInitializing",
			mode: ModeInitializing,
			want: "Initializing",
		},
		{
			name: "ModeMainDashboard",
			mode: ModeMainDashboard,
			want: "MainDashboard",
		},
		{
			name: "ModeNewConnectionInput",
			mode: ModeNewConnectionInput,
			want: "NewConnectionInput",
		},
		{
			name: "ModeHelpOverlay",
			mode: ModeHelpOverlay,
			want: "HelpOverlay",
		},
		{
			name: "ModeLogOverlay",
			mode: ModeLogOverlay,
			want: "LogOverlay",
		},
		{
			name: "ModeMcpConfigOverlay",
			mode: ModeMcpConfigOverlay,
			want: "McpConfigOverlay",
		},
		{
			name: "ModeMcpToolsOverlay",
			mode: ModeMcpToolsOverlay,
			want: "McpToolsOverlay",
		},
		{
			name: "ModeQuitting",
			mode: ModeQuitting,
			want: "Quitting",
		},
		{
			name: "Invalid mode",
			mode: AppMode(999),
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("AppMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetK8sConnectionHealth(t *testing.T) {
	m := &Model{
		K8sConnections: map[string]*api.K8sConnectionInfo{
			"conn1": {
				Label:      "conn1",
				Health:     "healthy",
				ReadyNodes: 3,
				TotalNodes: 3,
			},
			"conn2": {
				Label:      "conn2",
				Health:     "unhealthy",
				ReadyNodes: 2,
				TotalNodes: 3,
			},
		},
	}

	tests := []struct {
		name        string
		label       string
		wantReady   int
		wantTotal   int
		wantHealthy bool
	}{
		{
			name:        "healthy connection",
			label:       "conn1",
			wantReady:   3,
			wantTotal:   3,
			wantHealthy: true,
		},
		{
			name:        "unhealthy connection",
			label:       "conn2",
			wantReady:   2,
			wantTotal:   3,
			wantHealthy: false,
		},
		{
			name:        "non-existing connection",
			label:       "conn3",
			wantReady:   0,
			wantTotal:   0,
			wantHealthy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready, total, healthy := m.GetK8sConnectionHealth(tt.label)
			if ready != tt.wantReady {
				t.Errorf("GetK8sConnectionHealth() ready = %v, want %v", ready, tt.wantReady)
			}
			if total != tt.wantTotal {
				t.Errorf("GetK8sConnectionHealth() total = %v, want %v", total, tt.wantTotal)
			}
			if healthy != tt.wantHealthy {
				t.Errorf("GetK8sConnectionHealth() healthy = %v, want %v", healthy, tt.wantHealthy)
			}
		})
	}
}

func TestGetPortForwardStatus(t *testing.T) {
	m := &Model{
		PortForwards: map[string]*api.PortForwardServiceInfo{
			"pf1": {
				Label:      "pf1",
				State:      "running",
				LocalPort:  8080,
				RemotePort: 80,
			},
			"pf2": {
				Label:      "pf2",
				State:      "stopped",
				LocalPort:  9090,
				RemotePort: 90,
			},
		},
	}

	tests := []struct {
		name           string
		label          string
		wantRunning    bool
		wantLocalPort  int
		wantRemotePort int
	}{
		{
			name:           "running port forward",
			label:          "pf1",
			wantRunning:    true,
			wantLocalPort:  8080,
			wantRemotePort: 80,
		},
		{
			name:           "stopped port forward",
			label:          "pf2",
			wantRunning:    false,
			wantLocalPort:  9090,
			wantRemotePort: 90,
		},
		{
			name:           "non-existing port forward",
			label:          "pf3",
			wantRunning:    false,
			wantLocalPort:  0,
			wantRemotePort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			running, localPort, remotePort := m.GetPortForwardStatus(tt.label)
			if running != tt.wantRunning {
				t.Errorf("GetPortForwardStatus() running = %v, want %v", running, tt.wantRunning)
			}
			if localPort != tt.wantLocalPort {
				t.Errorf("GetPortForwardStatus() localPort = %v, want %v", localPort, tt.wantLocalPort)
			}
			if remotePort != tt.wantRemotePort {
				t.Errorf("GetPortForwardStatus() remotePort = %v, want %v", remotePort, tt.wantRemotePort)
			}
		})
	}
}

// Mock OrchestratorAPI for testing
type mockOrchestratorAPI struct {
	startServiceFunc   func(label string) error
	stopServiceFunc    func(label string) error
	restartServiceFunc func(label string) error
}

func (m *mockOrchestratorAPI) StartService(label string) error {
	if m.startServiceFunc != nil {
		return m.startServiceFunc(label)
	}
	return nil
}

func (m *mockOrchestratorAPI) StopService(label string) error {
	if m.stopServiceFunc != nil {
		return m.stopServiceFunc(label)
	}
	return nil
}

func (m *mockOrchestratorAPI) RestartService(label string) error {
	if m.restartServiceFunc != nil {
		return m.restartServiceFunc(label)
	}
	return nil
}

func (m *mockOrchestratorAPI) GetServiceStatus(label string) (*api.ServiceStatus, error) {
	return &api.ServiceStatus{
		Label: label,
		State: services.StateRunning,
	}, nil
}

func (m *mockOrchestratorAPI) GetAllServices() []api.ServiceStatus {
	return []api.ServiceStatus{}
}

func (m *mockOrchestratorAPI) SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent {
	ch := make(chan orchestrator.ServiceStateChangedEvent)
	close(ch)
	return ch
}

// Cluster management methods
func (m *mockOrchestratorAPI) GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition {
	return []config.ClusterDefinition{}
}

func (m *mockOrchestratorAPI) GetActiveCluster(role config.ClusterRole) (string, bool) {
	return "", false
}

func (m *mockOrchestratorAPI) SwitchCluster(role config.ClusterRole, clusterName string) error {
	return nil
}

func TestStartService(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		wantError bool
		apiError  error
	}{
		{
			name:      "successful start",
			label:     "test-service",
			wantError: false,
			apiError:  nil,
		},
		{
			name:      "failed start",
			label:     "test-service",
			wantError: true,
			apiError:  errors.New("start failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockOrchestratorAPI{
				startServiceFunc: func(label string) error {
					if label != tt.label {
						t.Errorf("StartService called with label = %v, want %v", label, tt.label)
					}
					return tt.apiError
				},
			}

			m := &Model{
				OrchestratorAPI: mockAPI,
			}

			cmd := m.StartService(tt.label)
			if cmd == nil {
				t.Fatal("StartService() returned nil command")
			}

			// Execute the command
			msg := cmd()

			if tt.wantError {
				errMsg, ok := msg.(ServiceErrorMsg)
				if !ok {
					t.Fatalf("Expected ServiceErrorMsg, got %T", msg)
				}
				if errMsg.Label != tt.label {
					t.Errorf("ServiceErrorMsg.Label = %v, want %v", errMsg.Label, tt.label)
				}
				if errMsg.Err == nil {
					t.Error("ServiceErrorMsg.Err is nil, want error")
				}
			} else {
				startedMsg, ok := msg.(ServiceStartedMsg)
				if !ok {
					t.Fatalf("Expected ServiceStartedMsg, got %T", msg)
				}
				if startedMsg.Label != tt.label {
					t.Errorf("ServiceStartedMsg.Label = %v, want %v", startedMsg.Label, tt.label)
				}
			}
		})
	}
}

func TestStopService(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		wantError bool
		apiError  error
	}{
		{
			name:      "successful stop",
			label:     "test-service",
			wantError: false,
			apiError:  nil,
		},
		{
			name:      "failed stop",
			label:     "test-service",
			wantError: true,
			apiError:  errors.New("stop failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockOrchestratorAPI{
				stopServiceFunc: func(label string) error {
					if label != tt.label {
						t.Errorf("StopService called with label = %v, want %v", label, tt.label)
					}
					return tt.apiError
				},
			}

			m := &Model{
				OrchestratorAPI: mockAPI,
			}

			cmd := m.StopService(tt.label)
			if cmd == nil {
				t.Fatal("StopService() returned nil command")
			}

			// Execute the command
			msg := cmd()

			if tt.wantError {
				errMsg, ok := msg.(ServiceErrorMsg)
				if !ok {
					t.Fatalf("Expected ServiceErrorMsg, got %T", msg)
				}
				if errMsg.Label != tt.label {
					t.Errorf("ServiceErrorMsg.Label = %v, want %v", errMsg.Label, tt.label)
				}
				if errMsg.Err == nil {
					t.Error("ServiceErrorMsg.Err is nil, want error")
				}
			} else {
				stoppedMsg, ok := msg.(ServiceStoppedMsg)
				if !ok {
					t.Fatalf("Expected ServiceStoppedMsg, got %T", msg)
				}
				if stoppedMsg.Label != tt.label {
					t.Errorf("ServiceStoppedMsg.Label = %v, want %v", stoppedMsg.Label, tt.label)
				}
			}
		})
	}
}

func TestRestartService(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		wantError bool
		apiError  error
	}{
		{
			name:      "successful restart",
			label:     "test-service",
			wantError: false,
			apiError:  nil,
		},
		{
			name:      "failed restart",
			label:     "test-service",
			wantError: true,
			apiError:  errors.New("restart failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockOrchestratorAPI{
				restartServiceFunc: func(label string) error {
					if label != tt.label {
						t.Errorf("RestartService called with label = %v, want %v", label, tt.label)
					}
					return tt.apiError
				},
			}

			m := &Model{
				OrchestratorAPI: mockAPI,
			}

			cmd := m.RestartService(tt.label)
			if cmd == nil {
				t.Fatal("RestartService() returned nil command")
			}

			// Execute the command
			msg := cmd()

			if tt.wantError {
				errMsg, ok := msg.(ServiceErrorMsg)
				if !ok {
					t.Fatalf("Expected ServiceErrorMsg, got %T", msg)
				}
				if errMsg.Label != tt.label {
					t.Errorf("ServiceErrorMsg.Label = %v, want %v", errMsg.Label, tt.label)
				}
				if errMsg.Err == nil {
					t.Error("ServiceErrorMsg.Err is nil, want error")
				}
			} else {
				restartedMsg, ok := msg.(ServiceRestartedMsg)
				if !ok {
					t.Fatalf("Expected ServiceRestartedMsg, got %T", msg)
				}
				if restartedMsg.Label != tt.label {
					t.Errorf("ServiceRestartedMsg.Label = %v, want %v", restartedMsg.Label, tt.label)
				}
			}
		})
	}
}
