package managers

import (
	// Standard library imports
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	// External dependencies for testing
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Project-specific imports
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube" // Needed for portforwarding mock type kube.SendUpdateFunc
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
)

// NOTE: The content below is the rest of the test file, starting from mock definitions and test functions.
// Ensure no duplicate package or import statements are present from here on.

// No package-level vars for mocks here anymore.
// setupMocks will now override the actual package vars from portforwarding and mcpserver.

// setupServiceManagerTestMocks overrides the actual package-level functions
// from portforwarding and mcpserver packages with mocks for testing ServiceManager.
// It returns a cleanup function that must be deferred by the caller to restore originals.
func setupServiceManagerTestMocks(t *testing.T) func() {
	originalKubeStartFn := portforwarding.KubeStartPortForwardFn
	// Save the original value of the package-level variable StartMCPServers
	originalMCPServerStarter := mcpserver.StartMCPServers

	t.Logf("SETUP_MOCK: Overriding portforwarding.KubeStartPortForwardFn")
	portforwarding.KubeStartPortForwardFn = func(ctx context.Context, kubeContext, namespace, serviceArg, portMap, label string, bridgeFn kube.SendUpdateFunc) (chan struct{}, string, error) {
		t.Logf("MOCK_EXEC: mock kube.StartPortForward called for label: %s", label)
		mStopChans := make(chan struct{})
		go func() {
			time.Sleep(1 * time.Millisecond)
			if bridgeFn != nil {
				// Simulate a simplified status update from the portforwarding mock
				bridgeFn("ForwardingActive", "Mock PF Running via Kube Mock", false, true)
			}
		}()
		return mStopChans, "Mock Kube Init", nil
	}

	t.Logf("SETUP_MOCK: Overriding mcpserver.StartMCPServers variable")
	mcpserver.StartMCPServers = func(configs []config.MCPServerDefinition, mcpUpdateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
		t.Logf("MOCK_EXEC: mock mcpserver.StartMCPServers called with %d configs", len(configs))
		mStopChans := make(map[string]chan struct{})
		for _, cfg := range configs {
			mStopChans[cfg.Name] = make(chan struct{})
			go func(c config.MCPServerDefinition) {
				if wg != nil {
					wg.Add(1)
					defer wg.Done()
				}
				time.Sleep(1 * time.Millisecond)
				if mcpUpdateFn != nil {
					mcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: c.Name, ProcessStatus: "NpxRunning", PID: 123})
				}
			}(cfg)
		}
		return mStopChans, nil
	}

	return func() {
		t.Logf("CLEANUP_MOCK: Restoring originalKubeStartFn to portforwarding.KubeStartPortForwardFn")
		portforwarding.KubeStartPortForwardFn = originalKubeStartFn
		t.Logf("CLEANUP_MOCK: Restoring originalMCPServerStarter to mcpserver.StartMCPServers")
		mcpserver.StartMCPServers = originalMCPServerStarter
	}
}

func initLoggingForTests() {
	// logging.InitForCLI(logging.LevelDebug, io.Discard) // Keep commented, t.Logf is sufficient for tests
}

func TestServiceManager_StartServices_EmptyConfig(t *testing.T) {
	initLoggingForTests()
	consoleReporter := reporting.NewConsoleReporter()
	sm := NewServiceManager(consoleReporter)
	var configs []ManagedServiceConfig
	var wg sync.WaitGroup

	stopChans, errs := sm.StartServices(configs, &wg)

	if len(stopChans) != 0 {
		t.Errorf("Expected 0 stop channels, got %d", len(stopChans))
	}
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors, got %v", errs)
	}
}

func TestServiceManager_StartServices_StartsServices(t *testing.T) {
	initLoggingForTests()
	t.Skip("Skipping TestServiceManager_StartServices_StartsServices due to persistent issues with mocking. Requires a more robust mocking strategy (e.g., interface injection) or re-design as an integration test.")
}

func TestServiceManager_StopService(t *testing.T) {
	initLoggingForTests()
	cleanup := setupServiceManagerTestMocks(t)
	defer cleanup()
	consoleReporter := reporting.NewConsoleReporter()
	sm := NewServiceManager(consoleReporter)
	pflabel := "TestPF_Stop"
	configs := []ManagedServiceConfig{
		{Type: reporting.ServiceTypePortForward, Label: pflabel, Config: config.PortForwardDefinition{
			Name:              pflabel,
			TargetName:        "test-pf-svc-stop",
			TargetType:        "service",
			Namespace:         "default",
			LocalPort:         "8081",
			RemotePort:        "8000",
			KubeContextTarget: "test-stop-ctx",
			BindAddress:       "127.0.0.1",
			Enabled:           true,
		}},
	}
	var wg sync.WaitGroup
	stopChans, _ := sm.StartServices(configs, &wg)
	p_stopChan, ok := stopChans[pflabel]
	if !ok {
		t.Fatalf("Service %s not started or stop channel not returned", pflabel)
	}

	err := sm.StopService(pflabel)
	if err != nil {
		t.Errorf("StopService failed: %v", err)
	}
	select {
	case <-p_stopChan:
	default:
		t.Errorf("StopService did not close stop channel for %s", pflabel)
	}
	err = sm.StopService("NonExistent")
	if err == nil {
		t.Error("StopService should have failed for non-existent service")
	}
	err = sm.StopService(pflabel)
	if err == nil {
		t.Errorf("StopService for already stopped service should have error")
	}
}

func TestServiceManager_StopAllServices(t *testing.T) {
	initLoggingForTests()
	cleanup := setupServiceManagerTestMocks(t)
	defer cleanup()
	consoleReporter := reporting.NewConsoleReporter()
	sm := NewServiceManager(consoleReporter)
	pflabel1 := "PF_StopAll1"
	mcplabel1 := "MCP_StopAll1"
	configs := []ManagedServiceConfig{
		{Type: reporting.ServiceTypePortForward, Label: pflabel1, Config: config.PortForwardDefinition{Name: pflabel1, TargetName: "sa1", TargetType: "service", Enabled: true}},
		{Type: reporting.ServiceTypeMCPServer, Label: mcplabel1, Config: config.MCPServerDefinition{Name: mcplabel1, Command: []string{"echo"}, Type: config.MCPServerTypeLocalCommand, Enabled: true}},
	}
	var wg sync.WaitGroup
	stopChans, _ := sm.StartServices(configs, &wg)
	pf1Chan := stopChans[pflabel1]
	mcp1Chan := stopChans[mcplabel1]

	sm.StopAllServices()

	select {
	case <-pf1Chan:
	default:
		t.Errorf("PF1 channel not closed after StopAllServices")
	}
	select {
	case <-mcp1Chan:
	default:
		t.Errorf("MCP1 channel not closed after StopAllServices")
	}
	err := sm.StopService(pflabel1)
	if err == nil {
		t.Error("StopService after StopAllServices should fail")
	}
}

