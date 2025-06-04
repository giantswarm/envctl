package adapters

import (
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"errors"
	"testing"
	"time"
)

// mockOrchestratorAPI implements a mock OrchestratorAPI for testing
type mockOrchestratorAPI struct {
	events           chan api.ServiceStateChangedEvent
	serviceCalled    string
	operationCalled  string
	returnError      error
	subscriptionDone chan struct{} // Channel to signal when subscription loop exits
}

func newMockOrchestratorAPI() *mockOrchestratorAPI {
	return &mockOrchestratorAPI{
		events: make(chan api.ServiceStateChangedEvent, 10),
	}
}

// Implement all required OrchestratorAPI methods

func (m *mockOrchestratorAPI) StartService(label string) error {
	m.serviceCalled = label
	m.operationCalled = "start"
	return m.returnError
}

func (m *mockOrchestratorAPI) StopService(label string) error {
	m.serviceCalled = label
	m.operationCalled = "stop"
	return m.returnError
}

func (m *mockOrchestratorAPI) RestartService(label string) error {
	m.serviceCalled = label
	m.operationCalled = "restart"
	return m.returnError
}

func (m *mockOrchestratorAPI) GetServiceStatus(label string) (*api.ServiceStatus, error) {
	return &api.ServiceStatus{}, nil
}

func (m *mockOrchestratorAPI) GetAllServices() []api.ServiceStatus {
	return []api.ServiceStatus{}
}

func (m *mockOrchestratorAPI) SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent {
	// Create a chan of orchestrator type and convert it
	orchChan := make(chan orchestrator.ServiceStateChangedEvent)
	go func() {
		// If subscriptionDone is set, signal when this goroutine exits
		if m.subscriptionDone != nil {
			defer close(m.subscriptionDone)
		}
		for event := range m.events {
			orchChan <- orchestrator.ServiceStateChangedEvent(event)
		}
		close(orchChan)
	}()
	return orchChan
}

// Cluster management methods
func (m *mockOrchestratorAPI) GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition {
	return []config.ClusterDefinition{}
}

func (m *mockOrchestratorAPI) GetActiveCluster(role config.ClusterRole) (string, bool) {
	return "", false
}

func (m *mockOrchestratorAPI) SwitchCluster(role config.ClusterRole, clusterName string) error {
	return nil
}

// Helper methods for testing

func (m *mockOrchestratorAPI) sendEvent(event api.ServiceStateChangedEvent) {
	m.events <- event
}

func (m *mockOrchestratorAPI) close() {
	close(m.events)
}

func TestNewOrchestratorEventAdapter(t *testing.T) {
	mockAPI := newMockOrchestratorAPI()
	adapter := NewOrchestratorEventAdapter(mockAPI)

	if adapter == nil {
		t.Fatal("NewOrchestratorEventAdapter returned nil")
	}

	// Verify the adapter is properly initialized
	if adapter.api == nil {
		t.Error("adapter.api should not be nil")
	}
}

func TestOrchestratorEventAdapter_SubscribeToStateChanges(t *testing.T) {
	mockAPI := newMockOrchestratorAPI()
	adapter := NewOrchestratorEventAdapter(mockAPI)

	// Subscribe to events
	eventChan := adapter.SubscribeToStateChanges()

	// Test event conversion
	testCases := []struct {
		name     string
		apiEvent api.ServiceStateChangedEvent
		expected aggregator.ServiceStateChangedEvent
	}{
		{
			name: "basic event conversion",
			apiEvent: api.ServiceStateChangedEvent{
				Label:       "test-service",
				ServiceType: "PortForward",
				OldState:    "stopped",
				NewState:    "running",
				Health:      "healthy",
				Error:       nil,
			},
			expected: aggregator.ServiceStateChangedEvent{
				Label:       "test-service",
				ServiceType: "PortForward",
				OldState:    "stopped",
				NewState:    "running",
				Health:      "healthy",
				Error:       nil,
			},
		},
		{
			name: "event with error",
			apiEvent: api.ServiceStateChangedEvent{
				Label:       "error-service",
				ServiceType: "MCPServer",
				OldState:    "running",
				NewState:    "error",
				Health:      "unhealthy",
				Error:       errors.New("connection failed"),
			},
			expected: aggregator.ServiceStateChangedEvent{
				Label:       "error-service",
				ServiceType: "MCPServer",
				OldState:    "running",
				NewState:    "error",
				Health:      "unhealthy",
				Error:       errors.New("connection failed"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Send API event
			mockAPI.sendEvent(tc.apiEvent)

			// Receive converted event
			select {
			case event := <-eventChan:
				if event.Label != tc.expected.Label {
					t.Errorf("Label mismatch: got %s, want %s", event.Label, tc.expected.Label)
				}
				if event.ServiceType != tc.expected.ServiceType {
					t.Errorf("ServiceType mismatch: got %s, want %s", event.ServiceType, tc.expected.ServiceType)
				}
				if event.OldState != tc.expected.OldState {
					t.Errorf("OldState mismatch: got %s, want %s", event.OldState, tc.expected.OldState)
				}
				if event.NewState != tc.expected.NewState {
					t.Errorf("NewState mismatch: got %s, want %s", event.NewState, tc.expected.NewState)
				}
				if event.Health != tc.expected.Health {
					t.Errorf("Health mismatch: got %s, want %s", event.Health, tc.expected.Health)
				}
				if (event.Error == nil) != (tc.expected.Error == nil) {
					t.Errorf("Error presence mismatch: got %v, want %v", event.Error, tc.expected.Error)
				}
			case <-time.After(100 * time.Millisecond):
				t.Error("Timeout waiting for event")
			}
		})
	}
}

func TestOrchestratorEventAdapter_ChannelClose(t *testing.T) {
	mockAPI := newMockOrchestratorAPI()
	adapter := NewOrchestratorEventAdapter(mockAPI)

	eventChan := adapter.SubscribeToStateChanges()

	// Close the source channel
	mockAPI.close()

	// The adapter channel should also close
	time.Sleep(50 * time.Millisecond) // Give goroutine time to process

	select {
	case _, ok := <-eventChan:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Channel did not close in time")
	}
}

func TestOrchestratorEventAdapter_ChannelFull(t *testing.T) {
	mockAPI := newMockOrchestratorAPI()
	adapter := NewOrchestratorEventAdapter(mockAPI)

	// Subscribe but don't read events
	_ = adapter.SubscribeToStateChanges()

	// Send many events to fill the channel
	for i := 0; i < 150; i++ {
		mockAPI.sendEvent(api.ServiceStateChangedEvent{
			Label:       "test-service",
			ServiceType: "PortForward",
			NewState:    "running",
		})
	}

	// Should not panic or block indefinitely
	// The adapter should drop events when the channel is full
}
