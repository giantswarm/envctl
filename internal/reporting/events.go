package reporting

import (
	"fmt"
	"time"
)

// EventType defines the type of event
type EventType string

const (
	// Service lifecycle events
	EventTypeServiceStarting EventType = "service.starting"
	EventTypeServiceRunning  EventType = "service.running"
	EventTypeServiceStopping EventType = "service.stopping"
	EventTypeServiceStopped  EventType = "service.stopped"
	EventTypeServiceFailed   EventType = "service.failed"
	EventTypeServiceRetrying EventType = "service.retrying"

	// Health events
	EventTypeHealthCheck    EventType = "health.check"
	EventTypeHealthRecovery EventType = "health.recovery"
	EventTypeHealthFailure  EventType = "health.failure"

	// Dependency events
	EventTypeDependencyStart EventType = "dependency.start"
	EventTypeDependencyStop  EventType = "dependency.stop"
	EventTypeCascadeStop     EventType = "cascade.stop"
	EventTypeCascadeStart    EventType = "cascade.start"

	// User action events
	EventTypeUserAction EventType = "user.action"

	// System events
	EventTypeSystemStartup  EventType = "system.startup"
	EventTypeSystemShutdown EventType = "system.shutdown"
	EventTypeConfigChange   EventType = "config.change"
)

// EventSeverity indicates the importance/severity of an event
type EventSeverity string

const (
	SeverityTrace EventSeverity = "trace"
	SeverityDebug EventSeverity = "debug"
	SeverityInfo  EventSeverity = "info"
	SeverityWarn  EventSeverity = "warn"
	SeverityError EventSeverity = "error"
	SeverityFatal EventSeverity = "fatal"
)

// Event is the base interface for all events in the system
type Event interface {
	// Type returns the event type
	Type() EventType

	// Source returns the source component/service that generated this event
	Source() string

	// Timestamp returns when the event occurred
	Timestamp() time.Time

	// Severity returns the event severity
	Severity() EventSeverity

	// CorrelationID returns the correlation ID for tracing related events
	CorrelationID() string

	// CausedBy returns what triggered this event
	CausedBy() string

	// ParentID returns the parent event ID if this is part of a cascade
	ParentID() string

	// Metadata returns additional event-specific data
	Metadata() map[string]interface{}

	// String returns a human-readable description of the event
	String() string
}

// BaseEvent provides common event functionality
type BaseEvent struct {
	EventType     EventType              `json:"type"`
	SourceLabel   string                 `json:"source"`
	EventTime     time.Time              `json:"timestamp"`
	EventSeverity EventSeverity          `json:"severity"`
	CorrelationId string                 `json:"correlation_id"`
	Cause         string                 `json:"caused_by"`
	Parent        string                 `json:"parent_id,omitempty"`
	Meta          map[string]interface{} `json:"metadata,omitempty"`
}

// Type implements Event interface
func (e BaseEvent) Type() EventType {
	return e.EventType
}

// Source implements Event interface
func (e BaseEvent) Source() string {
	return e.SourceLabel
}

// Timestamp implements Event interface
func (e BaseEvent) Timestamp() time.Time {
	return e.EventTime
}

// Severity implements Event interface
func (e BaseEvent) Severity() EventSeverity {
	return e.EventSeverity
}

// CorrelationID implements Event interface
func (e BaseEvent) CorrelationID() string {
	return e.CorrelationId
}

// CausedBy implements Event interface
func (e BaseEvent) CausedBy() string {
	return e.Cause
}

// ParentID implements Event interface
func (e BaseEvent) ParentID() string {
	return e.Parent
}

// Metadata implements Event interface
func (e BaseEvent) Metadata() map[string]interface{} {
	if e.Meta == nil {
		return make(map[string]interface{})
	}
	return e.Meta
}

// String implements Event interface
func (e BaseEvent) String() string {
	return string(e.EventType) + " from " + e.SourceLabel
}

// ServiceStateEvent represents a service state change
type ServiceStateEvent struct {
	BaseEvent
	ServiceType ServiceType  `json:"service_type"`
	OldState    ServiceState `json:"old_state"`
	NewState    ServiceState `json:"new_state"`
	IsReady     bool         `json:"is_ready"`
	Error       error        `json:"error,omitempty"`
	ProxyPort   int          `json:"proxy_port,omitempty"`
	PID         int          `json:"pid,omitempty"`
}

