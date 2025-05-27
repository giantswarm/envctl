package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
