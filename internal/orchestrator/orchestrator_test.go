package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"fmt"
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

// Mock ServiceManager
type mockServiceManager struct {
	mock.Mock
}

func (m *mockServiceManager) StartServices(configs []managers.ManagedServiceConfig, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
	args := m.Called(configs, wg)
	return args.Get(0).(map[string]chan struct{}), args.Get(1).([]error)
}

func (m *mockServiceManager) StartServicesWithDependencyOrder(configs []managers.ManagedServiceConfig, depGraph *dependency.Graph, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
	args := m.Called(configs, depGraph, wg)
	return args.Get(0).(map[string]chan struct{}), args.Get(1).([]error)
}

func (m *mockServiceManager) StopService(label string) error {
	args := m.Called(label)
	return args.Error(0)
}

func (m *mockServiceManager) StopServiceWithDependents(label string, depGraph *dependency.Graph) error {
	args := m.Called(label, depGraph)
	return args.Error(0)
}

func (m *mockServiceManager) StopAllServices() {
	m.Called()
}

func (m *mockServiceManager) RestartService(label string) error {
	args := m.Called(label)
	return args.Error(0)
}

func (m *mockServiceManager) SetReporter(reporter reporting.ServiceReporter) {
	m.Called(reporter)
}

func (m *mockServiceManager) StartServicesDependingOn(nodeID string, depGraph *dependency.Graph) error {
	args := m.Called(nodeID, depGraph)
	return args.Error(0)
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
	serviceMgr := &mockServiceManager{}
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
	serviceMgr.On("StartServicesWithDependencyOrder", mock.Anything, mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("StartServicesDependingOn", mock.Anything, mock.Anything).Return(nil).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait for first health check
	time.Sleep(150 * time.Millisecond)

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
	serviceMgr := &mockServiceManager{}
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
	serviceMgr.On("StartServicesWithDependencyOrder", mock.Anything, mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("StartServicesDependingOn", mock.Anything, mock.Anything).Return(nil).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()

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

func TestOrchestrator_HealthMonitoring_UnhealthyConnections(t *testing.T) {
	// This test verifies that the orchestrator correctly reports health status
	// for both healthy and unhealthy connections on initial startup.
	// It also verifies that no service lifecycle changes happen on initial state
	// (only on state transitions).

	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := &mockServiceManager{}
	reporter := &mockReporter{}

	// Configure orchestrator
	cfg := Config{
		MCName:              "unhealthy-mc",
		WCName:              "unhealthy-wc",
		HealthCheckInterval: 100 * time.Millisecond,
	}

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "unhealthy-mc").Return("teleport.giantswarm.io-unhealthy-mc")
	kubeMgr.On("BuildWcContextName", "unhealthy-mc", "unhealthy-wc").Return("teleport.giantswarm.io-unhealthy-mc-unhealthy-wc")

	// First health check - MC healthy, WC unhealthy
	testErr := fmt.Errorf("connection failed")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-unhealthy-mc").Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-unhealthy-mc-unhealthy-wc").Return(
		k8smanager.NodeHealth{Error: testErr}, testErr,
	).Maybe()

	// Service manager expectations
	serviceMgr.On("StartServicesWithDependencyOrder", mock.Anything, mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	// No StopServiceWithDependents should be called since connections don't change state from their initial state
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait for first health check
	time.Sleep(150 * time.Millisecond)

	// Check health reports
	healthUpdates := reporter.GetHealthUpdates()
	assert.GreaterOrEqual(t, len(healthUpdates), 2, "Should have at least 2 health updates")

	// Verify MC health update (healthy)
	mcUpdate := findHealthUpdate(healthUpdates, "unhealthy-mc", true)
	assert.NotNil(t, mcUpdate, "Should have MC health update")
	if mcUpdate != nil {
		assert.True(t, mcUpdate.IsHealthy)
		assert.Equal(t, 3, mcUpdate.ReadyNodes)
		assert.Equal(t, 3, mcUpdate.TotalNodes)
		assert.Nil(t, mcUpdate.Error)
	}

	// Verify WC health update (unhealthy)
	wcUpdate := findHealthUpdate(healthUpdates, "unhealthy-wc", false)
	assert.NotNil(t, wcUpdate, "Should have WC health update")
	if wcUpdate != nil {
		assert.False(t, wcUpdate.IsHealthy)
		assert.NotNil(t, wcUpdate.Error)
		assert.Equal(t, testErr.Error(), wcUpdate.Error.Error())
	}

	// Stop orchestrator
	orch.Stop()
}

func TestOrchestrator_ServiceLifecycleOnHealthChange(t *testing.T) {
	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := &mockServiceManager{}
	reporter := &mockReporter{}

	// Configure orchestrator with fast health check interval
	cfg := Config{
		MCName:              "lifecycle-mc",
		WCName:              "lifecycle-wc",
		HealthCheckInterval: 50 * time.Millisecond, // Very fast for testing
	}

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "lifecycle-mc").Return("teleport.giantswarm.io-lifecycle-mc")
	kubeMgr.On("BuildWcContextName", "lifecycle-mc", "lifecycle-wc").Return("teleport.giantswarm.io-lifecycle-mc-lifecycle-wc")

	// Service manager expectations
	serviceMgr.On("StartServicesWithDependencyOrder", mock.Anything, mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("StopServiceWithDependents", mock.Anything, mock.Anything).Return(nil).Maybe()
	serviceMgr.On("StartServicesDependingOn", mock.Anything, mock.Anything).Return(nil).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Reporter expectations
	reporter.On("ReportHealth", mock.Anything).Return()

	// First set of health checks - both healthy
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc").Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Times(2) // Expect 2 calls

	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc").Return(
		k8smanager.NodeHealth{ReadyNodes: 5, TotalNodes: 5}, nil,
	).Once() // First call returns healthy

	// Second health check - WC becomes unhealthy
	testErr := fmt.Errorf("node failure")
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc").Return(
		k8smanager.NodeHealth{ReadyNodes: 2, TotalNodes: 5, Error: testErr}, testErr,
	).Once() // Second call returns unhealthy

	// Third health check - WC becomes healthy again
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc").Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe() // MC stays healthy throughout

	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-lifecycle-mc-lifecycle-wc").Return(
		k8smanager.NodeHealth{ReadyNodes: 5, TotalNodes: 5}, nil,
	).Maybe() // WC becomes healthy again

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait for initial health check (both healthy)
	time.Sleep(60 * time.Millisecond)

	// Wait for second health check (WC becomes unhealthy)
	time.Sleep(60 * time.Millisecond)

	// Verify StopServiceWithDependents was called
	serviceMgr.AssertCalled(t, "StopServiceWithDependents", "k8s:teleport.giantswarm.io-lifecycle-mc-lifecycle-wc", mock.Anything)

	// Wait for third health check (WC becomes healthy again)
	time.Sleep(60 * time.Millisecond)

	// Verify StartServicesDependingOn was called
	serviceMgr.AssertCalled(t, "StartServicesDependingOn", "k8s:teleport.giantswarm.io-lifecycle-mc-lifecycle-wc", mock.Anything)

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

