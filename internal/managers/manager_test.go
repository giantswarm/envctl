package managers_test

import (
	"envctl/internal/managers"
	"envctl/internal/mcpserver"      // For config types
	"envctl/internal/portforwarding" // For config types
	"sync"
	"testing"
	"time"
)

// No package-level vars for mocks here anymore.
// setupMocks will now override the actual package vars from portforwarding and mcpserver.

// setupServiceManagerTestMocks overrides the actual package-level functions
// from portforwarding and mcpserver packages with mocks for testing ServiceManager.
// It returns a cleanup function that must be deferred by the caller to restore originals.
func setupServiceManagerTestMocks(t *testing.T) func() {
	// Store original functions if not already stored (e.g. by a previous test setup)
	// This simplistic global storage for originals isn't ideal for parallel tests but common for basic cases.
	// A more robust approach would involve ensuring originals are stored only once per test run.
	originalPFStarter := portforwarding.StartPortForwards
	originalMCPServerStarter := mcpserver.StartAndManageMCPServers

	portforwarding.StartPortForwards = func(configs []portforwarding.PortForwardingConfig, updateCb portforwarding.UpdateFunc, globalStopChan <-chan struct{}, wg *sync.WaitGroup) map[string]chan struct{} {
		t.Logf("mock portforwarding.StartPortForwards called with %d configs", len(configs))
		mStopChans := make(map[string]chan struct{})
		for _, cfg := range configs {
			mStopChans[cfg.Label] = make(chan struct{})
			go func(c portforwarding.PortForwardingConfig) {
				if wg != nil {
					wg.Add(1)
					defer wg.Done()
				}
				time.Sleep(1 * time.Millisecond)
				if updateCb != nil {
					updateCb(c.Label, "Mock PF Running", "", false, true)
				}
			}(cfg)
		}
		return mStopChans
	}

	mcpserver.StartAndManageMCPServers = func(configs []mcpserver.MCPServerConfig, mcpUpdateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (map[string]chan struct{}, []error) {
		t.Logf("mock mcpserver.StartAndManageMCPServers called with %d configs", len(configs))
		mStopChans := make(map[string]chan struct{})
		for _, cfg := range configs {
			mStopChans[cfg.Name] = make(chan struct{})
			go func(c mcpserver.MCPServerConfig) {
				if wg != nil {
					wg.Add(1)
					defer wg.Done()
				}
				time.Sleep(1 * time.Millisecond)
				if mcpUpdateFn != nil {
					mcpUpdateFn(mcpserver.McpProcessUpdate{Label: c.Name, Status: "Mock MCP Running"})
				}
			}(cfg)
		}
		return mStopChans, nil
	}

	return func() {
		portforwarding.StartPortForwards = originalPFStarter
		mcpserver.StartAndManageMCPServers = originalMCPServerStarter
	}
}

func TestServiceManager_StartServices_EmptyConfig(t *testing.T) {
	// No mocks needed for empty config test as underlying functions won't be called.
	sm := managers.NewServiceManager().(*managers.ServiceManager)
	var configs []managers.ManagedServiceConfig
	var wg sync.WaitGroup
	var updates []managers.ManagedServiceUpdate
	updateCb := func(u managers.ManagedServiceUpdate) { updates = append(updates, u) }

	stopChans, errs := sm.StartServices(configs, updateCb, &wg)

	if len(stopChans) != 0 {
		t.Errorf("Expected 0 stop channels for empty config, got %d", len(stopChans))
	}
	if len(errs) != 0 {
		t.Errorf("Expected 0 errors for empty config, got %v", errs)
	}
	if len(updates) != 0 {
		t.Errorf("Expected 0 updates for empty config, got %d", len(updates))
	}
}

func TestServiceManager_StartServices_StartsServices(t *testing.T) {
	cleanup := setupServiceManagerTestMocks(t)
	defer cleanup()

	sm := managers.NewServiceManager().(*managers.ServiceManager)
	pflabel1 := "TestPF1"
	mcplabel1 := "TestMCP1"

	configs := []managers.ManagedServiceConfig{
		{Type: managers.ServiceTypePortForward, Label: pflabel1, Config: portforwarding.PortForwardingConfig{Label: pflabel1, ServiceName: "test-pf-svc"}},
		{Type: managers.ServiceTypeMCPServer, Label: mcplabel1, Config: mcpserver.MCPServerConfig{Name: mcplabel1, Command: "echo", Args: []string{"hello"}}},
	}

	var wg sync.WaitGroup
	updateCh := make(chan managers.ManagedServiceUpdate, len(configs)*2)
	updateCb := func(u managers.ManagedServiceUpdate) {
		// Protect against send on closed channel if test timing is tricky
		// This is a workaround; ideally, channel lifecycle is perfectly managed.
		func() {
			defer func() {
				if recover() != nil {
					// t.Logf("Recovered from send on closed updateCh for label %s", u.Label)
				}
			}()
			updateCh <- u
		}()
	}

	stopChans, errs := sm.StartServices(configs, updateCb, &wg)

	if len(errs) > 0 {
		t.Logf("StartServices returned startup errors: %v", errs)
	}

	if len(stopChans) != len(configs) {
		t.Errorf("Expected %d stop channels, got %d", len(configs), len(stopChans))
	}
	if _, ok := stopChans[pflabel1]; !ok {
		t.Errorf("Stop channel for %s not found", pflabel1)
	}
	if _, ok := stopChans[mcplabel1]; !ok {
		t.Errorf("Stop channel for %s not found", mcplabel1)
	}

	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	timeout := time.After(200 * time.Millisecond) // Increased timeout
	var receivedUpdates []managers.ManagedServiceUpdate

	initialWaitDone := false
COLLECT_UPDATES:
	for {
		select {
		case <-waitChan:
			// Main wait group is done, but allow a little more time for updates to flush through.
			// This is because checkAndProcessRestart might spawn new things not covered by the initial wg.
			if !initialWaitDone {
				initialWaitDone = true
				// Give a very short additional time for any chained updates after wg.Done
				// This is a pragmatic approach for testing complex async flows.
				time.Sleep(50 * time.Millisecond)
			}
			// After initial wait and grace period, try to read remaining without blocking indefinitely
			// If updateCh is not closed yet, this select will continue.
			// If it is closed by a concurrent action, this will break.
			// The main fix is that updateCh should only be closed once all senders are guaranteed to be done.
			// For this test, we will collect for a bit then break.

		case u, ok := <-updateCh:
			if !ok { // Channel closed
				break COLLECT_UPDATES
			}
			receivedUpdates = append(receivedUpdates, u)
		case <-timeout:
			t.Log("Timed out collecting updates")
			break COLLECT_UPDATES
		}
		// Safety break if waitChan is done and updateCh is empty for a bit
		if initialWaitDone && len(updateCh) == 0 && len(receivedUpdates) >= len(configs) {
			time.Sleep(10 * time.Millisecond)
			if len(updateCh) == 0 {
				break COLLECT_UPDATES
			}
		}
	}

	// It's safer not to close updateCh from the test side if there could be concurrent writers
	// from checkAndProcessRestart that are not covered by `wg`.
	// Instead, we rely on collecting updates until timeout or expected number.

	if len(receivedUpdates) < len(configs) {
		t.Errorf("Expected at least %d updates, got %d. Updates: %v", len(configs), len(receivedUpdates), receivedUpdates)
	}
}

func TestServiceManager_StopService(t *testing.T) {
	cleanup := setupServiceManagerTestMocks(t)
	defer cleanup()

	sm := managers.NewServiceManager().(*managers.ServiceManager)
	pflabel := "TestPF_Stop"
	configs := []managers.ManagedServiceConfig{
		{Type: managers.ServiceTypePortForward, Label: pflabel, Config: portforwarding.PortForwardingConfig{Label: pflabel, ServiceName: "test-pf-svc-stop"}},
	}
	var wg sync.WaitGroup
	updateCb := func(u managers.ManagedServiceUpdate) {}

	stopChans, _ := sm.StartServices(configs, updateCb, &wg)
	p_stopChan, ok := stopChans[pflabel]
	if !ok {
		t.Fatalf("Service %s not started or stop channel not returned", pflabel)
	}

	err := sm.StopService(pflabel)
	if err != nil {
		t.Errorf("StopService failed: %v", err)
	}

	// Check if channel is closed (non-blocking read)
	select {
	case <-p_stopChan:
		// Successfully closed
	default:
		t.Errorf("StopService did not close the stop channel for %s", pflabel)
	}

	// Test stopping a non-existent service
	err = sm.StopService("NonExistent")
	if err == nil {
		t.Error("StopService should have failed for non-existent service")
	}

	// Test stopping an already stopped service (by trying to close its channel again via StopService)
	// The current implementation of StopService in ServiceManager will try to close again, which would panic.
	// Let's refine StopService in ServiceManager to be idempotent for the close operation,
	// or this test needs to expect an error if called twice without the service being re-added to activeServices.
	// For now, we assume the first StopService call worked and it's removed from activeServices by checkAndProcessRestart upon update.
	// So, calling StopService again for the same label should immediately return an error that it's not active.
	err = sm.StopService(pflabel) // Attempt to stop again
	if err == nil {
		t.Errorf("StopService for an already stopped/stopping service should have returned an error or indicated it was already stopped")
	} else {
		t.Logf("Stopping already stopped service correctly returned error: %v", err) // Expect something like "service not found or not active"
	}
}

func TestServiceManager_StopAllServices(t *testing.T) {
	cleanup := setupServiceManagerTestMocks(t)
	defer cleanup()

	sm := managers.NewServiceManager().(*managers.ServiceManager)
	pflabel1 := "PF_StopAll1"
	mcplabel1 := "MCP_StopAll1"
	configs := []managers.ManagedServiceConfig{
		{Type: managers.ServiceTypePortForward, Label: pflabel1, Config: portforwarding.PortForwardingConfig{Label: pflabel1, ServiceName: "sa1"}},
		{Type: managers.ServiceTypeMCPServer, Label: mcplabel1, Config: mcpserver.MCPServerConfig{Name: mcplabel1, Command: "echo"}},
	}
	var wg sync.WaitGroup
	updateCb := func(u managers.ManagedServiceUpdate) {}

	stopChans, _ := sm.StartServices(configs, updateCb, &wg)
	pf1Chan := stopChans[pflabel1]
	mcp1Chan := stopChans[mcplabel1]

	sm.StopAllServices()

	select {
	case <-pf1Chan:
		// ok
	default:
		t.Errorf("PF1 channel not closed after StopAllServices")
	}
	select {
	case <-mcp1Chan:
		// ok
	default:
		t.Errorf("MCP1 channel not closed after StopAllServices")
	}

	// Check if activeServices map is cleared (internal check, harder to do directly without exposing map)
	// We can infer by trying to stop one again, it should fail as if non-existent.
	err := sm.StopService(pflabel1)
	if err == nil {
		t.Error("StopService after StopAllServices should fail for previously active service")
	}
}

// TODO: Test RestartService
// ... (TODO list and synchronized helper)

// Helper for synchronized access in tests if needed (though callbacks should be goroutine-safe)
var testMu sync.Mutex

func synchronized(f func()) {
	testMu.Lock()
	defer testMu.Unlock()
	f()
}
