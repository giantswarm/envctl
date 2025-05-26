package reporting

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Global sequence counter for message ordering
var globalSequence int64

// ServiceType indicates the kind of component sending the update.
type ServiceType string

const (
	ServiceTypePortForward ServiceType = "PortForward"
	ServiceTypeMCPServer   ServiceType = "MCPServer"
	ServiceTypeSystem      ServiceType = "System"        // For general system events, e.g., ServiceManager, Controller, HealthPoller
	ServiceTypeKube        ServiceType = "KubeOperation" // For direct k8s operations like login, context switch
	ServiceTypeExternalCmd ServiceType = "ExternalCmd"   // For outputs from external commands like tsh
)

// String makes ServiceType satisfy the fmt.Stringer interface.
func (st ServiceType) String() string {
	return string(st)
}

// LogLevel defines the severity or nature of a status update or a log entry if used generally.
// For ManagedServiceUpdate, this will be ServiceLevel reflecting the status severity.
type LogLevel string

const (
	LogLevelTrace  LogLevel = "TRACE"
	LogLevelDebug  LogLevel = "DEBUG"
	LogLevelInfo   LogLevel = "INFO"
	LogLevelWarn   LogLevel = "WARN"
	LogLevelError  LogLevel = "ERROR"
	LogLevelFatal  LogLevel = "FATAL"  // For errors that might lead to termination
	LogLevelStdout LogLevel = "STDOUT" // For raw stdout from processes (primarily for direct logging, not status)
	LogLevelStderr LogLevel = "STDERR" // For raw stderr from processes (primarily for direct logging, not status)
)

// String makes LogLevel satisfy the fmt.Stringer interface.
func (ll LogLevel) String() string {
	return string(ll)
}

// ServiceState defines the discrete state of a managed service.
type ServiceState string

const (
	StateUnknown   ServiceState = "Unknown"
	StateStarting  ServiceState = "Starting"
	StateRunning   ServiceState = "Running"
	StateStopping  ServiceState = "Stopping"
	StateStopped   ServiceState = "Stopped"
	StateFailed    ServiceState = "Failed"
	StateRetrying  ServiceState = "Retrying" // If a service has retry logic
	// Add more states as needed, e.g., StateDegraded, StateConnected, StateDisconnected
)

// String makes ServiceState satisfy the fmt.Stringer interface.
func (ss ServiceState) String() string {
	return string(ss)
}

// GenerateCorrelationID creates a unique correlation ID for tracing related messages
func GenerateCorrelationID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("corr_%d", time.Now().UnixNano())
	}
	return "corr_" + hex.EncodeToString(bytes)
}

// K8sHealthData contains Kubernetes-specific health information
type K8sHealthData struct {
	ReadyNodes int  `json:"ready_nodes"`
	TotalNodes int  `json:"total_nodes"`
	IsMC       bool `json:"is_mc"`
}

// ManagedServiceUpdate represents a state update for a managed service.
// This is the primary data structure for communicating service state changes.
type ManagedServiceUpdate struct {
	// Core identification
	Timestamp   time.Time   `json:"timestamp"`
	SourceType  ServiceType `json:"source_type"`
	SourceLabel string      `json:"source_label"`

	// State information
	State       ServiceState `json:"state"`
	IsReady     bool         `json:"is_ready"`
	ErrorDetail error        `json:"error_detail,omitempty"`

	// Service-specific details
	ProxyPort int `json:"proxy_port,omitempty"` // For MCP servers
	PID       int `json:"pid,omitempty"`        // For processes

	// K8s-specific health data (optional)
	K8sHealth *K8sHealthData `json:"k8s_health,omitempty"`

	// Correlation and causation tracking
	CorrelationID string `json:"correlation_id,omitempty"`
	CausedBy      string `json:"caused_by,omitempty"`
	ParentID      string `json:"parent_id,omitempty"`
	Sequence      int64  `json:"sequence,omitempty"`
}

// NewManagedServiceUpdate creates a new ManagedServiceUpdate with auto-generated correlation ID and sequence number
func NewManagedServiceUpdate(sourceType ServiceType, sourceLabel string, state ServiceState) ManagedServiceUpdate {
	return ManagedServiceUpdate{
		Timestamp:     time.Now(),
		SourceType:    sourceType,
		SourceLabel:   sourceLabel,
		State:         state,
		IsReady:       (state == StateRunning),
		CorrelationID: GenerateCorrelationID(),
		CausedBy:      "unknown",
		Sequence:      atomic.AddInt64(&globalSequence, 1),
	}
}

