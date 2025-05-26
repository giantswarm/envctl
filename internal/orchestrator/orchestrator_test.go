package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock KubeManager
type mockKubeManager struct {
	mock.Mock
}

func (m *mockKubeManager) Login(clusterName string) (string, string, error) {
	args := m.Called(clusterName)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *mockKubeManager) ListClusters() (*k8smanager.ClusterList, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*k8smanager.ClusterList), args.Error(1)
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

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (k8smanager.NodeHealth, error) {
	args := m.Called(ctx, kubeContextName)
	return args.Get(0).(k8smanager.NodeHealth), args.Error(1)
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
	mu            sync.Mutex
	healthUpdates []reporting.HealthStatusUpdate
}

func (m *mockReporter) Report(update reporting.ManagedServiceUpdate) {
	m.Called(update)
}

func (m *mockReporter) ReportHealth(update reporting.HealthStatusUpdate) {
	m.Called(update)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthUpdates = append(m.healthUpdates, update)
}

func (m *mockReporter) GetHealthUpdates() []reporting.HealthStatusUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	copies := make([]reporting.HealthStatusUpdate, len(m.healthUpdates))
	copy(copies, m.healthUpdates)
	return copies
}

func TestOrchestrator_HealthMonitoring(t *testing.T) {
	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	// Configure orchestrator
	cfg := Config{
		MCName:              "test-mc",
		WCName:              "test-wc",
		HealthCheckInterval: 100 * time.Millisecond, // Fast for testing
	}

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("BuildWcContextName", "test-mc", "test-wc").Return("teleport.giantswarm.io-test-mc-test-wc")

	// First health check - both healthy
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-test-mc").Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-test-mc-test-wc").Return(
		k8smanager.NodeHealth{ReadyNodes: 5, TotalNodes: 5}, nil,
	).Maybe()

	// Service manager expectations
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("SetReporter", mock.Anything).Return()
	serviceMgr.On("StopService", mock.Anything).Return(nil).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()
	reporter.On("Report", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Give time for initial health check to complete and state to be recorded
	time.Sleep(20 * time.Millisecond)

	// Check health reports
	healthUpdates := reporter.GetHealthUpdates()
	assert.GreaterOrEqual(t, len(healthUpdates), 2, "Should have at least 2 health updates (MC and WC)")

	// Verify MC health update
	mcUpdate := findHealthUpdate(healthUpdates, "test-mc", true)
	assert.NotNil(t, mcUpdate, "Should have MC health update")
	if mcUpdate != nil {
		assert.True(t, mcUpdate.IsHealthy)
		assert.Equal(t, 3, mcUpdate.ReadyNodes)
		assert.Equal(t, 3, mcUpdate.TotalNodes)
		assert.Nil(t, mcUpdate.Error)
	}

	// Verify WC health update
	wcUpdate := findHealthUpdate(healthUpdates, "test-wc", false)
	assert.NotNil(t, wcUpdate, "Should have WC health update")
	if wcUpdate != nil {
		assert.True(t, wcUpdate.IsHealthy)
		assert.Equal(t, 5, wcUpdate.ReadyNodes)
		assert.Equal(t, 5, wcUpdate.TotalNodes)
		assert.Nil(t, wcUpdate.Error)
	}

	// Stop orchestrator
	orch.Stop()
}

