package managers

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/reporting"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing
type mockHealthChecker struct {
	mock.Mock
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockStateStore struct {
	mock.Mock
	states map[string]reporting.ServiceStateSnapshot
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		states: make(map[string]reporting.ServiceStateSnapshot),
	}
}

func (m *mockStateStore) GetServiceState(label string) (reporting.ServiceStateSnapshot, bool) {
	if snapshot, exists := m.states[label]; exists {
		return snapshot, true
	}
	return reporting.ServiceStateSnapshot{}, false
}

func (m *mockStateStore) SetServiceState(update reporting.ManagedServiceUpdate) (bool, error) {
	snapshot := reporting.ServiceStateSnapshot{
		Label:       update.SourceLabel,
		SourceType:  update.SourceType,
		State:       update.State,
		IsReady:     update.IsReady,
		ErrorDetail: update.ErrorDetail,
		ProxyPort:   update.ProxyPort,
		PID:         update.PID,
		LastUpdated: update.Timestamp,
	}
	m.states[update.SourceLabel] = snapshot
	return true, nil
}

func (m *mockStateStore) GetAllServiceStates() map[string]reporting.ServiceStateSnapshot {
	return m.states
}

func (m *mockStateStore) GetServicesByType(serviceType reporting.ServiceType) map[string]reporting.ServiceStateSnapshot {
	results := make(map[string]reporting.ServiceStateSnapshot)
	for label, snapshot := range m.states {
		if snapshot.SourceType == serviceType {
			results[label] = snapshot
		}
	}
	return results
}

func (m *mockStateStore) GetServicesByState(state reporting.ServiceState) map[string]reporting.ServiceStateSnapshot {
	results := make(map[string]reporting.ServiceStateSnapshot)
	for label, snapshot := range m.states {
		if snapshot.State == state {
			results[label] = snapshot
		}
	}
	return results
}

func (m *mockStateStore) Subscribe(label string) *reporting.StateSubscription {
	return &reporting.StateSubscription{
		ID:      "test-sub",
		Label:   label,
		Channel: make(chan reporting.StateChangeEvent, 100),
	}
}

func (m *mockStateStore) Unsubscribe(subscription *reporting.StateSubscription) {}

func (m *mockStateStore) Clear(label string) bool {
	delete(m.states, label)
	return true
}

func (m *mockStateStore) ClearAll() {
	m.states = make(map[string]reporting.ServiceStateSnapshot)
}

func (m *mockStateStore) GetMetrics() reporting.StateStoreMetrics {
	return reporting.StateStoreMetrics{}
}

func (m *mockStateStore) RecordStateTransition(transition reporting.StateTransition) error {
	return nil
}

func (m *mockStateStore) GetStateTransitions(label string) []reporting.StateTransition {
	return nil
}

func (m *mockStateStore) GetAllStateTransitions() []reporting.StateTransition {
	return nil
}

func (m *mockStateStore) RecordCascadeOperation(cascade reporting.CascadeInfo) error {
	return nil
}

func (m *mockStateStore) GetCascadeOperations() []reporting.CascadeInfo {
	return nil
}

func (m *mockStateStore) GetCascadesByCorrelationID(correlationID string) []reporting.CascadeInfo {
	return nil
}

type mockReconcilerReporter struct {
	mock.Mock
	stateStore *mockStateStore
	updates    []reporting.ManagedServiceUpdate
}

func newMockReconcilerReporter() *mockReconcilerReporter {
	return &mockReconcilerReporter{
		stateStore: newMockStateStore(),
		updates:    []reporting.ManagedServiceUpdate{},
	}
}

func (m *mockReconcilerReporter) Report(update reporting.ManagedServiceUpdate) {
	m.Called(update)
	m.updates = append(m.updates, update)

	// Update state store
	m.stateStore.SetServiceState(update)
}

func (m *mockReconcilerReporter) GetStateStore() reporting.StateStore {
	return m.stateStore
}

