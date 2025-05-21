package portforwarding

import (
	"envctl/internal/kube"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// Simple struct for collecting update parameters in tests
type testPortForwardUpdate struct {
	Label     string
	StatusMsg string
	OutputLog string
	IsError   bool
	IsReady   bool
	// Error field is not part of the simple callback but can be inferred for assertions
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
	// Correctly typed mock function matching kube.StartPortForwardClientGo's signature,
	// which uses kube.SendUpdateFunc for the callback.
	KubeStartPortForwardFn = func(
		kubeContext string,
		namespace string,
		serviceName string,
		portMap string,
		label string,
		bridgeFn kube.SendUpdateFunc, // Use the actual exported type from the kube package
	) (chan struct{}, string, error) {
		t.Logf("[Mock KubeStartPortForwardClientGoFn - Success] Called for label '%s'", label)
		// Simulate a successful call that invokes the bridgeFn to signal readiness
		go func() { // Simulate async ready signal
			t.Logf("[Mock KubeStartPortForwardClientGoFn - Success GOROUTINE %s] Simulating async ready signal by calling bridgeFn", label)
			time.Sleep(5 * time.Millisecond) // brief delay to simulate async
			bridgeFn("Forwarding from 127.0.0.1:8080 to 80", "", false, true)
			t.Logf("[Mock KubeStartPortForwardClientGoFn - Success GOROUTINE %s] bridgeFn called for ready signal", label)
		}()
		return mockStopChan, "Initializing TestService...", nil // Initial status from kube layer
	}

	cfg := PortForwardingConfig{
		InstanceKey: "test-pf",
		ServiceName: "TestService", // Matched with mock initial status
		Namespace:   "test-ns",
		LocalPort:   "8080",
		RemotePort:  "80",
		KubeContext: "test-ctx",
		Label:       "test-label",
	}

	var updates []testPortForwardUpdate
	var mu sync.Mutex
	updateFn := func(label, status, outputLog string, isError, isReady bool) {
		mu.Lock()
		defer mu.Unlock()
		t.Logf("TestStartAndManageIndividualPortForward_Success: Received update: %+v", testPortForwardUpdate{Label: label, StatusMsg: status, OutputLog: outputLog, IsError: isError, IsReady: isReady})
		updates = append(updates, testPortForwardUpdate{Label: label, StatusMsg: status, OutputLog: outputLog, IsError: isError, IsReady: isReady})
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

	syncChan := make(chan bool)
	t.Logf("TestStartAndManageIndividualPortForward_Success: Waiting for ready update signal via syncChan for '%s'", cfg.Label)
	go func() {
		for i := 0; i < 200; i++ { // Timeout after 2s approx
			mu.Lock()
			readyFound := false
			for _, u := range updates {
				// Check for the specific ready message from the mock KubeStartPortForwardFn via bridge
				if u.IsReady && u.StatusMsg == "Forwarding from 127.0.0.1:8080 to 80" && u.Label == cfg.Label {
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

	expectedUpdates := []testPortForwardUpdate{
		{Label: "test-label", StatusMsg: "Initializing", OutputLog: "", IsError: false, IsReady: false},
		{Label: "test-label", StatusMsg: "Initializing TestService...", OutputLog: "", IsError: false, IsReady: false},         // From initialStatus return
		{Label: "test-label", StatusMsg: "Forwarding from 127.0.0.1:8080 to 80", OutputLog: "", IsError: false, IsReady: true}, // From bridgeFn
	}

	t.Logf("TestStartAndManageIndividualPortForward_Success: Comparing %d actual updates with %d expected updates for '%s'", len(updates), len(expectedUpdates), cfg.Label)
	if len(updates) != len(expectedUpdates) {
		t.Fatalf("Expected %d updates, got %d. Updates: %+v", len(expectedUpdates), len(updates), updates)
	}

	for i, expected := range expectedUpdates {
		if !reflect.DeepEqual(updates[i], expected) {
			t.Errorf("Update %d for '%s': expected %+v, got %+v", i, cfg.Label, expected, updates[i])
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

	expectedErr := errors.New("kube error")
	// Correctly typed mock function
	KubeStartPortForwardFn = func(
		kubeContext string,
		namespace string,
		serviceName string,
		portMap string,
		label string,
		bridgeFn kube.SendUpdateFunc, // Use the actual exported type
	) (chan struct{}, string, error) {
		t.Logf("[Mock KubeStartPortForwardClientGoFn - Error] Called for label '%s', returning error: %v", label, expectedErr)
		return nil, "", expectedErr
	}

	cfg := PortForwardingConfig{InstanceKey: "error-pf", ServiceName: "ErrorService", Namespace: "err-ns", LocalPort: "123", RemotePort: "456", Label: "error-label"}
	var updates []testPortForwardUpdate
	var mu sync.Mutex
	updateFn := func(label, status, outputLog string, isError, isReady bool) {
		mu.Lock()
		defer mu.Unlock()
		t.Logf("TestStartAndManageIndividualPortForward_KubeError: Received update: %+v", testPortForwardUpdate{Label: label, StatusMsg: status, OutputLog: outputLog, IsError: isError, IsReady: isReady})
		updates = append(updates, testPortForwardUpdate{Label: label, StatusMsg: status, OutputLog: outputLog, IsError: isError, IsReady: isReady})
	}

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: Calling StartAndManageIndividualPortForward for '%s'", cfg.Label)
	_, err := StartAndManageIndividualPortForward(cfg, updateFn)
	t.Logf("TestStartAndManageIndividualPortForward_KubeError: StartAndManageIndividualPortForward for '%s' returned", cfg.Label)

	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	mu.Lock()
	defer mu.Unlock()

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: Comparing actual updates with expected for '%s'", cfg.Label)
	// Expect Initializing and then the Error update
	if len(updates) != 2 {
		t.Fatalf("Expected 2 updates, got %d. Updates: %+v", len(updates), updates)
	}

	expectedInitializingUpdate := testPortForwardUpdate{
		Label:     "error-label",
		StatusMsg: "Initializing",
		OutputLog: "",
		IsError:   false,
		IsReady:   false,
	}
	if !reflect.DeepEqual(updates[0], expectedInitializingUpdate) {
		t.Errorf("Update 0 for '%s': expected %+v, got %+v", cfg.Label, expectedInitializingUpdate, updates[0])
	}

	// Define the expected error update structure
	expectedErrorUpdate := testPortForwardUpdate{
		Label:     "error-label",
		StatusMsg: fmt.Sprintf("Failed to initialize port-forward: %v", expectedErr),
		OutputLog: "",
		IsError:   true,
		IsReady:   false,
	}

	// Custom comparison for the update that contains an error (updates[1])
	actualErrorUpdate := updates[1]

	// 1. Check error presence and message
	if actualErrorUpdate.IsError == false {
		t.Errorf("Update 1 for '%s': expected an error, but got nil", cfg.Label)
	} else if actualErrorUpdate.StatusMsg != expectedErrorUpdate.StatusMsg { // Compare error messages
		t.Errorf("Update 1 for '%s': error message mismatch. Expected %q, got %q",
			cfg.Label, expectedErrorUpdate.StatusMsg, actualErrorUpdate.StatusMsg)
	}

	// 2. Compare the rest of the struct fields by temporarily nil-ing out errors
	actualErrorUpdateForDeepEqual := actualErrorUpdate
	actualErrorUpdateForDeepEqual.IsError = false
	expectedStructForDeepEqual := expectedErrorUpdate
	expectedStructForDeepEqual.IsError = false

	if !reflect.DeepEqual(actualErrorUpdateForDeepEqual, expectedStructForDeepEqual) {
		t.Errorf("Update 1 for '%s' (non-error fields): expected %+v (error field ignored), got %+v (error field ignored)",
			cfg.Label, expectedStructForDeepEqual, actualErrorUpdateForDeepEqual)
	}

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: END")
}