func TestOrchestrator_HealthMonitoring_MCOnly(t *testing.T) {
	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	// Configure orchestrator with only MC (no WC)
	cfg := Config{
		MCName:              "solo-mc",
		WCName:              "", // No workload cluster
		HealthCheckInterval: 100 * time.Millisecond,
	}

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "solo-mc").Return("teleport.giantswarm.io-solo-mc")

	// Only MC health check should happen
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-solo-mc").Return(
		k8smanager.NodeHealth{ReadyNodes: 4, TotalNodes: 4}, nil,
	).Maybe()

	// Service manager expectations
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("SetReporter", mock.Anything).Return()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()
	reporter.On("Report", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait for first health check
	time.Sleep(150 * time.Millisecond)

	// Check health reports
	healthUpdates := reporter.GetHealthUpdates()
	assert.GreaterOrEqual(t, len(healthUpdates), 1, "Should have at least 1 health update (MC only)")

	// Verify MC health update
	mcUpdate := findHealthUpdate(healthUpdates, "solo-mc", true)
	assert.NotNil(t, mcUpdate, "Should have MC health update")
	if mcUpdate != nil {
		assert.True(t, mcUpdate.IsHealthy)
		assert.Equal(t, 4, mcUpdate.ReadyNodes)
		assert.Equal(t, 4, mcUpdate.TotalNodes)
		assert.Nil(t, mcUpdate.Error)
	}

	// Verify no WC health update
	wcUpdates := 0
	for _, u := range healthUpdates {
		if !u.IsMC {
			wcUpdates++
		}
	}
	assert.Equal(t, 0, wcUpdates, "Should have no WC health updates when WC is not configured")

	// Stop orchestrator
	orch.Stop()
}