// String returns a human-readable description
func (e ServiceStateEvent) String() string {
	if e.Error != nil {
		return e.SourceLabel + " (" + string(e.ServiceType) + ") " + string(e.OldState) + " → " + string(e.NewState) + " (error: " + e.Error.Error() + ")"
	}
	return e.SourceLabel + " (" + string(e.ServiceType) + ") " + string(e.OldState) + " → " + string(e.NewState)
}

// HealthEvent represents a health check event
type HealthEvent struct {
	BaseEvent
	ContextName      string `json:"context_name"`
	ClusterShortName string `json:"cluster_short_name"`
	IsMC             bool   `json:"is_mc"`
	IsHealthy        bool   `json:"is_healthy"`
	ReadyNodes       int    `json:"ready_nodes"`
	TotalNodes       int    `json:"total_nodes"`
	Error            error  `json:"error,omitempty"`
}

// String returns a human-readable description
func (e HealthEvent) String() string {
	clusterType := "WC"
	if e.IsMC {
		clusterType = "MC"
	}

	if e.IsHealthy {
		return fmt.Sprintf("%s %s healthy (%d/%d nodes)", clusterType, e.ClusterShortName, e.ReadyNodes, e.TotalNodes)
	}

	errorMsg := ""
	if e.Error != nil {
		errorMsg = " (error: " + e.Error.Error() + ")"
	}
	return fmt.Sprintf("%s %s unhealthy%s", clusterType, e.ClusterShortName, errorMsg)
}

// DependencyEvent represents dependency-related events (cascade start/stop)
type DependencyEvent struct {
	BaseEvent
	DependencyType string   `json:"dependency_type"` // "k8s", "port_forward", "mcp_server"
	AffectedNodes  []string `json:"affected_nodes"`  // List of affected service/node IDs
	Action         string   `json:"action"`          // "start", "stop", "restart"
}

// String returns a human-readable description
func (e DependencyEvent) String() string {
	return fmt.Sprintf("%s cascade from %s affecting %d services", e.Action, e.SourceLabel, len(e.AffectedNodes))
}

// UserActionEvent represents user-initiated actions
type UserActionEvent struct {
	BaseEvent
	Action     string `json:"action"`      // "start", "stop", "restart"
	TargetType string `json:"target_type"` // "service", "cluster"
	TargetName string `json:"target_name"`
}

// String returns a human-readable description
func (e UserActionEvent) String() string {
	return "User " + e.Action + " " + e.TargetType + " " + e.TargetName
}

// SystemEvent represents system-level events
type SystemEvent struct {
	BaseEvent
	Component string `json:"component"` // "orchestrator", "service_manager", "tui"
	Action    string `json:"action"`    // "startup", "shutdown", "config_change"
	Details   string `json:"details,omitempty"`
}

// String returns a human-readable description
func (e SystemEvent) String() string {
	if e.Details != "" {
		return e.Component + " " + e.Action + ": " + e.Details
	}
	return e.Component + " " + e.Action
}

// NewServiceStateEvent creates a new service state event
func NewServiceStateEvent(serviceType ServiceType, sourceLabel string, oldState, newState ServiceState) *ServiceStateEvent {
	eventType := mapStateToEventType(newState)
	severity := mapStateToSeverity(newState)

	return &ServiceStateEvent{
		BaseEvent: BaseEvent{
			EventType:     eventType,
			SourceLabel:   sourceLabel,
			EventTime:     time.Now(),
			EventSeverity: severity,
			CorrelationId: GenerateCorrelationID(),
			Cause:         "unknown",
			Meta:          make(map[string]interface{}),
		},
		ServiceType: serviceType,
		OldState:    oldState,
		NewState:    newState,
		IsReady:     (newState == StateRunning),
	}
}

// NewHealthEvent creates a new health event
func NewHealthEvent(contextName, clusterShortName string, isMC, isHealthy bool, readyNodes, totalNodes int) *HealthEvent {
	eventType := EventTypeHealthCheck
	severity := SeverityInfo

	if !isHealthy {
		eventType = EventTypeHealthFailure
		severity = SeverityError
	}

	return &HealthEvent{
		BaseEvent: BaseEvent{
			EventType:     eventType,
			SourceLabel:   "health-monitor",
			EventTime:     time.Now(),
			EventSeverity: severity,
			CorrelationId: GenerateCorrelationID(),
			Cause:         "health_check",
			Meta:          make(map[string]interface{}),
		},
		ContextName:      contextName,
		ClusterShortName: clusterShortName,
		IsMC:             isMC,
		IsHealthy:        isHealthy,
		ReadyNodes:       readyNodes,
		TotalNodes:       totalNodes,
	}
}

