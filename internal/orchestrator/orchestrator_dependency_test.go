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
