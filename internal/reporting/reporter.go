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

// ManagedServiceUpdate carries state updates from various components.
// This struct is the standardized way for components to report their state back to the TUI or console.
// It focuses on the *state* of the service, not verbose logs (which go through pkg/logging).
type ManagedServiceUpdate struct {
	Timestamp   time.Time
	SourceType  ServiceType
	SourceLabel string
	State       ServiceState // The current discrete state of the service.

	// ServiceLevel reflects the severity of the current State (e.g., StateFailed implies LogLevelError).
	ServiceLevel LogLevel
	IsReady      bool // Derived from State (e.g., true if State == StateRunning).

	ErrorDetail error // Associated Go error if State is Failed or a warning state has an error.
}

// String provides a simple string representation for debugging the update itself.
func (msu ManagedServiceUpdate) String() string {
	return fmt.Sprintf("StateUpdate(TS: %s, Source: %s-%s, State: %s, Level: %s, Ready: %t, ErrDetail: %v)",
		msu.Timestamp.Format(time.RFC3339), msu.SourceType, msu.SourceLabel, msu.State, msu.ServiceLevel, msu.IsReady, msu.ErrorDetail)
}

// ServiceReporter defines a unified interface for reporting service/component state updates.
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