// WithCause sets the cause and optionally parent ID for this update
func (msu ManagedServiceUpdate) WithCause(causedBy string, parentID ...string) ManagedServiceUpdate {
	msu.CausedBy = causedBy
	if len(parentID) > 0 {
		msu.ParentID = parentID[0]
	}
	return msu
}

// WithError adds error details to the update
func (msu ManagedServiceUpdate) WithError(err error) ManagedServiceUpdate {
	msu.ErrorDetail = err
	if err != nil && msu.State != StateFailed {
		msu.State = StateFailed
		msu.IsReady = false
	}
	return msu
}

// WithServiceData adds service-specific data (port, PID)
func (msu ManagedServiceUpdate) WithServiceData(proxyPort, pid int) ManagedServiceUpdate {
	msu.ProxyPort = proxyPort
	msu.PID = pid
	return msu
}

// WithK8sHealth adds Kubernetes health data to the update
func (msu ManagedServiceUpdate) WithK8sHealth(readyNodes, totalNodes int, isMC bool) ManagedServiceUpdate {
	msu.K8sHealth = &K8sHealthData{
		ReadyNodes: readyNodes,
		TotalNodes: totalNodes,
		IsMC:       isMC,
	}
	return msu
}

// String provides a simple string representation for debugging the update itself.
func (msu ManagedServiceUpdate) String() string {
	portInfo := ""
	if msu.ProxyPort > 0 {
		portInfo = fmt.Sprintf(", Port: %d", msu.ProxyPort)
	}
	if msu.PID > 0 {
		portInfo += fmt.Sprintf(", PID: %d", msu.PID)
	}

	correlationInfo := ""
	if msu.CorrelationID != "" {
		correlationInfo = fmt.Sprintf(", CorrelationID: %s", msu.CorrelationID)
	}
	if msu.CausedBy != "" && msu.CausedBy != "unknown" {
		correlationInfo += fmt.Sprintf(", CausedBy: %s", msu.CausedBy)
	}
	if msu.ParentID != "" {
		correlationInfo += fmt.Sprintf(", ParentID: %s", msu.ParentID)
	}

	return fmt.Sprintf("StateUpdate(TS: %s, Source: %s-%s, State: %s, Ready: %t%s%s, ErrDetail: %v)",
		msu.Timestamp.Format(time.RFC3339), msu.SourceType, msu.SourceLabel, msu.State, msu.IsReady, portInfo, correlationInfo, msu.ErrorDetail)
}

// ServiceReporter is the interface that components use to report their state.
// Implementations can be TUI-based, console-based, or even network-based.
type ServiceReporter interface {
	// Report processes a service state update
	Report(update ManagedServiceUpdate)

	// GetStateStore returns the underlying state store (may return nil if not supported)
	GetStateStore() StateStore
}

// ReporterUpdateMsg is the tea.Msg used to send service updates to the TUI
type ReporterUpdateMsg struct {
	Update ManagedServiceUpdate
}

// Ensure ReporterUpdateMsg implements tea.Msg
var _ tea.Msg = ReporterUpdateMsg{}

// BackpressureNotificationMsg notifies about dropped critical messages
type BackpressureNotificationMsg struct {
	Timestamp     time.Time
	ServiceLabel  string
	DroppedState  ServiceState
	Reason        string
	CorrelationID string
}

// Ensure BackpressureNotificationMsg implements tea.Msg
var _ tea.Msg = BackpressureNotificationMsg{}

// CascadeInfo tracks cascade relationships between services
type CascadeInfo struct {
	InitiatingService string      // The service that triggered the cascade
	AffectedServices  []string    // Services affected by the cascade
	Reason            string      // Why the cascade occurred
	CorrelationID     string      // Correlation ID linking all related operations
	Timestamp         time.Time   // When the cascade started
	CascadeType       CascadeType // Type of cascade operation
}

// CascadeType defines the type of cascade operation
type CascadeType string

const (
	CascadeTypeStop    CascadeType = "stop"    // Cascade stop operation
	CascadeTypeRestart CascadeType = "restart" // Cascade restart operation
	CascadeTypeHealth  CascadeType = "health"  // Health-related cascade
)

// StateTransition tracks state changes with full context
type StateTransition struct {
	Label         string                 // Service label
	FromState     ServiceState           // Previous state
	ToState       ServiceState           // New state
	Timestamp     time.Time              // When the transition occurred
	CorrelationID string                 // Correlation ID for tracking
	CausedBy      string                 // What caused this transition
	ParentID      string                 // Parent operation ID
	Sequence      int64                  // Sequence number for ordering
	Metadata      map[string]interface{} // Additional context data
}

// Ensure reporter.go is correctly placed and has the initial definitions.
