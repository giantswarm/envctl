package orchestrator

import (
	"context"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"sync"

	"github.com/stretchr/testify/mock"
)

// NodeHealth represents the health status of nodes in a cluster
type NodeHealth struct {
	ReadyNodes int
	TotalNodes int
	Error      error
}

// Mock KubeManager
type mockKubeManager struct {
	mock.Mock
}

func (m *mockKubeManager) Login(clusterName string) (string, string, error) {
	args := m.Called(clusterName)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockKubeManager) ListClusters() (interface{}, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func (m *mockKubeManager) GetCurrentContext() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockKubeManager) SwitchContext(target string) error {
	args := m.Called(target)
	return args.Error(0)
}

func (m *mockKubeManager) BuildMcContextName(mcShortName string) string {
	args := m.Called(mcShortName)
	return args.String(0)
}

func (m *mockKubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	args := m.Called(mcShortName, wcShortName)
	return args.String(0)
}

func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	args := m.Called(contextName)
	return args.String(0)
}

func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	args := m.Called(contextName)
	return args.Bool(0)
}

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (NodeHealth, error) {
	args := m.Called(ctx, kubeContextName)
	return args.Get(0).(NodeHealth), args.Error(1)
}

func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockKubeManager) SetReporter(reporter reporting.ServiceReporter) {
	m.Called(reporter)
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	args := m.Called(ctx, kubeContextName)
	return args.String(0), args.Error(1)
}

// Mock ServiceManager - updated to match the simplified interface
type mockServiceManager struct {
	mock.Mock
	activeServices map[string]bool
	mu             sync.Mutex
}

func newMockServiceManager() *mockServiceManager {
	return &mockServiceManager{
		activeServices: make(map[string]bool),
	}
}

func (m *mockServiceManager) StartServices(configs []managers.ManagedServiceConfig, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
	args := m.Called(configs, wg)

	// Track started services
	m.mu.Lock()
	for _, cfg := range configs {
		m.activeServices[cfg.Label] = true
	}
	m.mu.Unlock()

	return args.Get(0).(map[string]chan struct{}), args.Get(1).([]error)
}

func (m *mockServiceManager) StopService(label string) error {
	args := m.Called(label)

	// Track stopped service
	m.mu.Lock()
	delete(m.activeServices, label)
	m.mu.Unlock()

	return args.Error(0)
}

func (m *mockServiceManager) StopAllServices() {
	m.Called()

	// Clear all active services
	m.mu.Lock()
	m.activeServices = make(map[string]bool)
	m.mu.Unlock()
}

func (m *mockServiceManager) SetReporter(reporter reporting.ServiceReporter) {
	m.Called(reporter)
}

func (m *mockServiceManager) GetServiceConfig(label string) (managers.ManagedServiceConfig, bool) {
	args := m.Called(label)
	return args.Get(0).(managers.ManagedServiceConfig), args.Bool(1)
}

func (m *mockServiceManager) IsServiceActive(label string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeServices[label]
}

func (m *mockServiceManager) GetActiveServices() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var labels []string
	for label := range m.activeServices {
		labels = append(labels, label)
	}
	return labels
}

// Mock Reporter
type mockReporter struct {
	mock.Mock
}

func (m *mockReporter) Report(update reporting.ManagedServiceUpdate) {
	m.Called(update)
}

func (m *mockReporter) GetStateStore() reporting.StateStore {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(reporting.StateStore)
}
