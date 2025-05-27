package managers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
)

// setupMocks sets up the mocks for port forwarding and MCP server functions
func setupMocks(t *testing.T) func() {
	originalKubeStartFn := portforwarding.KubeStartPortForwardFn
	originalMCPServerStarter := mcpserver.StartMCPServers
	originalPFStarter := portforwarding.StartPortForwardings

	// Track active mock goroutines
	var mockGoroutines sync.WaitGroup

	// Mock the Kube start function
	portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext, namespace, serviceArg, portMap, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
		t.Logf("MOCK: kube.StartPortForward called for label: %s", label)
		stopChan := make(chan struct{})

		mockGoroutines.Add(1)
		go func() {
			defer mockGoroutines.Done()

			timer := time.NewTimer(10 * time.Millisecond)
			defer timer.Stop()

			select {
			case <-timer.C:
				if bridgeFn != nil {
					bridgeFn("ForwardingActive", "Mock PF Running", false, true)
				}
			case <-stopChan:
				// Service stopped
			}
		}()

		return stopChan, "Mock Kube Init", nil
	}

	// Mock StartPortForwardings
	portforwarding.StartPortForwardings = func(configs []config.PortForwardDefinition, pfUpdateFn portforwarding.PortForwardUpdateFunc, wg *sync.WaitGroup) map[string]chan struct{} {
		t.Logf("MOCK: StartPortForwardings with %d configs", len(configs))
		stopChans := make(map[string]chan struct{})

		for _, cfg := range configs {
			stopChan := make(chan struct{})
			stopChans[cfg.Name] = stopChan

			mockGoroutines.Add(1)
			go func(c config.PortForwardDefinition) {
				defer mockGoroutines.Done()

				if wg != nil {
					wg.Add(1)
					defer wg.Done()
				}

				// Report starting
				if pfUpdateFn != nil {
					pfUpdateFn(c.Name, portforwarding.StatusDetailInitializing, false, nil)

					timer := time.NewTimer(10 * time.Millisecond)
					defer timer.Stop()

					select {
					case <-timer.C:
						pfUpdateFn(c.Name, portforwarding.StatusDetailForwardingActive, true, nil)
					case <-stopChan:
						pfUpdateFn(c.Name, portforwarding.StatusDetailStopped, false, nil)
					}
				}
			}(cfg)
		}

		return stopChans
	}

	// Mock StartMCPServers
	mcpserver.StartMCPServers = func(configs []config.MCPServerDefinition, mcpUpdateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
		t.Logf("MOCK: StartMCPServers with %d configs", len(configs))
		stopChans := make(map[string]chan struct{})

		for _, cfg := range configs {
			stopChan := make(chan struct{})
			stopChans[cfg.Name] = stopChan

			mockGoroutines.Add(1)
			go func(c config.MCPServerDefinition) {
				defer mockGoroutines.Done()

				if wg != nil {
					wg.Add(1)
					defer wg.Done()
				}

				// Report starting
				if mcpUpdateFn != nil {
					mcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{
						Label:         c.Name,
						ProcessStatus: "ProcessInitializing",
						PID:           12345,
					})

					timer := time.NewTimer(10 * time.Millisecond)
					defer timer.Stop()

					select {
					case <-timer.C:
						mcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{
							Label:         c.Name,
							ProcessStatus: "ProcessRunning",
							PID:           12345,
						})
					case <-stopChan:
						mcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{
							Label:         c.Name,
							ProcessStatus: "ProcessStoppedByUser",
						})
					}
				}
			}(cfg)
		}

		return stopChans, nil
	}

	return func() {
		// Wait for all mock goroutines to finish
		done := make(chan struct{})
		go func() {
			mockGoroutines.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines finished
		case <-time.After(2 * time.Second):
			t.Errorf("Timeout waiting for mock goroutines to finish")
		}

		// Restore original functions
		portforwarding.KubeStartPortForwardFn = originalKubeStartFn
		mcpserver.StartMCPServers = originalMCPServerStarter
		portforwarding.StartPortForwardings = originalPFStarter
	}
}