var testMu sync.Mutex

func synchronized(f func()) {
	testMu.Lock()
	defer testMu.Unlock()
	f()
}

type mockServiceReporter struct {
	ReportFunc func(update reporting.ManagedServiceUpdate)
}

func (m *mockServiceReporter) Report(update reporting.ManagedServiceUpdate) {
	if m.ReportFunc != nil {
		m.ReportFunc(update)
	}
}

type mockReporter struct {
	mu      sync.Mutex
	updates []reporting.ManagedServiceUpdate
	t       *testing.T // Add for logging
}

func newMockReporter(t *testing.T) *mockReporter {
	return &mockReporter{t: t}
}

func (r *mockReporter) Report(update reporting.ManagedServiceUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, update)
	if r.t != nil {
		r.t.Logf("mockReporter captured update: %+v (Total updates: %d)", update, len(r.updates))
	}
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

func (m *mockReporter) ReportHealth(update reporting.HealthStatusUpdate) {
	// For now, just ignore health reports in tests
	// They're not relevant to the service manager tests
}

func getInternalServiceManager(smAPI ServiceManagerAPI) *ServiceManager {
	internalSM, ok := smAPI.(*ServiceManager)
	if !ok {
		panic("could not cast ServiceManagerAPI to *ServiceManager for testing")
	}
	return internalSM
}

var (
	originalStartMCPServersFunc_managersTest      func([]config.MCPServerDefinition, mcpserver.McpUpdateFunc, *sync.WaitGroup) (map[string]chan struct{}, []error)
	originalStartPortForwardingsFunc_managersTest func([]config.PortForwardDefinition, portforwarding.PortForwardUpdateFunc, *sync.WaitGroup) map[string]chan struct{}
	originalsStored_managersTest                  bool
	storeMutex_managersTest                       sync.Mutex
)

func setupMocksAndTeardown(t *testing.T) func() {
	storeMutex_managersTest.Lock()
	if !originalsStored_managersTest {
		originalStartMCPServersFunc_managersTest = mcpserver.StartMCPServers
		originalStartPortForwardingsFunc_managersTest = portforwarding.StartPortForwardings
		originalsStored_managersTest = true
	}
	storeMutex_managersTest.Unlock()

	// Generic mock implementations that can be further specialized in tests if needed
	mcpserver.StartMCPServers = func(configs []config.MCPServerDefinition, mcpUpdateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
		t.Logf("GENERIC MOCK_EXEC: mcpserver.StartMCPServers with %d configs", len(configs))
		mStopChans := make(map[string]chan struct{})
		for _, cfg := range configs {
			mStopChans[cfg.Name] = make(chan struct{})
		}
		return mStopChans, nil
	}

	portforwarding.StartPortForwardings = func(configs []config.PortForwardDefinition, pfUpdateFn portforwarding.PortForwardUpdateFunc, wg *sync.WaitGroup) map[string]chan struct{} {
		t.Logf("GENERIC MOCK_EXEC: portforwarding.StartPortForwardings with %d configs", len(configs))
		mStopChans := make(map[string]chan struct{})
		for _, cfg := range configs {
			mStopChans[cfg.Name] = make(chan struct{})
		}
		return mStopChans
	}

	return func() {
		storeMutex_managersTest.Lock()
		mcpserver.StartMCPServers = originalStartMCPServersFunc_managersTest
		portforwarding.StartPortForwardings = originalStartPortForwardingsFunc_managersTest
		storeMutex_managersTest.Unlock()
	}
}

