package portforwarding

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Mock implementation for StartAndManageIndividualPortForward
func mockStartAndManageIndividualPortForward(cfg PortForwardingConfig, updateFn PortForwardUpdateFunc, t *testing.T) (chan struct{}, error) {
	t.Logf("[mockStartAndManageIndividualPortForward - %s] Called", cfg.InstanceKey)
	mockStop := make(chan struct{})
	updateFn(PortForwardProcessUpdate{InstanceKey: cfg.InstanceKey, StatusMsg: "Mocked Running", Running: true})
	t.Logf("[mockStartAndManageIndividualPortForward - %s] Sent 'Mocked Running' update", cfg.InstanceKey)

	go func() {
		t.Logf("[mockStartAndManageIndividualPortForward - %s GOROUTINE] Waiting for stop signal", cfg.InstanceKey)
		<-mockStop // Wait for this specific PF to be stopped
		t.Logf("[mockStartAndManageIndividualPortForward - %s GOROUTINE] Stop signal received. Sending 'Mocked Stopped' update.", cfg.InstanceKey)
		updateFn(PortForwardProcessUpdate{InstanceKey: cfg.InstanceKey, StatusMsg: "Mocked Stopped", Running: false})
		t.Logf("[mockStartAndManageIndividualPortForward - %s GOROUTINE] Sent 'Mocked Stopped' update", cfg.InstanceKey)
	}()
	return mockStop, nil
}

func mockStartAndManageIndividualPortForwardError(cfg PortForwardingConfig, updateFn PortForwardUpdateFunc, t *testing.T) (chan struct{}, error) {
	t.Logf("[mockStartAndManageIndividualPortForwardError - %s] Called", cfg.InstanceKey)
	updateFn(PortForwardProcessUpdate{InstanceKey: cfg.InstanceKey, StatusMsg: "Mocked Initializing then Error", Running: false})
	t.Logf("[mockStartAndManageIndividualPortForwardError - %s] Sent 'Mocked Initializing then Error' update", cfg.InstanceKey)
	return nil, errors.New("mock startup error")
}