// mockReporter is a mock implementation of ServiceReporter for testing
type mockReporter struct {
	mu         sync.Mutex
	updates    []reporting.ManagedServiceUpdate
	t          *testing.T
	stateStore reporting.StateStore
}

func newMockReporter(t *testing.T) *mockReporter {
	return &mockReporter{
		t:          t,
		stateStore: reporting.NewStateStore(),
	}
}

func (r *mockReporter) Report(update reporting.ManagedServiceUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, update)
	if r.t != nil {
		r.t.Logf("Reporter captured update: %+v", update)
	}

	// Update the state store
	if r.stateStore != nil {
		r.stateStore.SetServiceState(update)
	}
}

func (r *mockReporter) GetStateStore() reporting.StateStore {
	return r.stateStore
}

func (r *mockReporter) GetUpdates() []reporting.ManagedServiceUpdate {
	r.mu.Lock()
	defer r.mu.Unlock()
	copiedUpdates := make([]reporting.ManagedServiceUpdate, len(r.updates))
	copy(copiedUpdates, r.updates)
	return copiedUpdates
}

func (r *mockReporter) ClearUpdates() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = nil
}

// Test basic service manager creation
func TestServiceManager_New(t *testing.T) {
	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	assert.NotNil(t, sm, "ServiceManager should not be nil")

	// Test with nil reporter
	sm2 := NewServiceManager(nil)
	assert.NotNil(t, sm2, "ServiceManager should handle nil reporter")
}

// Test starting services
func TestServiceManager_StartServices(t *testing.T) {
	teardown := setupMocks(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	configs := []ManagedServiceConfig{
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf",
			Config: config.PortForwardDefinition{
				Name:       "test-pf",
				Enabled:    true,
				LocalPort:  "8080",
				RemotePort: "80",
				Namespace:  "default",
				TargetType: "service",
				TargetName: "test-service",
			},
		},
		{
			Type:  reporting.ServiceTypeMCPServer,
			Label: "test-mcp",
			Config: config.MCPServerDefinition{
				Name:    "test-mcp",
				Enabled: true,
				Type:    config.MCPServerTypeLocalCommand,
				Command: []string{"echo", "test"},
			},
		},
	}

	var wg sync.WaitGroup
	stopChans, errs := sm.StartServices(configs, &wg)

	assert.Empty(t, errs, "Should not have errors starting services")
	assert.Len(t, stopChans, 2, "Should have 2 stop channels")
	assert.Contains(t, stopChans, "test-pf")
	assert.Contains(t, stopChans, "test-mcp")

	// Wait for services to report running
	time.Sleep(50 * time.Millisecond)

	// Check that services are active
	assert.True(t, sm.IsServiceActive("test-pf"))
	assert.True(t, sm.IsServiceActive("test-mcp"))

	// Check that state updates were reported
	updates := reporter.GetUpdates()
	assert.Greater(t, len(updates), 2, "Should have received state updates")

	// Clean up
	sm.StopAllServices()
}

// Test stopping a specific service
func TestServiceManager_StopService(t *testing.T) {
	teardown := setupMocks(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	configs := []ManagedServiceConfig{
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf",
			Config: config.PortForwardDefinition{
				Name:    "test-pf",
				Enabled: true,
			},
		},
	}

	var wg sync.WaitGroup
	stopChans, _ := sm.StartServices(configs, &wg)

	// Wait for service to start
	time.Sleep(50 * time.Millisecond)

	// Stop the service
	err := sm.StopService("test-pf")
	assert.NoError(t, err, "Should not error when stopping service")

	// Verify channel was closed
	select {
	case <-stopChans["test-pf"]:
		// Good, channel is closed
	default:
		t.Error("Stop channel should be closed")
	}

	// Service should no longer be active
	assert.False(t, sm.IsServiceActive("test-pf"))

	// Test stopping non-existent service
	err = sm.StopService("non-existent")
	assert.Error(t, err, "Should error when stopping non-existent service")
}

