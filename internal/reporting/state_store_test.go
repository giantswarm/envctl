package reporting

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStateStore(t *testing.T) {
	store := NewStateStore()
	assert.NotNil(t, store)

	metrics := store.GetMetrics()
	assert.Equal(t, 0, metrics.TotalServices)
	assert.Equal(t, 0, metrics.TotalSubscriptions)
	assert.Equal(t, int64(0), metrics.StateChanges)
	assert.NotNil(t, metrics.ServicesByType)
	assert.NotNil(t, metrics.ServicesByState)
}

func TestStateStore_SetAndGetServiceState(t *testing.T) {
	store := NewStateStore()

	// Test getting non-existent service
	_, exists := store.GetServiceState("nonexistent")
	assert.False(t, exists)

	// Test setting new service state
	update := NewManagedServiceUpdate(ServiceTypePortForward, "test-pf", StateStarting)
	update.ProxyPort = 8080
	update.PID = 1234

	changed, err := store.SetServiceState(update)
	assert.NoError(t, err)
	assert.True(t, changed)

	// Test getting the service state
	snapshot, exists := store.GetServiceState("test-pf")
	assert.True(t, exists)
	assert.Equal(t, "test-pf", snapshot.Label)
	assert.Equal(t, ServiceTypePortForward, snapshot.SourceType)
	assert.Equal(t, StateStarting, snapshot.State)
	assert.Equal(t, 8080, snapshot.ProxyPort)
	assert.Equal(t, 1234, snapshot.PID)
	assert.False(t, snapshot.IsReady)
	assert.NotEmpty(t, snapshot.CorrelationID)

	// Test updating to same state (should not change)
	update2 := NewManagedServiceUpdate(ServiceTypePortForward, "test-pf", StateStarting)
	changed, err = store.SetServiceState(update2)
	assert.NoError(t, err)
	assert.False(t, changed)

	// Test updating to different state (should change)
	update3 := NewManagedServiceUpdate(ServiceTypePortForward, "test-pf", StateRunning)
	changed, err = store.SetServiceState(update3)
	assert.NoError(t, err)
	assert.True(t, changed)

	snapshot, exists = store.GetServiceState("test-pf")
	assert.True(t, exists)
	assert.Equal(t, StateRunning, snapshot.State)
	assert.True(t, snapshot.IsReady)
}

func TestStateStore_GetAllServiceStates(t *testing.T) {
	store := NewStateStore()

	// Empty store
	states := store.GetAllServiceStates()
	assert.Empty(t, states)

	// Add some services
	update1 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateRunning)
	update2 := NewManagedServiceUpdate(ServiceTypeMCPServer, "mcp1", StateStarting)
	update3 := NewManagedServiceUpdate(ServiceTypePortForward, "pf2", StateFailed)

	store.SetServiceState(update1)
	store.SetServiceState(update2)
	store.SetServiceState(update3)

	states = store.GetAllServiceStates()
	assert.Len(t, states, 3)
	assert.Contains(t, states, "pf1")
	assert.Contains(t, states, "mcp1")
	assert.Contains(t, states, "pf2")

	assert.Equal(t, StateRunning, states["pf1"].State)
	assert.Equal(t, StateStarting, states["mcp1"].State)
	assert.Equal(t, StateFailed, states["pf2"].State)
}

func TestStateStore_GetServicesByType(t *testing.T) {
	store := NewStateStore()

	// Add services of different types
	update1 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateRunning)
	update2 := NewManagedServiceUpdate(ServiceTypePortForward, "pf2", StateStarting)
	update3 := NewManagedServiceUpdate(ServiceTypeMCPServer, "mcp1", StateRunning)
	update4 := NewManagedServiceUpdate(ServiceTypeSystem, "sys1", StateFailed)

	store.SetServiceState(update1)
	store.SetServiceState(update2)
	store.SetServiceState(update3)
	store.SetServiceState(update4)

	// Test port forwards
	pfServices := store.GetServicesByType(ServiceTypePortForward)
	assert.Len(t, pfServices, 2)
	assert.Contains(t, pfServices, "pf1")
	assert.Contains(t, pfServices, "pf2")

	// Test MCP servers
	mcpServices := store.GetServicesByType(ServiceTypeMCPServer)
	assert.Len(t, mcpServices, 1)
	assert.Contains(t, mcpServices, "mcp1")

	// Test system services
	sysServices := store.GetServicesByType(ServiceTypeSystem)
	assert.Len(t, sysServices, 1)
	assert.Contains(t, sysServices, "sys1")

	// Test non-existent type
	extServices := store.GetServicesByType(ServiceTypeExternalCmd)
	assert.Empty(t, extServices)
}

