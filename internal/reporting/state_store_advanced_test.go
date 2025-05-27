package reporting

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestStateStore_StateTransitions(t *testing.T) {
	store := NewStateStore()

	// Record some state transitions
	transition1 := StateTransition{
		Label:         "test-service",
		FromState:     StateUnknown,
		ToState:       StateStarting,
		Timestamp:     time.Now(),
		CorrelationID: "corr_123",
		CausedBy:      "user_action",
		Sequence:      1,
		Metadata:      map[string]interface{}{"test": "value"},
	}

	transition2 := StateTransition{
		Label:         "test-service",
		FromState:     StateStarting,
		ToState:       StateRunning,
		Timestamp:     time.Now(),
		CorrelationID: "corr_123",
		CausedBy:      "startup_complete",
		Sequence:      2,
		Metadata:      map[string]interface{}{"port": 8080},
	}

	err := store.RecordStateTransition(transition1)
	assert.NoError(t, err)

	err = store.RecordStateTransition(transition2)
	assert.NoError(t, err)

	// Get transitions for the service
	transitions := store.GetStateTransitions("test-service")
	assert.Len(t, transitions, 2)
	assert.Equal(t, StateUnknown, transitions[0].FromState)
	assert.Equal(t, StateStarting, transitions[0].ToState)
	assert.Equal(t, StateStarting, transitions[1].FromState)
	assert.Equal(t, StateRunning, transitions[1].ToState)

	// Get all transitions
	allTransitions := store.GetAllStateTransitions()
	assert.Len(t, allTransitions, 2)

	// Get transitions for non-existent service
	noTransitions := store.GetStateTransitions("non-existent")
	assert.Len(t, noTransitions, 0)
}

func TestStateStore_CascadeOperations(t *testing.T) {
	store := NewStateStore()

	// Record a cascade operation
	cascade := CascadeInfo{
		InitiatingService: "k8s:test-cluster",
		AffectedServices:  []string{"test-pf", "test-mcp"},
		Reason:            "health_check_failure",
		CorrelationID:     "corr_456",
		Timestamp:         time.Now(),
		CascadeType:       CascadeTypeStop,
	}

	err := store.RecordCascadeOperation(cascade)
	assert.NoError(t, err)

	// Get all cascade operations
	cascades := store.GetCascadeOperations()
	assert.Len(t, cascades, 1)
	assert.Equal(t, "k8s:test-cluster", cascades[0].InitiatingService)
	assert.Len(t, cascades[0].AffectedServices, 2)
	assert.Equal(t, CascadeTypeStop, cascades[0].CascadeType)

	// Get cascades by correlation ID
	correlatedCascades := store.GetCascadesByCorrelationID("corr_456")
	assert.Len(t, correlatedCascades, 1)
	assert.Equal(t, cascade.InitiatingService, correlatedCascades[0].InitiatingService)

	// Get cascades for non-existent correlation ID
	noCascades := store.GetCascadesByCorrelationID("non-existent")
	assert.Len(t, noCascades, 0)
}

func TestStateStore_TransitionRecordingOnStateUpdate(t *testing.T) {
	store := NewStateStore()

	// Create an update that should record a transition
	update := ManagedServiceUpdate{
		Timestamp:     time.Now(),
		SourceType:    ServiceTypePortForward,
		SourceLabel:   "test-pf",
		State:         StateRunning,
		IsReady:       true,
		CorrelationID: "corr_789",
		CausedBy:      "startup_complete",
		Sequence:      1,
		ProxyPort:     8080,
	}

	// Set the state (should record transition from Unknown to Running)
	changed, err := store.SetServiceState(update)
	assert.NoError(t, err)
	assert.True(t, changed)

	// Check that transition was recorded
	transitions := store.GetStateTransitions("test-pf")
	assert.Len(t, transitions, 1)
	assert.Equal(t, StateUnknown, transitions[0].FromState)
	assert.Equal(t, StateRunning, transitions[0].ToState)
	assert.Equal(t, "corr_789", transitions[0].CorrelationID)
	assert.Equal(t, 8080, transitions[0].Metadata["proxy_port"])

	// Update to failed state
	update.State = StateFailed
	update.ErrorDetail = fmt.Errorf("test error")
	update.Sequence = 2

	changed, err = store.SetServiceState(update)
	assert.NoError(t, err)
	assert.True(t, changed)

	// Check that second transition was recorded
	transitions = store.GetStateTransitions("test-pf")
	assert.Len(t, transitions, 2)
	assert.Equal(t, StateRunning, transitions[1].FromState)
	assert.Equal(t, StateFailed, transitions[1].ToState)
	assert.Equal(t, "test error", transitions[1].Metadata["error"])
}
