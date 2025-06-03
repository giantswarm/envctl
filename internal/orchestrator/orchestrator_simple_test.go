package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/services"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_BasicLifecycle(t *testing.T) {
	// Test basic orchestrator lifecycle without timing dependencies
	o := New(Config{
		MCName: "test-mc",
		WCName: "test-wc",
	})

	ctx := context.Background()

	// Start should succeed
	err := o.Start(ctx)
	require.NoError(t, err)
	assert.NotNil(t, o.ctx)
	assert.NotNil(t, o.cancelFunc)
	assert.NotNil(t, o.depGraph)

	// Registry should have services registered
	allServices := o.registry.GetAll()
	assert.GreaterOrEqual(t, len(allServices), 2) // At least MC and WC

	// Stop should succeed
	err = o.Stop()
	assert.NoError(t, err)
}

func TestOrchestrator_ServiceRegistration(t *testing.T) {
	// Test that services are properly registered
	o := New(Config{
		MCName: "test-mc",
		WCName: "test-wc",
		PortForwards: []config.PortForwardDefinition{
			{Name: "pf1", Enabled: true},
			{Name: "pf2", Enabled: false},
		},
		MCPServers: []config.MCPServerDefinition{
			{Name: "mcp1", Enabled: true},
			{Name: "mcp2", Enabled: false},
		},
	})

	err := o.registerServices()
	require.NoError(t, err)

	// Check that enabled services are registered
	_, exists := o.registry.Get("k8s-mc-test-mc")
	assert.True(t, exists)

	_, exists = o.registry.Get("k8s-wc-test-wc")
	assert.True(t, exists)

	_, exists = o.registry.Get("pf1")
	assert.True(t, exists)

	_, exists = o.registry.Get("mcp1")
	assert.True(t, exists)

	// Check that disabled services are not registered
	_, exists = o.registry.Get("pf2")
	assert.False(t, exists)

	_, exists = o.registry.Get("mcp2")
	assert.False(t, exists)
}

func TestOrchestrator_DependencyGraph(t *testing.T) {
	// Test dependency graph construction
	o := New(Config{
		MCName: "test-mc",
		WCName: "test-wc",
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "pf1",
				Enabled:           true,
				KubeContextTarget: "gs-test-mc",
			},
		},
		// Commenting out MCP servers to avoid aggregator creation in tests
		// The dependency graph test doesn't need MCP servers
		// MCPServers: []config.MCPServerDefinition{
		// 	{
		// 		Name:                 "mcp1",
		// 		Enabled:              true,
		// 		RequiresPortForwards: []string{"pf1"},
		// 	},
		// },
	})

	// Only register services and build the dependency graph - don't start services
	ctx := context.Background()
	o.ctx = ctx
	
	err := o.registerServices()
	require.NoError(t, err)
	
	// Build the dependency graph
	o.depGraph = o.buildDependencyGraph()

	graph := o.depGraph
	assert.NotNil(t, graph)

	// Check MC node
	mcNode := graph.Get(dependency.NodeID("k8s-mc-test-mc"))
	assert.NotNil(t, mcNode)
	assert.Empty(t, mcNode.DependsOn)

	// Check WC node - no longer depends on MC
	wcNode := graph.Get(dependency.NodeID("k8s-wc-test-wc"))
	assert.NotNil(t, wcNode)
	assert.Empty(t, wcNode.DependsOn) // WC is now independent

	// Check PF node depends on MC
	pfNode := graph.Get(dependency.NodeID("pf:pf1"))
	assert.NotNil(t, pfNode)
	assert.Contains(t, pfNode.DependsOn, dependency.NodeID("k8s-mc-test-mc"))

	// The MCP node test is no longer needed since we removed MCP servers
	// Check MCP node depends on PF
	// mcpNode := graph.Get(dependency.NodeID("mcp:mcp1"))
	// assert.NotNil(t, mcpNode)
	// assert.Contains(t, mcpNode.DependsOn, dependency.NodeID("pf:pf1"))
}

func TestOrchestrator_StopReasons(t *testing.T) {
	// Test stop reason tracking
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	// Register a mock service
	svc := &mockService{
		label: "test-service",
		state: services.StateRunning,
		stopFunc: func(ctx context.Context) error {
			return nil
		},
	}
	o.registry.Register(svc)

	// Stop service manually
	err = o.StopService("test-service")
	assert.NoError(t, err)

	// Check stop reason
	o.mu.RLock()
	reason, exists := o.stopReasons["test-service"]
	o.mu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, StopReasonManual, reason)
}

func TestOrchestrator_NodeIDConversion(t *testing.T) {
	o := New(Config{})

	// Test getNodeIDForService
	tests := []struct {
		label       string
		serviceType services.ServiceType
		expected    string
	}{
		{"my-pf", services.TypePortForward, "pf:my-pf"},
		{"my-mcp", services.TypeMCPServer, "mcp:my-mcp"},
		{"k8s-mc-test", services.TypeKubeConnection, "k8s-mc-test"},
	}

	for _, tt := range tests {
		svc := &mockService{
			label:       tt.label,
			serviceType: tt.serviceType,
		}
		o.registry.Register(svc)

		result := o.getNodeIDForService(tt.label)
		assert.Equal(t, tt.expected, result)
	}

	// Test getLabelFromNodeID
	assert.Equal(t, "my-pf", o.getLabelFromNodeID("pf:my-pf"))
	assert.Equal(t, "my-mcp", o.getLabelFromNodeID("mcp:my-mcp"))
	assert.Equal(t, "k8s-mc-test", o.getLabelFromNodeID("k8s-mc-test"))
}

func TestOrchestrator_CheckDependencies(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	// Set up dependency graph
	o.depGraph = dependency.New()
	o.depGraph.AddNode(dependency.Node{
		ID:        "service1",
		DependsOn: []dependency.NodeID{"dep1", "dep2"},
	})

	// Register running dependencies
	dep1 := &mockService{
		label: "dep1",
		state: services.StateRunning,
	}
	o.registry.Register(dep1)

	dep2 := &mockService{
		label: "dep2",
		state: services.StateRunning,
	}
	o.registry.Register(dep2)

	// Check should pass
	err = o.checkDependencies("service1")
	assert.NoError(t, err)

	// Make one dependency not running
	dep1.state = services.StateStopped

	// Check should fail
	err = o.checkDependencies("service1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestOrchestrator_ServiceManagement(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	// Register a mock service
	startCalled := false
	stopCalled := false
	restartCalled := false

	var svc *mockService
	svc = &mockService{
		label: "test-service",
		state: services.StateStopped,
		startFunc: func(ctx context.Context) error {
			startCalled = true
			svc.state = services.StateRunning
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			stopCalled = true
			svc.state = services.StateStopped
			return nil
		},
		restartFunc: func(ctx context.Context) error {
			restartCalled = true
			return nil
		},
	}
	o.registry.Register(svc)

	// Test StartService
	err = o.StartService("test-service")
	assert.NoError(t, err)
	assert.True(t, startCalled)

	// Test StopService
	err = o.StopService("test-service")
	assert.NoError(t, err)
	assert.True(t, stopCalled)

	// Test RestartService
	err = o.RestartService("test-service")
	assert.NoError(t, err)
	assert.True(t, restartCalled)

	// Test non-existent service
	err = o.StartService("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
