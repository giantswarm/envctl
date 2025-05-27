package reporting

import (
	"errors"
	"testing"

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
