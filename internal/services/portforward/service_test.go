package portforward

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/portforwarding"
	"envctl/internal/services"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockKubeManager implements kube.Manager for testing
type mockKubeManager struct {
	loginFunc                    func(clusterName string) (string, string, error)
	listClustersFunc             func() (*kube.ClusterInfo, error)
	getCurrentContextFunc        func() (string, error)
	switchContextFunc            func(targetContextName string) error
	getAvailableContextsFunc     func() ([]string, error)
	buildMcContextNameFunc       func(mcName string) string
	buildWcContextNameFunc       func(mcName, wcName string) string
	stripTeleportPrefixFunc      func(contextName string) string
	hasTeleportPrefixFunc        func(contextName string) bool
	getClusterNodeHealthFunc     func(ctx context.Context, kubeContextName string) (kube.NodeHealth, error)
	determineClusterProviderFunc func(ctx context.Context, kubeContextName string) (string, error)
}

func (m *mockKubeManager) Login(clusterName string) (string, string, error) {
	if m.loginFunc != nil {
		return m.loginFunc(clusterName)
	}
	return "", "", nil
}

func (m *mockKubeManager) ListClusters() (*kube.ClusterInfo, error) {
	if m.listClustersFunc != nil {
		return m.listClustersFunc()
	}
	return &kube.ClusterInfo{}, nil
}

func (m *mockKubeManager) GetCurrentContext() (string, error) {
	if m.getCurrentContextFunc != nil {
		return m.getCurrentContextFunc()
	}
	return "test-context", nil
}

func (m *mockKubeManager) SwitchContext(targetContextName string) error {
	if m.switchContextFunc != nil {
		return m.switchContextFunc(targetContextName)
	}
	return nil
}

func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	if m.getAvailableContextsFunc != nil {
		return m.getAvailableContextsFunc()
	}
	return []string{"test-context"}, nil
}

func (m *mockKubeManager) BuildMcContextName(mcName string) string {
	if m.buildMcContextNameFunc != nil {
		return m.buildMcContextNameFunc(mcName)
	}
	return fmt.Sprintf("mc-%s", mcName)
}

func (m *mockKubeManager) BuildWcContextName(mcName, wcName string) string {
	if m.buildWcContextNameFunc != nil {
		return m.buildWcContextNameFunc(mcName, wcName)
	}
	return fmt.Sprintf("mc-%s-wc-%s", mcName, wcName)
}

func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	if m.stripTeleportPrefixFunc != nil {
		return m.stripTeleportPrefixFunc(contextName)
	}
	return contextName
}

func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	if m.hasTeleportPrefixFunc != nil {
		return m.hasTeleportPrefixFunc(contextName)
	}
	return false
}

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (kube.NodeHealth, error) {
	if m.getClusterNodeHealthFunc != nil {
		return m.getClusterNodeHealthFunc(ctx, kubeContextName)
	}
	return kube.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	if m.determineClusterProviderFunc != nil {
		return m.determineClusterProviderFunc(ctx, kubeContextName)
	}
	return "aws", nil
}

func (m *mockKubeManager) CheckAPIHealth(ctx context.Context, kubeContextName string) error {
	// By default, return nil (healthy)
	return nil
}

func TestNewPortForwardService(t *testing.T) {
	tests := []struct {
		name           string
		cfg            config.PortForwardDefinition
		expectedLocal  int
		expectedRemote int
	}{
		{
			name: "basic configuration",
			cfg: config.PortForwardDefinition{
				Name:       "test-pf",
				Namespace:  "default",
				TargetType: "service",
				TargetName: "test-service",
				LocalPort:  "8080",
				RemotePort: "80",
			},
			expectedLocal:  8080,
			expectedRemote: 80,
		},
		{
			name: "invalid port numbers",
			cfg: config.PortForwardDefinition{
				Name:       "test-pf-invalid",
				Namespace:  "default",
				TargetType: "service",
				TargetName: "test-service",
				LocalPort:  "invalid",
				RemotePort: "invalid",
			},
			expectedLocal:  0,
			expectedRemote: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeMgr := &mockKubeManager{}
			svc := NewPortForwardService(tt.cfg, kubeMgr)

			if svc == nil {
				t.Fatal("expected service to be created")
			}

			if svc.GetLabel() != tt.cfg.Name {
				t.Errorf("expected label %s, got %s", tt.cfg.Name, svc.GetLabel())
			}

			if svc.GetType() != services.TypePortForward {
				t.Errorf("expected type %s, got %s", services.TypePortForward, svc.GetType())
			}

			if svc.localPort != tt.expectedLocal {
				t.Errorf("expected local port %d, got %d", tt.expectedLocal, svc.localPort)
			}

			if svc.remotePort != tt.expectedRemote {
				t.Errorf("expected remote port %d, got %d", tt.expectedRemote, svc.remotePort)
			}
		})
	}
}

