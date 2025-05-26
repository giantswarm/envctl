package reporting

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus()
	assert.NotNil(t, bus)

	metrics := bus.GetMetrics()
	assert.Equal(t, 0, metrics.TotalSubscriptions)
	assert.Equal(t, 0, metrics.ActiveSubscriptions)
	assert.Equal(t, int64(0), metrics.EventsPublished)
	assert.Equal(t, int64(0), metrics.EventsDelivered)
	assert.Equal(t, int64(0), metrics.EventsDropped)
}

func TestEventBus_Subscribe(t *testing.T) {
	bus := NewEventBus()

	var receivedEvents []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
	}

	filter := FilterByType(EventTypeServiceRunning)
	subscription := bus.Subscribe(filter, handler)

	assert.NotNil(t, subscription)
	assert.NotEmpty(t, subscription.ID)
	assert.False(t, subscription.IsClosed())

	metrics := bus.GetMetrics()
	assert.Equal(t, 1, metrics.TotalSubscriptions)
	assert.Equal(t, 1, metrics.ActiveSubscriptions)
}

func TestEventBus_SubscribeChannel(t *testing.T) {
	bus := NewEventBus()

	filter := FilterByType(EventTypeServiceRunning)
	subscription := bus.SubscribeChannel(filter, 10)

	assert.NotNil(t, subscription)
	assert.NotNil(t, subscription.Channel)
	assert.NotEmpty(t, subscription.ID)
	assert.False(t, subscription.IsClosed())

	metrics := bus.GetMetrics()
	assert.Equal(t, 1, metrics.TotalSubscriptions)
	assert.Equal(t, 1, metrics.ActiveSubscriptions)
}

func TestEventBus_Publish_Handler(t *testing.T) {
	bus := NewEventBus()

	var receivedEvents []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
	}

	// Subscribe to service events
	filter := FilterByType(EventTypeServiceRunning, EventTypeServiceFailed)
	subscription := bus.Subscribe(filter, handler)

	// Publish matching event
	event1 := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
	bus.Publish(event1)

	// Publish non-matching event
	event2 := NewHealthEvent("test-context", "test-cluster", true, true, 3, 5)
	bus.Publish(event2)

	// Publish another matching event
	event3 := NewServiceStateEvent(ServiceTypeMCPServer, "test-mcp", StateRunning, StateFailed)
	bus.Publish(event3)

	// Give handlers time to execute
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, receivedEvents, 2, "Should have received 2 matching events")

	// Check that we received the correct events (order may vary due to concurrency)
	receivedSources := make(map[string]bool)
	for _, event := range receivedEvents {
		receivedSources[event.Source()] = true
	}
	assert.True(t, receivedSources["test-pf"], "Should have received event from test-pf")
	assert.True(t, receivedSources["test-mcp"], "Should have received event from test-mcp")

	metrics := bus.GetMetrics()
	assert.Equal(t, int64(3), metrics.EventsPublished)
	assert.Equal(t, int64(2), metrics.EventsDelivered)
	assert.Equal(t, int64(0), metrics.EventsDropped)

	bus.Unsubscribe(subscription)
}

func TestEventBus_Publish_Channel(t *testing.T) {
	bus := NewEventBus()

	filter := FilterByType(EventTypeServiceRunning)
	subscription := bus.SubscribeChannel(filter, 5)

	// Publish matching event
	event := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
	bus.Publish(event)

	// Receive event from channel
	select {
	case receivedEvent := <-subscription.Channel:
		assert.Equal(t, event.Source(), receivedEvent.Source())
		assert.Equal(t, event.Type(), receivedEvent.Type())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive event from channel")
	}

	bus.Unsubscribe(subscription)
}

func TestEventBus_Publish_ChannelBufferOverflow(t *testing.T) {
	bus := NewEventBus()

	// Create subscription with small buffer
	filter := FilterByType(EventTypeServiceRunning)
	subscription := bus.SubscribeChannel(filter, 2)

	// Fill buffer beyond capacity
	for i := 0; i < 5; i++ {
		event := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
		bus.Publish(event)
	}

	metrics := bus.GetMetrics()
	assert.Equal(t, int64(5), metrics.EventsPublished)
	assert.Equal(t, int64(2), metrics.EventsDelivered) // Only 2 fit in buffer
	assert.Equal(t, int64(3), metrics.EventsDropped)   // 3 were dropped

	bus.Unsubscribe(subscription)
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) {}
	filter := FilterByType(EventTypeServiceRunning)
	subscription := bus.Subscribe(filter, handler)

	metrics := bus.GetMetrics()
	assert.Equal(t, 1, metrics.ActiveSubscriptions)

	bus.Unsubscribe(subscription)

	assert.True(t, subscription.IsClosed())

	metrics = bus.GetMetrics()
	assert.Equal(t, 0, metrics.ActiveSubscriptions)
	assert.Equal(t, 1, metrics.TotalSubscriptions) // Total doesn't decrease
}