func TestStateStore_GetServicesByState(t *testing.T) {
	store := NewStateStore()

	// Add services in different states
	update1 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateRunning)
	update2 := NewManagedServiceUpdate(ServiceTypePortForward, "pf2", StateRunning)
	update3 := NewManagedServiceUpdate(ServiceTypeMCPServer, "mcp1", StateStarting)
	update4 := NewManagedServiceUpdate(ServiceTypeSystem, "sys1", StateFailed)

	store.SetServiceState(update1)
	store.SetServiceState(update2)
	store.SetServiceState(update3)
	store.SetServiceState(update4)

	// Test running services
	runningServices := store.GetServicesByState(StateRunning)
	assert.Len(t, runningServices, 2)
	assert.Contains(t, runningServices, "pf1")
	assert.Contains(t, runningServices, "pf2")

	// Test starting services
	startingServices := store.GetServicesByState(StateStarting)
	assert.Len(t, startingServices, 1)
	assert.Contains(t, startingServices, "mcp1")

	// Test failed services
	failedServices := store.GetServicesByState(StateFailed)
	assert.Len(t, failedServices, 1)
	assert.Contains(t, failedServices, "sys1")

	// Test non-existent state
	stoppedServices := store.GetServicesByState(StateStopped)
	assert.Empty(t, stoppedServices)
}

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

func TestStateStore_Clear(t *testing.T) {
	store := NewStateStore()

	// Add some services
	update1 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateRunning)
	update2 := NewManagedServiceUpdate(ServiceTypeMCPServer, "mcp1", StateStarting)

	store.SetServiceState(update1)
	store.SetServiceState(update2)

	metrics := store.GetMetrics()
	assert.Equal(t, 2, metrics.TotalServices)

	// Clear non-existent service
	cleared := store.Clear("nonexistent")
	assert.False(t, cleared)

	// Clear existing service
	cleared = store.Clear("pf1")
	assert.True(t, cleared)

	// Verify it's gone
	_, exists := store.GetServiceState("pf1")
	assert.False(t, exists)

	// Verify other service still exists
	_, exists = store.GetServiceState("mcp1")
	assert.True(t, exists)

	metrics = store.GetMetrics()
	assert.Equal(t, 1, metrics.TotalServices)
}

func TestStateStore_ClearAll(t *testing.T) {
	store := NewStateStore()

	// Add some services
	update1 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateRunning)
	update2 := NewManagedServiceUpdate(ServiceTypeMCPServer, "mcp1", StateStarting)
	update3 := NewManagedServiceUpdate(ServiceTypeSystem, "sys1", StateFailed)

	store.SetServiceState(update1)
	store.SetServiceState(update2)
	store.SetServiceState(update3)

	metrics := store.GetMetrics()
	assert.Equal(t, 3, metrics.TotalServices)

	// Clear all
	store.ClearAll()

	// Verify all are gone
	states := store.GetAllServiceStates()
	assert.Empty(t, states)

	metrics = store.GetMetrics()
	assert.Equal(t, 0, metrics.TotalServices)
	assert.Empty(t, metrics.ServicesByType)
	assert.Empty(t, metrics.ServicesByState)
}

func TestStateStore_Metrics(t *testing.T) {
	store := NewStateStore()

	// Initial metrics
	metrics := store.GetMetrics()
	assert.Equal(t, 0, metrics.TotalServices)
	assert.Equal(t, int64(0), metrics.StateChanges)
	assert.True(t, metrics.LastStateChange.IsZero())

	// Add services and track metrics
	update1 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateStarting)
	store.SetServiceState(update1)

	update2 := NewManagedServiceUpdate(ServiceTypeMCPServer, "mcp1", StateRunning)
	store.SetServiceState(update2)

	metrics = store.GetMetrics()
	assert.Equal(t, 2, metrics.TotalServices)
	assert.Equal(t, int64(2), metrics.StateChanges)
	assert.False(t, metrics.LastStateChange.IsZero())

	// Check type counts
	assert.Equal(t, 1, metrics.ServicesByType[ServiceTypePortForward])
	assert.Equal(t, 1, metrics.ServicesByType[ServiceTypeMCPServer])

	// Check state counts
	assert.Equal(t, 1, metrics.ServicesByState[StateStarting])
	assert.Equal(t, 1, metrics.ServicesByState[StateRunning])

	// Update state
	update3 := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateRunning)
	store.SetServiceState(update3)

	metrics = store.GetMetrics()
	assert.Equal(t, int64(3), metrics.StateChanges)
	assert.Equal(t, 0, metrics.ServicesByState[StateStarting]) // Should be removed
	assert.Equal(t, 2, metrics.ServicesByState[StateRunning])
}

func TestStateStore_ErrorHandling(t *testing.T) {
	store := NewStateStore()

	// Test with error in update
	update := NewManagedServiceUpdate(ServiceTypePortForward, "pf1", StateFailed)
	update = update.WithError(errors.New("test error"))

	changed, err := store.SetServiceState(update)
	assert.NoError(t, err)
	assert.True(t, changed)

	snapshot, exists := store.GetServiceState("pf1")
	assert.True(t, exists)
	assert.Equal(t, StateFailed, snapshot.State)
	assert.NotNil(t, snapshot.ErrorDetail)
	assert.Equal(t, "test error", snapshot.ErrorDetail.Error())
}

func TestStateStore_ConcurrentAccess(t *testing.T) {
	store := NewStateStore()

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			update := NewManagedServiceUpdate(ServiceTypePortForward,
				fmt.Sprintf("pf%d", id), StateRunning)
			store.SetServiceState(update)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all services were added
	states := store.GetAllServiceStates()
	assert.Len(t, states, 10)

	// Test concurrent reads
	for i := 0; i < 10; i++ {
		go func(id int) {
			_, exists := store.GetServiceState(fmt.Sprintf("pf%d", id))
			assert.True(t, exists)
			done <- true
		}(i)
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}
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
