package reporting

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStateStore_Subscriptions(t *testing.T) {
	store := NewStateStore()

	// Subscribe to all changes
	allSub := store.Subscribe("")
	assert.NotNil(t, allSub)
	assert.NotEmpty(t, allSub.ID)
	assert.Empty(t, allSub.Label)
	assert.False(t, allSub.IsClosed())

	// Subscribe to specific service
	specificSub := store.Subscribe("test-service")
	assert.NotNil(t, specificSub)
	assert.Equal(t, "test-service", specificSub.Label)

	// Check metrics
	metrics := store.GetMetrics()
	assert.Equal(t, 2, metrics.SubscriptionMetrics.ActiveSubscriptions)

	// Add a service state change
	update := NewManagedServiceUpdate(ServiceTypePortForward, "test-service", StateStarting)
	store.SetServiceState(update)

	// Both subscriptions should receive the event
	select {
	case event := <-allSub.Channel:
		assert.Equal(t, "test-service", event.Label)
		assert.Equal(t, StateUnknown, event.OldState)
		assert.Equal(t, StateStarting, event.NewState)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on all subscription channel")
	}

	select {
	case event := <-specificSub.Channel:
		assert.Equal(t, "test-service", event.Label)
		assert.Equal(t, StateUnknown, event.OldState)
		assert.Equal(t, StateStarting, event.NewState)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on specific subscription channel")
	}

	// Add a different service - only all subscription should receive it
	update2 := NewManagedServiceUpdate(ServiceTypeMCPServer, "other-service", StateRunning)
	store.SetServiceState(update2)

	select {
	case event := <-allSub.Channel:
		assert.Equal(t, "other-service", event.Label)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event on all subscription channel")
	}

	// Specific subscription should not receive it
	select {
	case <-specificSub.Channel:
		t.Fatal("Should not receive event for different service")
	case <-time.After(50 * time.Millisecond):
		// Expected - no event
	}

	// Unsubscribe
	store.Unsubscribe(specificSub)
	assert.True(t, specificSub.IsClosed())

	metrics = store.GetMetrics()
	assert.Equal(t, 1, metrics.SubscriptionMetrics.ActiveSubscriptions)

	// Clean up
	store.Unsubscribe(allSub)
}

func TestStateSubscription_Close(t *testing.T) {
	sub := &StateSubscription{
		ID:      "test",
		Channel: make(chan StateChangeEvent, 1),
		Closed:  false,
	}

	assert.False(t, sub.IsClosed())

	sub.Close()
	assert.True(t, sub.IsClosed())

	// Closing again should be safe
	sub.Close()
	assert.True(t, sub.IsClosed())
}

func TestStateStore_SubscriptionBufferOverflow(t *testing.T) {
	store := NewStateStore()

	// Create subscription with small buffer
	sub := store.Subscribe("test-service")

	// Fill the buffer beyond capacity by alternating states to ensure state changes
	for i := 0; i < 150; i++ { // Buffer is 100, so this should overflow
		var state ServiceState
		if i%2 == 0 {
			state = StateRunning
		} else {
			state = StateStarting
		}
		update := NewManagedServiceUpdate(ServiceTypePortForward, "test-service", state)
		update.CorrelationID = fmt.Sprintf("corr_%d", i)
		store.SetServiceState(update)
	}

	// Check that some events were dropped
	metrics := store.GetMetrics()
	assert.Greater(t, metrics.SubscriptionMetrics.DroppedEvents, int64(0))

	// Clean up
	store.Unsubscribe(sub)
}