func TestEventBus_Close(t *testing.T) {
	bus := NewEventBus()

	// Create multiple subscriptions
	sub1 := bus.Subscribe(FilterByType(EventTypeServiceRunning), func(Event) {})
	sub2 := bus.SubscribeChannel(FilterByType(EventTypeHealthCheck), 5)

	metrics := bus.GetMetrics()
	assert.Equal(t, 2, metrics.ActiveSubscriptions)

	bus.Close()

	// All subscriptions should be closed
	assert.True(t, sub1.IsClosed())
	assert.True(t, sub2.IsClosed())

	metrics = bus.GetMetrics()
	assert.Equal(t, 0, metrics.ActiveSubscriptions)

	// Publishing after close should not crash
	event := NewServiceStateEvent(ServiceTypePortForward, "test", StateUnknown, StateRunning)
	bus.Publish(event) // Should not panic
}

func TestEventSubscription_Close(t *testing.T) {
	subscription := &EventSubscription{
		ID:      "test",
		Channel: make(chan Event, 1),
		Closed:  false,
	}

	assert.False(t, subscription.IsClosed())

	subscription.Close()
	assert.True(t, subscription.IsClosed())

	// Closing again should be safe
	subscription.Close()
	assert.True(t, subscription.IsClosed())
}

func TestFilterByType(t *testing.T) {
	filter := FilterByType(EventTypeServiceRunning, EventTypeServiceFailed)

	// Matching events
	event1 := NewServiceStateEvent(ServiceTypePortForward, "test", StateStarting, StateRunning)
	event2 := NewServiceStateEvent(ServiceTypeMCPServer, "test", StateRunning, StateFailed)

	assert.True(t, filter(event1))
	assert.True(t, filter(event2))

	// Non-matching event
	event3 := NewHealthEvent("test", "test", true, true, 3, 5)
	assert.False(t, filter(event3))
}

func TestFilterBySource(t *testing.T) {
	filter := FilterBySource("test-pf", "test-mcp")

	// Matching events
	event1 := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateUnknown, StateRunning)
	event2 := NewServiceStateEvent(ServiceTypeMCPServer, "test-mcp", StateUnknown, StateRunning)

	assert.True(t, filter(event1))
	assert.True(t, filter(event2))

	// Non-matching event
	event3 := NewServiceStateEvent(ServiceTypePortForward, "other-pf", StateUnknown, StateRunning)
	assert.False(t, filter(event3))
}

func TestFilterBySeverity(t *testing.T) {
	filter := FilterBySeverity(SeverityWarn)

	// Create events with different severities
	errorEvent := NewServiceStateEvent(ServiceTypePortForward, "test", StateRunning, StateFailed)
	warnEvent := NewServiceStateEvent(ServiceTypePortForward, "test", StateRunning, StateRetrying)
	infoEvent := NewServiceStateEvent(ServiceTypePortForward, "test", StateStarting, StateRunning)
	debugEvent := NewServiceStateEvent(ServiceTypePortForward, "test", StateUnknown, StateStarting)

	// Only warn and above should pass
	assert.True(t, filter(errorEvent))  // Error >= Warn
	assert.True(t, filter(warnEvent))   // Warn >= Warn
	assert.False(t, filter(infoEvent))  // Info < Warn
	assert.False(t, filter(debugEvent)) // Debug < Warn
}

func TestFilterByCorrelation(t *testing.T) {
	correlationID := "test-correlation-123"
	filter := FilterByCorrelation(correlationID)

	// Matching event
	event1 := NewServiceStateEvent(ServiceTypePortForward, "test", StateUnknown, StateRunning)
	event1.BaseEvent.WithCorrelation(correlationID, "test", "")
	assert.True(t, filter(event1))

	// Non-matching event
	event2 := NewServiceStateEvent(ServiceTypePortForward, "test", StateUnknown, StateRunning)
	event2.BaseEvent.WithCorrelation("different-correlation", "test", "")
	assert.False(t, filter(event2))
}

func TestCombineFilters(t *testing.T) {
	typeFilter := FilterByType(EventTypeServiceRunning)
	sourceFilter := FilterBySource("test-pf")
	combinedFilter := CombineFilters(typeFilter, sourceFilter)

	// Event matching both filters
	event1 := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
	assert.True(t, combinedFilter(event1))

	// Event matching only type filter
	event2 := NewServiceStateEvent(ServiceTypePortForward, "other-pf", StateStarting, StateRunning)
	assert.False(t, combinedFilter(event2))

	// Event matching only source filter
	event3 := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateRunning, StateFailed)
	assert.False(t, combinedFilter(event3))

	// Event matching neither filter
	event4 := NewHealthEvent("test", "test", true, true, 3, 5)
	assert.False(t, combinedFilter(event4))
}

