package api

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockOrchestratorService implements services.Service for testing
type mockOrchestratorService struct {
	label       string
	serviceType services.ServiceType
	state       services.ServiceState
	health      services.HealthStatus
	err         error
	deps        []string
}

func (m *mockOrchestratorService) GetLabel() string                  { return m.label }
func (m *mockOrchestratorService) GetType() services.ServiceType     { return m.serviceType }
func (m *mockOrchestratorService) GetState() services.ServiceState   { return m.state }
func (m *mockOrchestratorService) GetHealth() services.HealthStatus  { return m.health }
func (m *mockOrchestratorService) GetError() error                   { return m.err }
func (m *mockOrchestratorService) GetDependencies() []string         { return m.deps }
func (m *mockOrchestratorService) Start(ctx context.Context) error   { return nil }
func (m *mockOrchestratorService) Stop(ctx context.Context) error    { return nil }
func (m *mockOrchestratorService) Restart(ctx context.Context) error { return nil }
func (m *mockOrchestratorService) GetLastError() error               { return m.err }
func (m *mockOrchestratorService) SetStateChangeCallback(fn func(old, new services.ServiceState)) {
}

// orchestratorMockService implements services.Service for testing
type orchestratorMockService struct {
	label               string
	serviceType         services.ServiceType
	state               services.ServiceState
	health              services.HealthStatus
	dependencies        []string
	startErr            error
	stopErr             error
	restartErr          error
	lastErr             error
	mu                  sync.Mutex
	stateChangeCallback services.StateChangeCallback
}

func (m *orchestratorMockService) GetLabel() string                 { return m.label }
func (m *orchestratorMockService) GetType() services.ServiceType    { return m.serviceType }
func (m *orchestratorMockService) GetState() services.ServiceState  { return m.state }
func (m *orchestratorMockService) GetHealth() services.HealthStatus { return m.health }
func (m *orchestratorMockService) GetError() error                  { return m.lastErr }
func (m *orchestratorMockService) GetDependencies() []string        { return m.dependencies }
func (m *orchestratorMockService) GetLastError() error              { return m.lastErr }
func (m *orchestratorMockService) SetStateChangeCallback(cb services.StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateChangeCallback = cb
}

func (m *orchestratorMockService) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := m.state

	if m.startErr != nil {
		m.lastErr = m.startErr
		m.state = services.StateFailed
		if m.stateChangeCallback != nil {
			m.stateChangeCallback(m.label, oldState, m.state, m.health, m.startErr)
		}
		return m.startErr
	}

	// Simulate state transition
	m.state = services.StateStarting
	if m.stateChangeCallback != nil {
		m.stateChangeCallback(m.label, oldState, m.state, m.health, nil)
	}

	// Simulate successful start
	m.state = services.StateRunning
	m.health = services.HealthHealthy
	if m.stateChangeCallback != nil {
		m.stateChangeCallback(m.label, services.StateStarting, m.state, m.health, nil)
	}

	return nil
}

func (m *orchestratorMockService) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := m.state

	if m.stopErr != nil {
		m.lastErr = m.stopErr
		return m.stopErr
	}

	m.state = services.StateStopped
	m.health = services.HealthUnknown

	if m.stateChangeCallback != nil {
		m.stateChangeCallback(m.label, oldState, m.state, m.health, nil)
	}

	return nil
}

func (m *orchestratorMockService) Restart(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldState := m.state

	if m.restartErr != nil {
		m.lastErr = m.restartErr
		return m.restartErr
	}

	// Simulate restart - set to running after restart
	m.state = services.StateRunning

	if m.stateChangeCallback != nil {
		m.stateChangeCallback(m.label, oldState, m.state, m.health, nil)
	}

	return nil
}

func TestNewOrchestratorAPI(t *testing.T) {
	cfg := orchestrator.Config{}
	orch := orchestrator.New(cfg)
	registry := services.NewRegistry()

	api := NewOrchestratorAPI(orch, registry)

	if api == nil {
		t.Error("Expected NewOrchestratorAPI to return non-nil API")
	}

	// Test that it's the correct type
	if _, ok := api.(*orchestratorAPI); !ok {
		t.Error("Expected NewOrchestratorAPI to return *orchestratorAPI type")
	}
}

