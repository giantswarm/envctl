package reporting

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
	StateUnknown  ServiceState = "Unknown"
	StateStarting ServiceState = "Starting"
	StateRunning  ServiceState = "Running"
	StateStopping ServiceState = "Stopping"
	StateStopped  ServiceState = "Stopped"
	StateFailed   ServiceState = "Failed"
	StateRetrying ServiceState = "Retrying" // If a service has retry logic
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

// ManagedServiceUpdate carries state updates from various components.
// This struct is the standardized way for components to report their state back to the TUI or console.
// It focuses on the *state* of the service, not verbose logs (which go through pkg/logging).
type ManagedServiceUpdate struct {
	Timestamp   time.Time
	SourceType  ServiceType
	SourceLabel string
	State       ServiceState // The current discrete state of the service.

	IsReady bool // Derived from State (e.g., true if State == StateRunning).

	ErrorDetail error // Associated Go error if State is Failed or a warning state has an error.

	// Additional service-specific data
	ProxyPort int // For MCP servers: the port that mcp-proxy is listening on (0 if not applicable)
	PID       int // For MCP servers: the process ID (0 if not applicable or not yet started)

	// Correlation tracking for debugging and tracing cascading effects
	CorrelationID string // Unique ID to track related messages
	CausedBy      string // What triggered this update (e.g., "user_action", "health_check", "dependency_failure")
	ParentID      string // ID of the parent operation that caused this update
}

// NewManagedServiceUpdate creates a new ManagedServiceUpdate with auto-generated correlation ID
func NewManagedServiceUpdate(sourceType ServiceType, sourceLabel string, state ServiceState) ManagedServiceUpdate {
	return ManagedServiceUpdate{
		Timestamp:     time.Now(),
		SourceType:    sourceType,
		SourceLabel:   sourceLabel,
		State:         state,
		IsReady:       (state == StateRunning),
		CorrelationID: GenerateCorrelationID(),
		CausedBy:      "unknown",
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

// ServiceReporter defines a unified interface for reporting service/component state updates.
// Implementations will handle these updates appropriately (e.g., TUI display, console logging).
// This interface will be the abstraction point for all components that need to report status or logs.
type ServiceReporter interface {
	// Report processes an update. Implementations should be goroutine-safe
	// if they are to be called concurrently from multiple sources.
	Report(update ManagedServiceUpdate)

	// ReportHealth processes a health status update
	ReportHealth(update HealthStatusUpdate)
}

// ReporterUpdateMsg is the tea.Msg used by TUIReporter to send updates to the TUI.
// It embeds the ManagedServiceUpdate.
type ReporterUpdateMsg struct {
	Update ManagedServiceUpdate
}

// Ensure ReporterUpdateMsg implements tea.Msg (it does implicitly by being a struct).
var _ tea.Msg = ReporterUpdateMsg{}

// HealthStatusUpdate carries k8s cluster health status information
type HealthStatusUpdate struct {
	Timestamp        time.Time
	ContextName      string // The k8s context name
	ClusterShortName string // Short name of the cluster (MC or WC)
	IsMC             bool   // True if this is for management cluster
	IsHealthy        bool   // Overall health status
	ReadyNodes       int    // Number of ready nodes
	TotalNodes       int    // Total number of nodes
	Error            error  // Any error encountered during health check
}

// HealthStatusMsg is the tea.Msg used to send health updates to the TUI
type HealthStatusMsg struct {
	Update HealthStatusUpdate
}

// Ensure HealthStatusMsg implements tea.Msg
var _ tea.Msg = HealthStatusMsg{}

// Ensure reporter.go is correctly placed and has the initial definitions.
