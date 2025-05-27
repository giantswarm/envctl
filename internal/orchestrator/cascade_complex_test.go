package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// TestComplexCascadeScenario tests a complex cascade scenario with multiple levels
func TestComplexCascadeScenario(t *testing.T) {
	// Create mocks
	serviceMgr := newMockServiceManager()
	reporter := &mockReporter{}

	// Set up mock expectations
	serviceMgr.On("SetReporter", mock.Anything).Return()
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Return(map[string]chan struct{}{}, []error{}).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Add reporter expectations
	reporter.On("Report", mock.Anything).Return().Maybe()
	reporter.On("GetStateStore").Return(reporting.NewStateStore()).Maybe()

	// Configure complex scenario
	cfg := Config{
		MCName: "test-mc",
		WCName: "test-wc",
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "mc-prometheus",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-test-mc",
			},
			{
				Name:              "mc-grafana",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-test-mc",
			},
			{
				Name:              "wc-alloy",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-test-mc-test-wc",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:                 "prometheus",
				Enabled:              true,
				RequiresPortForwards: []string{"mc-prometheus"},
			},
			{
				Name:                 "grafana",
				Enabled:              true,
				RequiresPortForwards: []string{"mc-grafana"},
			},
		},
	}

	// Create orchestrator
	orch := New(serviceMgr, reporter, cfg)

	// Build the dependency graph (normally done in Start())
	orch.depGraph = orch.buildDependencyGraph()

	// Verify dependency graph structure
	depGraph := orch.GetDependencyGraph()
	if depGraph == nil {
		t.Fatal("Dependency graph should not be nil")
	}

	// Check MC dependencies
	mcDependents := depGraph.Dependents("k8s-mc-test-mc")
	expectedMCDependents := map[string]bool{
		"pf:mc-prometheus": true,
		"pf:mc-grafana":    true,
	}

	for _, dep := range mcDependents {
		if !expectedMCDependents[string(dep)] {
			t.Errorf("Unexpected MC dependent: %s", dep)
		}
		delete(expectedMCDependents, string(dep))
	}

	if len(expectedMCDependents) > 0 {
		t.Errorf("Missing MC dependents: %v", expectedMCDependents)
	}

	// Check port forward dependencies
	promDependents := depGraph.Dependents("pf:mc-prometheus")
	if len(promDependents) != 1 || promDependents[0] != "mcp:prometheus" {
		t.Errorf("Expected prometheus MCP to depend on mc-prometheus port forward, got %v", promDependents)
	}

	// Clean up
	orch.Stop()
}

// TestCascadeWithCorrelationTracking tests that cascade operations are properly tracked with correlation IDs
func TestCascadeWithCorrelationTracking(t *testing.T) {
	// Create mocks with state store
	stateStore := reporting.NewStateStore()
	reporter := &mockReporterWithStore{
		stateStore: stateStore,
	}
	serviceMgr := newMockServiceManager()

	// Define correlation ID early so it can be used in closures
	correlationID := "test-correlation-123"

	// Set up mock expectations
	serviceMgr.On("SetReporter", mock.Anything).Return()
	serviceMgr.On("StartServices", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		configs := args.Get(0).([]managers.ManagedServiceConfig)
		// Mark services as started in the mock
		serviceMgr.mu.Lock()
		for _, cfg := range configs {
			serviceMgr.activeServices[cfg.Label] = true
		}
		serviceMgr.mu.Unlock()
	}).Return(map[string]chan struct{}{}, []error{})
	serviceMgr.On("StopService", mock.Anything).Run(func(args mock.Arguments) {
		label := args.String(0)
		// Mark service as stopped
		serviceMgr.mu.Lock()
		delete(serviceMgr.activeServices, label)
		serviceMgr.mu.Unlock()
	}).Return(nil).Maybe()
	serviceMgr.On("StopAllServices").Return().Maybe()

	// Configure test scenario
	cfg := Config{
		MCName: "test-mc",
		PortForwards: []config.PortForwardDefinition{
			{
				Name:              "mc-prometheus",
				Enabled:           true,
				KubeContextTarget: "teleport.giantswarm.io-test-mc",
			},
		},
		MCPServers: []config.MCPServerDefinition{
			{
				Name:                 "prometheus",
				Enabled:              true,
				RequiresPortForwards: []string{"mc-prometheus"},
			},
		},
	}

	// Create orchestrator
	orch := New(serviceMgr, reporter, cfg)
	ctx := context.Background()

	// Start orchestrator
	err := orch.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	// Wait for services to start
	time.Sleep(100 * time.Millisecond)

	// Simulate K8s failure to trigger cascade
	nodeID := "k8s-mc-test-mc"

	// We need to use the proper method that will record cascade info
	// Since stopServiceWithDependentsCorrelated checks for reporter.GetStateStore()
	// and our reporter implements it, it should record the cascade
	err = orch.stopServiceWithDependentsCorrelated(nodeID, "test_failure", correlationID)
	if err != nil {
		t.Errorf("Failed to stop service with dependents: %v", err)
	}

	// Wait for cascade to complete
	time.Sleep(100 * time.Millisecond)

	// Verify cascade was recorded in state store
	// Check cascade operations instead of service states
	cascadeOps := stateStore.GetCascadesByCorrelationID(correlationID)
	t.Logf("Found %d cascade operations with correlation ID %s", len(cascadeOps), correlationID)

	if len(cascadeOps) == 0 {
		// Log all cascade operations for debugging
		allCascades := stateStore.GetCascadeOperations()
		t.Logf("All cascade operations: %d", len(allCascades))
		for i, cascade := range allCascades {
			t.Logf("Cascade %d: CorrelationID=%s, InitiatingService=%s, AffectedServices=%v",
				i, cascade.CorrelationID, cascade.InitiatingService, cascade.AffectedServices)
		}
		t.Error("Expected cascade operation to be recorded with correlation ID")
	} else {
		// Verify cascade details
		cascade := cascadeOps[0]
		if cascade.InitiatingService != nodeID {
			t.Errorf("Expected initiating service to be %s, got %s", nodeID, cascade.InitiatingService)
		}
		if cascade.Reason != "test_failure" {
			t.Errorf("Expected reason to be test_failure, got %s", cascade.Reason)
		}
		if cascade.CascadeType != reporting.CascadeTypeStop {
			t.Errorf("Expected cascade type to be Stop, got %v", cascade.CascadeType)
		}
		// Check that affected services include our port forward and MCP
		expectedAffected := map[string]bool{
			"pf:mc-prometheus": true,
			"mcp:prometheus":   true,
		}
		for _, affected := range cascade.AffectedServices {
			delete(expectedAffected, affected)
		}
		if len(expectedAffected) > 0 {
			t.Errorf("Missing affected services: %v", expectedAffected)
		}
	}

	// Clean up
	orch.Stop()
}

// mockReporterWithStore is a mock reporter that includes a state store
type mockReporterWithStore struct {
	stateStore reporting.StateStore
	mu         sync.Mutex
}

func (m *mockReporterWithStore) Report(update reporting.ManagedServiceUpdate) {
	// Record in state store
	if m.stateStore != nil {
		m.stateStore.SetServiceState(update)
	}
}

func (m *mockReporterWithStore) GetStateStore() reporting.StateStore {
	return m.stateStore
}