func TestPortForwardService_Start(t *testing.T) {
	// Save original function and restore after test
	originalFunc := portforwarding.KubeStartPortForwardFn
	defer func() {
		portforwarding.KubeStartPortForwardFn = originalFunc
	}()

	tests := []struct {
		name          string
		cfg           config.PortForwardDefinition
		mockBehavior  func(t *testing.T) (chan struct{}, error)
		expectedState services.ServiceState
		expectError   bool
	}{
		{
			name: "successful start",
			cfg: config.PortForwardDefinition{
				Name:       "test-pf",
				Namespace:  "default",
				TargetType: "service",
				TargetName: "test-service",
				LocalPort:  "8080",
				RemotePort: "80",
			},
			mockBehavior: func(t *testing.T) (chan struct{}, error) {
				stopChan := make(chan struct{})
				// Mock the KubeStartPortForwardFn
				portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext string, namespace string, serviceArg string, portMap string, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
					// Simulate successful initialization
					go func() {
						time.Sleep(10 * time.Millisecond)
						bridgeFn("Initializing", "Starting port forward", false, false)
						time.Sleep(10 * time.Millisecond)
						bridgeFn("ForwardingActive", "Port forward active", false, true)
					}()
					return stopChan, "Initializing", nil
				}
				return stopChan, nil
			},
			expectedState: services.StateStarting,
			expectError:   false,
		},
		{
			name: "failed start",
			cfg: config.PortForwardDefinition{
				Name:       "test-pf-fail",
				Namespace:  "default",
				TargetType: "service",
				TargetName: "test-service",
				LocalPort:  "8080",
				RemotePort: "80",
			},
			mockBehavior: func(t *testing.T) (chan struct{}, error) {
				// Mock the KubeStartPortForwardFn to return error
				portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext string, namespace string, serviceArg string, portMap string, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
					return nil, "Failed", errors.New("failed to start port forward")
				}
				return nil, errors.New("failed to start port forward")
			},
			expectedState: services.StateFailed,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = tt.mockBehavior(t)

			kubeMgr := &mockKubeManager{}
			svc := NewPortForwardService(tt.cfg, kubeMgr)

			ctx := context.Background()
			err := svc.Start(ctx)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Give some time for state updates
			time.Sleep(50 * time.Millisecond)

			state := svc.GetState()
			if state != tt.expectedState && state != services.StateRunning {
				t.Errorf("expected state %s or %s, got %s", tt.expectedState, services.StateRunning, state)
			}
		})
	}
}

func TestPortForwardService_Stop(t *testing.T) {
	// Save original function and restore after test
	originalFunc := portforwarding.KubeStartPortForwardFn
	defer func() {
		portforwarding.KubeStartPortForwardFn = originalFunc
	}()

	cfg := config.PortForwardDefinition{
		Name:       "test-pf",
		Namespace:  "default",
		TargetType: "service",
		TargetName: "test-service",
		LocalPort:  "8080",
		RemotePort: "80",
	}

	kubeMgr := &mockKubeManager{}
	svc := NewPortForwardService(cfg, kubeMgr)

	// Mock the KubeStartPortForwardFn
	stopChan := make(chan struct{})
	portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext string, namespace string, serviceArg string, portMap string, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
		go func() {
			bridgeFn("ForwardingActive", "Port forward active", false, true)
		}()
		return stopChan, "Initializing", nil
	}

	// Start the service first
	ctx := context.Background()
	err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Give time for the service to start
	time.Sleep(50 * time.Millisecond)

	// Test stopping
	err = svc.Stop(ctx)
	if err != nil {
		t.Errorf("unexpected error stopping service: %v", err)
	}

	// Verify state
	state := svc.GetState()
	if state != services.StateStopped {
		t.Errorf("expected state %s, got %s", services.StateStopped, state)
	}

	// Verify stopChan is nil
	svc.mu.RLock()
	if svc.stopChan != nil {
		t.Error("expected stopChan to be nil after stop")
	}
	svc.mu.RUnlock()
}