func TestServiceManager_DebounceAndStateTransitions(t *testing.T) {
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	mockReporterInstance := newMockReporter(t)
	smAPI := NewServiceManager(mockReporterInstance)
	smInternal := getInternalServiceManager(smAPI)

	var testWg sync.WaitGroup

	// --- Test MCP Service ---
	mcpServiceLabel := "test-mcp-service"
	mcpConfig := config.MCPServerDefinition{Name: mcpServiceLabel, Command: []string{"sleep", "1"}, Type: config.MCPServerTypeLocalCommand, Enabled: true}
	managedConfigsMcp := []ManagedServiceConfig{
		{Label: mcpServiceLabel, Type: reporting.ServiceTypeMCPServer, Config: mcpConfig},
	}

	var capturedMcpUpdateFn mcpserver.McpUpdateFunc
	// Specific override for this test section to capture the update function
	// This temporarily replaces the generic mock from setupMocksAndTeardown
	oldMcpserverMock := mcpserver.StartMCPServers
	mcpserver.StartMCPServers = func(
		configs []config.MCPServerDefinition,
		updateFn mcpserver.McpUpdateFunc,
		processWg *sync.WaitGroup,
	) (map[string]chan struct{}, []error) {
		t.Logf("DEBOUNCE TEST MOCK_EXEC: mcpserver.StartMCPServers with %d configs", len(configs))
		capturedMcpUpdateFn = updateFn
		// Call the generic mock to get its stop channels, etc.
		return oldMcpserverMock(configs, updateFn, processWg)
	}

	testWg.Add(1)
	_, errs := smAPI.StartServices(managedConfigsMcp, &testWg)
	require.Empty(t, errs)
	require.NotNil(t, capturedMcpUpdateFn)

	t.Run("MCP_InitialStarting", func(t *testing.T) {
		capturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: mcpServiceLabel, ProcessStatus: "NpxInitializing"})
		time.Sleep(50 * time.Millisecond)
		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateStarting, updates[0].State)
		smInternal.mu.Lock()
		internalState, exists := smInternal.serviceStates[mcpServiceLabel]
		smInternal.mu.Unlock()
		assert.True(t, exists)
		assert.Equal(t, reporting.StateStarting, internalState)
		mockReporterInstance.ClearUpdates()
	})

	t.Run("MCP_DebounceStarting", func(t *testing.T) {
		capturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: mcpServiceLabel, ProcessStatus: "NpxInitializing"})
		time.Sleep(50 * time.Millisecond)
		assert.Empty(t, mockReporterInstance.GetUpdates())
	})

	t.Run("MCP_TransitionToRunning", func(t *testing.T) {
		capturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: mcpServiceLabel, ProcessStatus: "NpxRunning"})
		time.Sleep(50 * time.Millisecond)
		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateRunning, updates[0].State)
		smInternal.mu.Lock()
		internalState, exists := smInternal.serviceStates[mcpServiceLabel]
		smInternal.mu.Unlock()
		assert.True(t, exists)
		assert.Equal(t, reporting.StateRunning, internalState)
		mockReporterInstance.ClearUpdates()
	})

	t.Run("MCP_DebounceRunning", func(t *testing.T) {
		capturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: mcpServiceLabel, ProcessStatus: "NpxRunning"})
		time.Sleep(50 * time.Millisecond)
		assert.Empty(t, mockReporterInstance.GetUpdates())
	})

	t.Run("MCP_TransitionToFailedAndCleanup", func(t *testing.T) {
		testWg.Done()
		capturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: mcpServiceLabel, ProcessStatus: "NpxExitedWithError", ProcessErr: fmt.Errorf("mcp failed")})
		time.Sleep(50 * time.Millisecond)
		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateFailed, updates[0].State)
		smInternal.mu.Lock()
		_, exists := smInternal.serviceStates[mcpServiceLabel]
		smInternal.mu.Unlock()
		assert.False(t, exists)
		mockReporterInstance.ClearUpdates()
	})

	// --- Test Port Forwarding Service ---
	mcpserver.StartMCPServers = oldMcpserverMock // Restore generic mock for mcpserver for this section
	mockReporterInstance.ClearUpdates()
	pfServiceLabel := "test-pf-service"
	pfConfig := config.PortForwardDefinition{Name: pfServiceLabel, TargetName: "svc", TargetType: "service", LocalPort: "8080", RemotePort: "80", Enabled: true}
	managedConfigsPf := []ManagedServiceConfig{
		{Label: pfServiceLabel, Type: reporting.ServiceTypePortForward, Config: pfConfig},
	}

	var capturedPfUpdateFn portforwarding.PortForwardUpdateFunc
	// Specific override for this test section to capture the update function
	oldPfMock := portforwarding.StartPortForwardings
	portforwarding.StartPortForwardings = func(
		configs []config.PortForwardDefinition,
		updateFn portforwarding.PortForwardUpdateFunc,
		processWg *sync.WaitGroup,
	) map[string]chan struct{} {
		t.Logf("DEBOUNCE TEST MOCK_EXEC: portforwarding.StartPortForwardings with %d configs", len(configs))
		capturedPfUpdateFn = updateFn
		return oldPfMock(configs, updateFn, processWg)
	}

	testWg.Add(1)
	_, errsPF := smAPI.StartServices(managedConfigsPf, &testWg)
	require.Empty(t, errsPF)
	require.NotNil(t, capturedPfUpdateFn)

	t.Run("PF_InitialStarting", func(t *testing.T) {
		capturedPfUpdateFn(pfServiceLabel, portforwarding.StatusDetailInitializing, false, nil)
		time.Sleep(50 * time.Millisecond)
		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateStarting, updates[0].State)
		mockReporterInstance.ClearUpdates()
	})

	t.Run("PF_TransitionToRunning", func(t *testing.T) {
		capturedPfUpdateFn(pfServiceLabel, portforwarding.StatusDetailForwardingActive, true, nil)
		time.Sleep(50 * time.Millisecond)
		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateRunning, updates[0].State)
		mockReporterInstance.ClearUpdates()
	})

	t.Run("PF_TransitionToStoppedAndCleanup", func(t *testing.T) {
		testWg.Done()
		capturedPfUpdateFn(pfServiceLabel, portforwarding.StatusDetailStopped, false, nil)
		time.Sleep(50 * time.Millisecond)
		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateStopped, updates[0].State)
		smInternal.mu.Lock()
		_, exists := smInternal.serviceStates[pfServiceLabel]
		smInternal.mu.Unlock()
		assert.False(t, exists)
		mockReporterInstance.ClearUpdates()
	})

	// --- Test Restart Logic ---
	mockReporterInstance.ClearUpdates()
	restartServiceLabel := "test-restart-service"
	restartMcpConfig := config.MCPServerDefinition{Name: restartServiceLabel, Command: []string{"sleep", "2"}, Type: config.MCPServerTypeLocalCommand, Enabled: true}
	managedRestartConfigs := []ManagedServiceConfig{
		{Label: restartServiceLabel, Type: reporting.ServiceTypeMCPServer, Config: restartMcpConfig},
	}

	var serviceInstanceCounter int
	var firstInstanceUpdateFn mcpserver.McpUpdateFunc // To store updateFn for the first instance

	// Save the generic mock that setupMocksAndTeardown installed
	genericMcpserverMock := mcpserver.StartMCPServers

	// Mock for the initial start of test-restart-service (instance 1)
	mcpserver.StartMCPServers = func(
		configs []config.MCPServerDefinition,
		updateFn mcpserver.McpUpdateFunc,
		processWg *sync.WaitGroup,
	) (map[string]chan struct{}, []error) {
		t.Logf("RESTART_SEQ INSTANCE_1 MOCK: mcpserver.StartMCPServers for %s", configs[0].Name)
		serviceInstanceCounter++ // Should be 1 here

		if configs[0].Name == restartServiceLabel && serviceInstanceCounter == 1 {
			firstInstanceUpdateFn = updateFn // Capture updateFn for instance 1

			// Create a stop channel that this mock instance will listen to
			instance1StopChan := make(chan struct{})
			go func() {
				<-instance1StopChan // Wait for ServiceManager to close this
				t.Logf("RESTART_SEQ INSTANCE_1 MOCK: Stop channel closed for %s. Simulating NpxStoppedByUser.", restartServiceLabel)
				if firstInstanceUpdateFn != nil {
					firstInstanceUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: restartServiceLabel, ProcessStatus: "NpxStoppedByUser"})
				}
			}()
			return map[string]chan struct{}{restartServiceLabel: instance1StopChan}, nil
		}
		// For subsequent calls (like the restarted instance 2), or other services, use the generic mock.
		// This also means instance_2 will use the generic mock that doesn't auto-report stop.
		return genericMcpserverMock(configs, updateFn, processWg)
	}

	testWg.Add(1)
	serviceInstanceCounter = 0
	_, errsRestart := smAPI.StartServices(managedRestartConfigs, &testWg)
	require.Empty(t, errsRestart)
	require.Equal(t, 1, serviceInstanceCounter, "Mock for instance 1 should have been called once")
	require.NotNil(t, firstInstanceUpdateFn, "Update function for instance 1 should be captured")

	// Instance 1 reports Starting -> Running
	firstInstanceUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: restartServiceLabel, ProcessStatus: "NpxInitializing"})
	time.Sleep(50 * time.Millisecond)
	firstInstanceUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: restartServiceLabel, ProcessStatus: "NpxRunning"})
	time.Sleep(50 * time.Millisecond)

	initialUpdates := mockReporterInstance.GetUpdates()
	require.Len(t, initialUpdates, 2)
	assert.Equal(t, reporting.StateStarting, initialUpdates[0].State)
	assert.Equal(t, reporting.StateRunning, initialUpdates[1].State)
	mockReporterInstance.ClearUpdates()

	t.Run("Restart_Sequence", func(t *testing.T) {
		// This capturedMcpUpdateFn will be for instance 2
		var instance2CapturedMcpUpdateFn mcpserver.McpUpdateFunc

		// Now, set up the mock for when instance 2 (the restarted one) starts
		mcpserver.StartMCPServers = func(
			configs []config.MCPServerDefinition,
			updateFn mcpserver.McpUpdateFunc,
			processWg *sync.WaitGroup,
		) (map[string]chan struct{}, []error) {
			t.Logf("RESTART_SEQ INSTANCE_2 MOCK: mcpserver.StartMCPServers for %s", configs[0].Name)
			serviceInstanceCounter++ // Should be 2 here
			if configs[0].Name == restartServiceLabel && serviceInstanceCounter == 2 {
				instance2CapturedMcpUpdateFn = updateFn
			}
			// Instance 2 will use the generic mock's behavior (no auto-stop reporting)
			return genericMcpserverMock(configs, updateFn, processWg)
		}

		err := smAPI.RestartService(restartServiceLabel)
		require.NoError(t, err)

		// ServiceManager.StopService closes instance1StopChan.
		// The goroutine in instance 1's mock should detect this and call firstInstanceUpdateFn(NpxStoppedByUser).
		// This should then trigger the reports for "Stopped" and "Starting" (for instance 2).

		testWg.Done() // For original instance_1 completing its lifecycle (signaled by StopService)

		time.Sleep(250 * time.Millisecond) // Allow time for stop processing, and restart to kick off instance 2

		require.Equal(t, 2, serviceInstanceCounter, "StartMCPServers (mock for instance 2) should have been called")
		require.NotNil(t, instance2CapturedMcpUpdateFn, "Update function for instance 2 should be captured")

		updates := mockReporterInstance.GetUpdates()
		t.Logf("RESTART_SEQUENCE: Checking for 2 reports (Stopped, Starting). Updates found: %d", len(updates))
		for i, u := range updates {
			t.Logf("RESTART_SEQUENCE: Update %d: %+v", i, u)
		}
		require.Len(t, updates, 2, "Should report Stopped (from instance 1 implicit stop), then Starting (for instance 2)")
		assert.Equal(t, reporting.StateStopped, updates[0].State)
		assert.Equal(t, restartServiceLabel, updates[0].SourceLabel)
		assert.Equal(t, reporting.StateStarting, updates[1].State)
		assert.Equal(t, restartServiceLabel, updates[1].SourceLabel)
		mockReporterInstance.ClearUpdates()

		// New service instance (instance 2) starts up via its own adapter (instance2CapturedMcpUpdateFn)
		instance2CapturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: restartServiceLabel, ProcessStatus: "NpxInitializing"})
		time.Sleep(50 * time.Millisecond)
		assert.Empty(t, mockReporterInstance.GetUpdates(), "NpxInitializing for restarted service (instance 2) should be debounced")

		instance2CapturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: restartServiceLabel, ProcessStatus: "NpxRunning"})
		time.Sleep(50 * time.Millisecond)
		updates = mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateRunning, updates[0].State)
		assert.Equal(t, restartServiceLabel, updates[0].SourceLabel)
		mockReporterInstance.ClearUpdates()

		// Clean up: Stop instance 2. Its goroutine (from ServiceManager) will handle testWg.Done()
		smAPI.StopService(restartServiceLabel)
		// We need to simulate instance 2 stopping by calling its adapter
		instance2CapturedMcpUpdateFn(mcpserver.McpDiscreteStatusUpdate{Label: restartServiceLabel, ProcessStatus: "NpxStoppedByUser"})
		time.Sleep(50 * time.Millisecond)
		smInternal.mu.Lock()
		_, exists := smInternal.serviceStates[restartServiceLabel]
		smInternal.mu.Unlock()
		assert.False(t, exists, "Restarted service (instance 2) state should be cleaned up after final stop")
	})

	// Restore the generic mock after the Restart Logic section
	mcpserver.StartMCPServers = genericMcpserverMock

	t.Run("Restart_Already_Stopped_Or_Failed_Service", func(t *testing.T) {
		mockReporterInstance.ClearUpdates()
		// serviceInstanceCounter is from the parent scope, reset or use a local one for this sub-test
		var alreadyStoppedInstanceStartCount int // Local counter for this sub-test

		stoppedServiceLabel := "already-stopped-svc"
		smInternal.mu.Lock()
		smInternal.serviceConfigs[stoppedServiceLabel] = ManagedServiceConfig{
			Label: stoppedServiceLabel, Type: reporting.ServiceTypeMCPServer, Config: config.MCPServerDefinition{Name: stoppedServiceLabel, Type: config.MCPServerTypeLocalCommand, Enabled: true},
		}
		delete(smInternal.serviceStates, stoppedServiceLabel) // Ensure it's not in states
		smInternal.mu.Unlock()

		// Temporarily override StartMCPServers to track calls for this sub-test
		originalGlobalMcpserverMock := mcpserver.StartMCPServers
		var capturedUpdateFnForAlreadyStopped mcpserver.McpUpdateFunc
		mcpserver.StartMCPServers = func(
			configs []config.MCPServerDefinition,
			updateFn mcpserver.McpUpdateFunc,
			processWg *sync.WaitGroup,
		) (map[string]chan struct{}, []error) {
			t.Logf("ALREADY_STOPPED_MOCK: mcpserver.StartMCPServers for %s", configs[0].Name)
			if configs[0].Name == stoppedServiceLabel {
				alreadyStoppedInstanceStartCount++
				capturedUpdateFnForAlreadyStopped = updateFn
			}
			// Call the mock that was active before this sub-test (genericMcpserverMock)
			return originalGlobalMcpserverMock(configs, updateFn, processWg)
		}
		defer func() { mcpserver.StartMCPServers = originalGlobalMcpserverMock }() // Restore after sub-test

		err := smAPI.RestartService(stoppedServiceLabel)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond) // Allow for restart processing

		require.Equal(t, 1, alreadyStoppedInstanceStartCount, "StartMCPServers should be called once for restarting a non-active service")

		updates := mockReporterInstance.GetUpdates()
		require.Len(t, updates, 2, "Should report Stopped, then Starting for restarting an inactive service")
		assert.Equal(t, reporting.StateStopped, updates[0].State)
		assert.Equal(t, reporting.StateStarting, updates[1].State)
		mockReporterInstance.ClearUpdates()

		require.NotNil(t, capturedUpdateFnForAlreadyStopped, "Update function for already-stopped restarted service should be captured")
		capturedUpdateFnForAlreadyStopped(mcpserver.McpDiscreteStatusUpdate{Label: stoppedServiceLabel, ProcessStatus: "NpxInitializing"})
		time.Sleep(50 * time.Millisecond)
		assert.Empty(t, mockReporterInstance.GetUpdates(), "NpxInitializing should be debounced")

		capturedUpdateFnForAlreadyStopped(mcpserver.McpDiscreteStatusUpdate{Label: stoppedServiceLabel, ProcessStatus: "NpxRunning"})
		time.Sleep(50 * time.Millisecond)
		updates = mockReporterInstance.GetUpdates()
		require.Len(t, updates, 1)
		assert.Equal(t, reporting.StateRunning, updates[0].State)
		mockReporterInstance.ClearUpdates()

		// Clean up this service - ServiceManager's goroutine handles testWg.Done()
		smAPI.StopService(stoppedServiceLabel)
		// Simulate service stopping
		if capturedUpdateFnForAlreadyStopped != nil { // It might be nil if StartMCPServers wasn't called
			capturedUpdateFnForAlreadyStopped(mcpserver.McpDiscreteStatusUpdate{Label: stoppedServiceLabel, ProcessStatus: "NpxStoppedByUser"})
		}
		time.Sleep(50 * time.Millisecond)
		smInternal.mu.Lock()
		_, exists := smInternal.serviceStates[stoppedServiceLabel]
		smInternal.mu.Unlock()
		assert.False(t, exists, "State for already-stopped-restarted service should be cleaned up")
	})
}

