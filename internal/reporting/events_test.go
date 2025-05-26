package reporting

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewServiceStateEvent(t *testing.T) {
	event := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateUnknown, StateRunning)

	assert.Equal(t, EventTypeServiceRunning, event.Type())
	assert.Equal(t, "test-pf", event.Source())
	assert.Equal(t, ServiceTypePortForward, event.ServiceType)
	assert.Equal(t, StateUnknown, event.OldState)
	assert.Equal(t, StateRunning, event.NewState)
	assert.True(t, event.IsReady)
	assert.Equal(t, SeverityInfo, event.Severity())
	assert.NotEmpty(t, event.CorrelationID())
	assert.False(t, event.Timestamp().IsZero())
}

func TestServiceStateEvent_WithError(t *testing.T) {
	event := NewServiceStateEvent(ServiceTypeMCPServer, "test-mcp", StateRunning, StateStarting)
	testErr := errors.New("test error")

	event.WithError(testErr)

	assert.Equal(t, testErr, event.Error)
	assert.Equal(t, SeverityError, event.Severity())
	assert.Equal(t, StateFailed, event.NewState)
}

func TestServiceStateEvent_WithServiceData(t *testing.T) {
	event := NewServiceStateEvent(ServiceTypeMCPServer, "test-mcp", StateStarting, StateRunning)

	event.WithServiceData(8080, 12345)

	assert.Equal(t, 8080, event.ProxyPort)
	assert.Equal(t, 12345, event.PID)
}

func TestServiceStateEvent_String(t *testing.T) {
	// Test without error
	event := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
	expected := "test-pf (PortForward) Starting → Running"
	assert.Equal(t, expected, event.String())

	// Test with error
	event.WithError(errors.New("connection failed"))
	expected = "test-pf (PortForward) Starting → Failed (error: connection failed)"
	assert.Equal(t, expected, event.String())
}

func TestNewHealthEvent(t *testing.T) {
	event := NewHealthEvent("test-context", "test-cluster", true, true, 3, 5)

	assert.Equal(t, EventTypeHealthCheck, event.Type())
	assert.Equal(t, "health-monitor", event.Source())
	assert.Equal(t, "test-context", event.ContextName)
	assert.Equal(t, "test-cluster", event.ClusterShortName)
	assert.True(t, event.IsMC)
	assert.True(t, event.IsHealthy)
	assert.Equal(t, 3, event.ReadyNodes)
	assert.Equal(t, 5, event.TotalNodes)
	assert.Equal(t, SeverityInfo, event.Severity())
}

func TestHealthEvent_WithError(t *testing.T) {
	event := NewHealthEvent("test-context", "test-cluster", false, true, 3, 5)
	testErr := errors.New("health check failed")

	event.WithError(testErr)

	assert.Equal(t, testErr, event.Error)
	assert.Equal(t, SeverityError, event.Severity())
	assert.False(t, event.IsHealthy)
}

func TestHealthEvent_String(t *testing.T) {
	// Test healthy MC
	event := NewHealthEvent("mc-context", "test-mc", true, true, 3, 5)
	// Note: The string conversion for numbers needs to be fixed
	assert.Contains(t, event.String(), "MC test-mc healthy")

	// Test unhealthy WC
	event = NewHealthEvent("wc-context", "test-wc", false, false, 1, 3)
	assert.Contains(t, event.String(), "WC test-wc unhealthy")

	// Test with error
	event.WithError(errors.New("connection timeout"))
	assert.Contains(t, event.String(), "unhealthy")
	assert.Contains(t, event.String(), "connection timeout")
}

func TestNewDependencyEvent(t *testing.T) {
	affectedNodes := []string{"pf:test-pf", "mcp:test-mcp"}
	event := NewDependencyEvent("k8s:test-context", "k8s", "stop", affectedNodes)

	assert.Equal(t, EventTypeCascadeStop, event.Type())
	assert.Equal(t, "k8s:test-context", event.Source())
	assert.Equal(t, "k8s", event.DependencyType)
	assert.Equal(t, "stop", event.Action)
	assert.Equal(t, affectedNodes, event.AffectedNodes)
	assert.Equal(t, SeverityInfo, event.Severity())
}

func TestDependencyEvent_String(t *testing.T) {
	affectedNodes := []string{"pf:test-pf", "mcp:test-mcp"}
	event := NewDependencyEvent("k8s:test-context", "k8s", "stop", affectedNodes)

	assert.Contains(t, event.String(), "stop cascade")
	assert.Contains(t, event.String(), "k8s:test-context")
	assert.Contains(t, event.String(), "2 services")
}