// TestOrchestrator_ServiceStartupOrder tests that services are started in the correct dependency order
func TestOrchestrator_ServiceStartupOrder(t *testing.T) {
	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := &mockServiceManager{}
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
	var startOrder []string
	var startOrderMutex sync.Mutex

	serviceMgr.On("StartServicesWithDependencyOrder", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			configs := args.Get(0).([]managers.ManagedServiceConfig)
			startOrderMutex.Lock()
			for _, cfg := range configs {
				startOrder = append(startOrder, cfg.Label)
			}
			startOrderMutex.Unlock()
		}).
		Return(map[string]chan struct{}{}, []error{})

	// Health check expectations
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, mock.Anything).Return(
		k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil,
	).Maybe()

	serviceMgr.On("StopAllServices").Return().Maybe()
	reporter.On("ReportHealth", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Give it a moment to ensure services are started
	time.Sleep(50 * time.Millisecond)

	// Verify the dependency graph was built correctly
	depGraph := orch.GetDependencyGraph()
	assert.NotNil(t, depGraph)

	// Check k8s connection nodes exist
	mcNode := depGraph.Get("k8s:teleport.giantswarm.io-test-mc")
	assert.NotNil(t, mcNode, "MC k8s connection node should exist")
	wcNode := depGraph.Get("k8s:teleport.giantswarm.io-test-mc-test-wc")
	assert.NotNil(t, wcNode, "WC k8s connection node should exist")

	// Check port forward nodes and their dependencies
	pfMcNode := depGraph.Get("pf:mc-prometheus")
	assert.NotNil(t, pfMcNode, "MC prometheus port forward node should exist")
	assert.Contains(t, pfMcNode.DependsOn, dependency.NodeID("k8s:teleport.giantswarm.io-test-mc"),
		"MC port forward should depend on MC k8s connection")

	pfWcNode := depGraph.Get("pf:wc-grafana")
	assert.NotNil(t, pfWcNode, "WC grafana port forward node should exist")
	assert.Contains(t, pfWcNode.DependsOn, dependency.NodeID("k8s:teleport.giantswarm.io-test-mc-test-wc"),
		"WC port forward should depend on WC k8s connection")

	// Check MCP nodes and their dependencies
	mcpPromNode := depGraph.Get("mcp:prometheus")
	assert.NotNil(t, mcpPromNode, "Prometheus MCP node should exist")
	assert.Contains(t, mcpPromNode.DependsOn, dependency.NodeID("pf:mc-prometheus"),
		"Prometheus MCP should depend on its port forward")

	mcpGrafanaNode := depGraph.Get("mcp:grafana")
	assert.NotNil(t, mcpGrafanaNode, "Grafana MCP node should exist")
	assert.Contains(t, mcpGrafanaNode.DependsOn, dependency.NodeID("pf:wc-grafana"),
		"Grafana MCP should depend on its port forward")

	mcpK8sNode := depGraph.Get("mcp:kubernetes")
	assert.NotNil(t, mcpK8sNode, "Kubernetes MCP node should exist")
	assert.Contains(t, mcpK8sNode.DependsOn, dependency.NodeID("k8s:teleport.giantswarm.io-test-mc"),
		"Kubernetes MCP should depend on MC k8s connection")

	// Verify start order - this is what StartServicesWithDependencyOrder should have done
	startOrderMutex.Lock()
	actualOrder := make([]string, len(startOrder))
	copy(actualOrder, startOrder)
	startOrderMutex.Unlock()

	// The order should be: port forwards first, then MCPs
	// We should see all port forwards before any MCPs that depend on them
	t.Logf("Service start order: %v", actualOrder)

	// Find indices of services
	var mcPromPFIndex, wcGrafanaPFIndex, promMCPIndex, grafanaMCPIndex int
	foundMcPromPF, foundWcGrafanaPF, foundPromMCP, foundGrafanaMCP := false, false, false, false

	for i, svc := range actualOrder {
		switch svc {
		case "mc-prometheus":
			mcPromPFIndex = i
			foundMcPromPF = true
		case "wc-grafana":
			wcGrafanaPFIndex = i
			foundWcGrafanaPF = true
		case "prometheus":
			promMCPIndex = i
			foundPromMCP = true
		case "grafana":
			grafanaMCPIndex = i
			foundGrafanaMCP = true
		}
	}

	// Verify all expected services were started
	assert.True(t, foundMcPromPF, "mc-prometheus port forward should be started")
	assert.True(t, foundWcGrafanaPF, "wc-grafana port forward should be started")
	assert.True(t, foundPromMCP, "prometheus MCP should be started")
	assert.True(t, foundGrafanaMCP, "grafana MCP should be started")

	// Verify ordering: port forwards should start before their dependent MCPs
	if foundMcPromPF && foundPromMCP {
		assert.Less(t, mcPromPFIndex, promMCPIndex,
			"mc-prometheus port forward should start before prometheus MCP")
	}
	if foundWcGrafanaPF && foundGrafanaMCP {
		assert.Less(t, wcGrafanaPFIndex, grafanaMCPIndex,
			"wc-grafana port forward should start before grafana MCP")
	}

	// Stop orchestrator
	orch.Stop()
}

