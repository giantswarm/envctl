package reporting

import (
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

// LogLevel defines the severity or nature of the update.
// These can be used by reporters to filter or style output.
type LogLevel string

const (
	LogLevelTrace  LogLevel = "TRACE"
	LogLevelDebug  LogLevel = "DEBUG"
	LogLevelInfo   LogLevel = "INFO"
	LogLevelWarn   LogLevel = "WARN"
	LogLevelError  LogLevel = "ERROR"
	LogLevelFatal  LogLevel = "FATAL"  // For errors that might lead to termination
	LogLevelStdout LogLevel = "STDOUT" // For raw stdout from processes
	LogLevelStderr LogLevel = "STDERR" // For raw stderr from processes
)

// String makes LogLevel satisfy the fmt.Stringer interface.
func (ll LogLevel) String() string {
	return string(ll)
}

// ManagedServiceUpdate carries status and log updates from various components.
// This struct is the standardized way for components to report back to the TUI or console.
// The goal is to make it rich enough to convey all necessary information for different reporters (TUI, console).
// TODO: Review fields, especially how to handle pure log lines vs. status changes.
// Maybe add a LogLevel field or a separate LogMessage struct/interface if updates become too broad.
type ManagedServiceUpdate struct {
	// Timestamp of when the event occurred or was reported.
	Timestamp time.Time

	// SourceType identifies the kind of component sending the update.
	SourceType ServiceType
	// SourceLabel uniquely identifies the specific instance of the service/component (e.g., "Prometheus (MC)", "tsh-login").
	SourceLabel string

	// Level defines the severity or nature of the update.
	Level LogLevel
	// Message provides a human-readable status (e.g., "Running", "Attempting to start...", "Error").
	// Can be empty if this update is purely a log message or a metric update.
	Message string
	// Details contains any detailed log lines associated with this update. Can be multi-line.
	Details string

	// State indicators for services/components, primarily used by the TUI model to update its state.
	// These might not be relevant for all LogLevels (e.g., a simple LogLevelInfo might not change IsReady).
	IsError     bool
	IsReady     bool
	ErrorDetail error
}

// String provides a simple string representation for debugging the update itself.
func (msu ManagedServiceUpdate) String() string {
	return fmt.Sprintf("Update(TS: %s, Source: %s-%s, Level: %s, Msg: '%s', Ready: %t, Error: %t, ErrDetail: %v, Details: '%s')",
		msu.Timestamp.Format(time.RFC3339), msu.SourceType, msu.SourceLabel, msu.Level, msu.Message, msu.IsReady, msu.IsError, msu.ErrorDetail, msu.Details)
}

// ServiceReporter defines a unified interface for reporting service/component updates.
// Implementations will handle these updates appropriately (e.g., TUI display, console logging).
// This interface will be the abstraction point for all components that need to report status or logs.
type ServiceReporter interface {
	// Report processes an update. Implementations should be goroutine-safe
	// if they are to be called concurrently from multiple sources.
	Report(update ManagedServiceUpdate)
}

// ReporterUpdateMsg is the tea.Msg used by TUIReporter to send updates to the TUI.
// It embeds the ManagedServiceUpdate.
type ReporterUpdateMsg struct {
	Update ManagedServiceUpdate
}

// Ensure ReporterUpdateMsg implements tea.Msg (it does implicitly by being a struct).
var _ tea.Msg = ReporterUpdateMsg{}

// Ensure reporter.go is correctly placed and has the initial definitions.