func TestStartAllConfiguredPortForwards_Success(t *testing.T) {
	t.Logf("TestStartAllConfiguredPortForwards_Success: BEGIN")
	originalStartFn := startAndManageIndividualPortForwardFn
	startAndManageIndividualPortForwardFn = func(cfg PortForwardingConfig, updateFn PortForwardUpdateFunc) (chan struct{}, error) {
		return mockStartAndManageIndividualPortForward(cfg, updateFn, t)
	}
	defer func() {
		startAndManageIndividualPortForwardFn = originalStartFn
		t.Logf("TestStartAllConfiguredPortForwards_Success: Restored original startFn")
	}()

	configs := []PortForwardingConfig{
		{InstanceKey: "pf1", Label: "PF1"},
		{InstanceKey: "pf2", Label: "PF2"},
	}

	var updates []PortForwardProcessUpdate
	var mu sync.Mutex
	updateFn := func(update PortForwardProcessUpdate) {
		mu.Lock()
		defer mu.Unlock()
		t.Logf("TestStartAllConfiguredPortForwards_Success: Received update: %+v", update)
		updates = append(updates, update)
	}

	globalStopChan := make(chan struct{})
	t.Logf("TestStartAllConfiguredPortForwards_Success: Calling StartAllConfiguredPortForwards")

	// Close globalStopChan in a separate goroutine after a short delay
	// to allow StartAllConfiguredPortForwards to enter its processing loops and select statements.
	go func() {
		t.Logf("TestStartAllConfiguredPortForwards_Success: Goroutine to close globalStopChan: sleeping briefly...")
		time.Sleep(100 * time.Millisecond) // Ensure PFs have tried to start
		t.Logf("TestStartAllConfiguredPortForwards_Success: Goroutine to close globalStopChan: Closing now.")
		close(globalStopChan)
		t.Logf("TestStartAllConfiguredPortForwards_Success: Goroutine to close globalStopChan: Closed.")
	}()

	startedPfs, err := StartAllConfiguredPortForwards(configs, updateFn, globalStopChan)
	t.Logf("TestStartAllConfiguredPortForwards_Success: StartAllConfiguredPortForwards returned")

	if err != nil {
		t.Fatalf("Expected no error from StartAllConfiguredPortForwards, got %v", err)
	}
	if len(startedPfs) != 2 {
		t.Fatalf("Expected 2 managed port forwards, got %d", len(startedPfs))
	}

	mu.Lock()
	runningCount := 0
	for _, u := range updates {
		if u.StatusMsg == "Mocked Running" && u.Running {
			runningCount++
		}
	}
	mu.Unlock()
	t.Logf("TestStartAllConfiguredPortForwards_Success: Initial running count: %d (expected %d)", runningCount, len(configs))
	if runningCount != len(configs) {
		t.Fatalf("Expected %d 'Mocked Running' updates, got %d. Updates: %+v", len(configs), runningCount, updates)
	}

	maxWait := time.After(5 * time.Second) // Increased timeout for debugging
	stoppedEventuallyCount := 0
	t.Logf("TestStartAllConfiguredPortForwards_Success: Entering loop to wait for 'Mocked Stopped' updates")
Loop:
	for {
		select {
		case <-maxWait:
			mu.Lock()
			t.Logf("TestStartAllConfiguredPortForwards_Success: Timeout waiting for all 'Mocked Stopped' updates. Current updates: %+v", updates)
			mu.Unlock()
			break Loop
		default:
			mu.Lock()
			currentStoppedCount := 0
			for _, u := range updates {
				if u.StatusMsg == "Mocked Stopped" && !u.Running {
					currentStoppedCount++
				}
			}
			mu.Unlock()
			if currentStoppedCount == len(configs) {
				stoppedEventuallyCount = currentStoppedCount
				t.Logf("TestStartAllConfiguredPortForwards_Success: All %d processes sent 'Mocked Stopped'.", len(configs))
				break Loop
			}
			time.Sleep(20 * time.Millisecond) // Increased sleep
		}
	}

	if stoppedEventuallyCount != len(configs) {
		mu.Lock()
		t.Errorf("Expected %d 'Mocked Stopped' updates, got %d. All updates: %+v", len(configs), stoppedEventuallyCount, updates)
		mu.Unlock()
	}

	t.Logf("TestStartAllConfiguredPortForwards_Success: Checking individual stop channels in ManagedPortForwardInfo")
	for i, pfInfo := range startedPfs {
		t.Logf("TestStartAllConfiguredPortForwards_Success: Checking pfInfo %d for %s", i, pfInfo.Config.Label)
		if pfInfo.InitialError != nil {
			t.Errorf("Expected no initial error for pf %s, got %v", pfInfo.Config.Label, pfInfo.InitialError)
		}
		select {
		case <-pfInfo.StopChan:
			t.Logf("TestStartAllConfiguredPortForwards_Success: StopChan for %s is closed (as expected).", pfInfo.Config.Label)
		default:
			t.Errorf("Expected StopChan for %s to be closed by StartAllConfiguredPortForwards logic", pfInfo.Config.Label)
		}
	}
	t.Logf("TestStartAllConfiguredPortForwards_Success: END")
}