func TestOrchestrator_ServiceLifecycleOnHealthChange(t *testing.T) {
	// Enable debug logging for this test
	oldLogLevel := os.Getenv("LOG_LEVEL")
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Setenv("LOG_LEVEL", oldLogLevel)
	
	// Initialize logging for the test
	logging.InitForCLI(logging.LevelDebug, os.Stdout)
	
	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	// Configure orchestrator with controlled health check interval
	cfg := Config{
		MCName:              "lifecycle-mc",
		WCName:              "lifecycle-wc",
		HealthCheckInterval: 50 * time.Millisecond, // Short interval for faster test
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "test-pf",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:                 "test-mcp",
				Enabled:              true,
				RequiresPortForwards: []string{"test-pf"},
			},
		},
	}

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "lifecycle-mc").Return("teleport.giantswarm.io-lifecycle-mc")
	kubeMgr.On("BuildWcContextName", "lifecycle-mc", "lifecycle-wc").Return("teleport.giantswarm.io-lifecycle-mc-lifecycle-wc")

	// Track which services are stopped
	var stoppedServices []string
	var stoppedMutex sync.Mutex
	
	// Track which services have been started
	var startedServices = make(map[string]bool)
	
	// Track health check calls
	var healthCheckCount int
	var healthCheckMutex sync.Mutex
	
	// Service manager expectations
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		configs := args.Get(0).([]managers.ManagedServiceConfig)
		stoppedMutex.Lock()
		for _, cfg := range configs {
			startedServices[cfg.Label] = true
			t.Logf("Started service: %s", cfg.Label)
		}
		stoppedMutex.Unlock()
	}).Return(map[string]chan struct{}{}, []error{})
	
	serviceMgr.On("SetReporter", mock.Anything).Return()
	
	serviceMgr.On("StopService", mock.Anything).Run(func(args mock.Arguments) {
		label := args.String(0)
		stoppedMutex.Lock()
		stoppedServices = append(stoppedServices, label)
		delete(startedServices, label)
		t.Logf("Stopped service: %s", label)
		stoppedMutex.Unlock()
	}).Return(nil).Maybe()
	
	serviceMgr.On("StopAllServices").Return().Maybe()
	
	// Return dynamic active status based on started/stopped services
	serviceMgr.On("IsServiceActive", mock.Anything).Return(func(label string) bool {
		stoppedMutex.Lock()
		defer stoppedMutex.Unlock()
		// Service is active if it has been started and not stopped
		isActive := startedServices[label]
		t.Logf("IsServiceActive(%s) = %v", label, isActive)
		return isActive
	})

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()
	reporter.On("Report", mock.Anything).Return().Maybe()

	// MC always stays healthy
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc").Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	// WC health check - set up expectations in order
	// Initial checks during Start() plus first few periodic checks: healthy
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc").
		Return(k8smanager.NodeHealth{ReadyNodes: 5, TotalNodes: 5}, nil).
		Run(func(args mock.Arguments) {
			healthCheckMutex.Lock()
			healthCheckCount++
			count := healthCheckCount
			healthCheckMutex.Unlock()
			t.Logf("WC health check #%d (healthy)", count)
		}).Times(4) // Allow 4 healthy checks to ensure services start
	
	// Fifth check: unhealthy
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc").
		Return(k8smanager.NodeHealth{ReadyNodes: 2, TotalNodes: 5, Error: fmt.Errorf("node failure")}, fmt.Errorf("node failure")).
		Run(func(args mock.Arguments) {
			healthCheckMutex.Lock()
			healthCheckCount++
			count := healthCheckCount
			healthCheckMutex.Unlock()
			t.Logf("WC health check #%d (unhealthy)", count)
		}).Once()
	
	// Sixth check and beyond: healthy again
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc").
		Return(k8smanager.NodeHealth{ReadyNodes: 5, TotalNodes: 5}, nil).
		Run(func(args mock.Arguments) {
			healthCheckMutex.Lock()
			healthCheckCount++
			count := healthCheckCount
			healthCheckMutex.Unlock()
			t.Logf("WC health check #%d (healthy again)", count)
		}).Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Log the dependency graph structure
	depGraph := orch.GetDependencyGraph()
	if depGraph != nil {
		t.Logf("Dependency graph nodes:")
		// Check specific dependencies
		wcNode := depGraph.Get("k8s:teleport.giantswarm.io-lifecycle-mc-lifecycle-wc")
		if wcNode != nil {
			t.Logf("WC k8s node exists")
			wcDependents := depGraph.Dependents("k8s:teleport.giantswarm.io-lifecycle-mc-lifecycle-wc")
			t.Logf("Direct dependents of WC k8s: %v", wcDependents)
		}
		
		pfNode := depGraph.Get("pf:test-pf")
		if pfNode != nil {
			t.Logf("PF node exists, depends on: %v", pfNode.DependsOn)
			pfDependents := depGraph.Dependents("pf:test-pf")
			t.Logf("Direct dependents of test-pf: %v", pfDependents)
		}
		
		mcpNode := depGraph.Get("mcp:test-mcp")
		if mcpNode != nil {
			t.Logf("MCP node exists, depends on: %v", mcpNode.DependsOn)
		}
	}

	// Wait for initial startup to complete and first periodic check
	time.Sleep(100 * time.Millisecond)

	// Wait for WC to become unhealthy (should be on 5th health check)
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		
		healthCheckMutex.Lock()
		count := healthCheckCount
		healthCheckMutex.Unlock()
		
		if count >= 5 {
			// Give time for cascade stop to complete
			time.Sleep(100 * time.Millisecond)
			break
		}
	}

	// Verify dependent services were stopped
	stoppedMutex.Lock()
	stoppedCount := len(stoppedServices)
	stoppedServicesCopy := make([]string, len(stoppedServices))
	copy(stoppedServicesCopy, stoppedServices)
	t.Logf("Stopped services after WC failure: %v", stoppedServicesCopy)
	stoppedMutex.Unlock()
	
	assert.GreaterOrEqual(t, stoppedCount, 1, "Should have stopped at least 1 service (pf)")
	assert.Contains(t, stoppedServicesCopy, "test-pf", "Port forward should be stopped")
	// Note: test-mcp might not be stopped if it wasn't started yet

	// Clear stopped services before recovery
	stoppedMutex.Lock()
	stoppedServices = nil
	stoppedMutex.Unlock()

	// Wait for WC to become healthy again (6th check)
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		
		healthCheckMutex.Lock()
		count := healthCheckCount
		healthCheckMutex.Unlock()
		
		if count >= 6 {
			// Give time for services to restart
			time.Sleep(100 * time.Millisecond)
			break
		}
	}

	// Stop orchestrator
	orch.Stop()
}

