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
func (cr *ConsoleReporter) Report(update ManagedServiceUpdate) {
	// The subsystem for the log message will be composed from SourceType and SourceLabel.
	subsystem := string(update.SourceType)
	if update.SourceLabel != "" {
		subsystem = fmt.Sprintf("%s-%s", update.SourceType, update.SourceLabel)
	}

	// The primary message is the state.
	logMessage := fmt.Sprintf("State: %s", update.State)

	// Determine the log level and function based on update.ServiceLevel and update.ErrorDetail.
	if update.ErrorDetail != nil {
		// If ErrorDetail is present, it's definitely an error or a warning with an error.
		// The ServiceLevel should ideally reflect this.
		// We pass update.ErrorDetail directly to logging.Error or logging.Warn.
		if update.ServiceLevel == LogLevelWarn {
			logging.Warn(subsystem, "%s (Error: %v)", logMessage, update.ErrorDetail)
		} else {
			// Default to Error if ErrorDetail is present and not explicitly Warn.
			logging.Error(subsystem, update.ErrorDetail, "%s", logMessage)
		}
	} else {
		// No ErrorDetail, so use ServiceLevel to determine Info, Warn, Debug.
		switch update.ServiceLevel {
		case LogLevelError, LogLevelFatal:
			// This case should ideally have ErrorDetail, but if not, log message as error.
			logging.Error(subsystem, nil, "%s (Reported as Error/Fatal without details)", logMessage)
		case LogLevelWarn:
			logging.Warn(subsystem, "%s", logMessage)
		case LogLevelDebug:
			logging.Debug(subsystem, "%s", logMessage)
		case LogLevelTrace:
			// Assuming pkg/logging might not have Trace, map to Debug or handle if it does.
			logging.Debug(subsystem, "%s (Trace)", logMessage)
		default: // LogLevelInfo, LogLevelStdout, LogLevelStderr, or unknown
			logging.Info(subsystem, "%s", logMessage)
		}
	}
}
