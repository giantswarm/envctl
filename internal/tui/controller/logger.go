package controller

import (
	// "envctl/internal/reporting" // No longer needed by logger.go directly
	"envctl/internal/tui/model"
	"fmt"
	"strings"
	// "time" // No longer needed by logger.go directly
)

// The functions in this file provide a unified way for all handlers and
// background goroutines to append messages to the global activity log while
// ensuring length limits and a consistent prefix format. Having them as
// methods on *model keeps access to shared state simple and avoids the need
// for a separate logger instance.

// LogInfo appends an informational message to the activity log.
func LogInfo(m *model.Model, format string, a ...interface{}) {
	appendLogLine(m, "[INFO] "+fmt.Sprintf(format, a...))
}

// LogDebug appends a debug-level message to the activity log.
func LogDebug(m *model.Model, format string, a ...interface{}) {
	if m != nil && m.DebugMode {
		appendLogLine(m, "[DEBUG] "+fmt.Sprintf(format, a...))
	}
}

// LogWarn appends a warning message to the activity log.
func LogWarn(m *model.Model, format string, a ...interface{}) {
	appendLogLine(m, "[WARN] "+fmt.Sprintf(format, a...))
}

// LogError appends an error message to the activity log.
func LogError(m *model.Model, format string, a ...interface{}) {
	appendLogLine(m, "[ERROR] "+fmt.Sprintf(format, a...))
}

// appendLogLine is a small helper that performs the actual slice append and
// enforces the MaxActivityLogLines invariant.
// THIS WILL BE REMOVED once all logging goes through the reporter and handleReporterUpdate.
func appendLogLine(m *model.Model, line string) {
	if m == nil {
		return
	}
	m.ActivityLog = append(m.ActivityLog, line)
	if len(m.ActivityLog) > model.MaxActivityLogLines {
		m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-model.MaxActivityLogLines:]
	}
	m.ActivityLogDirty = true
}

// LogStdout logs multiple lines from a process's stdout as INFO level logs.
func LogStdout(m *model.Model, source string, outputLines string) {
	if m == nil || outputLines == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(outputLines, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Calls the local LogInfo, which now calls appendLogLine directly.
			LogInfo(m, "[%s] %s", source, line)
		}
	}
}

// LogStderr logs multiple lines from a process's stderr as ERROR level logs.
func LogStderr(m *model.Model, source string, errorLines string) {
	if m == nil || errorLines == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(errorLines, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Calls the local LogError, which now calls appendLogLine directly.
			LogError(m, "[%s stderr] %s", source, line)
		}
	}
}