func TestOrchestratorAPI_StartService(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		startError  error
		expectError bool
	}{
		{
			name:        "successful start",
			label:       "test-service",
			startError:  nil,
			expectError: false,
		},
		{
			name:        "service start error",
			label:       "test-service",
			startError:  errors.New("start failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := orchestrator.Config{}
			orch := orchestrator.New(cfg)

			// Start the orchestrator
			ctx := context.Background()
			err := orch.Start(ctx)
			if err != nil {
				t.Fatalf("Failed to start orchestrator: %v", err)
			}
			defer orch.Stop()

			registry := orch.GetServiceRegistry()

			// Register a test service
			testService := &orchestratorMockService{
				label:       tt.label,
				serviceType: services.TypePortForward,
				state:       services.StateStopped,
				health:      services.HealthUnknown,
				startErr:    tt.startError,
			}
			registry.Register(testService)

			api := NewOrchestratorAPI(orch, registry)

			err = api.StartService(tt.label)

			if (err != nil) != tt.expectError {
				t.Errorf("StartService() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestOrchestratorAPI_StopService(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		stopError   error
		expectError bool
	}{
		{
			name:        "successful stop",
			label:       "test-service",
			stopError:   nil,
			expectError: false,
		},
		{
			name:        "service stop error",
			label:       "test-service",
			stopError:   errors.New("stop failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := orchestrator.Config{}
			orch := orchestrator.New(cfg)

			// Start the orchestrator
			ctx := context.Background()
			err := orch.Start(ctx)
			if err != nil {
				t.Fatalf("Failed to start orchestrator: %v", err)
			}
			defer orch.Stop()

			registry := orch.GetServiceRegistry()

			// Register a test service
			testService := &orchestratorMockService{
				label:       tt.label,
				serviceType: services.TypePortForward,
				state:       services.StateRunning,
				health:      services.HealthHealthy,
				stopErr:     tt.stopError,
			}
			registry.Register(testService)

			api := NewOrchestratorAPI(orch, registry)

			err = api.StopService(tt.label)

			if (err != nil) != tt.expectError {
				t.Errorf("StopService() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestOrchestratorAPI_RestartService(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		restartErr  error
		expectError bool
	}{
		{
			name:        "successful restart",
			label:       "test-service",
			restartErr:  nil,
			expectError: false,
		},
		{
			name:        "service restart error",
			label:       "test-service",
			restartErr:  errors.New("restart failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := orchestrator.Config{}
			orch := orchestrator.New(cfg)

			// Start the orchestrator
			ctx := context.Background()
			err := orch.Start(ctx)
			if err != nil {
				t.Fatalf("Failed to start orchestrator: %v", err)
			}
			defer orch.Stop()

			registry := orch.GetServiceRegistry()

			// Register a test service
			testService := &orchestratorMockService{
				label:       tt.label,
				serviceType: services.TypePortForward,
				state:       services.StateRunning,
				health:      services.HealthHealthy,
				restartErr:  tt.restartErr,
			}
			registry.Register(testService)

			api := NewOrchestratorAPI(orch, registry)

			err = api.RestartService(tt.label)

			if (err != nil) != tt.expectError {
				t.Errorf("RestartService() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestOrchestratorAPI_GetServiceStatus(t *testing.T) {
	cfg := orchestrator.Config{}
	orch := orchestrator.New(cfg)

	// Start the orchestrator
	ctx := context.Background()
	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer orch.Stop()

	registry := orch.GetServiceRegistry()

	// Create API
	api := NewOrchestratorAPI(orch, registry)

	// Register a test service
	svc := &orchestratorMockService{
		label:        "test-service",
		serviceType:  services.TypePortForward,
		state:        services.StateRunning,
		health:       services.HealthHealthy,
		dependencies: []string{"dep1", "dep2"},
	}
	registry.Register(svc)

	// Test existing service
	status, err := api.GetServiceStatus("test-service")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check status fields
	if status.Label != "test-service" {
		t.Errorf("Expected label %s, got %s", "test-service", status.Label)
	}
	if status.ServiceType != "PortForward" {
		t.Errorf("Expected type %s, got %s", "PortForward", status.ServiceType)
	}
	if status.State != services.StateRunning {
		t.Errorf("Expected state %s, got %s", services.StateRunning, status.State)
	}
	if status.Health != services.HealthHealthy {
		t.Errorf("Expected health %s, got %s", services.HealthHealthy, status.Health)
	}

	// Test non-existing service
	_, err = api.GetServiceStatus("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}

func TestOrchestratorAPI_SubscribeToStateChanges(t *testing.T) {
	// Register a test service BEFORE starting the orchestrator
	svc := &orchestratorMockService{
		label:       "test-service",
		serviceType: services.TypePortForward,
		state:       services.StateStopped,
		health:      services.HealthUnknown,
	}

	cfg := orchestrator.Config{}
	orch := orchestrator.New(cfg)
	registry := orch.GetServiceRegistry()

	// Register the service before starting
	registry.Register(svc)

	// Start the orchestrator
	ctx := context.Background()
	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer orch.Stop()

	api := NewOrchestratorAPI(orch, registry)

	// Subscribe to state changes
	ch := api.SubscribeToStateChanges()

	if ch == nil {
		t.Error("Expected non-nil channel")
	}

	// Give the orchestrator time to process
	time.Sleep(100 * time.Millisecond)

	// Start the service to trigger a state change
	err = orch.StartService("test-service")
	if err != nil {
		t.Errorf("Failed to start service: %v", err)
	}

	// Try to receive events multiple times with shorter timeouts
	received := false
	for i := 0; i < 5; i++ {
		select {
		case event := <-ch:
			t.Logf("Received event: Label=%s, OldState=%s, NewState=%s", event.Label, event.OldState, event.NewState)
			if event.Label == "test-service" {
				received = true
				// The event might be starting -> running transition
				if event.NewState != string(services.StateStarting) && event.NewState != string(services.StateRunning) {
					t.Errorf("Expected new state to be Starting or Running, got %s", event.NewState)
				}
			}
		case <-time.After(200 * time.Millisecond):
			// Try again
			continue
		}
		if received {
			break
		}
	}

	if !received {
		t.Error("Expected to receive state change event")
	}
}

func TestServiceStateChangedEvent_Structure(t *testing.T) {
	// Test that the event structure has all expected fields
	event := orchestrator.ServiceStateChangedEvent{
		Label:       "test",
		ServiceType: "PortForward",
		OldState:    "stopped",
		NewState:    "running",
		Health:      "healthy",
		Error:       errors.New("test error"),
	}

	if event.Label != "test" {
		t.Errorf("Expected Label 'test', got %s", event.Label)
	}

	if event.ServiceType != "PortForward" {
		t.Errorf("Expected ServiceType 'PortForward', got %s", event.ServiceType)
	}

	if event.OldState != "stopped" {
		t.Errorf("Expected OldState 'stopped', got %s", event.OldState)
	}

	if event.NewState != "running" {
		t.Errorf("Expected NewState 'running', got %s", event.NewState)
	}

	if event.Health != "healthy" {
		t.Errorf("Expected Health 'healthy', got %s", event.Health)
	}

	if event.Error == nil || event.Error.Error() != "test error" {
		t.Errorf("Expected Error 'test error', got %v", event.Error)
	}
}

func TestOrchestratorAPI_GetAllServices(t *testing.T) {
	cfg := orchestrator.Config{}
	orch := orchestrator.New(cfg)

	// Start the orchestrator
	ctx := context.Background()
	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer orch.Stop()

	registry := orch.GetServiceRegistry()

	// Create API
	api := NewOrchestratorAPI(orch, registry)

	// Register test services
	svc1 := &orchestratorMockService{
		label:       "service1",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}
	svc2 := &orchestratorMockService{
		label:       "service2",
		serviceType: services.TypeMCPServer,
		state:       services.StateStopped,
		health:      services.HealthUnknown,
	}
	registry.Register(svc1)
	registry.Register(svc2)

	// Test listing services
	statuses := api.GetAllServices()
	if len(statuses) != 2 {
		t.Errorf("Expected 2 services, got %d", len(statuses))
	}

	// Verify both services are in the list
	labels := []string{}
	for _, s := range statuses {
		labels = append(labels, s.Label)
	}

	found1 := false
	found2 := false
	for _, label := range labels {
		if label == "service1" {
			found1 = true
		}
		if label == "service2" {
			found2 = true
		}
	}

	if !found1 {
		t.Error("service1 not found in list")
	}
	if !found2 {
		t.Error("service2 not found in list")
	}
}

func TestOrchestratorAPI_ClusterManagement(t *testing.T) {
	cfg := orchestrator.Config{
		Clusters: []config.ClusterDefinition{
			{
				Name:        "mc-test",
				Context:     "test-context",
				Role:        config.ClusterRoleObservability,
				DisplayName: "Test MC",
			},
			{
				Name:        "wc-test",
				Context:     "wc-context",
				Role:        config.ClusterRoleTarget,
				DisplayName: "Test WC",
			},
		},
		ActiveClusters: map[config.ClusterRole]string{
			config.ClusterRoleObservability: "mc-test",
			config.ClusterRoleTarget:        "wc-test",
		},
	}
	orch := orchestrator.New(cfg)

	// Start the orchestrator
	ctx := context.Background()
	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}
	defer orch.Stop()

	registry := orch.GetServiceRegistry()

	// Create API
	api := NewOrchestratorAPI(orch, registry)

	// Test GetAvailableClusters
	clusters := api.GetAvailableClusters(config.ClusterRoleObservability)
	if len(clusters) != 1 {
		t.Errorf("Expected 1 observability cluster, got %d", len(clusters))
	}
	if clusters[0].Name != "mc-test" {
		t.Errorf("Expected cluster name mc-test, got %s", clusters[0].Name)
	}

	// Test GetActiveCluster
	active, exists := api.GetActiveCluster(config.ClusterRoleTarget)
	if !exists {
		t.Error("Expected active target cluster to exist")
	}
	if active != "wc-test" {
		t.Errorf("Expected active cluster wc-test, got %s", active)
	}

	// Test SwitchCluster - this would require more setup in a real test
	// For now, just test that the method exists and can be called
	err = api.SwitchCluster(config.ClusterRoleTarget, "wc-test")
	if err != nil {
		t.Errorf("Unexpected error switching cluster: %v", err)
	}
}