func TestPortForwardService_Restart(t *testing.T) {
	// Save original function and restore after test
	originalFunc := portforwarding.KubeStartPortForwardFn
	defer func() {
		portforwarding.KubeStartPortForwardFn = originalFunc
	}()

	cfg := config.PortForwardDefinition{
		Name:       "test-pf",
		Namespace:  "default",
		TargetType: "service",
		TargetName: "test-service",
		LocalPort:  "8080",
		RemotePort: "80",
	}

	kubeMgr := &mockKubeManager{}
	svc := NewPortForwardService(cfg, kubeMgr)

	callCount := 0
	var stopChans []chan struct{}
	var mu sync.Mutex

	// Mock the KubeStartPortForwardFn
	portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext string, namespace string, serviceArg string, portMap string, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
		mu.Lock()
		callCount++
		stopChan := make(chan struct{})
		stopChans = append(stopChans, stopChan)
		mu.Unlock()

		go func() {
			bridgeFn("ForwardingActive", "Port forward active", false, true)
		}()
		return stopChan, "Initializing", nil
	}

	// Start the service first
	ctx := context.Background()
	err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Give time for the service to start
	time.Sleep(50 * time.Millisecond)

	// Test restart
	err = svc.Restart(ctx)
	if err != nil {
		t.Errorf("unexpected error restarting service: %v", err)
	}

	// Give time for restart to complete
	time.Sleep(1100 * time.Millisecond)

	// Verify the service was started twice
	mu.Lock()
	if callCount != 2 {
		t.Errorf("expected portforward to be called 2 times, got %d", callCount)
	}
	mu.Unlock()
}

func TestPortForwardService_GetServiceData(t *testing.T) {
	cfg := config.PortForwardDefinition{
		Name:              "test-pf",
		Namespace:         "default",
		TargetType:        "service",
		TargetName:        "test-service",
		LocalPort:         "8080",
		RemotePort:        "80",
		BindAddress:       "127.0.0.1",
		Enabled:           true,
		Icon:              "🔌",
		Category:          "networking",
		KubeContextTarget: "test-context",
	}

	kubeMgr := &mockKubeManager{}
	svc := NewPortForwardService(cfg, kubeMgr)

	data := svc.GetServiceData()

	// Verify all expected fields
	expectedFields := map[string]interface{}{
		"name":        "test-pf",
		"namespace":   "default",
		"targetType":  "service",
		"targetName":  "test-service",
		"localPort":   8080,
		"remotePort":  80,
		"bindAddress": "127.0.0.1",
		"enabled":     true,
		"icon":        "🔌",
		"category":    "networking",
		"context":     "test-context",
	}

	for key, expectedValue := range expectedFields {
		if value, ok := data[key]; !ok {
			t.Errorf("missing field %s in service data", key)
		} else if value != expectedValue {
			t.Errorf("field %s: expected %v, got %v", key, expectedValue, value)
		}
	}
}

func TestPortForwardService_CheckHealth(t *testing.T) {
	tests := []struct {
		name           string
		currentState   services.ServiceState
		expectedHealth services.HealthStatus
		expectError    bool
	}{
		{
			name:           "running state",
			currentState:   services.StateRunning,
			expectedHealth: services.HealthHealthy,
			expectError:    false,
		},
		{
			name:           "failed state",
			currentState:   services.StateFailed,
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
		{
			name:           "starting state",
			currentState:   services.StateStarting,
			expectedHealth: services.HealthChecking,
			expectError:    false,
		},
		{
			name:           "stopped state",
			currentState:   services.StateStopped,
			expectedHealth: services.HealthUnknown,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.PortForwardDefinition{
				Name: "test-pf",
			}

			kubeMgr := &mockKubeManager{}
			svc := NewPortForwardService(cfg, kubeMgr)

			// Set the state
			svc.UpdateState(tt.currentState, services.HealthUnknown, nil)

			ctx := context.Background()
			health, err := svc.CheckHealth(ctx)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if health != tt.expectedHealth {
				t.Errorf("expected health %s, got %s", tt.expectedHealth, health)
			}
		})
	}
}