func TestAnyFilter(t *testing.T) {
	typeFilter := FilterByType(EventTypeServiceRunning)
	sourceFilter := FilterBySource("test-pf")
	anyFilter := AnyFilter(typeFilter, sourceFilter)

	// Event matching both filters
	event1 := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
	assert.True(t, anyFilter(event1))

	// Event matching only type filter
	event2 := NewServiceStateEvent(ServiceTypePortForward, "other-pf", StateStarting, StateRunning)
	assert.True(t, anyFilter(event2))

	// Event matching only source filter
	event3 := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateRunning, StateFailed)
	assert.True(t, anyFilter(event3))

	// Event matching neither filter
	event4 := NewHealthEvent("test", "test", true, true, 3, 5)
	assert.False(t, anyFilter(event4))
}

func TestEventBusAdapter(t *testing.T) {
	eventBus := NewEventBus()
	stateStore := NewStateStore()
	adapter := NewEventBusAdapter(eventBus, stateStore)

	assert.NotNil(t, adapter)
	assert.Equal(t, eventBus, adapter.GetEventBus())
	assert.Equal(t, stateStore, adapter.GetStateStore())
}

func TestEventBusAdapter_Report(t *testing.T) {
	eventBus := NewEventBus()
	stateStore := NewStateStore()
	adapter := NewEventBusAdapter(eventBus, stateStore)

	var receivedEvents []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
	}

	// Subscribe to service state events
	filter := FilterByType(EventTypeServiceRunning)
	eventBus.Subscribe(filter, handler)

	// Report a service update
	update := NewManagedServiceUpdate(ServiceTypePortForward, "test-pf", StateRunning)
	adapter.Report(update)

	// Give handler time to execute
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, receivedEvents, 1)

	serviceEvent, ok := receivedEvents[0].(*ServiceStateEvent)
	assert.True(t, ok)
	assert.Equal(t, "test-pf", serviceEvent.Source())
	assert.Equal(t, StateUnknown, serviceEvent.OldState) // No previous state
	assert.Equal(t, StateRunning, serviceEvent.NewState)

	// Verify state was updated in store
	snapshot, exists := stateStore.GetServiceState("test-pf")
	assert.True(t, exists)
	assert.Equal(t, StateRunning, snapshot.State)
}

func TestEventBusAdapter_ReportHealth(t *testing.T) {
	eventBus := NewEventBus()
	stateStore := NewStateStore()
	adapter := NewEventBusAdapter(eventBus, stateStore)

	var receivedEvents []Event
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, event)
	}

	// Subscribe to health events
	filter := FilterByType(EventTypeHealthCheck)
	eventBus.Subscribe(filter, handler)

	// Report health update
	healthUpdate := HealthStatusUpdate{
		Timestamp:        time.Now(),
		ContextName:      "test-context",
		ClusterShortName: "test-cluster",
		IsMC:             true,
		IsHealthy:        true,
		ReadyNodes:       3,
		TotalNodes:       5,
	}
	adapter.ReportHealth(healthUpdate)

	// Give handler time to execute
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	assert.Len(t, receivedEvents, 1)

	healthEvent, ok := receivedEvents[0].(*HealthEvent)
	assert.True(t, ok)
	assert.Equal(t, "health-monitor", healthEvent.Source())
	assert.Equal(t, "test-context", healthEvent.ContextName)
	assert.Equal(t, "test-cluster", healthEvent.ClusterShortName)
	assert.True(t, healthEvent.IsMC)
	assert.True(t, healthEvent.IsHealthy)
	assert.Equal(t, 3, healthEvent.ReadyNodes)
	assert.Equal(t, 5, healthEvent.TotalNodes)
}

func TestEventBusAdapter_NilParameters(t *testing.T) {
	// Test with nil parameters
	adapter := NewEventBusAdapter(nil, nil)

	assert.NotNil(t, adapter.GetEventBus())
	assert.NotNil(t, adapter.GetStateStore())
}

func TestEventBus_ConcurrentAccess(t *testing.T) {
	bus := NewEventBus()

	var receivedCount int64
	var mu sync.Mutex

	handler := func(event Event) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
	}

	// Create multiple subscriptions
	for i := 0; i < 5; i++ {
		filter := FilterByType(EventTypeServiceRunning)
		bus.Subscribe(filter, handler)
	}

	// Publish events concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := NewServiceStateEvent(ServiceTypePortForward, "test", StateStarting, StateRunning)
			bus.Publish(event)
		}(i)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond) // Give handlers time to execute

	mu.Lock()
	finalCount := receivedCount
	mu.Unlock()

	// Each event should be delivered to all 5 subscriptions
	assert.Equal(t, int64(50), finalCount)

	metrics := bus.GetMetrics()
	assert.Equal(t, int64(10), metrics.EventsPublished)
	assert.Equal(t, int64(50), metrics.EventsDelivered)
}
