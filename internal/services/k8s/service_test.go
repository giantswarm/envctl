package k8s

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/services"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockKubeManager implements the kube.Manager interface for testing
type mockKubeManager struct {
	nodeHealth kube.NodeHealth
	healthErr  error
}

func (m *mockKubeManager) Login(clusterName string) (string, string, error) {
	return "", "", nil
}

func (m *mockKubeManager) ListClusters() (*kube.ClusterInfo, error) {
	return nil, nil
}

func (m *mockKubeManager) GetCurrentContext() (string, error) {
	return "", nil
}

func (m *mockKubeManager) SwitchContext(targetContextName string) error {
	return nil
}

func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	return nil, nil
}

func (m *mockKubeManager) BuildMcContextName(mcName string) string {
	return "teleport.giantswarm.io-" + mcName
}

func (m *mockKubeManager) BuildWcContextName(mcName, wcName string) string {
	return "teleport.giantswarm.io-" + mcName + "-" + wcName
}

func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	return contextName
}

func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	return false
}

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (kube.NodeHealth, error) {
	return m.nodeHealth, m.healthErr
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "", nil
}

func TestNewK8sConnectionService(t *testing.T) {
	kubeMgr := &mockKubeManager{}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	if service == nil {
		t.Error("Expected NewK8sConnectionService to return non-nil service")
	}

	if service.GetLabel() != "test-k8s" {
		t.Errorf("Expected label 'test-k8s', got %s", service.GetLabel())
	}

	if service.GetType() != services.TypeKubeConnection {
		t.Errorf("Expected type %s, got %s", services.TypeKubeConnection, service.GetType())
	}

	if service.contextName != "test-context" {
		t.Errorf("Expected context 'test-context', got %s", service.contextName)
	}

	if !service.isMC {
		t.Error("Expected isMC to be true")
	}

	if service.readyNodes != -1 {
		t.Errorf("Expected readyNodes to be -1, got %d", service.readyNodes)
	}

	if service.totalNodes != -1 {
		t.Errorf("Expected totalNodes to be -1, got %d", service.totalNodes)
	}
}

func TestK8sConnectionService_GetServiceData(t *testing.T) {
	kubeMgr := &mockKubeManager{}
	service := NewK8sConnectionService("test-k8s", "test-context", false, kubeMgr)

	// Set some test data
	service.mu.Lock()
	service.readyNodes = 3
	service.totalNodes = 5
	service.lastHealthCheck = time.Now()
	service.healthError = errors.New("test error")
	service.mu.Unlock()

	data := service.GetServiceData()

	if data["label"] != "test-k8s" {
		t.Errorf("Expected label 'test-k8s', got %v", data["label"])
	}

	if data["context"] != "test-context" {
		t.Errorf("Expected context 'test-context', got %v", data["context"])
	}

	if data["isMC"] != false {
		t.Errorf("Expected isMC false, got %v", data["isMC"])
	}

	if data["readyNodes"] != 3 {
		t.Errorf("Expected readyNodes 3, got %v", data["readyNodes"])
	}

	if data["totalNodes"] != 5 {
		t.Errorf("Expected totalNodes 5, got %v", data["totalNodes"])
	}

	if data["healthError"] != "test error" {
		t.Errorf("Expected healthError 'test error', got %v", data["healthError"])
	}
}

func TestK8sConnectionService_CheckHealth_Healthy(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
			Error:      nil,
		},
		healthErr: nil,
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if health != services.HealthHealthy {
		t.Errorf("Expected health %s, got %s", services.HealthHealthy, health)
	}

	// Check that internal state was updated
	service.mu.RLock()
	if service.readyNodes != 3 {
		t.Errorf("Expected readyNodes 3, got %d", service.readyNodes)
	}
	if service.totalNodes != 3 {
		t.Errorf("Expected totalNodes 3, got %d", service.totalNodes)
	}
	if service.healthError != nil {
		t.Errorf("Expected no health error, got %v", service.healthError)
	}
	service.mu.RUnlock()
}

