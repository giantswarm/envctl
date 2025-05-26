package reporting

import (
	"envctl/pkg/logging" // Import the new logging package
	"fmt"
	"time"
	// "os" // No longer needed for direct Fprintf
	// "strings" // No longer needed for string manipulation here
	// "time" // No longer needed for timestamp formatting here
)

// ConsoleReporter is an implementation of ServiceReporter that logs updates to the console
// via the pkg/logging package and maintains state in a StateStore.
type ConsoleReporter struct {
	stateStore StateStore // Centralized state management
}

// NewConsoleReporter creates a new ConsoleReporter
func NewConsoleReporter() *ConsoleReporter {
	return &ConsoleReporter{
		stateStore: NewStateStore(),
	}
}

// NewConsoleReporterWithStateStore creates a new ConsoleReporter with a specific state store
func NewConsoleReporterWithStateStore(stateStore StateStore) *ConsoleReporter {
	if stateStore == nil {
		stateStore = NewStateStore()
	}
	return &ConsoleReporter{
		stateStore: stateStore,
	}
}

// Report processes a ManagedServiceUpdate by logging it to the console and updating the state store
func (c *ConsoleReporter) Report(update ManagedServiceUpdate) {
	// Set timestamp if not provided
	if update.Timestamp.IsZero() {
		update.Timestamp = time.Now()
	}

	// Ensure correlation ID is set
	if update.CorrelationID == "" {
		update.CorrelationID = GenerateCorrelationID()
	}

	// Update the centralized state store first
	stateChanged := false
	if c.stateStore != nil {
		changed, err := c.stateStore.SetServiceState(update)
		if err != nil {
			logging.Error("ConsoleReporter", err, "Failed to update state store for service %s", update.SourceLabel)
		}
		stateChanged = changed
	}

	// Only log actual state changes to reduce noise
	if !stateChanged && update.State != StateFailed && update.ErrorDetail == nil {
		return
	}

	// Determine the subsystem for logging
	subsystem := string(update.SourceType)
	if update.SourceLabel != "" {
		subsystem = string(update.SourceType) + "-" + update.SourceLabel
	}

	// Build the log message
	logMessage := "State: " + string(update.State)
	if update.ProxyPort > 0 {
		logMessage += fmt.Sprintf(", Port: %d", update.ProxyPort)
	}
	if update.PID > 0 {
		logMessage += fmt.Sprintf(", PID: %d", update.PID)
	}
	if update.CorrelationID != "" {
		logMessage += ", CorrelationID: " + update.CorrelationID
	}
	if update.CausedBy != "" && update.CausedBy != "unknown" {
		logMessage += ", CausedBy: " + update.CausedBy
	}

	// Log based on state and error
	switch {
	case update.ErrorDetail != nil:
		logging.Error(subsystem, update.ErrorDetail, "%s", logMessage)
	case update.State == StateFailed:
		logging.Error(subsystem, nil, "%s", logMessage)
	case update.State == StateUnknown:
		logging.Warn(subsystem, "%s", logMessage)
	case update.State == StateRunning || update.State == StateStopped:
		logging.Info(subsystem, "%s", logMessage)
	case update.State == StateStarting || update.State == StateStopping || update.State == StateRetrying:
		logging.Debug(subsystem, "%s", logMessage)
	default:
		logging.Info(subsystem, "%s", logMessage)
	}
}

// GetStateStore returns the underlying state store
func (c *ConsoleReporter) GetStateStore() StateStore {
	return c.stateStore
}