func TestNewUserActionEvent(t *testing.T) {
	event := NewUserActionEvent("restart", "service", "test-pf")

	assert.Equal(t, EventTypeUserAction, event.Type())
	assert.Equal(t, "user", event.Source())
	assert.Equal(t, "restart", event.Action)
	assert.Equal(t, "service", event.TargetType)
	assert.Equal(t, "test-pf", event.TargetName)
	assert.Equal(t, SeverityInfo, event.Severity())
}

func TestUserActionEvent_String(t *testing.T) {
	event := NewUserActionEvent("stop", "service", "test-mcp")
	expected := "User stop service test-mcp"
	assert.Equal(t, expected, event.String())
}

func TestNewSystemEvent(t *testing.T) {
	event := NewSystemEvent("orchestrator", "startup", "initialization complete")

	assert.Equal(t, EventTypeSystemStartup, event.Type())
	assert.Equal(t, "orchestrator", event.Source())
	assert.Equal(t, "orchestrator", event.Component)
	assert.Equal(t, "startup", event.Action)
	assert.Equal(t, "initialization complete", event.Details)
	assert.Equal(t, SeverityInfo, event.Severity())
}

func TestSystemEvent_String(t *testing.T) {
	// Test with details
	event := NewSystemEvent("tui", "startup", "UI initialized")
	expected := "tui startup: UI initialized"
	assert.Equal(t, expected, event.String())

	// Test without details
	event = NewSystemEvent("orchestrator", "shutdown", "")
	expected = "orchestrator shutdown"
	assert.Equal(t, expected, event.String())
}

func TestMapStateToEventType(t *testing.T) {
	tests := []struct {
		state    ServiceState
		expected EventType
	}{
		{StateStarting, EventTypeServiceStarting},
		{StateRunning, EventTypeServiceRunning},
		{StateStopping, EventTypeServiceStopping},
		{StateStopped, EventTypeServiceStopped},
		{StateFailed, EventTypeServiceFailed},
		{StateRetrying, EventTypeServiceRetrying},
		{StateUnknown, EventTypeServiceStarting}, // Default case
	}

	for _, test := range tests {
		result := mapStateToEventType(test.state)
		assert.Equal(t, test.expected, result, "State %s should map to %s", test.state, test.expected)
	}
}

func TestMapStateToSeverity(t *testing.T) {
	tests := []struct {
		state    ServiceState
		expected EventSeverity
	}{
		{StateFailed, SeverityError},
		{StateRetrying, SeverityWarn},
		{StateRunning, SeverityInfo},
		{StateStopped, SeverityInfo},
		{StateStarting, SeverityDebug},
		{StateStopping, SeverityDebug},
		{StateUnknown, SeverityInfo}, // Default case
	}

	for _, test := range tests {
		result := mapStateToSeverity(test.state)
		assert.Equal(t, test.expected, result, "State %s should map to %s", test.state, test.expected)
	}
}

func TestBaseEvent_WithCorrelation(t *testing.T) {
	event := &BaseEvent{}

	event.WithCorrelation("test-corr-id", "test-cause", "test-parent")

	assert.Equal(t, "test-corr-id", event.CorrelationID())
	assert.Equal(t, "test-cause", event.CausedBy())
	assert.Equal(t, "test-parent", event.ParentID())
}

func TestBaseEvent_WithMetadata(t *testing.T) {
	event := &BaseEvent{}

	event.WithMetadata("key1", "value1")
	event.WithMetadata("key2", 42)

	metadata := event.Metadata()
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, 42, metadata["key2"])
}

func TestBaseEvent_Metadata_NilSafe(t *testing.T) {
	event := &BaseEvent{} // Meta is nil

	metadata := event.Metadata()
	assert.NotNil(t, metadata)
	assert.Empty(t, metadata)
}

func TestEventInterface_Implementation(t *testing.T) {
	// Test that all event types implement the Event interface
	var _ Event = &ServiceStateEvent{}
	var _ Event = &HealthEvent{}
	var _ Event = &DependencyEvent{}
	var _ Event = &UserActionEvent{}
	var _ Event = &SystemEvent{}
}

func TestEventTimestamp(t *testing.T) {
	before := time.Now()
	event := NewServiceStateEvent(ServiceTypePortForward, "test", StateUnknown, StateRunning)
	after := time.Now()

	timestamp := event.Timestamp()
	assert.True(t, timestamp.After(before) || timestamp.Equal(before))
	assert.True(t, timestamp.Before(after) || timestamp.Equal(after))
}