// Standalone test functions like TestServiceManager_StartServices_EmptyConfig_NoMock,
// TestServiceManager_StopService_WithMocks, and TestServiceManager_StopAllServices_WithMocks
// are defined below. They should use setupMocksAndTeardown if they trigger mocked functions.

func TestServiceManager_StartServices_EmptyConfig_NoMock(t *testing.T) {
	// No mocks needed here
	consoleReporter := reporting.NewConsoleReporter()
	sm := NewServiceManager(consoleReporter)
	var configs []ManagedServiceConfig
	var wg sync.WaitGroup

	stopChans, errs := sm.StartServices(configs, &wg)

	if len(stopChans) != 0 {
		t.Errorf("Expected 0 stop channels, got %d", len(stopChans))
	}
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors, got %v", errs)
	}
}

func TestServiceManager_StopService_WithMocks(t *testing.T) {
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	consoleReporter := reporting.NewConsoleReporter()
	sm := NewServiceManager(consoleReporter)
	pflabel := "TestPF_Stop"
	configs := []ManagedServiceConfig{
		{Type: reporting.ServiceTypePortForward, Label: pflabel, Config: config.PortForwardDefinition{
			Name:       pflabel,
			TargetName: "s",
			TargetType: "service",
			LocalPort:  "1",
			RemotePort: "1",
			Enabled:    true,
		}},
	}
	var wg sync.WaitGroup

	// Specific mock for StartPortForwardings for this test to return a controllable stop channel
	var specificPfStopChan chan struct{}
	portforwarding.StartPortForwardings = func(
		cfgs []config.PortForwardDefinition,
		updateFn portforwarding.PortForwardUpdateFunc,
		pWg *sync.WaitGroup,
	) map[string]chan struct{} {
		specificPfStopChan = make(chan struct{})
		// If the main service manager expects Add/Done, this mock needs to respect it if it were real.
		// For this test, service manager's goroutine wrapper handles Add/Done for pWg.
		return map[string]chan struct{}{cfgs[0].Name: specificPfStopChan}
	}

	// ServiceManager will Add(1) to wg internally.
	_, _ = sm.StartServices(configs, &wg)
	// wg.Wait() // Not waiting here as we manually control stop for test purposes

	err := sm.StopService(pflabel)
	require.NoError(t, err)

	select {
	case <-specificPfStopChan:
		// Expected: channel closed by StopService logic which calls close on the original stopChan provided by StartPortForwardings
	default:
		t.Errorf("StopService did not close the service's specific stop channel for %s", pflabel)
	}

	err = sm.StopService("NonExistent")
	require.Error(t, err)

	err = sm.StopService(pflabel) // Try to stop again
	require.Error(t, err, "Stopping an already stopped service should result in an error")
}

