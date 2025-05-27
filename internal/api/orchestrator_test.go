package api

import (
	"context"
	"envctl/internal/services"
	"errors"
	"testing"
	"time"
)

// mockServiceOrchestrator implements ServiceOrchestrator for testing
type mockServiceOrchestrator struct {
	startServiceFunc   func(label string) error
	stopServiceFunc    func(label string) error
	restartServiceFunc func(label string) error
}

func (m *mockServiceOrchestrator) StartService(label string) error {
	if m.startServiceFunc != nil {
		return m.startServiceFunc(label)
	}
	return nil
}

func (m *mockServiceOrchestrator) StopService(label string) error {
	if m.stopServiceFunc != nil {
		return m.stopServiceFunc(label)
	}
	return nil
}

func (m *mockServiceOrchestrator) RestartService(label string) error {
	if m.restartServiceFunc != nil {
		return m.restartServiceFunc(label)
	}
	return nil
}

// mockOrchestratorRegistry implements services.ServiceRegistry for testing
type mockOrchestratorRegistry struct {
	services map[string]services.Service
}

func newMockOrchestratorRegistry() *mockOrchestratorRegistry {
	return &mockOrchestratorRegistry{
		services: make(map[string]services.Service),
	}
}

func (m *mockOrchestratorRegistry) Register(service services.Service) error {
	m.services[service.GetLabel()] = service
	return nil
}

func (m *mockOrchestratorRegistry) Unregister(label string) error {
	delete(m.services, label)
	return nil
}

func (m *mockOrchestratorRegistry) Get(label string) (services.Service, bool) {
	service, exists := m.services[label]
	return service, exists
}

func (m *mockOrchestratorRegistry) GetAll() []services.Service {
	var list []services.Service
	for _, service := range m.services {
		list = append(list, service)
	}
	return list
}

func (m *mockOrchestratorRegistry) GetByType(serviceType services.ServiceType) []services.Service {
	var list []services.Service
	for _, service := range m.services {
		if service.GetType() == serviceType {
			list = append(list, service)
		}
	}
	return list
}

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

