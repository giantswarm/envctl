package reporting

import (
	"envctl/pkg/logging" // Import the new logging package
	"fmt"
	// "os" // No longer needed for direct Fprintf
	// "strings" // No longer needed for string manipulation here
	// "time" // No longer needed for timestamp formatting here
)

// ConsoleReporter is an implementation of ServiceReporter that prints updates to the console
// by leveraging the centralized logging package.
type ConsoleReporter struct{}

// NewConsoleReporter creates a new ConsoleReporter.
func NewConsoleReporter() *ConsoleReporter {
	return &ConsoleReporter{}
}

// Report translates the ManagedServiceUpdate into a structured log message via the logging package.
// The log level is inferred from the State and ErrorDetail.
func (cr *ConsoleReporter) Report(update ManagedServiceUpdate) {
	// The subsystem for the log message will be composed from SourceType and SourceLabel.
	subsystem := string(update.SourceType)
	if update.SourceLabel != "" {
		subsystem = fmt.Sprintf("%s-%s", update.SourceType, update.SourceLabel)
	}

	// The primary message is the state.
	logMessage := fmt.Sprintf("State: %s", update.State)

	if update.ErrorDetail != nil {
		// If ErrorDetail is present, it's an error.
		logging.Error(subsystem, update.ErrorDetail, "%s", logMessage)
	} else {
		// No ErrorDetail, so determine log level based on State.
		switch update.State {
		case StateFailed:
			logging.Error(subsystem, nil, "%s", logMessage)
		case StateUnknown:
			// "Unknown" states are often transient or less critical than outright failures without error.
			logging.Warn(subsystem, "%s", logMessage)
		case StateStarting, StateRetrying, StateStopping:
			// Transient states that can be noisy; log as Debug.
			logging.Debug(subsystem, "%s", logMessage)
		case StateRunning, StateStopped:
			// Significant, stable states.
			logging.Info(subsystem, "%s", logMessage)
		default:
			// For any other unclassified state, default to Info.
			logging.Info(subsystem, "%s", logMessage)
		}
	}
}
