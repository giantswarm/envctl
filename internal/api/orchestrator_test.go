package api

import (
	"context"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockServiceOrchestrator implements ServiceOrchestrator for testing
type mockServiceOrchestrator struct {
	startServiceFunc   func(label string) error
	stopServiceFunc    func(label string) error
	restartServiceFunc func(label string) error
	subscribers        []chan orchestrator.ServiceStateChangedEvent
	mu                 sync.Mutex
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

func (m *mockServiceOrchestrator) SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent {
	// Create a new channel for each subscriber (like the real orchestrator)
	ch := make(chan orchestrator.ServiceStateChangedEvent, 100)

	m.mu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.mu.Unlock()

	return ch
}

// sendEvent is a helper method for tests to send events
func (m *mockServiceOrchestrator) sendEvent(event orchestrator.ServiceStateChangedEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Send to all subscribers
	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, ignore
		}
	}
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

			err := api.StartService(tt.label)

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

			err := api.StopService(tt.label)

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

			err := api.RestartService(tt.label)

			if (err != nil) != tt.expectError {
				t.Errorf("RestartService() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestOrchestratorAPI_GetServiceStatus(t *testing.T) {
	// Create mocks
	orch := &mockServiceOrchestrator{}
	registry := services.NewRegistry()

	// Create API
	api := NewOrchestratorAPI(orch, registry)

	// Register a test service
	svc := &mockService{
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
	if status.Type != "PortForward" {
		t.Errorf("Expected type %s, got %s", "PortForward", status.Type)
	}
	if status.State != "Running" {
		t.Errorf("Expected state %s, got %s", "Running", status.State)
	}
	if status.Health != "Healthy" {
		t.Errorf("Expected health %s, got %s", "Healthy", status.Health)
	}

	// Test non-existing service
	_, err = api.GetServiceStatus("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent service")
	}
}

func TestOrchestratorAPI_SubscribeToStateChanges(t *testing.T) {
	// Create mocks
	orch := &mockServiceOrchestrator{}
	registry := services.NewRegistry()

	api := NewOrchestratorAPI(orch, registry).(*orchestratorAPI)

	// Subscribe to state changes
	ch := api.SubscribeToStateChanges()

	if ch == nil {
		t.Error("Expected non-nil channel")
	}

	// Test forwarding a state change by sending an event through the orchestrator
	testEvent := orchestrator.ServiceStateChangedEvent{
		Label:       "test-service",
		ServiceType: string(services.TypePortForward),
		OldState:    string(services.StateStopped),
		NewState:    string(services.StateRunning),
		Health:      string(services.HealthHealthy),
		Error:       nil,
	}

	// Send the event through the mock orchestrator
	orch.sendEvent(testEvent)

	// Give the goroutine time to process the event
	time.Sleep(10 * time.Millisecond)

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
	// Create mocks
	orch := &mockServiceOrchestrator{}
	registry := services.NewRegistry()

	// Create API
	api := NewOrchestratorAPI(orch, registry)

	// Register test services
	svc1 := &mockService{
		label:       "service1",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}
	svc2 := &mockService{
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

func TestOrchestratorAPI_MultipleSubscribers(t *testing.T) {
	// Create mocks
	orch := &mockServiceOrchestrator{}
	registry := services.NewRegistry()

	api := NewOrchestratorAPI(orch, registry)

	// Create multiple subscribers
	ch1 := api.SubscribeToStateChanges()
	ch2 := api.SubscribeToStateChanges()
	ch3 := api.SubscribeToStateChanges()

	// All channels should be different
	if ch1 == nil || ch2 == nil || ch3 == nil {
		t.Error("Expected non-nil channels")
	}

	// Test event
	testEvent := orchestrator.ServiceStateChangedEvent{
		Label:       "test-service",
		ServiceType: string(services.TypeMCPServer),
		OldState:    string(services.StateStopped),
		NewState:    string(services.StateRunning),
		Health:      string(services.HealthHealthy),
		Error:       nil,
	}

	// Send the event
	orch.sendEvent(testEvent)

	// Give goroutines time to process
	time.Sleep(50 * time.Millisecond)

	// All subscribers should receive the event
	received := 0

	select {
	case event := <-ch1:
		if event.Label == testEvent.Label {
			received++
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Subscriber 1 did not receive event")
	}

	select {
	case event := <-ch2:
		if event.Label == testEvent.Label {
			received++
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Subscriber 2 did not receive event")
	}

	select {
	case event := <-ch3:
		if event.Label == testEvent.Label {
			received++
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Subscriber 3 did not receive event")
	}

	if received != 3 {
		t.Errorf("Expected 3 subscribers to receive event, got %d", received)
	}
}
