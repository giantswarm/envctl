package reporting

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ConsoleReporter is an implementation of ServiceReporter that prints updates to the console.
type ConsoleReporter struct {
	// TODO: Add configuration if needed, e.g., a log level filter for console output.
}

// NewConsoleReporter creates a new ConsoleReporter.
func NewConsoleReporter() *ConsoleReporter {
	return &ConsoleReporter{}
}

// Report formats and prints the ManagedServiceUpdate to stdout or stderr.
func (cr *ConsoleReporter) Report(update ManagedServiceUpdate) {
	// Default to using Message as the primary content for display.
	// If Message is empty but Details is not, use Details.
	outputMessage := update.Message
	if strings.TrimSpace(outputMessage) == "" && strings.TrimSpace(update.Details) != "" {
		outputMessage = update.Details
	}

	// Fallback if both Message and Details are empty but it's an error with ErrorDetail
	if strings.TrimSpace(outputMessage) == "" && update.ErrorDetail != nil {
		outputMessage = update.ErrorDetail.Error()
	}

	// Prefix for the log line
	// [TIME] [LEVEL] [SOURCE_TYPE-SOURCE_LABEL] Message
	// Details will be printed on subsequent lines if present and not already part of outputMessage.

	timestamp := update.Timestamp
	if timestamp.IsZero() { // Ensure timestamp is set
		timestamp = time.Now()
	}
	tsFormatted := timestamp.Format("15:04:05.000") // Include milliseconds for better debugging

	var logPrefixBuilder strings.Builder
	logPrefixBuilder.WriteString(fmt.Sprintf("[%s] ", tsFormatted))

	if update.Level != "" {
		logPrefixBuilder.WriteString(fmt.Sprintf("[%s] ", strings.ToUpper(string(update.Level))))
	}

	if update.SourceType != "" || update.SourceLabel != "" {
		logPrefixBuilder.WriteString("[")
		if update.SourceType != "" {
			logPrefixBuilder.WriteString(string(update.SourceType))
		}
		if update.SourceType != "" && update.SourceLabel != "" {
			logPrefixBuilder.WriteString(" - ")
		}
		if update.SourceLabel != "" {
			logPrefixBuilder.WriteString(update.SourceLabel)
		}
		logPrefixBuilder.WriteString("] ")
	}

	// Determine output stream (stdout or stderr)
	outStream := os.Stdout
	if update.IsError || update.Level == LogLevelError || update.Level == LogLevelFatal || update.Level == LogLevelStderr {
		outStream = os.Stderr
	}

	// Print the main message
	fmt.Fprintf(outStream, "%s%s\n", logPrefixBuilder.String(), outputMessage)

	// Print multi-line details if Details was different from what was used in outputMessage
	// and Details is not empty.
	if update.Details != "" && outputMessage != update.Details {
		for _, line := range strings.Split(strings.TrimSuffix(update.Details, "\n"), "\n") {
			fmt.Fprintf(outStream, "%s  %s\n", logPrefixBuilder.String(), line) // Indent details
		}
	}

	// If ErrorDetail is present and IsError is true, and it wasn't the primary outputMessage, print it.
	if update.ErrorDetail != nil && update.IsError && outputMessage != update.ErrorDetail.Error() {
		fmt.Fprintf(outStream, "%s  ERROR_DETAIL: %v\n", logPrefixBuilder.String(), update.ErrorDetail)
	}
}