func TestOrchestrator_CascadeStop(t *testing.T) {
	// Test that stopping a service cascades to its dependents
	kubeMgr := &mockKubeManager{}
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	cfg := Config{
		MCName: "test-mc",
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "test-pf",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-test-mc",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:                 "test-mcp",
				Enabled:              true,
				RequiresPortForwards: []string{"test-pf"},
			},
		},
	}

	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("SetReporter", mock.Anything).Return()
	serviceMgr.On("StopAllServices").Return().Maybe()
	serviceMgr.On("IsServiceActive", "test-pf").Return(true)
	serviceMgr.On("IsServiceActive", "test-mcp").Return(true)
	
	// Track stopped services
	var stoppedServices []string
	var stoppedMutex sync.Mutex
	
	serviceMgr.On("StopService", mock.Anything).Run(func(args mock.Arguments) {
		stoppedMutex.Lock()
		stoppedServices = append(stoppedServices, args.String(0))
		stoppedMutex.Unlock()
	}).Return(nil)

	reporter.On("ReportHealth", mock.Anything).Return().Maybe()
	reporter.On("Report", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Stop the port forward through orchestrator
	err = orch.StopService("test-pf")
	assert.NoError(t, err)

	// Verify both services were stopped (cascade)
	stoppedMutex.Lock()
	assert.Contains(t, stoppedServices, "test-mcp", "MCP should be stopped when its port forward is stopped")
	assert.Contains(t, stoppedServices, "test-pf", "Port forward should be stopped")
	stoppedMutex.Unlock()

	// Stop orchestrator
	orch.Stop()
}

func TestOrchestrator_RestartService(t *testing.T) {
	// Test service restart functionality
	kubeMgr := &mockKubeManager{}
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	cfg := Config{
		MCName: "test-mc",
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "test-pf",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-test-mc",
			},
		},
	}

	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	// Track service operations
	var startCalls int
	var stopCalls int
	var serviceActive = true
	var mu sync.Mutex
	var interceptor *serviceStateInterceptor
	
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		startCalls++
		serviceActive = true
		mu.Unlock()
	}).Return(map[string]chan struct{}{}, []error{})
	
	serviceMgr.On("SetReporter", mock.Anything).Run(func(args mock.Arguments) {
		// Store the interceptor for later use
		interceptor = args.Get(0).(*serviceStateInterceptor)
	}).Return()
	
	serviceMgr.On("StopService", "test-pf").Run(func(args mock.Arguments) {
		mu.Lock()
		stopCalls++
		serviceActive = false
		mu.Unlock()
		
		// Simulate the service reporting stopped state after a short delay
		if interceptor != nil {
			go func() {
				time.Sleep(20 * time.Millisecond)
				interceptor.Report(reporting.ManagedServiceUpdate{
					Timestamp:   time.Now(),
					SourceType:  reporting.ServiceTypePortForward,
					SourceLabel: "test-pf",
					State:       reporting.StateStopped,
					IsReady:     false,
				})
			}()
		}
	}).Return(nil)
	
	serviceMgr.On("StopAllServices").Return().Maybe()
	serviceMgr.On("IsServiceActive", "test-pf").Return(func(label string) bool {
		mu.Lock()
		defer mu.Unlock()
		return serviceActive
	})

	reporter.On("ReportHealth", mock.Anything).Return().Maybe()
	reporter.On("Report", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Verify initial start
	mu.Lock()
	initialStarts := startCalls
	mu.Unlock()
	assert.Equal(t, 1, initialStarts, "Service should be started once initially")

	// Restart the service
	err = orch.RestartService("test-pf")
	assert.NoError(t, err)

	// Wait for restart to complete
	time.Sleep(150 * time.Millisecond)

	// Verify stop was called
	mu.Lock()
	finalStops := stopCalls
	finalStarts := startCalls
	mu.Unlock()
	
	assert.Equal(t, 1, finalStops, "Service should be stopped once")
	assert.GreaterOrEqual(t, finalStarts, 2, "Service should be started at least twice (initial + restart)")

	// Stop orchestrator
	orch.Stop()
}