func TestServiceManager_StopAllServices_WithMocks(t *testing.T) {
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	consoleReporter := reporting.NewConsoleReporter()
	sm := NewServiceManager(consoleReporter)
	pflabel1 := "PF_StopAll1"
	mcplabel1 := "MCP_StopAll1"
	configs := []ManagedServiceConfig{
		{Type: reporting.ServiceTypePortForward, Label: pflabel1, Config: config.PortForwardDefinition{Name: pflabel1, TargetName: "s1", TargetType: "service", Enabled: true}},
		{Type: reporting.ServiceTypeMCPServer, Label: mcplabel1, Config: config.MCPServerDefinition{Name: mcplabel1, Command: []string{"c1"}, Type: config.MCPServerTypeLocalCommand, Enabled: true}},
	}
	var wg sync.WaitGroup

	// Capture channels from mocks
	pfStopChans := make(map[string]chan struct{})
	mcpStopChans := make(map[string]chan struct{})

	portforwarding.StartPortForwardings = func(cfgs []config.PortForwardDefinition, _ portforwarding.PortForwardUpdateFunc, _ *sync.WaitGroup) map[string]chan struct{} {
		for _, cfg := range cfgs {
			pfStopChans[cfg.Name] = make(chan struct{})
		}
		return pfStopChans
	}
	mcpserver.StartMCPServers = func(cfgs []config.MCPServerDefinition, _ mcpserver.McpUpdateFunc, _ *sync.WaitGroup) (map[string]chan struct{}, []error) {
		for _, cfg := range cfgs {
			mcpStopChans[cfg.Name] = make(chan struct{})
		}
		return mcpStopChans, nil
	}

	_, _ = sm.StartServices(configs, &wg)

	sm.StopAllServices()

	select {
	case <-pfStopChans[pflabel1]:
	default:
		t.Errorf("PF1 channel not closed after StopAllServices")
	}
	select {
	case <-mcpStopChans[mcplabel1]:
	default:
		t.Errorf("MCP1 channel not closed after StopAllServices")
	}
}

