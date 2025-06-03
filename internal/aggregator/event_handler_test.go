package aggregator

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockStateEventProvider implements StateEventProvider for testing
type mockStateEventProvider struct {
	eventChan chan ServiceStateEvent
}

func newMockStateEventProvider() *mockStateEventProvider {
	return &mockStateEventProvider{
		eventChan: make(chan ServiceStateEvent, 10),
	}
}

func (m *mockStateEventProvider) SubscribeToStateChanges() <-chan ServiceStateEvent {
	return m.eventChan
}

func (m *mockStateEventProvider) sendEvent(event ServiceStateEvent) {
	m.eventChan <- event
}

func (m *mockStateEventProvider) close() {
	close(m.eventChan)
}

// mockRefreshFunc tracks calls to the refresh function
type mockRefreshFunc struct {
	mu        sync.Mutex
	calls     []context.Context
	callErr   error
	callDelay time.Duration
}

func newMockRefreshFunc() *mockRefreshFunc {
	return &mockRefreshFunc{
		calls: make([]context.Context, 0),
	}
}

func (m *mockRefreshFunc) refresh(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, ctx)

	if m.callDelay > 0 {
		time.Sleep(m.callDelay)
	}

	return m.callErr
}

func (m *mockRefreshFunc) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockRefreshFunc) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callErr = err
}

func (m *mockRefreshFunc) setDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callDelay = delay
}

func TestEventHandler_NewEventHandler(t *testing.T) {
	provider := newMockStateEventProvider()
	refreshFunc := newMockRefreshFunc()

	handler := NewEventHandler(provider, refreshFunc.refresh)

	if handler == nil {
		t.Fatal("NewEventHandler returned nil")
	}

	if handler.stateProvider != provider {
		t.Error("StateProvider not set correctly")
	}

	if handler.IsRunning() {
		t.Error("Handler should not be running initially")
	}
}

func TestEventHandler_StartStop(t *testing.T) {
	provider := newMockStateEventProvider()
	refreshFunc := newMockRefreshFunc()
	handler := NewEventHandler(provider, refreshFunc.refresh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test Start
	err := handler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}

	if !handler.IsRunning() {
		t.Error("Handler should be running after Start")
	}

	// Test double start (should not error)
	err = handler.Start(ctx)
	if err != nil {
		t.Errorf("Double start should not error: %v", err)
	}

	// Test Stop
	err = handler.Stop()
	if err != nil {
		t.Fatalf("Failed to stop handler: %v", err)
	}

	if handler.IsRunning() {
		t.Error("Handler should not be running after Stop")
	}

	// Test double stop (should not error)
	err = handler.Stop()
	if err != nil {
		t.Errorf("Double stop should not error: %v", err)
	}
}

func TestEventHandler_FiltersMCPEvents(t *testing.T) {
	provider := newMockStateEventProvider()
	refreshFunc := newMockRefreshFunc()
	handler := NewEventHandler(provider, refreshFunc.refresh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}
	defer handler.Stop()

	// Send non-MCP event (should NOT trigger refresh)
	provider.sendEvent(ServiceStateEvent{
		Label:    "k8s-mc-test",
		OldState: "Stopped",
		NewState: "Running",
	})

	// Send MCP event (should trigger refresh)
	provider.sendEvent(ServiceStateEvent{
		Label:    "kubernetes",
		OldState: "Stopped",
		NewState: "Running",
	})

	// Send aggregator event (should NOT trigger refresh)
	provider.sendEvent(ServiceStateEvent{
		Label:    "mcp-aggregator",
		OldState: "Stopped",
		NewState: "Running",
	})

	// Send port forward event (should NOT trigger refresh)
	provider.sendEvent(ServiceStateEvent{
		Label:    "pf:mc-prometheus",
		OldState: "Stopped",
		NewState: "Running",
	})

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Should have only 1 refresh call (only the kubernetes event)
	if refreshFunc.getCallCount() != 1 {
		t.Errorf("Expected 1 refresh call, got %d", refreshFunc.getCallCount())
	}
}

func TestEventHandler_RefreshTriggerConditions(t *testing.T) {
	testCases := []struct {
		name          string
		oldState      string
		newState      string
		expectRefresh bool
	}{
		{
			name:          "service becomes running",
			oldState:      "Stopped",
			newState:      "Running",
			expectRefresh: true,
		},
		{
			name:          "service stops being running",
			oldState:      "Running",
			newState:      "Stopped",
			expectRefresh: true,
		},
		{
			name:          "service fails",
			oldState:      "Running",
			newState:      "Failed",
			expectRefresh: true,
		},
		{
			name:          "service stays stopped",
			oldState:      "Stopped",
			newState:      "Stopped",
			expectRefresh: false,
		},
		{
			name:          "service stays running",
			oldState:      "Running",
			newState:      "Running",
			expectRefresh: false,
		},
		{
			name:          "service goes from failed to stopped",
			oldState:      "Failed",
			newState:      "Stopped",
			expectRefresh: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := newMockStateEventProvider()
			refreshFunc := newMockRefreshFunc()
			handler := NewEventHandler(provider, refreshFunc.refresh)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := handler.Start(ctx)
			if err != nil {
				t.Fatalf("Failed to start handler: %v", err)
			}
			defer handler.Stop()

			// Send event (we now accept all events, not just MCP)
			provider.sendEvent(ServiceStateEvent{
				Label:    "kubernetes",
				OldState: tc.oldState,
				NewState: tc.newState,
			})

			// Give time for event to be processed
			time.Sleep(100 * time.Millisecond)

			expectedCalls := 0
			if tc.expectRefresh {
				expectedCalls = 1
			}

			if refreshFunc.getCallCount() != expectedCalls {
				t.Errorf("Expected %d refresh calls, got %d", expectedCalls, refreshFunc.getCallCount())
			}
		})
	}
}

func TestEventHandler_HandlesChannelClose(t *testing.T) {
	provider := newMockStateEventProvider()
	refreshFunc := newMockRefreshFunc()
	handler := NewEventHandler(provider, refreshFunc.refresh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := handler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}

	// Close the event channel
	provider.close()

	// Give time for handler to detect channel close and stop
	time.Sleep(100 * time.Millisecond)

	// Handler should have stopped itself
	if handler.IsRunning() {
		t.Error("Handler should have stopped when event channel was closed")
	}
}

func TestEventHandler_HandlesContextCancellation(t *testing.T) {
	provider := newMockStateEventProvider()
	refreshFunc := newMockRefreshFunc()
	handler := NewEventHandler(provider, refreshFunc.refresh)

	ctx, cancel := context.WithCancel(context.Background())

	err := handler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start handler: %v", err)
	}

	// Cancel the context
	cancel()

	// Give time for handler to detect cancellation and stop
	time.Sleep(100 * time.Millisecond)

	// Stop should complete without hanging
	err = handler.Stop()
	if err != nil {
		t.Errorf("Stop failed after context cancellation: %v", err)
	}
}