func TestStartAllConfiguredPortForwards_IndividualError(t *testing.T) {
	t.Logf("TestStartAllConfiguredPortForwards_IndividualError: BEGIN")
	originalStartFn := startAndManageIndividualPortForwardFn
	startAndManageIndividualPortForwardFn = func(cfg PortForwardingConfig, updateFn PortForwardUpdateFunc) (chan struct{}, error) {
		return mockStartAndManageIndividualPortForwardError(cfg, updateFn, t)
	}
	defer func() {
		startAndManageIndividualPortForwardFn = originalStartFn
		t.Logf("TestStartAllConfiguredPortForwards_IndividualError: Restored original startFn")
	}()

	configs := []PortForwardingConfig{
		{InstanceKey: "pf-err", Label: "PF Error"},
	}

	var updates []PortForwardProcessUpdate
	var mu sync.Mutex
	updateFn := func(update PortForwardProcessUpdate) {
		mu.Lock()
		defer mu.Unlock()
		t.Logf("TestStartAllConfiguredPortForwards_IndividualError: Received update: %+v", update)
		updates = append(updates, update)
	}

	globalStopChan := make(chan struct{})
	t.Logf("TestStartAllConfiguredPortForwards_IndividualError: Calling StartAllConfiguredPortForwards")
	startedPfs, err := StartAllConfiguredPortForwards(configs, updateFn, globalStopChan)
	t.Logf("TestStartAllConfiguredPortForwards_IndividualError: StartAllConfiguredPortForwards returned")

	if err == nil {
		t.Fatal("Expected an error from StartAllConfiguredPortForwards due to individual startup error, got nil")
	}
	if !strings.Contains(err.Error(), "mock startup error") {
		t.Errorf("Expected error to contain 'mock startup error', got %v", err)
	}
	if len(startedPfs) != 1 {
		t.Fatalf("Expected 1 managed port forward info (even on error), got %d", len(startedPfs))
	}
	if startedPfs[0].InitialError == nil {
		t.Errorf("Expected InitialError to be set in ManagedPortForwardInfo for %s", startedPfs[0].Config.Label)
	}

	mu.Lock()
	if len(updates) == 0 {
		t.Fatalf("Expected updates for the failing port forward, got none")
	}
	if updates[0].InstanceKey != "pf-err" || !strings.Contains(updates[0].StatusMsg, "Mocked Initializing then Error") {
		t.Errorf("Unexpected initial update for failing PF: %+v", updates[0])
	}
	mu.Unlock()
	t.Logf("TestStartAllConfiguredPortForwards_IndividualError: END")
}

func TestNonTUIUpdater(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		w.Close()
	}()

	oldStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr
	defer func() {
		os.Stderr = oldStderr
		wErr.Close()
	}()

	tests := []struct {
		name        string
		update      PortForwardProcessUpdate
		expectedOut string
		expectedErr string
		isStdout    bool // true if expected on stdout, false for stderr
	}{
		{
			name:        "Status update",
			update:      PortForwardProcessUpdate{InstanceKey: "test1", LocalPort: "8080", RemotePort: "80", StatusMsg: "Running"},
			expectedOut: "[test1 PF 8080:80] STATUS: Running\n",
			isStdout:    true,
		},
		{
			name:        "Log output",
			update:      PortForwardProcessUpdate{InstanceKey: "test2", LocalPort: "3000", RemotePort: "3000", OutputLog: "Some log line"},
			expectedOut: "[test2 PF 3000:3000] LOG: Some log line\n",
			isStdout:    true,
		},
		{
			name:        "Error update",
			update:      PortForwardProcessUpdate{InstanceKey: "test3", LocalPort: "9090", RemotePort: "90", StatusMsg: "Failed", Error: errors.New("epic fail")},
			expectedErr: "[test3 PF 9090:90] ERROR: Failed - epic fail\n",
			isStdout:    false,
		},
		{
			name:        "Ready forwarding message",
			update:      PortForwardProcessUpdate{InstanceKey: "test4", LocalPort: "8888", RemotePort: "88", StatusMsg: "Active", OutputLog: "Forwarding from 127.0.0.1:8888 -> 88"},
			expectedOut: "[test4 PF 8888:88] STATUS: Active - Forwarding from 127.0.0.1:8888 -> 88\n",
			isStdout:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NonTUIUpdater(tt.update)

			var outBytes bytes.Buffer
			var errBytes bytes.Buffer

			// Need to close the write ends to allow ReadAll to unblock
			if tt.isStdout {
				w.Close() // Close stdout write pipe
				stdoutData, _ := io.ReadAll(r)
				outBytes.Write(stdoutData)
				r, w, _ = os.Pipe() // Re-open for next test iteration
				os.Stdout = w
			} else {
				wErr.Close() // Close stderr write pipe
				stderrData, _ := io.ReadAll(rErr)
				errBytes.Write(stderrData)
				rErr, wErr, _ = os.Pipe() // Re-open for next test iteration
				os.Stderr = wErr
			}

			if tt.isStdout {
				if !strings.Contains(outBytes.String(), tt.expectedOut) {
					t.Errorf("Expected stdout to contain %q, got %q", tt.expectedOut, outBytes.String())
				}
			} else {
				if !strings.Contains(errBytes.String(), tt.expectedErr) {
					t.Errorf("Expected stderr to contain %q, got %q", tt.expectedErr, errBytes.String())
				}
			}
		})
	}
}