func findHealthUpdate(updates []reporting.HealthStatusUpdate, clusterName string, isMC bool) *reporting.HealthStatusUpdate {
	for _, u := range updates {
		if u.ClusterShortName == clusterName && u.IsMC == isMC {
			return &u
		}
	}
	return nil
}

// TestOrchestrator_DependencyOrdering tests that services are started in the correct dependency order
func TestOrchestrator_DependencyOrdering(t *testing.T) {
	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	// Configure orchestrator with port forwards and MCPs
	cfg := Config{
		MCName: "test-mc",
		WCName: "test-wc",
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "mc-prometheus",
				Enabled:           true,
				LocalPort:         "9090",
				RemotePort:        "9090",
				Namespace:         "monitoring",
				TargetType:        "service",
				TargetName:        "prometheus",
				KubeContextTarget: "teleport.giantswarm.io-test-mc",
			},
			{
				Name:              "wc-grafana",
				Enabled:           true,
				LocalPort:         "3000",
				RemotePort:        "3000",
				Namespace:         "monitoring",
				TargetType:        "service",
				TargetName:        "grafana",
				KubeContextTarget: "teleport.giantswarm.io-test-mc-test-wc",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:                 "prometheus",
				Enabled:              true,
				Type:                 config.MCPServerTypeLocalCommand,
				RequiresPortForwards: []string{"mc-prometheus"},
			},
			{
				Name:                 "grafana",
				Enabled:              true,
				Type:                 config.MCPServerTypeLocalCommand,
				RequiresPortForwards: []string{"wc-grafana"},
			},
			{
				Name:    "kubernetes",
				Enabled: true,
				Type:    config.MCPServerTypeLocalCommand,
				// kubernetes MCP depends on MC k8s connection, not port forwards
			},
		},
		HealthCheckInterval: 100 * time.Millisecond,
	}

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("BuildWcContextName", "test-mc", "test-wc").Return("teleport.giantswarm.io-test-mc-test-wc")

	// Capture the order of services started
	var startOrder [][]string
	var startOrderMutex sync.Mutex

	serviceMgr.On("StartServices", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			configs := args.Get(0).([]managers.ManagedServiceConfig)
			startOrderMutex.Lock()
			var levelServices []string
			for _, cfg := range configs {
				levelServices = append(levelServices, cfg.Label)
			}
			startOrder = append(startOrder, levelServices)
			startOrderMutex.Unlock()
		}).
		Return(map[string]chan struct{}{}, []error{})

	serviceMgr.On("SetReporter", mock.Anything).Return()

	// Health check expectations
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	serviceMgr.On("StopAllServices").Return().Maybe()
	reporter.On("ReportHealth", mock.Anything).Return().Maybe()
	reporter.On("Report", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Give it a moment to ensure services are started
	time.Sleep(100 * time.Millisecond)

	// Get the actual start order
	startOrderMutex.Lock()
	actualOrder := make([][]string, len(startOrder))
	copy(actualOrder, startOrder)
	startOrderMutex.Unlock()

	t.Logf("Service start order by levels: %v", actualOrder)

	// Verify we have multiple levels (dependency ordering)
	assert.Greater(t, len(actualOrder), 1, "Should have multiple dependency levels")

	// Check that port forwards are in earlier levels than their dependent MCPs
	pfLevel := -1
	mcpLevel := -1
	
	for level, services := range actualOrder {
		for _, svc := range services {
			if svc == "mc-prometheus" || svc == "wc-grafana" {
				if pfLevel == -1 || level < pfLevel {
					pfLevel = level
				}
			}
			if svc == "prometheus" || svc == "grafana" {
				if mcpLevel == -1 || level > mcpLevel {
					mcpLevel = level
				}
			}
		}
	}

	assert.NotEqual(t, -1, pfLevel, "Should have found port forwards")
	assert.NotEqual(t, -1, mcpLevel, "Should have found MCPs")
	assert.Less(t, pfLevel, mcpLevel, "Port forwards should start before dependent MCPs")

	// Stop orchestrator
	orch.Stop()
}
