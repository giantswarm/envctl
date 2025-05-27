package api

import (
	"context"
	"envctl/internal/services"
	"errors"
	"testing"
	"time"
)

// mockService implements the Service interface for testing
type mockService struct {
	label        string
	serviceType  services.ServiceType
	state        services.ServiceState
	health       services.HealthStatus
	lastError    error
	dependencies []string
	serviceData  map[string]interface{}
}

func (m *mockService) Start(ctx context.Context) error { return nil }
func (m *mockService) Stop(ctx context.Context) error  { return nil }
func (m *mockService) Restart(ctx context.Context) error {
	return nil
}
func (m *mockService) GetState() services.ServiceState                              { return m.state }
func (m *mockService) GetHealth() services.HealthStatus                             { return m.health }
func (m *mockService) GetLastError() error                                          { return m.lastError }
func (m *mockService) GetLabel() string                                             { return m.label }
func (m *mockService) GetType() services.ServiceType                                { return m.serviceType }
func (m *mockService) GetDependencies() []string                                    { return m.dependencies }
func (m *mockService) SetStateChangeCallback(callback services.StateChangeCallback) {}
func (m *mockService) GetServiceData() map[string]interface{}                       { return m.serviceData }

// mockRegistry implements ServiceRegistry for testing
type mockRegistry struct {
	services map[string]services.Service
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		services: make(map[string]services.Service),
	}
}

func (r *mockRegistry) Register(service services.Service) error {
	r.services[service.GetLabel()] = service
	return nil
}

func (r *mockRegistry) Unregister(label string) error {
	delete(r.services, label)
	return nil
}

func (r *mockRegistry) Get(label string) (services.Service, bool) {
	service, exists := r.services[label]
	return service, exists
}

func (r *mockRegistry) GetAll() []services.Service {
	var result []services.Service
	for _, service := range r.services {
		result = append(result, service)
	}
	return result
}

func (r *mockRegistry) GetByType(serviceType services.ServiceType) []services.Service {
	var result []services.Service
	for _, service := range r.services {
		if service.GetType() == serviceType {
			result = append(result, service)
		}
	}
	return result
}

func TestNewK8sServiceAPI(t *testing.T) {
	registry := newMockRegistry()
	api := NewK8sServiceAPI(registry)

	if api == nil {
		t.Error("Expected NewK8sServiceAPI to return non-nil API")
	}
}

func TestGetConnectionInfo(t *testing.T) {
	registry := newMockRegistry()
	api := NewK8sServiceAPI(registry)

	// Test service not found
	_, err := api.GetConnectionInfo(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}

	// Create a mock K8s service
	testTime := time.Now()
	mockSvc := &mockService{
		label:       "test-k8s",
		serviceType: services.TypeKubeConnection,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			"context":    "test-context",
			"isMC":       true,
			"readyNodes": 3,
			"totalNodes": 3,
			"lastCheck":  testTime,
		},
	}

	registry.Register(mockSvc)

	// Test successful retrieval
	info, err := api.GetConnectionInfo(context.Background(), "test-k8s")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Label != "test-k8s" {
		t.Errorf("Expected label 'test-k8s', got %s", info.Label)
	}

	if info.State != "Running" {
		t.Errorf("Expected state 'Running', got %s", info.State)
	}

	if info.Health != "Healthy" {
		t.Errorf("Expected health 'Healthy', got %s", info.Health)
	}

	if info.Context != "test-context" {
		t.Errorf("Expected context 'test-context', got %s", info.Context)
	}

	if !info.IsMC {
		t.Error("Expected IsMC to be true")
	}

	if info.ReadyNodes != 3 {
		t.Errorf("Expected ReadyNodes 3, got %d", info.ReadyNodes)
	}

	if info.TotalNodes != 3 {
		t.Errorf("Expected TotalNodes 3, got %d", info.TotalNodes)
	}

	if !info.LastCheck.Equal(testTime) {
		t.Errorf("Expected LastCheck %v, got %v", testTime, info.LastCheck)
	}
}

func TestGetConnectionInfoWithError(t *testing.T) {
	registry := newMockRegistry()
	api := NewK8sServiceAPI(registry)

	// Create a mock service with error
	testErr := errors.New("test service error")
	mockSvc := &mockService{
		label:       "error-k8s",
		serviceType: services.TypeKubeConnection,
		state:       services.StateFailed,
		health:      services.HealthUnhealthy,
		lastError:   testErr,
	}

	registry.Register(mockSvc)

	info, err := api.GetConnectionInfo(context.Background(), "error-k8s")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Error != testErr.Error() {
		t.Errorf("Expected error message '%s', got '%s'", testErr.Error(), info.Error)
	}
}

func TestGetConnectionInfoWrongType(t *testing.T) {
	registry := newMockRegistry()
	api := NewK8sServiceAPI(registry)

	// Create a mock service of wrong type
	mockSvc := &mockService{
		label:       "wrong-type",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}

	registry.Register(mockSvc)

	_, err := api.GetConnectionInfo(context.Background(), "wrong-type")
	if err == nil {
		t.Error("Expected error for wrong service type")
	}

	if err.Error() != "service wrong-type is not a K8s connection" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestListConnections(t *testing.T) {
	registry := newMockRegistry()
	api := NewK8sServiceAPI(registry)

	// Create multiple mock K8s services
	mockSvc1 := &mockService{
		label:       "k8s-1",
		serviceType: services.TypeKubeConnection,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			"context": "context-1",
			"isMC":    true,
		},
	}

	mockSvc2 := &mockService{
		label:       "k8s-2",
		serviceType: services.TypeKubeConnection,
		state:       services.StateStarting,
		health:      services.HealthChecking,
		serviceData: map[string]interface{}{
			"context": "context-2",
			"isMC":    false,
		},
	}

	// Add a non-K8s service (should be filtered out)
	mockSvc3 := &mockService{
		label:       "port-forward",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}

	registry.Register(mockSvc1)
	registry.Register(mockSvc2)
	registry.Register(mockSvc3)

	connections, err := api.ListConnections(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(connections))
	}

	// Check that we got the right services
	labels := make(map[string]bool)
	for _, conn := range connections {
		labels[conn.Label] = true
	}

	if !labels["k8s-1"] {
		t.Error("Expected k8s-1 in connections")
	}

	if !labels["k8s-2"] {
		t.Error("Expected k8s-2 in connections")
	}

	if labels["port-forward"] {
		t.Error("Did not expect port-forward in connections")
	}
}

func TestGetConnectionByContext(t *testing.T) {
	registry := newMockRegistry()
	api := NewK8sServiceAPI(registry)

	// Create mock K8s services with different contexts
	mockSvc1 := &mockService{
		label:       "k8s-1",
		serviceType: services.TypeKubeConnection,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			"context": "target-context",
			"isMC":    true,
		},
	}

	mockSvc2 := &mockService{
		label:       "k8s-2",
		serviceType: services.TypeKubeConnection,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			"context": "other-context",
			"isMC":    false,
		},
	}

	registry.Register(mockSvc1)
	registry.Register(mockSvc2)

	// Test finding by context
	info, err := api.GetConnectionByContext(context.Background(), "target-context")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Label != "k8s-1" {
		t.Errorf("Expected label 'k8s-1', got %s", info.Label)
	}

	if info.Context != "target-context" {
		t.Errorf("Expected context 'target-context', got %s", info.Context)
	}

	// Test context not found
	_, err = api.GetConnectionByContext(context.Background(), "nonexistent-context")
	if err == nil {
		t.Error("Expected error for nonexistent context")
	}

	expectedErr := "no K8s connection found for context nonexistent-context"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}
