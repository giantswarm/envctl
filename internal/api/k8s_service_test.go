package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewK8sServiceAPI(t *testing.T) {
	api := NewK8sServiceAPI()

	if api == nil {
		t.Error("Expected NewK8sServiceAPI to return non-nil API")
	}
}

func TestGetConnectionInfo(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()

	// Test service not found
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewK8sServiceAPI()

	_, err := api.GetConnectionInfo(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}

	// Create a mock K8s service
	testTime := time.Now()
	mockSvc := &mockServiceInfo{
		label:   "test-k8s",
		svcType: TypeKubeConnection,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"context":    "test-context",
			"isMC":       true,
			"readyNodes": 3,
			"totalNodes": 3,
			"lastCheck":  testTime,
		},
	}

	registry.addService(mockSvc)

	// Test successful retrieval
	info, err := api.GetConnectionInfo(context.Background(), "test-k8s")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Label != "test-k8s" {
		t.Errorf("Expected label 'test-k8s', got %s", info.Label)
	}

	if info.State != "running" {
		t.Errorf("Expected state 'running', got %s", info.State)
	}

	if info.Health != "healthy" {
		t.Errorf("Expected health 'healthy', got %s", info.Health)
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
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewK8sServiceAPI()

	// Create a mock service with error
	testErr := errors.New("test service error")
	mockSvc := &mockServiceInfo{
		label:   "error-k8s",
		svcType: TypeKubeConnection,
		state:   StateError,
		health:  HealthUnhealthy,
		lastErr: testErr,
	}

	registry.addService(mockSvc)

	info, err := api.GetConnectionInfo(context.Background(), "error-k8s")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Error != testErr.Error() {
		t.Errorf("Expected error message '%s', got '%s'", testErr.Error(), info.Error)
	}
}

func TestGetConnectionInfoWrongType(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewK8sServiceAPI()

	// Create a mock service of wrong type
	mockSvc := &mockServiceInfo{
		label:   "wrong-type",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
	}

	registry.addService(mockSvc)

	_, err := api.GetConnectionInfo(context.Background(), "wrong-type")
	if err == nil {
		t.Error("Expected error for wrong service type")
	}

	if err.Error() != "service wrong-type is not a K8s connection" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestListConnections(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewK8sServiceAPI()

	// Create multiple mock K8s services
	mockSvc1 := &mockServiceInfo{
		label:   "k8s-1",
		svcType: TypeKubeConnection,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"context": "context-1",
			"isMC":    true,
		},
	}

	mockSvc2 := &mockServiceInfo{
		label:   "k8s-2",
		svcType: TypeKubeConnection,
		state:   StateStarting,
		health:  HealthUnknown,
		data: map[string]interface{}{
			"context": "context-2",
			"isMC":    false,
		},
	}

	// Add a non-K8s service (should be filtered out)
	mockSvc3 := &mockServiceInfo{
		label:   "port-forward",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
	}

	registry.addService(mockSvc1)
	registry.addService(mockSvc2)
	registry.addService(mockSvc3)

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
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewK8sServiceAPI()

	// Create mock K8s services with different contexts
	mockSvc1 := &mockServiceInfo{
		label:   "k8s-1",
		svcType: TypeKubeConnection,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"context": "target-context",
			"isMC":    true,
		},
	}

	mockSvc2 := &mockServiceInfo{
		label:   "k8s-2",
		svcType: TypeKubeConnection,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"context": "other-context",
			"isMC":    false,
		},
	}

	registry.addService(mockSvc1)
	registry.addService(mockSvc2)

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