func TestPortForwardService_GetHealthCheckInterval(t *testing.T) {
	tests := []struct {
		name             string
		configInterval   time.Duration
		expectedInterval time.Duration
	}{
		{
			name:             "default interval",
			configInterval:   0,
			expectedInterval: 10 * time.Second,
		},
		{
			name:             "custom interval",
			configInterval:   30 * time.Second,
			expectedInterval: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.PortForwardDefinition{
				Name:                "test-pf",
				HealthCheckInterval: tt.configInterval,
			}

			kubeMgr := &mockKubeManager{}
			svc := NewPortForwardService(cfg, kubeMgr)

			interval := svc.GetHealthCheckInterval()
			if interval != tt.expectedInterval {
				t.Errorf("expected interval %v, got %v", tt.expectedInterval, interval)
			}
		})
	}
}

func TestPortForwardService_StatusUpdates(t *testing.T) {
	// Save original function and restore after test
	originalFunc := portforwarding.KubeStartPortForwardFn
	defer func() {
		portforwarding.KubeStartPortForwardFn = originalFunc
	}()

	cfg := config.PortForwardDefinition{
		Name:       "test-pf",
		Namespace:  "default",
		TargetType: "service",
		TargetName: "test-service",
		LocalPort:  "8080",
		RemotePort: "80",
	}

	kubeMgr := &mockKubeManager{}
	svc := NewPortForwardService(cfg, kubeMgr)

	// Track state changes
	var stateChanges []services.ServiceState
	var mu sync.Mutex

	svc.SetStateChangeCallback(func(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
		mu.Lock()
		stateChanges = append(stateChanges, newState)
		mu.Unlock()
	})

	// Mock the KubeStartPortForwardFn
	portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext string, namespace string, serviceArg string, portMap string, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
		stopChan := make(chan struct{})

		// Simulate status updates
		go func() {
			time.Sleep(10 * time.Millisecond)
			bridgeFn("Initializing", "Starting port forward", false, false)
			time.Sleep(10 * time.Millisecond)
			bridgeFn("ForwardingActive", "Port forward active", false, true)
			time.Sleep(10 * time.Millisecond)
			bridgeFn("Failed", "Connection lost", true, false)
		}()

		return stopChan, "Initializing", nil
	}

	ctx := context.Background()
	err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Wait for status updates
	time.Sleep(100 * time.Millisecond)

	// Verify state transitions
	mu.Lock()
	defer mu.Unlock()

	expectedStates := []services.ServiceState{
		services.StateStarting,
		services.StateRunning,
		services.StateFailed,
	}

	if len(stateChanges) < len(expectedStates) {
		t.Errorf("expected at least %d state changes, got %d", len(expectedStates), len(stateChanges))
	}
}

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name           string
		localPortStr   string
		remotePortStr  string
		expectedLocal  int
		expectedRemote int
	}{
		{
			name:           "valid ports",
			localPortStr:   "8080",
			remotePortStr:  "80",
			expectedLocal:  8080,
			expectedRemote: 80,
		},
		{
			name:           "invalid ports",
			localPortStr:   "invalid",
			remotePortStr:  "invalid",
			expectedLocal:  0,
			expectedRemote: 0,
		},
		{
			name:           "empty ports",
			localPortStr:   "",
			remotePortStr:  "",
			expectedLocal:  0,
			expectedRemote: 0,
		},
		{
			name:           "mixed valid and invalid",
			localPortStr:   "9090",
			remotePortStr:  "not-a-port",
			expectedLocal:  9090,
			expectedRemote: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localPort, remotePort := parsePortSpec(tt.localPortStr, tt.remotePortStr)

			if localPort != tt.expectedLocal {
				t.Errorf("expected local port %d, got %d", tt.expectedLocal, localPort)
			}

			if remotePort != tt.expectedRemote {
				t.Errorf("expected remote port %d, got %d", tt.expectedRemote, remotePort)
			}
		})
	}
}