// TestReconcilerLifecycle tests basic start/stop functionality
func TestReconcilerLifecycle(t *testing.T) {
	reporter := newMockReconcilerReporter()
	reporter.On("Report", mock.Anything).Maybe()

	manager := &ServiceManager{
		activeServices: make(map[string]chan struct{}),
		serviceConfigs: make(map[string]ManagedServiceConfig),
		reporter:       reporter,
		mu:             sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Test starting with a cancelled context to avoid the monitoring goroutine
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := reconciler.StartHealthMonitoring(ctx)
	assert.NoError(t, err)

	// Wait for the goroutine to exit properly
	reconciler.StopHealthMonitoring()

	// Test double start after stop
	ctx2, cancel2 := context.WithCancel(context.Background())

	err = reconciler.StartHealthMonitoring(ctx2)
	assert.NoError(t, err)

	// Test double start while running
	err = reconciler.StartHealthMonitoring(ctx2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already started")

	// Clean up properly
	cancel2()
	reconciler.StopHealthMonitoring()
}

// TestHealthCheckInterval tests setting custom health check intervals
func TestHealthCheckInterval(t *testing.T) {
	reporter := newMockReconcilerReporter()
	reporter.On("Report", mock.Anything).Maybe()

	manager := &ServiceManager{
		activeServices: make(map[string]chan struct{}),
		serviceConfigs: make(map[string]ManagedServiceConfig),
		reporter:       reporter,
		mu:             sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Test default interval
	assert.Equal(t, 30*time.Second, reconciler.healthCheckInterval)

	// Test setting custom interval
	reconciler.SetHealthCheckInterval(10 * time.Second)
	assert.Equal(t, 10*time.Second, reconciler.healthCheckInterval)
}

// TestServiceSpecificHealthCheckers tests health checker creation for each service type
func TestServiceSpecificHealthCheckers(t *testing.T) {
	reporter := newMockReconcilerReporter()
	reporter.On("Report", mock.Anything).Maybe()

	// Set up initial states for MCP server
	reporter.stateStore.SetServiceState(reporting.ManagedServiceUpdate{
		SourceType:  reporting.ServiceTypeMCPServer,
		SourceLabel: "test-mcp",
		State:       reporting.StateRunning,
		ProxyPort:   8080,
		Timestamp:   time.Now(),
	})

	manager := &ServiceManager{
		activeServices: make(map[string]chan struct{}),
		serviceConfigs: map[string]ManagedServiceConfig{
			"test-k8s": {
				Type:  reporting.ServiceTypeKube,
				Label: "test-k8s",
				Config: K8sConnectionConfig{
					Name:                "test-k8s",
					ContextName:         "test-context",
					HealthCheckInterval: 5 * time.Second,
				},
			},
			"test-pf": {
				Type:  reporting.ServiceTypePortForward,
				Label: "test-pf",
				Config: config.PortForwardDefinition{
					Name:                "test-pf",
					LocalPort:           "8080",
					RemotePort:          "80",
					HealthCheckInterval: 10 * time.Second,
				},
			},
			"test-mcp": {
				Type:  reporting.ServiceTypeMCPServer,
				Label: "test-mcp",
				Config: config.MCPServerDefinition{
					Name:                "test-mcp",
					HealthCheckInterval: 15 * time.Second,
				},
			},
		},
		reporter: reporter,
		mu:       sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Test K8s health checker creation
	err := reconciler.createHealthChecker("test-k8s")
	assert.NoError(t, err)
	assert.NotNil(t, reconciler.healthCheckers["test-k8s"])
	assert.IsType(t, &k8sConnectionHealthChecker{}, reconciler.healthCheckers["test-k8s"])
	assert.Equal(t, 5*time.Second, reconciler.serviceIntervals["test-k8s"])

	// Test Port Forward health checker creation
	err = reconciler.createHealthChecker("test-pf")
	assert.NoError(t, err)
	assert.NotNil(t, reconciler.healthCheckers["test-pf"])
	assert.IsType(t, &portForwardHealthChecker{}, reconciler.healthCheckers["test-pf"])
	assert.Equal(t, 10*time.Second, reconciler.serviceIntervals["test-pf"])

	// Test MCP Server health checker creation
	err = reconciler.createHealthChecker("test-mcp")
	assert.NoError(t, err)
	assert.NotNil(t, reconciler.healthCheckers["test-mcp"])
	assert.IsType(t, &mcpServerHealthChecker{}, reconciler.healthCheckers["test-mcp"])
	assert.Equal(t, 15*time.Second, reconciler.serviceIntervals["test-mcp"])

	// Test unknown service
	err = reconciler.createHealthChecker("unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service configuration not found")
}

// TestIsReadyConsistency tests that IsReady is tracked consistently across service types
func TestIsReadyConsistency(t *testing.T) {
	reporter := newMockReconcilerReporter()
	reporter.On("Report", mock.Anything).Maybe()

	// Set up initial states
	reporter.stateStore.SetServiceState(reporting.ManagedServiceUpdate{
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "test-k8s",
		State:       reporting.StateRunning,
		IsReady:     false, // Not ready yet
		Timestamp:   time.Now(),
	})

	reporter.stateStore.SetServiceState(reporting.ManagedServiceUpdate{
		SourceType:  reporting.ServiceTypePortForward,
		SourceLabel: "test-pf",
		State:       reporting.StateRunning,
		IsReady:     true, // Already ready from callback
		Timestamp:   time.Now(),
	})

	reporter.stateStore.SetServiceState(reporting.ManagedServiceUpdate{
		SourceType:  reporting.ServiceTypeMCPServer,
		SourceLabel: "test-mcp",
		State:       reporting.StateRunning,
		IsReady:     false, // Not ready yet
		ProxyPort:   8080,
		Timestamp:   time.Now(),
	})

	manager := &ServiceManager{
		activeServices: map[string]chan struct{}{
			"test-k8s": make(chan struct{}),
			"test-pf":  make(chan struct{}),
			"test-mcp": make(chan struct{}),
		},
		serviceConfigs: map[string]ManagedServiceConfig{
			"test-k8s": {
				Type:  reporting.ServiceTypeKube,
				Label: "test-k8s",
				Config: K8sConnectionConfig{
					Name:        "test-k8s",
					ContextName: "test-context",
				},
			},
			"test-pf": {
				Type:  reporting.ServiceTypePortForward,
				Label: "test-pf",
				Config: config.PortForwardDefinition{
					Name:       "test-pf",
					LocalPort:  "8080",
					RemotePort: "80",
				},
			},
			"test-mcp": {
				Type:  reporting.ServiceTypeMCPServer,
				Label: "test-mcp",
				Config: config.MCPServerDefinition{
					Name: "test-mcp",
				},
			},
		},
		reporter: reporter,
		mu:       sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Create mock health checkers
	k8sChecker := &mockHealthChecker{}
	pfChecker := &mockHealthChecker{}
	mcpChecker := &mockHealthChecker{}

	reconciler.healthCheckers["test-k8s"] = k8sChecker
	reconciler.healthCheckers["test-pf"] = pfChecker
	reconciler.healthCheckers["test-mcp"] = mcpChecker

	// Test successful health checks - should set IsReady to true
	ctx := context.Background()

	// K8s health check passes
	k8sChecker.On("CheckHealth", mock.Anything).Return(nil).Once()
	err := reconciler.CheckServiceHealth(ctx, "test-k8s")
	assert.NoError(t, err)

	// Verify IsReady was updated
	time.Sleep(10 * time.Millisecond) // Allow for async update
	snapshot, exists := reporter.stateStore.GetServiceState("test-k8s")
	assert.True(t, exists)
	assert.True(t, snapshot.IsReady, "K8s service should be ready after successful health check")

	// Port forward already ready, health check should maintain it
	pfChecker.On("CheckHealth", mock.Anything).Return(nil).Once()
	err = reconciler.CheckServiceHealth(ctx, "test-pf")
	assert.NoError(t, err)

	// MCP health check passes
	mcpChecker.On("CheckHealth", mock.Anything).Return(nil).Once()
	err = reconciler.CheckServiceHealth(ctx, "test-mcp")
	assert.NoError(t, err)

	// Verify IsReady was updated
	time.Sleep(10 * time.Millisecond) // Allow for async update
	snapshot, exists = reporter.stateStore.GetServiceState("test-mcp")
	assert.True(t, exists)
	assert.True(t, snapshot.IsReady, "MCP service should be ready after successful health check")

	// Test failed health checks - should set IsReady to false
	k8sChecker.On("CheckHealth", mock.Anything).Return(assert.AnError).Once()
	err = reconciler.CheckServiceHealth(ctx, "test-k8s")
	assert.Error(t, err)

	// Verify IsReady was updated to false
	time.Sleep(10 * time.Millisecond) // Allow for async update
	snapshot, exists = reporter.stateStore.GetServiceState("test-k8s")
	assert.True(t, exists)
	assert.False(t, snapshot.IsReady, "K8s service should not be ready after failed health check")

	// Verify all mock expectations were met
	k8sChecker.AssertExpectations(t)
	pfChecker.AssertExpectations(t)
	mcpChecker.AssertExpectations(t)
}

// TestHealthStatusReporting tests that health changes are only reported when IsReady changes
func TestHealthStatusReporting(t *testing.T) {
	reporter := newMockReconcilerReporter()

	// Set up initial state
	reporter.stateStore.SetServiceState(reporting.ManagedServiceUpdate{
		SourceType:  reporting.ServiceTypeMCPServer,
		SourceLabel: "test-service",
		State:       reporting.StateRunning,
		IsReady:     true,
		ProxyPort:   8080,
		Timestamp:   time.Now(),
	})

	manager := &ServiceManager{
		activeServices: map[string]chan struct{}{
			"test-service": make(chan struct{}),
		},
		serviceConfigs: map[string]ManagedServiceConfig{
			"test-service": {
				Type:  reporting.ServiceTypeMCPServer,
				Label: "test-service",
				Config: config.MCPServerDefinition{
					Name: "test-service",
				},
			},
		},
		reporter: reporter,
		mu:       sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Create mock health checker
	checker := &mockHealthChecker{}
	reconciler.healthCheckers["test-service"] = checker

	ctx := context.Background()

	// First health check passes - no report since IsReady is already true
	checker.On("CheckHealth", mock.Anything).Return(nil).Once()

	err := reconciler.CheckServiceHealth(ctx, "test-service")
	assert.NoError(t, err)

	// Verify no new reports (IsReady didn't change)
	time.Sleep(10 * time.Millisecond) // Allow for any async processing
	assert.Equal(t, 0, len(reporter.updates))

	// Second health check fails - should report since IsReady changes
	checker.On("CheckHealth", mock.Anything).Return(assert.AnError).Once()
	reporter.On("Report", mock.MatchedBy(func(update reporting.ManagedServiceUpdate) bool {
		return update.SourceLabel == "test-service" &&
			update.IsReady == false &&
			update.State == reporting.StateRunning && // State should remain Running
			update.CausedBy == "health_check"
	})).Once()

	err = reconciler.CheckServiceHealth(ctx, "test-service")
	assert.Error(t, err)

	// Give time for the report to be processed
	time.Sleep(10 * time.Millisecond)

	// Third health check still fails - no report since IsReady is already false
	checker.On("CheckHealth", mock.Anything).Return(assert.AnError).Once()

	err = reconciler.CheckServiceHealth(ctx, "test-service")
	assert.Error(t, err)

	// Verify expectations
	checker.AssertExpectations(t)
	reporter.AssertExpectations(t)
}

// TestCleanupOnServiceStop tests that health checkers are cleaned up when services stop
func TestCleanupOnServiceStop(t *testing.T) {
	reporter := newMockReconcilerReporter()
	reporter.On("Report", mock.Anything).Maybe()

	manager := &ServiceManager{
		activeServices: make(map[string]chan struct{}),
		serviceConfigs: make(map[string]ManagedServiceConfig),
		reporter:       reporter,
		mu:             sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Add some health checkers and status
	reconciler.healthCheckers["service1"] = &mockHealthChecker{}
	reconciler.healthCheckers["service2"] = &mockHealthChecker{}
	reconciler.healthStatus["service1"] = &healthStatusEntry{isHealthy: true}
	reconciler.healthStatus["service2"] = &healthStatusEntry{isHealthy: false}
	reconciler.serviceIntervals["service1"] = 10 * time.Second
	reconciler.serviceIntervals["service2"] = 20 * time.Second

	// Clean up service1
	reconciler.cleanupHealthChecker("service1")

	// Verify service1 was cleaned up
	assert.Nil(t, reconciler.healthCheckers["service1"])
	assert.Nil(t, reconciler.healthStatus["service1"])
	assert.Equal(t, time.Duration(0), reconciler.serviceIntervals["service1"])

	// Verify service2 is still there
	assert.NotNil(t, reconciler.healthCheckers["service2"])
	assert.NotNil(t, reconciler.healthStatus["service2"])
	assert.Equal(t, 20*time.Second, reconciler.serviceIntervals["service2"])
}

// TestReconcilerMinimal tests the reconciler without starting monitoring
func TestReconcilerMinimal(t *testing.T) {
	reporter := newMockReconcilerReporter()
	reporter.On("Report", mock.Anything).Maybe()

	manager := &ServiceManager{
		activeServices: make(map[string]chan struct{}),
		serviceConfigs: make(map[string]ManagedServiceConfig),
		reporter:       reporter,
		mu:             sync.Mutex{},
	}

	reconciler := newServiceReconciler(manager)

	// Test that we can call GetActiveServices
	services := manager.GetActiveServices()
	assert.NotNil(t, services)
	assert.Equal(t, 0, len(services))

	// Test health check interval
	assert.Equal(t, 30*time.Second, reconciler.healthCheckInterval)
}