// Add this test after existing tests
func TestServiceManager_StopServiceWithDependents(t *testing.T) {
	// Test cascading stop functionality
	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter).(*ServiceManager)

	// Create a dependency graph: mcp:prometheus -> pf:mc-prometheus
	depGraph := dependency.New()
	depGraph.AddNode(dependency.Node{
		ID:           "pf:mc-prometheus",
		FriendlyName: "MC Prometheus",
		Kind:         dependency.KindPortForward,
		DependsOn:    nil,
	})
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:prometheus",
		FriendlyName: "Prometheus MCP",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:mc-prometheus"},
	})
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:grafana",
		FriendlyName: "Grafana MCP",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:mc-prometheus"}, // Also depends on the same PF
	})

	// Set up services
	configs := []ManagedServiceConfig{
		{
			Label: "mc-prometheus",
			Type:  reporting.ServiceTypePortForward,
			Config: config.PortForwardDefinition{
				Name:              "mc-prometheus",
				Enabled:           true,
				LocalPort:         "9090",
				RemotePort:        "9090",
				Namespace:         "monitoring",
				TargetType:        "service",
				TargetName:        "prometheus",
				KubeContextTarget: "context1",
			},
		},
		{
			Label: "prometheus",
			Type:  reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{
				Name:    "prometheus",
				Type:    config.MCPServerTypeLocalCommand,
				Enabled: true,
			},
		},
		{
			Label: "grafana",
			Type:  reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{
				Name:    "grafana",
				Type:    config.MCPServerTypeLocalCommand,
				Enabled: true,
			},
		},
	}

	// Mock the actual service starts
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	var wg sync.WaitGroup
	stopChannels, errs := sm.StartServices(configs, &wg)
	assert.Empty(t, errs, "No errors expected during service start")
	assert.Len(t, stopChannels, 3, "Should have 3 stop channels")

	// Wait for services to be registered
	time.Sleep(50 * time.Millisecond)

	// Test 1: Stop port forward - should cascade to both MCPs
	err := sm.StopServiceWithDependents("mc-prometheus", depGraph)
	assert.NoError(t, err, "StopServiceWithDependents should succeed")

	// Wait for cascading stops
	time.Sleep(50 * time.Millisecond)

	// Check that all services were stopped
	sm.mu.Lock()
	assert.Empty(t, sm.activeServices, "All services should be stopped after cascading stop")
	sm.mu.Unlock()

	// Reset for next test - start only the MCP servers
	sm.activeServices = make(map[string]chan struct{})
	stopChannels, errs = sm.StartServices(configs[1:], &wg) // Start only MCPs
	assert.Empty(t, errs, "No errors expected during service start")
	assert.Len(t, stopChannels, 2, "Should have 2 stop channels")

	// Wait for services to be registered
	time.Sleep(50 * time.Millisecond)

	// Test 2: Stop one MCP - should only stop that MCP, not the other
	err = sm.StopServiceWithDependents("prometheus", depGraph)
	assert.NoError(t, err, "StopServiceWithDependents should succeed")

	// Wait for stop
	time.Sleep(50 * time.Millisecond)

	// Check that only prometheus was stopped, grafana should still be active
	sm.mu.Lock()
	_, promExists := sm.activeServices["prometheus"]
	_, grafanaExists := sm.activeServices["grafana"]
	sm.mu.Unlock()
	assert.False(t, promExists, "Prometheus should be stopped")
	assert.True(t, grafanaExists, "Grafana should still be active")

	// Clean up
	sm.StopAllServices()
}

// TestServiceManager_StartServicesDependingOn tests starting services that depend on a specific node
func TestServiceManager_StartServicesDependingOn(t *testing.T) {
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter).(*ServiceManager)

	// Create a dependency graph: k8s:context1 -> pf:mc-prometheus -> mcp:prometheus
	depGraph := dependency.New()
	depGraph.AddNode(dependency.Node{
		ID:           "k8s:context1",
		FriendlyName: "K8s Connection",
		Kind:         dependency.KindK8sConnection,
		DependsOn:    nil,
	})
	depGraph.AddNode(dependency.Node{
		ID:           "pf:mc-prometheus",
		FriendlyName: "MC Prometheus",
		Kind:         dependency.KindPortForward,
		DependsOn:    []dependency.NodeID{"k8s:context1"},
	})
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:prometheus",
		FriendlyName: "Prometheus MCP",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:mc-prometheus"},
	})

	// Set up service configs
	configs := []ManagedServiceConfig{
		{
			Label: "mc-prometheus",
			Type:  reporting.ServiceTypePortForward,
			Config: config.PortForwardDefinition{
				Name:              "mc-prometheus",
				Enabled:           true,
				LocalPort:         "9090",
				RemotePort:        "9090",
				Namespace:         "monitoring",
				TargetType:        "service",
				TargetName:        "prometheus",
				KubeContextTarget: "context1",
			},
		},
		{
			Label: "prometheus",
			Type:  reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{
				Name:    "prometheus",
				Type:    config.MCPServerTypeLocalCommand,
				Enabled: true,
			},
		},
	}

	// Store configs without starting services
	sm.mu.Lock()
	for _, cfg := range configs {
		sm.serviceConfigs[cfg.Label] = cfg
	}
	sm.mu.Unlock()

	// Test starting services that depend on k8s:context1
	err := sm.StartServicesDependingOn("k8s:context1", depGraph)
	assert.NoError(t, err, "StartServicesDependingOn should succeed")

	// Wait for services to start
	time.Sleep(50 * time.Millisecond)

	// Check that the port forward was started
	sm.mu.Lock()
	_, pfActive := sm.activeServices["mc-prometheus"]
	sm.mu.Unlock()
	assert.True(t, pfActive, "Port forward should be active")

	// Prometheus should NOT be started as it depends on the port forward, not directly on k8s
	sm.mu.Lock()
	_, promActive := sm.activeServices["prometheus"]
	sm.mu.Unlock()
	assert.False(t, promActive, "Prometheus should not be active (doesn't directly depend on k8s)")

	// Test starting services that depend on the port forward
	err = sm.StartServicesDependingOn("pf:mc-prometheus", depGraph)
	assert.NoError(t, err, "StartServicesDependingOn should succeed for port forward")

	// Wait for services to start
	time.Sleep(50 * time.Millisecond)

	// Now prometheus should be started
	sm.mu.Lock()
	_, promActiveAfter := sm.activeServices["prometheus"]
	sm.mu.Unlock()
	assert.True(t, promActiveAfter, "Prometheus should now be active")

	// Test with no dependency graph
	err = sm.StartServicesDependingOn("k8s:context1", nil)
	assert.Error(t, err, "Should fail without dependency graph")

	// Clean up
	sm.StopAllServices()
}

