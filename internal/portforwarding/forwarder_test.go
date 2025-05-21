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

	cfg := PortForwardConfig{
		InstanceKey: "test-pf",
		ServiceName: "TestService", // Matched with mock initial status
		Namespace:   "test-ns",
		LocalPort:   "8080",
		RemotePort:  "80",
		KubeContext: "test-ctx",
		Label:       "test-label",
	}

	var updates []PortForwardProcessUpdate
	var mu sync.Mutex
	updateFn := func(update PortForwardProcessUpdate) {
		mu.Lock()
		defer mu.Unlock()
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

	syncChan := make(chan bool)
	t.Logf("TestStartAndManageIndividualPortForward_Success: Waiting for ready update signal via syncChan for '%s'", cfg.Label)
	go func(){
		loopStart := time.Now()
		for {
			if time.Since(loopStart) > 2*time.Second {
				t.Logf("TestStartAndManageIndividualPortForward_Success: Timeout in syncChan goroutine for '%s'", cfg.Label)
				syncChan <- false // Signal timeout
				return
			}
			mu.Lock()
			readyFound := false
			for _, u := range updates {
				if u.Running && u.StatusMsg == "Forwarding from 127.0.0.1:8080 to 80"{
					readyFound = true
					break
				}
			}
			mu.Unlock()
			if readyFound {
				t.Logf("TestStartAndManageIndividualPortForward_Success: Ready update found for '%s'. Signaling syncChan.", cfg.Label)
				syncChan <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	if !<-syncChan {
        t.Fatalf("TestStartAndManageIndividualPortForward_Success: Timed out waiting for the ready update for '%s'", cfg.Label)
    }
	t.Logf("TestStartAndManageIndividualPortForward_Success: Received signal from syncChan for '%s'", cfg.Label)

	mu.Lock()
	defer mu.Unlock()

	expectedUpdates := []PortForwardProcessUpdate{
		{InstanceKey: "test-pf", ServiceName: "TestService", Namespace: "test-ns", LocalPort: "8080", RemotePort: "80", StatusMsg: "Initializing", Running: false},
		{InstanceKey: "test-pf", ServiceName: "TestService", Namespace: "test-ns", LocalPort: "8080", RemotePort: "80", StatusMsg: "Initializing TestService...", OutputLog: "", Running: true}, // Initial status from mock, should be Running:true
		{InstanceKey: "test-pf", ServiceName: "TestService", Namespace: "test-ns", LocalPort: "8080", RemotePort: "80", StatusMsg: "Forwarding from 127.0.0.1:8080 to 80", OutputLog: "", Running: true}, // Ready status from mock via bridgeFn
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

	cfg := PortForwardConfig{InstanceKey: "error-pf", ServiceName: "ErrorService", Namespace: "err-ns", LocalPort: "123", RemotePort: "456", Label: "error-label"}
	var updates []PortForwardProcessUpdate
	var mu sync.Mutex
	updateFn := func(update PortForwardProcessUpdate) {
		mu.Lock()
		defer mu.Unlock()
		t.Logf("TestStartAndManageIndividualPortForward_KubeError: Received update: %+v", update)
		updates = append(updates, update)
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

	expectedInitializingUpdate := PortForwardProcessUpdate{
		InstanceKey: "error-pf", ServiceName: "ErrorService", Namespace: "err-ns", LocalPort: "123", RemotePort: "456",
		StatusMsg: "Initializing", Running: false,
	}
	if !reflect.DeepEqual(updates[0], expectedInitializingUpdate) {
		t.Errorf("Update 0 for '%s': expected %+v, got %+v", cfg.Label, expectedInitializingUpdate, updates[0])
	}

	// Define the expected error update structure
	expectedErrorUpdate := PortForwardProcessUpdate{
		InstanceKey: "error-pf", ServiceName: "ErrorService", Namespace: "err-ns", LocalPort: "123", RemotePort: "456",
		StatusMsg:   "Error",
		OutputLog:   "kube error",
		Error:       fmt.Errorf("kube error"), // This is the expected error form
		Running:     false,
	}

	// Custom comparison for the update that contains an error (updates[1])
	actualErrorUpdate := updates[1]

	// 1. Check error presence and message
	if actualErrorUpdate.Error == nil {
		t.Errorf("Update 1 for '%s': expected an error, but got nil", cfg.Label)
	} else if actualErrorUpdate.Error.Error() != expectedErrorUpdate.Error.Error() { // Compare error messages
		t.Errorf("Update 1 for '%s': error message mismatch. Expected %q, got %q", 
			cfg.Label, expectedErrorUpdate.Error.Error(), actualErrorUpdate.Error.Error())
	}

	// 2. Compare the rest of the struct fields by temporarily nil-ing out errors
	actualErrorUpdateForDeepEqual := actualErrorUpdate
	actualErrorUpdateForDeepEqual.Error = nil
	expectedStructForDeepEqual := expectedErrorUpdate
	expectedStructForDeepEqual.Error = nil

	if !reflect.DeepEqual(actualErrorUpdateForDeepEqual, expectedStructForDeepEqual) {
		t.Errorf("Update 1 for '%s' (non-error fields): expected %+v (error field ignored), got %+v (error field ignored)", 
			cfg.Label, expectedStructForDeepEqual, actualErrorUpdateForDeepEqual)
	}

	t.Logf("TestStartAndManageIndividualPortForward_KubeError: END")
} 