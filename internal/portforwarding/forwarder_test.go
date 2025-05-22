package portforwarding

import (
	"envctl/internal/kube"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testPortForwardUpdate struct for collecting update parameters in tests.
// Adjusted to match the new PortForwardUpdateFunc signature implicitly.
type testPortForwardUpdate struct {
	Label        string
	StatusDetail PortForwardStatusDetail
	IsOpReady    bool
	OperationErr error
	OutputLog    string
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
		t.Logf("[Mock KubeStartPortForwardFn - Success] Called for label '%s'", label)
		// Simulate StartPortForwardClientGo sending "Initializing" first
		initMsg := fmt.Sprintf("Mock Kube Init for %s", label)
		bridgeFn("Initializing", initMsg, false, false)
		
		go func() {
			t.Logf("[Mock KubeStartPortForwardClientGoFn - Success GOROUTINE %s] Simulating async ready signal by calling bridgeFn", label)
			time.Sleep(25 * time.Millisecond) // Increased delay slightly to ensure it's processed after init
			fwdDetail := "Forwarding from 127.0.0.1:8080 to 80"
			bridgeFn("ForwardingActive", fwdDetail, false, true) // Simulate kube saying it's ready
			t.Logf("[Mock KubeStartPortForwardClientGoFn - Success GOROUTINE %s] bridgeFn called for ready signal", label)
		}()
		return mockStopChan, "Initializing", nil // initialStatus string from Kube layer
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
	updateFn := func(serviceLabel string, statusDetail PortForwardStatusDetail, isOpReady bool, operationErr error) {
		mu.Lock()
		defer mu.Unlock()
		// In a real scenario, the bridgeCallback in forwarder.go would get the outputLog from sendUpdate.
		// Here, we simulate that bridgeCallback would correctly parse the status and pass relevant details.
		// For simplicity in the test mock, we won't capture outputLog directly into testPortForwardUpdate from this mock updateFn,
		// but from the KubeStartPortForwardFn mock's call to bridgeFn.
		// We should capture the outputLog from the *mocked* bridgeFn calls if we want to assert it.
		// However, testPortForwardUpdate doesn't have OutputLog yet. Let's add it.
		
		// To correctly test, the `updateFn` here is what `StartAndManageIndividualPortForward` calls.
		// The `bridgeFn` inside the mocked `KubeStartPortForwardFn` is what `StartPortForwardClientGo` calls.
		// The `outputLog` from `bridgeFn` is used by `forwarder.go`'s `bridgeCallback` to form `operationErr` or details for `reportIfChanged`.
		// The `reportIfChanged` then calls this `updateFn`.
		// For this test, we care about what `updateFn` receives.
		
		// The outputLog associated with the status detail would be set by the real bridgeCallback.
		// Our testPortForwardUpdate should capture this.
		var capturedOutputLog string
		if statusDetail == StatusDetailForwardingActive {
			capturedOutputLog = "Forwarding from 127.0.0.1:8080 to 80" // From the mock
		} else if statusDetail == StatusDetailInitializing {
			capturedOutputLog = fmt.Sprintf("Mock Kube Init for %s", serviceLabel) // From the mock
		}

		update := testPortForwardUpdate{Label: serviceLabel, StatusDetail: statusDetail, IsOpReady: isOpReady, OperationErr: operationErr, OutputLog: capturedOutputLog}
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

	// Wait for the expected "Ready" update (ForwardingActive)
	syncChan := make(chan bool)
	t.Logf("TestStartAndManageIndividualPortForward_Success: Waiting for ForwardingActive update signal for '%s'", cfg.Label)
	go func() {
		for i := 0; i < 400; i++ { // Increased iterations for potentially slower CI
			mu.Lock()
			readyFound := false
			for _, u := range updates {
				if u.StatusDetail == StatusDetailForwardingActive && u.IsOpReady && u.Label == cfg.Label && u.OperationErr == nil {
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
		mu.Lock()
		finalUpdates := updates
		mu.Unlock()
		t.Fatalf("TestStartAndManageIndividualPortForward_Success: Timed out waiting for the ForwardingActive update for '%s'. Updates received: %+v", cfg.Label, finalUpdates)
	}
	t.Logf("TestStartAndManageIndividualPortForward_Success: Received signal from syncChan for '%s'", cfg.Label)

	mu.Lock()
	defer mu.Unlock()

	expectedUpdates := []testPortForwardUpdate{
		{Label: "test-label", StatusDetail: StatusDetailInitializing, IsOpReady: false, OperationErr: nil, OutputLog: "Mock Kube Init for test-label"},
		{Label: "test-label", StatusDetail: StatusDetailForwardingActive, IsOpReady: true, OperationErr: nil, OutputLog: "Forwarding from 127.0.0.1:8080 to 80"},
	}

	t.Logf("TestStartAndManageIndividualPortForward_Success: Comparing %d actual updates with %d expected updates for '%s'", len(updates), len(expectedUpdates), cfg.Label)

	assert.Len(t, updates, len(expectedUpdates), "Number of updates should match")
	for i, expected := range expectedUpdates {
		if i < len(updates) {
			actual := updates[i]
			assert.Equal(t, expected.Label, actual.Label, "Update %d Label mismatch", i)
			assert.Equal(t, expected.StatusDetail, actual.StatusDetail, "Update %d StatusDetail mismatch", i)
			assert.Equal(t, expected.IsOpReady, actual.IsOpReady, "Update %d IsOpReady mismatch", i)
			assert.True(t, errors.Is(actual.OperationErr, expected.OperationErr), "Update %d OperationErr mismatch. Got %v, want %v", i, actual.OperationErr, expected.OperationErr)
			// OutputLog for ForwardingActive might have specific port numbers based on GetPorts(), 
			// but our mock sends a fixed string. For Initializing, it's also mocked.
			assert.Equal(t, expected.OutputLog, actual.OutputLog, "Update %d OutputLog mismatch", i)
		} 
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

	expectedKubeErr := errors.New("kube error direct return")
	// This mock simulates KubeStartPortForwardClientGo returning an error *immediately*,
	// before it would have a chance to send an "Initializing" update itself.
	KubeStartPortForwardFn = func(
		kubeContext string, namespace string, serviceName string, portMap string, label string, bridgeFn kube.SendUpdateFunc,
	) (chan struct{}, string, error) {
		t.Logf("[Mock KubeStartPortForwardClientGoFn - Error] Called for label '%s', returning error: %v immediately", label, expectedKubeErr)
		// No call to bridgeFn("Initializing", ...) because the error is immediate
		return nil, "KubeInitStatus: Immediate Error", expectedKubeErr 
	}

	cfg := PortForwardingConfig{Label: "error-label", ServiceName: "ErrorService", Namespace: "err-ns", LocalPort: "123", RemotePort: "456"}
	var updates []testPortForwardUpdate
	var mu sync.Mutex
	updateFn := func(serviceLabel string, statusDetail PortForwardStatusDetail, isOpReady bool, operationErr error) {
		mu.Lock()
		defer mu.Unlock()
		// Simulate how bridgeCallback in forwarder.go would extract detail for the updateFn
		var detailMsg string
		if operationErr != nil {
			detailMsg = operationErr.Error()
		} else if statusDetail == StatusDetailInitializing { // Though not expected in this specific test flow
			detailMsg = "Initializing by test"
		}
		update := testPortForwardUpdate{Label: serviceLabel, StatusDetail: statusDetail, IsOpReady: isOpReady, OperationErr: operationErr, OutputLog: detailMsg}
		t.Logf("TestStartAndManageIndividualPortForward_KubeError: Received update: %+v", update)
		updates = append(updates, update)
	}

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: Calling StartAndManageIndividualPortForward for '%s'", cfg.Label)
	_, err := StartAndManageIndividualPortForward(cfg, updateFn)
	t.Logf("TestStartAndManageIndividualPortForward_KubeError: StartAndManageIndividualPortForward for '%s' returned", cfg.Label)

	if err == nil {
		t.Fatal("Expected an error from StartAndManageIndividualPortForward, got nil")
	}
	if !errors.Is(err, expectedKubeErr) {
		t.Errorf("Expected error %v, got %v", expectedKubeErr, err)
	}

	mu.Lock()
	defer mu.Unlock()

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: Comparing actual updates with expected for '%s'", cfg.Label)

	// Since KubeStartPortForwardFn returns an error immediately, StartPortForwardClientGo (the real one)
	// would not have sent an "Initializing" update yet. StartAndManageIndividualPortForward
	// catches this initialErr and directly reports "Failed".
	expectedUpdates := []testPortForwardUpdate{
		{Label: "error-label", StatusDetail: StatusDetailFailed, IsOpReady: false, OperationErr: expectedKubeErr, OutputLog: expectedKubeErr.Error()},
	}

	if len(updates) != len(expectedUpdates) {
		t.Fatalf("Expected %d update(s), got %d. Updates: %+v", len(expectedUpdates), len(updates), updates)
	}

	for i, expected := range expectedUpdates {
		actual := updates[i]
		assert.Equal(t, expected.Label, actual.Label, "Update %d Label mismatch", i)
		assert.Equal(t, expected.StatusDetail, actual.StatusDetail, "Update %d StatusDetail mismatch", i)
		assert.Equal(t, expected.IsOpReady, actual.IsOpReady, "Update %d IsOpReady mismatch", i)
		assert.True(t, errors.Is(actual.OperationErr, expected.OperationErr), "Update %d OperationErr mismatch. Got %v, want %v", i, actual.OperationErr, expected.OperationErr)
		assert.Equal(t, expected.OutputLog, actual.OutputLog, "Update %d OutputLog mismatch", i)
	}
	t.Logf("TestStartAndManageIndividualPortForward_KubeError: END")
}
