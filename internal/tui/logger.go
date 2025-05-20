package tui

import (
	"fmt"
	"strings"
)

// The functions in this file provide a unified way for all handlers and
// background goroutines to append messages to the global activity log while
// ensuring length limits and a consistent prefix format. Having them as
// methods on *model keeps access to shared state simple and avoids the need
// for a separate logger instance.

// LogInfo appends an informational message to the activity log.
func (m *model) LogInfo(format string, a ...interface{}) {
	m.appendLogLine("[INFO] " + fmt.Sprintf(format, a...))
}

// LogDebug appends a debug-level message to the activity log.
// Debug messages are only logged when debug mode is active (toggled with 'z').
func (m *model) LogDebug(format string, a ...interface{}) {
	// Only log debug messages when debug mode is enabled
	if m != nil && m.debugMode {
		m.appendLogLine("[DEBUG] " + fmt.Sprintf(format, a...))
	}
}

// LogWarn appends a warning message to the activity log.
func (m *model) LogWarn(format string, a ...interface{}) {
	m.appendLogLine("[WARN] " + fmt.Sprintf(format, a...))
}

// LogError appends an error message to the activity log.
func (m *model) LogError(format string, a ...interface{}) {
	m.appendLogLine("[ERROR] " + fmt.Sprintf(format, a...))
}

// appendLogLine is a small helper that performs the actual slice append and
// enforces the maxActivityLogLines invariant.
func (m *model) appendLogLine(line string) {
	if m == nil {
		return
	}
	m.activityLog = append(m.activityLog, line)
	if len(m.activityLog) > maxActivityLogLines {
		m.activityLog = m.activityLog[len(m.activityLog)-maxActivityLogLines:]
	}

	// Mark log content dirty so viewports can refresh lazily.
	m.activityLogDirty = true
}

// LogStdout logs multiple lines from a process's stdout as INFO level logs.
func (m *model) LogStdout(source string, outputLines string) {
	if m == nil || outputLines == "" {
		return
	}

	lines := strings.Split(strings.TrimRight(outputLines, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			m.LogInfo("[%s] %s", source, line)
		}
	}
}

// LogStderr logs multiple lines from a process's stderr as ERROR level logs.
func (m *model) LogStderr(source string, errorLines string) {
	if m == nil || errorLines == "" {
		return
	}

	lines := strings.Split(strings.TrimRight(errorLines, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			m.LogError("[%s stderr] %s", source, line)
		}
	}
}