// TestOrchestrator_NoDependencyManagementInTUI verifies that dependency management
// is handled by the orchestrator and not duplicated in the TUI
func TestOrchestrator_NoDependencyManagementInTUI(t *testing.T) {
	// This test ensures that the orchestrator is responsible for all dependency management
	// The TUI should only display status and handle user input, not manage dependencies

	// Create mocks
	kubeMgr := &mockKubeManager{}
	serviceMgr := &mockServiceManager{}
	reporter := &mockReporter{}

	// Configure orchestrator with shorter health check interval for faster test
	cfg := Config{
		MCName:              "test-mc",
		HealthCheckInterval: 50 * time.Millisecond, // Faster for testing
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

	// Create orchestrator
	orch := New(kubeMgr, serviceMgr, reporter, cfg)

	// Set up expectations
	kubeMgr.On("BuildMcContextName", "test-mc").Return("teleport.giantswarm.io-test-mc")

	// Track if StopServiceWithDependents is called (orchestrator should handle this)
	stopWithDependentsCalled := false
	serviceMgr.On("StopServiceWithDependents", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			stopWithDependentsCalled = true
			t.Logf("StopServiceWithDependents called with: %v", args.Get(0))
		}).
		Return(nil).Maybe()

	// Track if StartServicesDependingOn is called (orchestrator should handle this)
	startDependingOnCalled := false
	serviceMgr.On("StartServicesDependingOn", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			startDependingOnCalled = true
			t.Logf("StartServicesDependingOn called with: %v", args.Get(0))
		}).
		Return(nil).Maybe()

	serviceMgr.On("StartServicesWithDependencyOrder", mock.Anything, mock.Anything, mock.Anything).
		Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Set up health check sequence:
	// 1. First check: healthy
	// 2. Second check: unhealthy (trigger stop)
	// 3. Third check: healthy again (trigger start)
	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-test-mc").
		Return(k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil).
		Once() // First call - healthy

	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-test-mc").
		Return(k8smanager.NodeHealth{}, fmt.Errorf("connection lost")).
		Once() // Second call - unhealthy

	kubeMgr.On("GetClusterNodeHealth", mock.Anything, "teleport.giantswarm.io-test-mc").
		Return(k8smanager.NodeHealth{ReadyNodes: 3, TotalNodes: 3}, nil).
		Maybe() // Third+ calls - healthy again

	reporter.On("ReportHealth", mock.Anything).Return().Maybe()

	// Start orchestrator
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orch.Start(ctx)
	assert.NoError(t, err)

	// Wait for initial health check (healthy)
	time.Sleep(60 * time.Millisecond)

	// Wait for second health check (unhealthy - should trigger stop)
	time.Sleep(60 * time.Millisecond)

	// At this point, StopServiceWithDependents should have been called
	assert.True(t, stopWithDependentsCalled,
		"StopServiceWithDependents should be called when connection becomes unhealthy")

	// Wait for third health check (healthy again - should trigger start)
	time.Sleep(60 * time.Millisecond)

	// At this point, StartServicesDependingOn should have been called
	assert.True(t, startDependingOnCalled,
		"StartServicesDependingOn should be called when connection becomes healthy again")

	// The TUI should NOT be doing any of this dependency management
	// It should only:
	// 1. Display service status from the reporter
	// 2. Handle user input (start/stop/restart commands)
	// 3. Pass commands to the orchestrator (not service manager directly)
	// The orchestrator handles all the dependency logic

	// Stop orchestrator
	orch.Stop()
}
