package managers

import (
	"envctl/internal/reporting"
	"sync"
	"testing"
	"time"
)

// mockK8sHealthChecker is a mock implementation of health checking
type mockK8sHealthChecker struct {
	healthStatus map[string]bool
	errorStatus  map[string]error
	mu           sync.Mutex
}

func newMockK8sHealthChecker() *mockK8sHealthChecker {
	return &mockK8sHealthChecker{
		healthStatus: make(map[string]bool),
		errorStatus:  make(map[string]error),
	}
}

func (m *mockK8sHealthChecker) setHealth(context string, healthy bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthStatus[context] = healthy
	m.errorStatus[context] = err
}

func (m *mockK8sHealthChecker) checkHealth(context string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.healthStatus[context], m.errorStatus[context]
}

func TestStartK8sConnectionServices(t *testing.T) {
	tests := []struct {
		name     string
		configs  []K8sConnectionConfig
		expected int
	}{
		{
			name:     "no configs",
			configs:  []K8sConnectionConfig{},
			expected: 0,
		},
		{
			name: "single MC connection",
			configs: []K8sConnectionConfig{
				{
					Name:                "k8s-mc-test",
					ContextName:         "test-context",
					IsMC:                true,
					HealthCheckInterval: 100 * time.Millisecond,
				},
			},
			expected: 1,
		},
		{
			name: "MC and WC connections",
			configs: []K8sConnectionConfig{
				{
					Name:                "k8s-mc-test",
					ContextName:         "mc-context",
					IsMC:                true,
					HealthCheckInterval: 100 * time.Millisecond,
				},
				{
					Name:                "k8s-wc-test",
					ContextName:         "wc-context",
					IsMC:                false,
					HealthCheckInterval: 100 * time.Millisecond,
				},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := reporting.NewConsoleReporter()
			var wg sync.WaitGroup

			stopChans := StartK8sConnectionServices(tt.configs, reporter, &wg)

			if len(stopChans) != tt.expected {
				t.Errorf("expected %d stop channels, got %d", tt.expected, len(stopChans))
			}

			// Verify all services are running
			for label, stopChan := range stopChans {
				select {
				case <-stopChan:
					t.Errorf("service %s stopped unexpectedly", label)
				default:
					// Service is running as expected
				}
			}

			// Stop all services
			for _, stopChan := range stopChans {
				close(stopChan)
			}

			// Wait for all services to stop
			done := make(chan bool)
			go func() {
				wg.Wait()
				done <- true
			}()

			select {
			case <-done:
				// All services stopped successfully
			case <-time.After(2 * time.Second):
				t.Error("timeout waiting for services to stop")
			}
		})
	}
}

func TestK8sConnectionService_Lifecycle(t *testing.T) {
	// Skip this test as it requires mocking kubernetes client
	t.Skip("Skipping test that requires kubernetes client mocking")
}

func TestK8sConnectionService_StateReporting(t *testing.T) {
	// Create a custom reporter to capture state changes
	stateChanges := make([]reporting.ServiceState, 0)
	var mu sync.Mutex

	reporter := &testK8sReporter{
		reportFunc: func(update reporting.ManagedServiceUpdate) {
			mu.Lock()
			stateChanges = append(stateChanges, update.State)
			mu.Unlock()
		},
	}

	config := K8sConnectionConfig{
		Name:                "k8s-state-test",
		ContextName:         "test-context",
		IsMC:                true,
		HealthCheckInterval: 50 * time.Millisecond,
	}

	service := &K8sConnectionService{
		config:   config,
		reporter: reporter,
		stopChan: make(chan struct{}),
	}

	// Start the service in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.Run()
	}()

	// Give service time to start and report initial state
	time.Sleep(100 * time.Millisecond)

	// Stop the service
	close(service.stopChan)
	wg.Wait()

	// Check state transitions
	mu.Lock()
	defer mu.Unlock()

	// We should at least see Starting and Stopped states
	if len(stateChanges) < 2 {
		t.Errorf("expected at least 2 state changes, got %d", len(stateChanges))
		t.Logf("State changes: %v", stateChanges)
		return
	}

	// Verify we have Starting state
	hasStarting := false
	hasStopped := false
	for _, state := range stateChanges {
		if state == reporting.StateStarting {
			hasStarting = true
		}
		if state == reporting.StateStopped {
			hasStopped = true
		}
	}

	if !hasStarting {
		t.Error("expected StateStarting in state transitions")
	}
	if !hasStopped {
		t.Error("expected StateStopped in state transitions")
	}
}

func TestK8sConnectionService_ConcurrentStop(t *testing.T) {
	reporter := reporting.NewConsoleReporter()

	config := K8sConnectionConfig{
		Name:                "k8s-concurrent-test",
		ContextName:         "test-context",
		IsMC:                true,
		HealthCheckInterval: 50 * time.Millisecond,
	}

	service := &K8sConnectionService{
		config:   config,
		reporter: reporter,
		stopChan: make(chan struct{}),
	}

	// Start the service
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.Run()
	}()

	// Give service time to start
	time.Sleep(50 * time.Millisecond)

	// Try to close stop channel multiple times concurrently
	var stopWg sync.WaitGroup
	for i := 0; i < 5; i++ {
		stopWg.Add(1)
		go func() {
			defer stopWg.Done()
			// This should not panic even if called multiple times
			select {
			case <-service.stopChan:
				// Already closed
			default:
				close(service.stopChan)
			}
		}()
	}

	stopWg.Wait()
	wg.Wait()

	// If we get here without panic, the test passes
}

// testK8sReporter is a test implementation of ServiceReporter
type testK8sReporter struct {
	mu         sync.Mutex
	reports    []reporting.ManagedServiceUpdate
	reportFunc func(update reporting.ManagedServiceUpdate)
}

func (m *testK8sReporter) Report(update reporting.ManagedServiceUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reports = append(m.reports, update)
	if m.reportFunc != nil {
		m.reportFunc(update)
	}
}

func (m *testK8sReporter) GetStateStore() reporting.StateStore {
	return nil
}

// TestGetNodeStatus tests the kube package's GetNodeStatus function
func TestGetNodeStatus(t *testing.T) {
	// This test would require a mock kubernetes clientset
	// For now, we'll skip it
	t.Skip("Skipping test that requires kubernetes clientset mocking")
}
