package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestOrchestrator_HealthMonitoring(t *testing.T) {
	t.Skip("Health monitoring is now handled by K8s connection services")
}

func TestOrchestrator_HealthMonitoring_MCOnly(t *testing.T) {
	t.Skip("Health monitoring is now handled by K8s connection services")
}

func TestOrchestrator_ServiceLifecycleOnHealthChange(t *testing.T) {
	t.Skip("Health-based service lifecycle is now handled by K8s connection services and their dependencies")
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

	orch := New(serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
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

	reporter.On("Report", mock.Anything).Return().Maybe()
	reporter.On("GetStateStore").Return(reporting.NewStateStore()).Maybe()

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

	orch := New(serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	// Note: K8s connections will be marked as healthy when the K8s service reports its state

	// Track service operations
	var startCalls int
	var stopCalls int
	activeServices := make(map[string]bool)
	var mu sync.Mutex
	var interceptor *serviceStateInterceptor

	// Expect K8s connection services to be started first
	serviceMgr.On("StartServices", mock.MatchedBy(func(configs []managers.ManagedServiceConfig) bool {
		// Check if this is K8s connection services
		for _, cfg := range configs {
			if cfg.Type == reporting.ServiceTypeKube {
				return true
			}
		}
		return false
	}), mock.Anything).Return(map[string]chan struct{}{}, []error{}).Once()

	// Then expect other services to be started
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		configs := args.Get(0).([]managers.ManagedServiceConfig)
		mu.Lock()
		for _, cfg := range configs {
			if cfg.Label == "test-pf" {
				startCalls++
				t.Logf("Starting service test-pf (call #%d)", startCalls)
			}
			activeServices[cfg.Label] = true
		}
		mu.Unlock()
	}).Return(map[string]chan struct{}{}, []error{})

	serviceMgr.On("SetReporter", mock.Anything).Run(func(args mock.Arguments) {
		// Store the interceptor for later use
		interceptor = args.Get(0).(*serviceStateInterceptor)
	}).Return()

	serviceMgr.On("StopService", "test-pf").Run(func(args mock.Arguments) {
		mu.Lock()
		stopCalls++
		activeServices["test-pf"] = false
		t.Logf("Stopping service test-pf (call #%d)", stopCalls)
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

	serviceMgr.On("StopService", mock.Anything).Return(nil).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()
	serviceMgr.On("IsServiceActive", mock.Anything).Return(func(label string) bool {
		mu.Lock()
		defer mu.Unlock()
		active := activeServices[label]
		t.Logf("IsServiceActive(%s) = %v", label, active)
		return active
	})

	reporter.On("Report", mock.Anything).Return().Maybe()
	reporter.On("GetStateStore").Return(reporting.NewStateStore()).Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait for services to be started (they start asynchronously after K8s becomes healthy)
	// Poll for the service to be started
	started := false
	for i := 0; i < 30; i++ { // Try for up to 3 seconds
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		if startCalls > 0 {
			started = true
			mu.Unlock()
			break
		}
		mu.Unlock()
	}
	assert.True(t, started, "Service should have been started")

	// Verify initial start
	mu.Lock()
	initialStarts := startCalls
	mu.Unlock()
	assert.GreaterOrEqual(t, initialStarts, 1, "Service should be started at least once initially")

	// Restart the service
	err = orch.RestartService("test-pf")
	assert.NoError(t, err)

	// Wait for restart to complete (includes 1 second delay + processing time)
	time.Sleep(1200 * time.Millisecond)

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

func TestOrchestrator_RestartServiceWithDependencies(t *testing.T) {
	// Test that restarting a service also restarts its dependencies that were stopped due to cascade
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

	orch := New(serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	// Note: K8s connections will be marked as healthy when the K8s service reports its state

	// Track service states
	activeServices := map[string]bool{
		"test-pf":  false,
		"test-mcp": false,
	}
	var mu sync.Mutex
	var interceptor *serviceStateInterceptor

	// Since K8s is already healthy, expect all services to be started at once
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		configs := args.Get(0).([]managers.ManagedServiceConfig)
		mu.Lock()
		for _, cfg := range configs {
			activeServices[cfg.Label] = true
			t.Logf("Started service: %s", cfg.Label)
		}
		mu.Unlock()
	}).Return(map[string]chan struct{}{}, []error{})

	serviceMgr.On("SetReporter", mock.Anything).Run(func(args mock.Arguments) {
		interceptor = args.Get(0).(*serviceStateInterceptor)
	}).Return()

	serviceMgr.On("StopService", mock.Anything).Run(func(args mock.Arguments) {
		label := args.String(0)
		mu.Lock()
		activeServices[label] = false
		t.Logf("Stopped service: %s", label)
		mu.Unlock()

		// Simulate the service reporting stopped state
		if interceptor != nil {
			go func() {
				time.Sleep(20 * time.Millisecond)
				serviceType := reporting.ServiceTypePortForward
				if label == "test-mcp" {
					serviceType = reporting.ServiceTypeMCPServer
				}
				interceptor.Report(reporting.ManagedServiceUpdate{
					Timestamp:   time.Now(),
					SourceType:  serviceType,
					SourceLabel: label,
					State:       reporting.StateStopped,
					IsReady:     false,
				})
			}()
		}
	}).Return(nil)

	serviceMgr.On("StopAllServices").Return().Maybe()
	serviceMgr.On("IsServiceActive", mock.Anything).Return(func(label string) bool {
		mu.Lock()
		defer mu.Unlock()
		return activeServices[label]
	})

	reporter.On("Report", mock.Anything).Return().Maybe()
	reporter.On("GetStateStore").Return(reporting.NewStateStore()).Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait a bit for initial startup
	time.Sleep(100 * time.Millisecond)

	// Verify both services are running
	mu.Lock()
	assert.True(t, activeServices["test-pf"], "Port forward should be running")
	assert.True(t, activeServices["test-mcp"], "MCP should be running")
	mu.Unlock()

	// Stop the port forward (this should cascade stop the MCP)
	err = orch.StopService("test-pf")
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// Verify both services are stopped
	mu.Lock()
	assert.False(t, activeServices["test-pf"], "Port forward should be stopped")
	assert.False(t, activeServices["test-mcp"], "MCP should be stopped due to cascade")
	mu.Unlock()

	// Now restart the MCP - this should also restart the port forward dependency
	err = orch.RestartService("test-mcp")
	assert.NoError(t, err)
	time.Sleep(1200 * time.Millisecond) // Wait for restart including 1 second delay

	// Verify both services are running again
	mu.Lock()
	pfRunning := activeServices["test-pf"]
	mcpRunning := activeServices["test-mcp"]
	mu.Unlock()

	assert.True(t, pfRunning, "Port forward should be restarted as dependency")
	assert.True(t, mcpRunning, "MCP should be restarted")

	// Stop orchestrator
	orch.Stop()
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
	orch := New(serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")
	kubeMgr.On("BuildWcContextName", "test-mc", "test-wc").Return("teleport.giantswarm.io-test-mc-test-wc")

	// Note: K8s connections will be marked as healthy when the K8s service reports its state

	// Track service operations
	var startOrder [][]string
	var startOrderMutex sync.Mutex

	// Since K8s is already healthy, expect services to be started in dependency order
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
		NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	serviceMgr.On("StopAllServices").Return().Maybe()
	serviceMgr.On("IsServiceActive", mock.Anything).Return(false).Maybe()
	reporter.On("Report", mock.Anything).Return().Maybe()
	reporter.On("GetStateStore").Return(reporting.NewStateStore()).Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Give it time to start services
	time.Sleep(100 * time.Millisecond)

	// Get the actual start order
	startOrderMutex.Lock()
	actualOrder := make([][]string, len(startOrder))
	copy(actualOrder, startOrder)
	startOrderMutex.Unlock()

	t.Logf("Service start order by levels: %v", actualOrder)

	// We should have multiple levels showing dependency ordering
	assert.GreaterOrEqual(t, len(actualOrder), 2, "Should have multiple dependency levels")

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

	// Only check dependency ordering if we found both types
	if pfLevel != -1 && mcpLevel != -1 {
		assert.Less(t, pfLevel, mcpLevel, "Port forwards should start before dependent MCPs")
	}

	// Stop orchestrator
	orch.Stop()
}

func TestBuildDependencyGraph(t *testing.T) {
	tests := []struct {
		name         string
		mcName       string
		wcName       string
		portForwards []config.PortForwardDefinition
		mcpServers   []config.MCPServerDefinition
		wantNodes    []string
		wantDeps     map[string][]string
	}{
		{
			name:   "MC only with port forwards",
			mcName: "test-mc",
			wcName: "",
			portForwards: []config.PortForwardDefinition{
				{
					Name:              "prometheus",
					Enabled:           true,
					KubeContextTarget: kube.BuildMcContext("test-mc"),
				},
			},
			mcpServers: []config.MCPServerDefinition{
				{
					Name:                 "prometheus-mcp",
					Enabled:              true,
					RequiresPortForwards: []string{"prometheus"},
				},
			},
			wantNodes: []string{"k8s-mc-test-mc", "pf:prometheus", "mcp:prometheus-mcp"},
			wantDeps: map[string][]string{
				"pf:prometheus":      {"k8s-mc-test-mc"},
				"mcp:prometheus-mcp": {"pf:prometheus"},
			},
		},
		{
			name:   "MC and WC with dependencies",
			mcName: "test-mc",
			wcName: "test-wc",
			portForwards: []config.PortForwardDefinition{
				{
					Name:              "mc-prometheus",
					Enabled:           true,
					KubeContextTarget: kube.BuildMcContext("test-mc"),
				},
				{
					Name:              "wc-alloy",
					Enabled:           true,
					KubeContextTarget: kube.BuildWcContext("test-mc", "test-wc"),
				},
			},
			mcpServers: []config.MCPServerDefinition{
				{
					Name:    "kubernetes",
					Enabled: true,
				},
				{
					Name:                 "prometheus-mcp",
					Enabled:              true,
					RequiresPortForwards: []string{"mc-prometheus"},
				},
			},
			wantNodes: []string{
				"k8s-mc-test-mc",
				"k8s-wc-test-wc",
				"pf:mc-prometheus",
				"pf:wc-alloy",
				"mcp:kubernetes",
				"mcp:prometheus-mcp",
			},
			wantDeps: map[string][]string{
				"pf:mc-prometheus":   {"k8s-mc-test-mc"},
				"pf:wc-alloy":        {"k8s-wc-test-wc"},
				"mcp:kubernetes":     {"k8s-mc-test-mc"},
				"mcp:prometheus-mcp": {"pf:mc-prometheus"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock service manager and reporter
			serviceMgr := newMockServiceManager()
			reporter := &mockReporter{}

			// Create orchestrator using New to ensure kubeMgr is initialized
			cfg := Config{
				MCName:       tt.mcName,
				WCName:       tt.wcName,
				PortForwards: tt.portForwards,
				MCPServers:   tt.mcpServers,
			}
			o := New(serviceMgr, reporter, cfg)

			graph := o.buildDependencyGraph()

			// Check nodes exist
			for _, nodeID := range tt.wantNodes {
				node := graph.Get(dependency.NodeID(nodeID))
				assert.NotNil(t, node, "Expected node %s to exist", nodeID)
			}

			// Check dependencies
			for nodeID, expectedDeps := range tt.wantDeps {
				node := graph.Get(dependency.NodeID(nodeID))
				if assert.NotNil(t, node, "Node %s should exist", nodeID) {
					assert.Equal(t, len(expectedDeps), len(node.DependsOn),
						"Node %s should have %d dependencies", nodeID, len(expectedDeps))
					for _, dep := range expectedDeps {
						assert.Contains(t, node.DependsOn, dependency.NodeID(dep),
							"Node %s should depend on %s", nodeID, dep)
					}
				}
			}
		})
	}
}