// NewDependencyEvent creates a new dependency event
func NewDependencyEvent(sourceLabel, dependencyType, action string, affectedNodes []string) *DependencyEvent {
	eventType := EventTypeDependencyStart
	if action == "stop" {
		eventType = EventTypeCascadeStop
	} else if action == "start" {
		eventType = EventTypeCascadeStart
	}

	return &DependencyEvent{
		BaseEvent: BaseEvent{
			EventType:     eventType,
			SourceLabel:   sourceLabel,
			EventTime:     time.Now(),
			EventSeverity: SeverityInfo,
			CorrelationId: GenerateCorrelationID(),
			Cause:         "dependency_change",
			Meta:          make(map[string]interface{}),
		},
		DependencyType: dependencyType,
		AffectedNodes:  affectedNodes,
		Action:         action,
	}
}

// NewUserActionEvent creates a new user action event
func NewUserActionEvent(action, targetType, targetName string) *UserActionEvent {
	return &UserActionEvent{
		BaseEvent: BaseEvent{
			EventType:     EventTypeUserAction,
			SourceLabel:   "user",
			EventTime:     time.Now(),
			EventSeverity: SeverityInfo,
			CorrelationId: GenerateCorrelationID(),
			Cause:         "user_action",
			Meta:          make(map[string]interface{}),
		},
		Action:     action,
		TargetType: targetType,
		TargetName: targetName,
	}
}

// NewSystemEvent creates a new system event
func NewSystemEvent(component, action, details string) *SystemEvent {
	severity := SeverityInfo
	if action == "shutdown" || action == "error" {
		severity = SeverityWarn
	}

	return &SystemEvent{
		BaseEvent: BaseEvent{
			EventType:     EventTypeSystemStartup,
			SourceLabel:   component,
			EventTime:     time.Now(),
			EventSeverity: severity,
			CorrelationId: GenerateCorrelationID(),
			Cause:         "system_operation",
			Meta:          make(map[string]interface{}),
		},
		Component: component,
		Action:    action,
		Details:   details,
	}
}

// Helper functions to map states to event types and severities
func mapStateToEventType(state ServiceState) EventType {
	switch state {
	case StateStarting:
		return EventTypeServiceStarting
	case StateRunning:
		return EventTypeServiceRunning
	case StateStopping:
		return EventTypeServiceStopping
	case StateStopped:
		return EventTypeServiceStopped
	case StateFailed:
		return EventTypeServiceFailed
	case StateRetrying:
		return EventTypeServiceRetrying
	default:
		return EventTypeServiceStarting
	}
}

func mapStateToSeverity(state ServiceState) EventSeverity {
	switch state {
	case StateFailed:
		return SeverityError
	case StateRetrying:
		return SeverityWarn
	case StateRunning, StateStopped:
		return SeverityInfo
	case StateStarting, StateStopping:
		return SeverityDebug
	default:
		return SeverityInfo
	}
}

// WithCorrelation sets correlation tracking information
func (e *BaseEvent) WithCorrelation(correlationID, causedBy, parentID string) *BaseEvent {
	e.CorrelationId = correlationID
	e.Cause = causedBy
	e.Parent = parentID
	return e
}

// WithMetadata adds metadata to the event
func (e *BaseEvent) WithMetadata(key string, value interface{}) *BaseEvent {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta[key] = value
	return e
}

// WithError adds error information to service state events
func (e *ServiceStateEvent) WithError(err error) *ServiceStateEvent {
	e.Error = err
	if err != nil {
		e.EventSeverity = SeverityError
		e.NewState = StateFailed
	}
	return e
}

// WithServiceData adds service-specific data (port, PID)
func (e *ServiceStateEvent) WithServiceData(proxyPort, pid int) *ServiceStateEvent {
	e.ProxyPort = proxyPort
	e.PID = pid
	return e
}

// WithError adds error information to health events
func (e *HealthEvent) WithError(err error) *HealthEvent {
	e.Error = err
	if err != nil {
		e.EventSeverity = SeverityError
		e.IsHealthy = false
	}
	return e
}