// Test stopping all services
func TestServiceManager_StopAllServices(t *testing.T) {
	teardown := setupMocks(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	configs := []ManagedServiceConfig{
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf1",
			Config: config.PortForwardDefinition{
				Name:    "test-pf1",
				Enabled: true,
			},
		},
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf2",
			Config: config.PortForwardDefinition{
				Name:    "test-pf2",
				Enabled: true,
			},
		},
		{
			Type:  reporting.ServiceTypeMCPServer,
			Label: "test-mcp",
			Config: config.MCPServerDefinition{
				Name:    "test-mcp",
				Enabled: true,
			},
		},
	}

	var wg sync.WaitGroup
	_, _ = sm.StartServices(configs, &wg)

	// Wait for services to start
	time.Sleep(50 * time.Millisecond)

	// Stop all services
	sm.StopAllServices()

	// No services should be active
	assert.False(t, sm.IsServiceActive("test-pf1"))
	assert.False(t, sm.IsServiceActive("test-pf2"))
	assert.False(t, sm.IsServiceActive("test-mcp"))
	assert.Empty(t, sm.GetActiveServices())
}

// Test get service config
func TestServiceManager_GetServiceConfig(t *testing.T) {
	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	config := ManagedServiceConfig{
		Type:  reporting.ServiceTypePortForward,
		Label: "test-pf",
		Config: config.PortForwardDefinition{
			Name:    "test-pf",
			Enabled: true,
		},
	}

	var wg sync.WaitGroup
	_, _ = sm.StartServices([]ManagedServiceConfig{config}, &wg)

	// Get existing config
	retrievedConfig, exists := sm.GetServiceConfig("test-pf")
	assert.True(t, exists, "Config should exist")
	assert.Equal(t, config.Label, retrievedConfig.Label)
	assert.Equal(t, config.Type, retrievedConfig.Type)

	// Get non-existent config
	_, exists = sm.GetServiceConfig("non-existent")
	assert.False(t, exists, "Config should not exist")
}

// Test get active services
func TestServiceManager_GetActiveServices(t *testing.T) {
	teardown := setupMocks(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	// Initially no active services
	assert.Empty(t, sm.GetActiveServices())

	configs := []ManagedServiceConfig{
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf1",
			Config: config.PortForwardDefinition{
				Name:    "test-pf1",
				Enabled: true,
			},
		},
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf2",
			Config: config.PortForwardDefinition{
				Name:    "test-pf2",
				Enabled: true,
			},
		},
	}

	var wg sync.WaitGroup
	_, _ = sm.StartServices(configs, &wg)

	// Wait for services to start
	time.Sleep(50 * time.Millisecond)

	activeServices := sm.GetActiveServices()
	assert.Len(t, activeServices, 2, "Should have 2 active services")
	assert.Contains(t, activeServices, "test-pf1")
	assert.Contains(t, activeServices, "test-pf2")
}

// Test state change reporting
func TestServiceManager_StateChangeReporting(t *testing.T) {
	teardown := setupMocks(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	configs := []ManagedServiceConfig{
		{
			Type:  reporting.ServiceTypeMCPServer,
			Label: "test-mcp",
			Config: config.MCPServerDefinition{
				Name:    "test-mcp",
				Enabled: true,
			},
		},
	}

	var wg sync.WaitGroup
	_, _ = sm.StartServices(configs, &wg)

	// Wait for service to go through states
	time.Sleep(100 * time.Millisecond)

	updates := reporter.GetUpdates()
	assert.GreaterOrEqual(t, len(updates), 2, "Should have at least 2 updates (starting, running)")

	// Verify state transitions
	foundStarting := false
	foundRunning := false
	for _, update := range updates {
		if update.SourceLabel == "test-mcp" {
			if update.State == reporting.StateStarting {
				foundStarting = true
			}
			if update.State == reporting.StateRunning {
				foundRunning = true
			}
		}
	}

	assert.True(t, foundStarting, "Should have reported starting state")
	assert.True(t, foundRunning, "Should have reported running state")

	// Clean up
	sm.StopAllServices()
}

// Test empty service start
func TestServiceManager_StartServices_Empty(t *testing.T) {
	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter)

	var configs []ManagedServiceConfig
	var wg sync.WaitGroup

	stopChans, errs := sm.StartServices(configs, &wg)

	assert.Empty(t, errs, "Should not have errors with empty config")
	assert.Empty(t, stopChans, "Should not have stop channels with empty config")
}