func TestNewOrchestratorAPI(t *testing.T) {
	orch := &mockServiceOrchestrator{}
	registry := newMockOrchestratorRegistry()

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
		orchError   error
		expectError bool
	}{
		{
			name:        "successful start",
			label:       "test-service",
			orchError:   nil,
			expectError: false,
		},
		{
			name:        "orchestrator error",
			label:       "test-service",
			orchError:   errors.New("start failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := &mockServiceOrchestrator{
				startServiceFunc: func(label string) error {
					if label != tt.label {
						t.Errorf("Expected label %s, got %s", tt.label, label)
					}
					return tt.orchError
				},
			}
			registry := newMockOrchestratorRegistry()
			// Register a test service so it exists in the registry
			testService := &mockService{
				label:       tt.label,
				serviceType: services.TypePortForward,
				state:       services.StateStopped,
				health:      services.HealthUnknown,
			}
			registry.Register(testService)

			api := NewOrchestratorAPI(orch, registry)

			err := api.StartService(context.Background(), tt.label)

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
		orchError   error
		expectError bool
	}{
		{
			name:        "successful stop",
			label:       "test-service",
			orchError:   nil,
			expectError: false,
		},
		{
			name:        "orchestrator error",
			label:       "test-service",
			orchError:   errors.New("stop failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := &mockServiceOrchestrator{
				stopServiceFunc: func(label string) error {
					if label != tt.label {
						t.Errorf("Expected label %s, got %s", tt.label, label)
					}
					return tt.orchError
				},
			}
			registry := newMockOrchestratorRegistry()
			// Register a test service so it exists in the registry
			testService := &mockService{
				label:       tt.label,
				serviceType: services.TypePortForward,
				state:       services.StateRunning,
				health:      services.HealthHealthy,
			}
			registry.Register(testService)

			api := NewOrchestratorAPI(orch, registry)

			err := api.StopService(context.Background(), tt.label)

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
		orchError   error
		expectError bool
	}{
		{
			name:        "successful restart",
			label:       "test-service",
			orchError:   nil,
			expectError: false,
		},
		{
			name:        "orchestrator error",
			label:       "test-service",
			orchError:   errors.New("restart failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := &mockServiceOrchestrator{
				restartServiceFunc: func(label string) error {
					if label != tt.label {
						t.Errorf("Expected label %s, got %s", tt.label, label)
					}
					return tt.orchError
				},
			}
			registry := newMockOrchestratorRegistry()
			// Register a test service so it exists in the registry
			testService := &mockService{
				label:       tt.label,
				serviceType: services.TypePortForward,
				state:       services.StateRunning,
				health:      services.HealthHealthy,
			}
			registry.Register(testService)

			api := NewOrchestratorAPI(orch, registry)

			err := api.RestartService(context.Background(), tt.label)

			if (err != nil) != tt.expectError {
				t.Errorf("RestartService() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestOrchestratorAPI_GetServiceStatus(t *testing.T) {
	orch := &mockServiceOrchestrator{}
	registry := newMockOrchestratorRegistry()

	// Add a test service using the existing mockService type
	testService := &mockService{
		label:        "test-service",
		serviceType:  services.TypePortForward,
		state:        services.StateRunning,
		health:       services.HealthHealthy,
		lastError:    nil,
		dependencies: []string{"dep1", "dep2"},
	}
	registry.Register(testService)

	api := NewOrchestratorAPI(orch, registry)

	// Test existing service
	status, err := api.GetServiceStatus(context.Background(), "test-service")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	if status.Label != "test-service" {
		t.Errorf("Expected label 'test-service', got %s", status.Label)
	}

	if status.Type != string(services.TypePortForward) {
		t.Errorf("Expected type %s, got %s", services.TypePortForward, status.Type)
	}

	if status.State != string(services.StateRunning) {
		t.Errorf("Expected state %s, got %s", services.StateRunning, status.State)
	}

	if status.Health != string(services.HealthHealthy) {
		t.Errorf("Expected health %s, got %s", services.HealthHealthy, status.Health)
	}

	if len(status.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(status.Dependencies))
	}

	// Test non-existing service
	_, err = api.GetServiceStatus(context.Background(), "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}

func TestOrchestratorAPI_ListServices(t *testing.T) {
	orch := &mockServiceOrchestrator{}
	registry := newMockOrchestratorRegistry()

	// Add test services
	services := []*mockService{
		{
			label:       "service1",
			serviceType: services.TypePortForward,
			state:       services.StateRunning,
			health:      services.HealthHealthy,
		},
		{
			label:       "service2",
			serviceType: services.TypeMCPServer,
			state:       services.StateStopped,
			health:      services.HealthUnknown,
		},
	}

	for _, svc := range services {
		registry.Register(svc)
	}

	api := NewOrchestratorAPI(orch, registry)

	list, err := api.ListServices(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("Expected 2 services, got %d", len(list))
	}

	// Check that all services are in the list
	foundServices := make(map[string]bool)
	for _, status := range list {
		foundServices[status.Label] = true
	}

	if !foundServices["service1"] {
		t.Error("Expected to find service1 in list")
	}

	if !foundServices["service2"] {
		t.Error("Expected to find service2 in list")
	}
}

func TestOrchestratorAPI_SubscribeToStateChanges(t *testing.T) {
	orch := &mockServiceOrchestrator{}
	registry := newMockOrchestratorRegistry()
	api := NewOrchestratorAPI(orch, registry).(*orchestratorAPI)

	// Subscribe to state changes
	ch := api.SubscribeToStateChanges()

	if ch == nil {
		t.Error("Expected non-nil channel")
	}

	// Test forwarding a state change
	testEvent := ServiceStateChangedEvent{
		Label:    "test-service",
		OldState: string(services.StateStopped),
		NewState: string(services.StateRunning),
		Health:   string(services.HealthHealthy),
		Error:    nil,
	}

	// Forward the event
	api.forwardStateChange(
		testEvent.Label,
		services.ServiceState(testEvent.OldState),
		services.ServiceState(testEvent.NewState),
		services.HealthStatus(testEvent.Health),
		testEvent.Error,
	)

	// Check if event was received
	select {
	case event := <-ch:
		if event.Label != testEvent.Label {
			t.Errorf("Expected label %s, got %s", testEvent.Label, event.Label)
		}
		if event.OldState != testEvent.OldState {
			t.Errorf("Expected old state %s, got %s", testEvent.OldState, event.OldState)
		}
		if event.NewState != testEvent.NewState {
			t.Errorf("Expected new state %s, got %s", testEvent.NewState, event.NewState)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive state change event")
	}
}

func TestServiceStateChangedEvent_Structure(t *testing.T) {
	// Test that the event structure has all expected fields
	event := ServiceStateChangedEvent{
		Label:    "test",
		OldState: "stopped",
		NewState: "running",
		Health:   "healthy",
		Error:    errors.New("test error"),
	}

	if event.Label != "test" {
		t.Errorf("Expected Label 'test', got %s", event.Label)
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
