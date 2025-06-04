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
	nodeHealth    kube.NodeHealth
	nodeHealthErr error
	apiHealthErr  error
	loginErr      error
}

func (m *mockKubeManager) Login(clusterName string) (string, string, error) {
	if m.loginErr != nil {
		return "", "login error", m.loginErr
	}
	return "Logged in successfully", "", nil
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
	prefix := "teleport.giantswarm.io-"
	if strings.HasPrefix(contextName, prefix) {
		return strings.TrimPrefix(contextName, prefix)
	}
	return contextName
}

func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	return strings.HasPrefix(contextName, "teleport.giantswarm.io-")
}

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (kube.NodeHealth, error) {
	return m.nodeHealth, m.nodeHealthErr
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "", nil
}

func (m *mockKubeManager) CheckAPIHealth(ctx context.Context, kubeContextName string) error {
	return m.apiHealthErr
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

func TestK8sConnectionService_CheckHealth_APIHealthy_AllNodesReady(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
			Error:      nil,
		},
		nodeHealthErr: nil,
		apiHealthErr:  nil, // API is healthy
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

func TestK8sConnectionService_CheckHealth_APIHealthy_DegradedNodes(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 2,
			TotalNodes: 3,
			Error:      nil,
		},
		nodeHealthErr: nil,
		apiHealthErr:  nil, // API is healthy
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	// With the new behavior, API healthy means service is healthy
	if err != nil {
		t.Error("Expected no error for degraded nodes when API is healthy")
	}

	if health != services.HealthHealthy {
		t.Errorf("Expected health %s, got %s", services.HealthHealthy, health)
	}

	// Check that node counts are still tracked
	service.mu.RLock()
	if service.readyNodes != 2 {
		t.Errorf("Expected readyNodes 2, got %d", service.readyNodes)
	}
	if service.totalNodes != 3 {
		t.Errorf("Expected totalNodes 3, got %d", service.totalNodes)
	}
	service.mu.RUnlock()
}

func TestK8sConnectionService_CheckHealth_APIHealthy_NoNodesReady(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 0,
			TotalNodes: 3,
			Error:      nil,
		},
		nodeHealthErr: nil,
		apiHealthErr:  nil, // API is healthy
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	// With the new behavior, API healthy means service is healthy
	if err != nil {
		t.Error("Expected no error when API is healthy even with no ready nodes")
	}

	if health != services.HealthHealthy {
		t.Errorf("Expected health %s, got %s", services.HealthHealthy, health)
	}

	// Check that node counts are still tracked
	service.mu.RLock()
	if service.readyNodes != 0 {
		t.Errorf("Expected readyNodes 0, got %d", service.readyNodes)
	}
	if service.totalNodes != 3 {
		t.Errorf("Expected totalNodes 3, got %d", service.totalNodes)
	}
	service.mu.RUnlock()
}

func TestK8sConnectionService_CheckHealth_APIUnhealthy(t *testing.T) {
	testErr := errors.New("API server not responding")
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
			Error:      nil,
		},
		nodeHealthErr: nil,
		apiHealthErr:  testErr, // API is unhealthy
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

func TestK8sConnectionService_CheckHealth_NodeInfoError(t *testing.T) {
	nodeErr := errors.New("failed to get nodes")
	kubeMgr := &mockKubeManager{
		nodeHealth:    kube.NodeHealth{},
		nodeHealthErr: nodeErr,
		apiHealthErr:  nil, // API is healthy
	}

	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	health, err := service.CheckHealth(context.Background())

	// API is healthy, so service should be healthy despite node info error
	if err != nil {
		t.Errorf("Expected no error when API is healthy, got %v", err)
	}

	if health != services.HealthHealthy {
		t.Errorf("Expected health %s, got %s", services.HealthHealthy, health)
	}

	// Check that internal state reflects the node error
	service.mu.RLock()
	if service.readyNodes != -1 {
		t.Errorf("Expected readyNodes -1, got %d", service.readyNodes)
	}
	if service.totalNodes != -1 {
		t.Errorf("Expected totalNodes -1, got %d", service.totalNodes)
	}
	if service.healthError != nodeErr {
		t.Errorf("Expected health error %v, got %v", nodeErr, service.healthError)
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
		nodeHealthErr: nil,
		apiHealthErr:  nil, // API is healthy
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

func TestK8sConnectionService_Start_WithLogin(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
			Error:      nil,
		},
		nodeHealthErr: nil,
		apiHealthErr:  nil,
	}

	// Use a teleport context that requires login
	service := NewK8sConnectionService("mc-test", "teleport.giantswarm.io-test", true, kubeMgr)

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting service: %v", err)
	}

	if service.GetState() != services.StateRunning {
		t.Errorf("Expected state %s, got %s", services.StateRunning, service.GetState())
	}
}

func TestK8sConnectionService_Start_LoginFailure(t *testing.T) {
	loginErr := errors.New("authentication failed")
	kubeMgr := &mockKubeManager{
		loginErr: loginErr,
	}

	// Use a teleport context that requires login
	service := NewK8sConnectionService("mc-test", "teleport.giantswarm.io-test", true, kubeMgr)

	ctx := context.Background()
	err := service.Start(ctx)
	if err == nil {
		t.Error("Expected error when login fails")
	}

	if !strings.Contains(err.Error(), "failed to login to cluster") {
		t.Errorf("Expected login error, got: %v", err)
	}

	if service.GetState() != services.StateFailed {
		t.Errorf("Expected state %s, got %s", services.StateFailed, service.GetState())
	}
}

func TestK8sConnectionService_Start_NoLoginForNonTeleportContext(t *testing.T) {
	kubeMgr := &mockKubeManager{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
			Error:      nil,
		},
		nodeHealthErr: nil,
		apiHealthErr:  nil,
		loginErr:      errors.New("should not be called"),
	}

	// Use a non-teleport context that should not trigger login
	service := NewK8sConnectionService("test-k8s", "test-context", true, kubeMgr)

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting service: %v", err)
	}

	if service.GetState() != services.StateRunning {
		t.Errorf("Expected state %s, got %s", services.StateRunning, service.GetState())
	}
}