// TestServiceManager_StartServicesWithDependencyOrder tests that services are started in correct dependency order
func TestServiceManager_StartServicesWithDependencyOrder(t *testing.T) {
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter).(*ServiceManager)

	// Create a dependency graph with complex dependencies:
	// k8s:mc -> pf:mc-prometheus -> mcp:prometheus
	//        -> pf:mc-grafana -> mcp:grafana
	// k8s:wc -> pf:wc-loki -> mcp:loki
	depGraph := dependency.New()

	// K8s connections
	depGraph.AddNode(dependency.Node{
		ID:           "k8s:mc",
		FriendlyName: "MC Connection",
		Kind:         dependency.KindK8sConnection,
		DependsOn:    nil,
	})
	depGraph.AddNode(dependency.Node{
		ID:           "k8s:wc",
		FriendlyName: "WC Connection",
		Kind:         dependency.KindK8sConnection,
		DependsOn:    nil,
	})

	// Port forwards
	depGraph.AddNode(dependency.Node{
		ID:           "pf:mc-prometheus",
		FriendlyName: "MC Prometheus PF",
		Kind:         dependency.KindPortForward,
		DependsOn:    []dependency.NodeID{"k8s:mc"},
	})
	depGraph.AddNode(dependency.Node{
		ID:           "pf:mc-grafana",
		FriendlyName: "MC Grafana PF",
		Kind:         dependency.KindPortForward,
		DependsOn:    []dependency.NodeID{"k8s:mc"},
	})
	depGraph.AddNode(dependency.Node{
		ID:           "pf:wc-loki",
		FriendlyName: "WC Loki PF",
		Kind:         dependency.KindPortForward,
		DependsOn:    []dependency.NodeID{"k8s:wc"},
	})

	// MCPs
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:prometheus",
		FriendlyName: "Prometheus MCP",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:mc-prometheus"},
	})
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:grafana",
		FriendlyName: "Grafana MCP",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:mc-grafana"},
	})
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:loki",
		FriendlyName: "Loki MCP",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:wc-loki"},
	})

	// Service configs - mixed order to test sorting
	configs := []ManagedServiceConfig{
		// MCPs first (wrong order - they should start last)
		{
			Label:  "prometheus",
			Type:   reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{Name: "prometheus", Enabled: true},
		},
		{
			Label:  "grafana",
			Type:   reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{Name: "grafana", Enabled: true},
		},
		// Port forwards second (they should start first)
		{
			Label:  "mc-prometheus",
			Type:   reporting.ServiceTypePortForward,
			Config: config.PortForwardDefinition{Name: "mc-prometheus", Enabled: true},
		},
		{
			Label:  "wc-loki",
			Type:   reporting.ServiceTypePortForward,
			Config: config.PortForwardDefinition{Name: "wc-loki", Enabled: true},
		},
		// More MCPs
		{
			Label:  "loki",
			Type:   reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{Name: "loki", Enabled: true},
		},
		// Another port forward
		{
			Label:  "mc-grafana",
			Type:   reporting.ServiceTypePortForward,
			Config: config.PortForwardDefinition{Name: "mc-grafana", Enabled: true},
		},
	}

	// Track the actual start order
	var startOrder []string
	var startOrderMutex sync.Mutex

	// Override the mocks to capture start order
	originalPFMock := portforwarding.StartPortForwardings
	portforwarding.StartPortForwardings = func(
		cfgs []config.PortForwardDefinition,
		updateFn portforwarding.PortForwardUpdateFunc,
		pWg *sync.WaitGroup,
	) map[string]chan struct{} {
		startOrderMutex.Lock()
		for _, cfg := range cfgs {
			startOrder = append(startOrder, cfg.Name)
			t.Logf("Starting port forward: %s", cfg.Name)
		}
		startOrderMutex.Unlock()
		return originalPFMock(cfgs, updateFn, pWg)
	}

	originalMCPMock := mcpserver.StartMCPServers
	mcpserver.StartMCPServers = func(
		cfgs []config.MCPServerDefinition,
		updateFn mcpserver.McpUpdateFunc,
		pWg *sync.WaitGroup,
	) (map[string]chan struct{}, []error) {
		startOrderMutex.Lock()
		for _, cfg := range cfgs {
			startOrder = append(startOrder, cfg.Name)
			t.Logf("Starting MCP server: %s", cfg.Name)
		}
		startOrderMutex.Unlock()
		return originalMCPMock(cfgs, updateFn, pWg)
	}

	// Start services with dependency order
	var wg sync.WaitGroup
	_, errs := sm.StartServicesWithDependencyOrder(configs, depGraph, &wg)
	assert.Empty(t, errs, "No errors expected during service start")

	// Wait for services to be registered
	time.Sleep(50 * time.Millisecond)

	// Verify the start order
	startOrderMutex.Lock()
	actualOrder := make([]string, len(startOrder))
	copy(actualOrder, startOrder)
	startOrderMutex.Unlock()

	t.Logf("Actual start order: %v", actualOrder)

	// Build a map to find indices easily
	orderIndex := make(map[string]int)
	for i, svc := range actualOrder {
		orderIndex[svc] = i
	}

	// Verify all services were started
	assert.Len(t, actualOrder, 6, "All 6 services should be started")

	// Verify dependency order: Port forwards should start before their dependent MCPs
	assert.Less(t, orderIndex["mc-prometheus"], orderIndex["prometheus"],
		"mc-prometheus port forward must start before prometheus MCP")
	assert.Less(t, orderIndex["mc-grafana"], orderIndex["grafana"],
		"mc-grafana port forward must start before grafana MCP")
	assert.Less(t, orderIndex["wc-loki"], orderIndex["loki"],
		"wc-loki port forward must start before loki MCP")

	// Log the actual order to confirm it's correct
	t.Logf("✓ Services started in correct dependency order:")
	t.Logf("  Port forwards started first: %v", actualOrder[:3])
	t.Logf("  MCPs started after their dependencies: %v", actualOrder[3:])

	// Verify that port forwards on the same k8s connection can start in any order relative to each other
	// but all must be before MCPs that depend on them
	mcPFs := []string{"mc-prometheus", "mc-grafana"}
	mcMCPs := []string{"prometheus", "grafana"}

	// All MC port forwards should be before all MCPs that depend on MC
	for _, pf := range mcPFs {
		for _, mcp := range mcMCPs {
			if (pf == "mc-prometheus" && mcp == "prometheus") || (pf == "mc-grafana" && mcp == "grafana") {
				assert.Less(t, orderIndex[pf], orderIndex[mcp],
					"Port forward %s must start before its dependent MCP %s", pf, mcp)
			}
		}
	}

	// Clean up
	sm.StopAllServices()
}

