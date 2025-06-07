package api

import (
	"context"
	"errors"
	"testing"
)

func TestNewPortForwardServiceAPI(t *testing.T) {
	api := NewPortForwardServiceAPI()

	if api == nil {
		t.Error("Expected NewPortForwardServiceAPI to return non-nil API")
	}
}

func TestGetPortForwardInfo(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewPortForwardServiceAPI()

	// Test service not found
	_, err := api.GetForwardInfo(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}

	// Create a mock port forward service
	mockSvc := &mockServiceInfo{
		label:   "test-pf",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"name":        "test-pf",
			"namespace":   "default",
			"targetType":  "service",
			"targetName":  "test-service",
			"localPort":   8080,
			"remotePort":  80,
			"bindAddress": "127.0.0.1",
			"enabled":     true,
			"icon":        "🌐",
			"category":    "web",
			"context":     "test-context",
			"targetPod":   "test-pod-123",
		},
	}

	registry.addService(mockSvc)

	// Test successful retrieval
	info, err := api.GetForwardInfo(context.Background(), "test-pf")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Label != "test-pf" {
		t.Errorf("Expected label 'test-pf', got %s", info.Label)
	}

	if info.State != "running" {
		t.Errorf("Expected state 'running', got %s", info.State)
	}

	if info.Health != "healthy" {
		t.Errorf("Expected health 'healthy', got %s", info.Health)
	}

	if info.Name != "test-pf" {
		t.Errorf("Expected name 'test-pf', got %s", info.Name)
	}

	if info.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got %s", info.Namespace)
	}

	if info.TargetType != "service" {
		t.Errorf("Expected targetType 'service', got %s", info.TargetType)
	}

	if info.TargetName != "test-service" {
		t.Errorf("Expected targetName 'test-service', got %s", info.TargetName)
	}

	if info.LocalPort != 8080 {
		t.Errorf("Expected localPort 8080, got %d", info.LocalPort)
	}

	if info.RemotePort != 80 {
		t.Errorf("Expected remotePort 80, got %d", info.RemotePort)
	}

	if info.BindAddress != "127.0.0.1" {
		t.Errorf("Expected bindAddress '127.0.0.1', got %s", info.BindAddress)
	}

	if !info.Enabled {
		t.Error("Expected enabled to be true")
	}

	if info.Icon != "🌐" {
		t.Errorf("Expected icon '🌐', got %s", info.Icon)
	}

	if info.Category != "web" {
		t.Errorf("Expected category 'web', got %s", info.Category)
	}

	if info.Context != "test-context" {
		t.Errorf("Expected context 'test-context', got %s", info.Context)
	}

	if info.TargetPod != "test-pod-123" {
		t.Errorf("Expected targetPod 'test-pod-123', got %s", info.TargetPod)
	}
}

func TestGetPortForwardInfoWithError(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewPortForwardServiceAPI()

	// Create a mock service with error
	testErr := errors.New("port forward failed")
	mockSvc := &mockServiceInfo{
		label:   "error-pf",
		svcType: TypePortForward,
		state:   StateError,
		health:  HealthUnhealthy,
		lastErr: testErr,
		data: map[string]interface{}{
			"name": "error-pf",
		},
	}

	registry.addService(mockSvc)

	info, err := api.GetForwardInfo(context.Background(), "error-pf")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Error != testErr.Error() {
		t.Errorf("Expected error message '%s', got '%s'", testErr.Error(), info.Error)
	}
}

func TestGetPortForwardInfoWrongType(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewPortForwardServiceAPI()

	// Create a mock service of wrong type
	mockSvc := &mockServiceInfo{
		label:   "wrong-type",
		svcType: TypeKubeConnection,
		state:   StateRunning,
		health:  HealthHealthy,
	}

	registry.addService(mockSvc)

	_, err := api.GetForwardInfo(context.Background(), "wrong-type")
	if err == nil {
		t.Error("Expected error for wrong service type")
	}

	if err.Error() != "service wrong-type is not a port forward" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestListPortForwards(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewPortForwardServiceAPI()

	// Create multiple mock port forward services
	mockSvc1 := &mockServiceInfo{
		label:   "pf-1",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"name":       "pf-1",
			"namespace":  "default",
			"localPort":  8080,
			"remotePort": 80,
		},
	}

	mockSvc2 := &mockServiceInfo{
		label:   "pf-2",
		svcType: TypePortForward,
		state:   StateStarting,
		health:  HealthUnknown,
		data: map[string]interface{}{
			"name":       "pf-2",
			"namespace":  "kube-system",
			"localPort":  9090,
			"remotePort": 9090,
		},
	}

	// Add a non-port-forward service (should be filtered out)
	mockSvc3 := &mockServiceInfo{
		label:   "k8s-conn",
		svcType: TypeKubeConnection,
		state:   StateRunning,
		health:  HealthHealthy,
	}

	registry.addService(mockSvc1)
	registry.addService(mockSvc2)
	registry.addService(mockSvc3)

	portForwards, err := api.ListForwards(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(portForwards) != 2 {
		t.Errorf("Expected 2 port forwards, got %d", len(portForwards))
	}

	// Check that we got the right services
	labels := make(map[string]bool)
	for _, pf := range portForwards {
		labels[pf.Label] = true
	}

	if !labels["pf-1"] {
		t.Error("Expected pf-1 in port forwards")
	}

	if !labels["pf-2"] {
		t.Error("Expected pf-2 in port forwards")
	}

	if labels["k8s-conn"] {
		t.Error("Did not expect k8s-conn in port forwards")
	}
}

func TestPortForwardInfo_Defaults(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewPortForwardServiceAPI()

	// Create a mock service with minimal data
	mockSvc := &mockServiceInfo{
		label:   "minimal-pf",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			// Only required fields
			"name": "minimal-pf",
		},
	}

	registry.addService(mockSvc)

	info, err := api.GetForwardInfo(context.Background(), "minimal-pf")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check default values
	if info.Namespace != "" {
		t.Errorf("Expected empty namespace, got %s", info.Namespace)
	}

	if info.TargetType != "" {
		t.Errorf("Expected empty targetType, got %s", info.TargetType)
	}

	if info.LocalPort != 0 {
		t.Errorf("Expected localPort 0, got %d", info.LocalPort)
	}

	if info.RemotePort != 0 {
		t.Errorf("Expected remotePort 0, got %d", info.RemotePort)
	}

	if info.Enabled {
		t.Error("Expected enabled to be false by default")
	}
}
