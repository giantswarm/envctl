package orchestrator

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/services"
	"sync"
	"time"
)

// mockService is a mock implementation of services.Service for testing
type mockService struct {
	mu sync.RWMutex

	// Configurable fields
	label        string
	serviceType  services.ServiceType
	state        services.ServiceState
	health       services.HealthStatus
	lastError    error
	dependencies []string

	// Function hooks for testing
	startFunc   func(ctx context.Context) error
	stopFunc    func(ctx context.Context) error
	restartFunc func(ctx context.Context) error

	// State change callback
	stateChangeCallback services.StateChangeCallback
}

func (m *mockService) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.startFunc != nil {
		return m.startFunc(ctx)
	}

	oldState := m.state
	m.state = services.StateRunning
	if m.stateChangeCallback != nil {
		m.stateChangeCallback(m.label, oldState, m.state, m.health, nil)
	}
	return nil
}

func (m *mockService) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopFunc != nil {
		return m.stopFunc(ctx)
	}

	oldState := m.state
	m.state = services.StateStopped
	if m.stateChangeCallback != nil {
		m.stateChangeCallback(m.label, oldState, m.state, m.health, nil)
	}
	return nil
}

func (m *mockService) Restart(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.restartFunc != nil {
		return m.restartFunc(ctx)
	}

	// Simple restart: stop then start
	m.state = services.StateStopped
	m.state = services.StateRunning
	return nil
}

func (m *mockService) GetState() services.ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == "" {
		return services.StateUnknown
	}
	return m.state
}

func (m *mockService) GetHealth() services.HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.health == "" {
		return services.HealthUnknown
	}
	return m.health
}

func (m *mockService) GetLastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastError
}

func (m *mockService) GetLabel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.label
}

func (m *mockService) GetType() services.ServiceType {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.serviceType == "" {
		return services.TypePortForward
	}
	return m.serviceType
}

func (m *mockService) GetDependencies() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dependencies
}

func (m *mockService) SetStateChangeCallback(callback services.StateChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateChangeCallback = callback
}

// mockHealthChecker is a mock service that also implements HealthChecker
type mockHealthChecker struct {
	mockService
	checkHealthFunc     func(ctx context.Context) (services.HealthStatus, error)
	healthCheckInterval time.Duration
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	if m.checkHealthFunc != nil {
		return m.checkHealthFunc(ctx)
	}
	return services.HealthHealthy, nil
}

func (m *mockHealthChecker) GetHealthCheckInterval() time.Duration {
	if m.healthCheckInterval == 0 {
		return 30 * time.Second
	}
	return m.healthCheckInterval
}

// mockKubeManager is a mock implementation of kube.Manager
type mockKubeManager struct {
	loginFunc                    func(clusterName string) (stdout string, stderr string, err error)
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
	return "gs-" + mcName
}

func (m *mockKubeManager) BuildWcContextName(mcName, wcName string) string {
	if m.buildWcContextNameFunc != nil {
		return m.buildWcContextNameFunc(mcName, wcName)
	}
	return "gs-" + mcName + "-" + wcName
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