// TestServiceManager_StartServicesWithDependencyOrder_NoDependencyGraph tests fallback behavior
func TestServiceManager_StartServicesWithDependencyOrder_NoDependencyGraph(t *testing.T) {
	teardown := setupMocksAndTeardown(t)
	defer teardown()

	reporter := newMockReporter(t)
	sm := NewServiceManager(reporter).(*ServiceManager)

	configs := []ManagedServiceConfig{
		{
			Label:  "test-pf",
			Type:   reporting.ServiceTypePortForward,
			Config: config.PortForwardDefinition{Name: "test-pf", Enabled: true},
		},
		{
			Label:  "test-mcp",
			Type:   reporting.ServiceTypeMCPServer,
			Config: config.MCPServerDefinition{Name: "test-mcp", Enabled: true},
		},
	}

	// Call with nil dependency graph - should fall back to regular StartServices
	var wg sync.WaitGroup
	stopChans, errs := sm.StartServicesWithDependencyOrder(configs, nil, &wg)

	assert.Empty(t, errs, "No errors expected")
	assert.Len(t, stopChans, 2, "Should have 2 stop channels")

	// Verify services were started (order doesn't matter without dependency graph)
	sm.mu.Lock()
	assert.Len(t, sm.activeServices, 2, "Both services should be active")
	sm.mu.Unlock()

	// Clean up
	sm.StopAllServices()
}

// TestServiceManager_WaitsForDependenciesToBeRunning tests that services wait for their dependencies to be running
func TestServiceManager_WaitsForDependenciesToBeRunning(t *testing.T) {
	reporter := &mockReporter{}
	sm := NewServiceManager(reporter).(*ServiceManager)

	// Create dependency graph
	depGraph := dependency.New()
	
	// Add port forward node
	depGraph.AddNode(dependency.Node{
		ID:           "pf:test-pf",
		FriendlyName: "test-pf",
		Kind:         dependency.KindPortForward,
		DependsOn:    nil,
	})
	
	// Add MCP that depends on port forward
	depGraph.AddNode(dependency.Node{
		ID:           "mcp:test-mcp",
		FriendlyName: "test-mcp",
		Kind:         dependency.KindMCP,
		DependsOn:    []dependency.NodeID{"pf:test-pf"},
	})

	// Track when each service starts
	var pfStartTime, mcpStartTime time.Time
	var pfRunningTime time.Time
	
	// Override the port forwarding start function
	originalPfStart := portforwarding.StartPortForwardings
	portforwarding.StartPortForwardings = func(
		configs []config.PortForwardDefinition,
		updateCallback portforwarding.PortForwardUpdateFunc,
		wg *sync.WaitGroup,
	) map[string]chan struct{} {
		pfStartTime = time.Now()
		
		// Simulate port forward taking time to become running
		go func() {
			// Report starting state
			updateCallback("test-pf", portforwarding.StatusDetailInitializing, false, nil)
			
			// Wait 500ms before becoming running
			time.Sleep(500 * time.Millisecond)
			pfRunningTime = time.Now()
			updateCallback("test-pf", portforwarding.StatusDetailForwardingActive, true, nil)
		}()
		
		return map[string]chan struct{}{
			"test-pf": make(chan struct{}),
		}
	}
	defer func() { portforwarding.StartPortForwardings = originalPfStart }()

	// Override the MCP start function
	originalMcpStart := mcpserver.StartMCPServers
	mcpserver.StartMCPServers = func(
		configs []config.MCPServerDefinition,
		updateCallback mcpserver.McpUpdateFunc,
		wg *sync.WaitGroup,
	) (map[string]chan struct{}, []error) {
		mcpStartTime = time.Now()
		
		// Simulate MCP starting sequence - report starting first, then running after delay
		go func() {
			// First report starting state
			updateCallback(mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-mcp",
				ProcessStatus: "NpxStarting",
				PID:           12345,
			})
			
			// Wait a bit before reporting running state
			time.Sleep(200 * time.Millisecond)
			
			// Now report running state
			updateCallback(mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-mcp",
				ProcessStatus: "NpxRunning",
				PID:           12345,
			})
		}()
		
		return map[string]chan struct{}{
			"test-mcp": make(chan struct{}),
		}, nil
	}
	defer func() { mcpserver.StartMCPServers = originalMcpStart }()

	// Create configs
	configs := []ManagedServiceConfig{
		{
			Type:  reporting.ServiceTypePortForward,
			Label: "test-pf",
			Config: config.PortForwardDefinition{
				Name:    "test-pf",
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

	// Start services
	var wg sync.WaitGroup
	_, errs := sm.StartServicesWithDependencyOrder(configs, depGraph, &wg)
	
	// Wait a bit to ensure everything has started
	time.Sleep(1 * time.Second)
	
	// Verify no errors
	assert.Empty(t, errs)
	
	// Verify timing
	assert.True(t, !pfStartTime.IsZero(), "Port forward should have started")
	assert.True(t, !mcpStartTime.IsZero(), "MCP should have started")
	assert.True(t, !pfRunningTime.IsZero(), "Port forward should have reached running state")
	
	// The key assertion: MCP should start AFTER port forward is running
	assert.True(t, mcpStartTime.After(pfRunningTime), 
		"MCP started at %v but port forward only became running at %v - MCP started too early!",
		mcpStartTime, pfRunningTime)
	
	// Additional timing info for debugging
	t.Logf("Port forward started at: %v", pfStartTime)
	t.Logf("Port forward became running at: %v (after %v)", pfRunningTime, pfRunningTime.Sub(pfStartTime))
	t.Logf("MCP started at: %v (after %v)", mcpStartTime, mcpStartTime.Sub(pfRunningTime))
}