func TestK8sConnectionService_CheckHealth_Degraded(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 2,
			TotalNodes: 3,
			Error:      nil,
		},
		healthErr: nil,
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	if err == nil {
		t.Error("Expected error for degraded cluster")
	}

	if health != services.HealthUnhealthy {
		t.Errorf("Expected health %s, got %s", services.HealthUnhealthy, health)
	}

	if !strings.Contains(err.Error(), "cluster degraded") {
		t.Errorf("Expected 'cluster degraded' in error, got: %s", err.Error())
	}
}

func TestK8sConnectionService_CheckHealth_NoNodes(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 0,
			TotalNodes: 3,
			Error:      nil,
		},
		healthErr: nil,
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	if err == nil {
		t.Error("Expected error for no ready nodes")
	}

	if health != services.HealthUnhealthy {
		t.Errorf("Expected health %s, got %s", services.HealthUnhealthy, health)
	}

	if !strings.Contains(err.Error(), "no nodes ready") {
		t.Errorf("Expected 'no nodes ready' in error, got: %s", err.Error())
	}
}

func TestK8sConnectionService_CheckHealth_Error(t *testing.T) {
	testErr := errors.New("cluster unreachable")
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{},
		healthErr:  testErr,
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}

	if health != services.HealthUnhealthy {
		t.Errorf("Expected health %s, got %s", services.HealthUnhealthy, health)
	}

	// Check that internal state was updated
	service.mu.RLock()
	if service.readyNodes != -1 {
		t.Errorf("Expected readyNodes -1, got %d", service.readyNodes)
	}
	if service.totalNodes != -1 {
		t.Errorf("Expected totalNodes -1, got %d", service.totalNodes)
	}
	if service.healthError != testErr {
		t.Errorf("Expected health error %v, got %v", testErr, service.healthError)
	}
	service.mu.RUnlock()
}

func TestK8sConnectionService_GetHealthCheckInterval(t *testing.T) {
	kubeMgr := &mockKubeManager{}
	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	interval := service.GetHealthCheckInterval()
	expected := 15 * time.Second

	if interval != expected {
		t.Errorf("Expected interval %v, got %v", expected, interval)
	}
}

func TestK8sConnectionService_Stop(t *testing.T) {
	kubeMgr := &mockKubeManager{}
	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	// Start the service first
	ctx := context.Background()
	service.Start(ctx)

	// Verify cancel function is set
	service.mu.RLock()
	hasCancelFunc := service.cancelFunc != nil
	service.mu.RUnlock()

	if !hasCancelFunc {
		t.Error("Expected cancel function to be set after Start")
	}

	// Stop the service
	err := service.Stop(ctx)
	if err != nil {
		t.Errorf("Unexpected error stopping service: %v", err)
	}

	// Verify cancel function is cleared
	service.mu.RLock()
	hasCancelFunc = service.cancelFunc != nil
	service.mu.RUnlock()

	if hasCancelFunc {
		t.Error("Expected cancel function to be cleared after Stop")
	}

	if service.GetState() != services.StateStopped {
		t.Errorf("Expected state %s, got %s", services.StateStopped, service.GetState())
	}
}

func TestK8sConnectionService_Restart(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
			Error:      nil,
		},
		healthErr: nil,
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	ctx := context.Background()

	// Start the service first
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Unexpected error starting service: %v", err)
	}

	// Restart the service
	err = service.Restart(ctx)
	if err != nil {
		t.Errorf("Unexpected error restarting service: %v", err)
	}

	if service.GetState() != services.StateRunning {
		t.Errorf("Expected state %s after restart, got %s", services.StateRunning, service.GetState())
	}
}

func TestK8sConnectionService_Interfaces(t *testing.T) {
	kubeMgr := &mockKubeManager{}
	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	// Test that service implements required interfaces
	var _ services.Service = service
	var _ services.ServiceDataProvider = service
}
