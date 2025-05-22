package portforwarding

import (
	"envctl/internal/kube"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// testPortForwardUpdate struct for collecting update parameters in tests.
// Adjusted to match the new PortForwardUpdateFunc signature implicitly.
type testPortForwardUpdate struct {
	Label        string
	StatusDetail string // Renamed from StatusMsg
	IsOpReady    bool   // Renamed from IsReady
	OperationErr error  // New field for the error object
	// OutputLog is removed as it's now logged directly by the forwarder via pkg/logging
}

// TestStartAndManageIndividualPortForward_Success tests the successful startup of a port forward.
func TestStartAndManageIndividualPortForward_Success(t *testing.T) {
	t.Logf("TestStartAndManageIndividualPortForward_Success: BEGIN")
	originalKubeStartFn := KubeStartPortForwardFn
	defer func() {
		KubeStartPortForwardFn = originalKubeStartFn
		t.Logf("TestStartAndManageIndividualPortForward_Success: Restored original KubeStartPortForwardClientGoFn")
	}()

	mockStopChan := make(chan struct{})
	KubeStartPortForwardFn = func(
		kubeContext string,
		namespace string,
		serviceName string,
		portMap string,
		label string,
		bridgeFn kube.SendUpdateFunc, // kube.SendUpdateFunc is (status, outputLog string, isError, isReady bool)
	) (chan struct{}, string, error) {
		t.Logf("[Mock KubeStartPortForwardClientGoFn - Success] Called for label '%s'", label)
		go func() {
			t.Logf("[Mock KubeStartPortForwardClientGoFn - Success GOROUTINE %s] Simulating async ready signal by calling bridgeFn", label)
			time.Sleep(5 * time.Millisecond)
			// bridgeFn still uses its old signature (status, outputLog, isError, isReady)
			// The forwarder's bridgeCallback will translate this to the new PortForwardUpdateFunc signature.
			bridgeFn("Forwarding from 127.0.0.1:8080 to 80", "", false, true) // Simulate kube saying it's ready
			t.Logf("[Mock KubeStartPortForwardClientGoFn - Success GOROUTINE %s] bridgeFn called for ready signal", label)
		}()
		return mockStopChan, "KubeInitStatus: OK", nil // initialStatus for kube layer
	}

	cfg := PortForwardingConfig{
		Label:       "test-label",
		ServiceName: "TestService",
		Namespace:   "test-ns",
		LocalPort:   "8080",
		RemotePort:  "80",
		KubeContext: "test-ctx",
	}

	var updates []testPortForwardUpdate
	var mu sync.Mutex
	// updateFn now matches the new PortForwardUpdateFunc signature
	updateFn := func(serviceLabel, statusDetail string, isOpReady bool, operationErr error) {
		mu.Lock()
		defer mu.Unlock()
		update := testPortForwardUpdate{Label: serviceLabel, StatusDetail: statusDetail, IsOpReady: isOpReady, OperationErr: operationErr}
		t.Logf("TestStartAndManageIndividualPortForward_Success: Received update: %+v", update)
		updates = append(updates, update)
	}

	t.Logf("TestStartAndManageIndividualPortForward_Success: Calling StartAndManageIndividualPortForward for '%s'", cfg.Label)
	returnedStopChan, err := StartAndManageIndividualPortForward(cfg, updateFn)
	t.Logf("TestStartAndManageIndividualPortForward_Success: StartAndManageIndividualPortForward for '%s' returned", cfg.Label)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if returnedStopChan != mockStopChan {
		t.Errorf("Expected returned stop channel to be the mockStopChan")
	}

	// Wait for the expected "Ready" update (which is now IsOpReady = true)
	syncChan := make(chan bool)
	t.Logf("TestStartAndManageIndividualPortForward_Success: Waiting for ready update signal via syncChan for '%s'", cfg.Label)
	go func() {
		for i := 0; i < 200; i++ { 
			mu.Lock()
			readyFound := false
			for _, u := range updates {
				// Check for the specific ready message from the mock KubeStartPortForwardFn via bridge
				// The forwarder's bridgeCallback translates kube's (status, outputLog, isError, isReady) 
				// to the new (serviceLabel, statusDetail, isOpReady, operationErr).
				// The kubeStatus "Forwarding from..." becomes statusDetail.
				if u.IsOpReady && u.StatusDetail == "Forwarding from 127.0.0.1:8080 to 80" && u.Label == cfg.Label && u.OperationErr == nil {
					readyFound = true
					break
				}
			}
			mu.Unlock()
			if readyFound {
				syncChan <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		syncChan <- false // Timeout
	}()

	if !<-syncChan {
		t.Fatalf("TestStartAndManageIndividualPortForward_Success: Timed out waiting for the ready update for '%s'", cfg.Label)
	}
	t.Logf("TestStartAndManageIndividualPortForward_Success: Received signal from syncChan for '%s'", cfg.Label)

	mu.Lock()
	defer mu.Unlock()

	// Expected updates reflect the new signature and behavior.
	// OutputLog is no longer part of testPortForwardUpdate as it's logged directly.
	expectedUpdates := []testPortForwardUpdate{
		{Label: "test-label", StatusDetail: "Starting", IsOpReady: false, OperationErr: nil},
		// The direct initialStatus from kube.StartPortForwardClientGo is not directly passed to PortForwardUpdateFunc in the same way.
		// The bridgeCallback will be the one sending updates based on what kube.SendUpdateFunc provides.
		// The mock kube.SendUpdateFunc directly sends "Forwarding from..."
		{Label: "test-label", StatusDetail: "Forwarding from 127.0.0.1:8080 to 80", IsOpReady: true, OperationErr: nil},
	}

	t.Logf("TestStartAndManageIndividualPortForward_Success: Comparing %d actual updates with %d expected updates for '%s'", len(updates), len(expectedUpdates), cfg.Label)
	
	// Check a subset or key updates due to async nature and internal logging not captured here.
	foundStarting := false
	foundRunning := false
	for _, u := range updates {
		if u.Label == "test-label" && u.StatusDetail == "Starting" && !u.IsOpReady && u.OperationErr == nil {
			foundStarting = true
		}
		if u.Label == "test-label" && u.StatusDetail == "Forwarding from 127.0.0.1:8080 to 80" && u.IsOpReady && u.OperationErr == nil {
			foundRunning = true
		}
	}

	if !foundStarting {
		t.Errorf("Expected 'Starting' update not found. Updates: %+v", updates)
	}
	if !foundRunning {
		t.Errorf("Expected 'Forwarding from...' (running) update not found. Updates: %+v", updates)
	}

	t.Logf("TestStartAndManageIndividualPortForward_Success: END")
}

// TestStartAndManageIndividualPortForward_KubeError tests error handling when the kube client call fails.
func TestStartAndManageIndividualPortForward_KubeError(t *testing.T) {
	t.Logf("TestStartAndManageIndividualPortForward_KubeError: BEGIN")
	originalKubeStartFn := KubeStartPortForwardFn
	defer func() {
		KubeStartPortForwardFn = originalKubeStartFn
		t.Logf("TestStartAndManageIndividualPortForward_KubeError: Restored original KubeStartPortForwardClientGoFn")
	}()

	expectedKubeErr := errors.New("kube error")
	KubeStartPortForwardFn = func(
		kubeContext string,
		namespace string,
		serviceName string,
		portMap string,
		label string,
		bridgeFn kube.SendUpdateFunc,
	) (chan struct{}, string, error) {
		t.Logf("[Mock KubeStartPortForwardClientGoFn - Error] Called for label '%s', returning error: %v", label, expectedKubeErr)
		return nil, "KubeInitStatus: Error", expectedKubeErr // Simulate initial error from kube layer
	}

	cfg := PortForwardingConfig{Label: "error-label", ServiceName: "ErrorService", Namespace: "err-ns", LocalPort: "123", RemotePort: "456"}
	var updates []testPortForwardUpdate
	var mu sync.Mutex
	// updateFn now matches the new PortForwardUpdateFunc signature
	updateFn := func(serviceLabel, statusDetail string, isOpReady bool, operationErr error) {
		mu.Lock()
		defer mu.Unlock()
		update := testPortForwardUpdate{Label: serviceLabel, StatusDetail: statusDetail, IsOpReady: isOpReady, OperationErr: operationErr}
		t.Logf("TestStartAndManageIndividualPortForward_KubeError: Received update: %+v", update)
		updates = append(updates, update)
	}

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: Calling StartAndManageIndividualPortForward for '%s'", cfg.Label)
	_, err := StartAndManageIndividualPortForward(cfg, updateFn)
	t.Logf("TestStartAndManageIndividualPortForward_KubeError: StartAndManageIndividualPortForward for '%s' returned", cfg.Label)

	if err == nil {
		t.Fatal("Expected an error from StartAndManageIndividualPortForward, got nil")
	}
	// The error returned by StartAndManageIndividualPortForward should be the initialErr from KubeStartPortForwardFn
	if !errors.Is(err, expectedKubeErr) {
		t.Errorf("Expected error %v, got %v", expectedKubeErr, err)
	}

	mu.Lock()
	defer mu.Unlock()

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: Comparing actual updates with expected for '%s'", cfg.Label)
	
	// Expected updates: Initial "Starting", then the "Failed" update due to initialErr
	expectedUpdates := []testPortForwardUpdate{
		{Label: "error-label", StatusDetail: "Starting", IsOpReady: false, OperationErr: nil},
		{Label: "error-label", StatusDetail: fmt.Sprintf("Failed: %v", expectedKubeErr), IsOpReady: false, OperationErr: expectedKubeErr},
	}

	if len(updates) != len(expectedUpdates) {
		t.Fatalf("Expected %d updates, got %d. Updates: %+v", len(expectedUpdates), len(updates), updates)
	}

	for i, expected := range expectedUpdates {
		actual := updates[i]
		if actual.Label != expected.Label || 
		   actual.StatusDetail != expected.StatusDetail || 
		   actual.IsOpReady != expected.IsOpReady || 
		   !errors.Is(actual.OperationErr, expected.OperationErr) { // Use errors.Is for error comparison
			t.Errorf("Update %d for '%s': expected %+v, got %+v", i, cfg.Label, expected, actual)
		}
	}
	t.Logf("TestStartAndManageIndividualPortForward_KubeError: END")
}
