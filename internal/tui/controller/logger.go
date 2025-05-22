package controller

import (
	// "envctl/internal/reporting" // No longer needed by logger.go directly
	"envctl/internal/tui/model"
	"envctl/pkg/logging"
	"strings"
	// "time" // No longer needed by logger.go directly
)

const controllerSubsystem = "Controller"
const tuiSubsystem = "TUI"

// The functions in this file provide a unified way for all handlers and
// background goroutines to append messages to the global activity log while
// ensuring length limits and a consistent prefix format. Having them as
// methods on *model keeps access to shared state simple and avoids the need
// for a separate logger instance.

// LogInfo logs an informational message using the new logging package.
// The subsystem is derived from the context (e.g., "Controller", "KeyHandler").
// The model 'm' is no longer needed here directly as logging is global.
func LogInfo(subsystem string, format string, a ...interface{}) {
	logging.Info(subsystem, format, a...)
}

// LogDebug logs a debug-level message using the new logging package.
// It respects the TUI model's DebugMode flag.
func LogDebug(m *model.Model, subsystem string, format string, a ...interface{}) {
	if m != nil && m.DebugMode {
		logging.Debug(subsystem, format, a...)
	}
}

// LogWarn logs a warning message using the new logging package.
func LogWarn(subsystem string, format string, a ...interface{}) {
	logging.Warn(subsystem, format, a...)
}

// LogError logs an error message using the new logging package.
// It now takes an error object as well.
func LogError(subsystem string, err error, format string, a ...interface{}) {
	logging.Error(subsystem, err, format, a...)
}

// LogStdout logs multiple lines from a process's stdout as INFO level logs via the new logging package.
func LogStdout(source string, outputLines string) {
	if outputLines == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(outputLines, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			logging.Info(source+"-stdout", "%s", line)
		}
	}
}

// LogStderr logs multiple lines from a process's stderr as ERROR level logs via the new logging package.
func LogStderr(source string, errorLines string) {
	if errorLines == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(errorLines, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Passing nil for error as the error is the line itself
			logging.Error(source+"-stderr", nil, "%s", line)
		}
	}
}

// appendLogLine is now REMOVED as logging is handled by pkg/logging and TUI controller appends to ActivityLog.
