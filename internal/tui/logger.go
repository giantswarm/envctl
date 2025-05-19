package tui

import "fmt"

// The functions in this file provide a unified way for all handlers and
// background goroutines to append messages to the global activity log while
// ensuring length limits and a consistent prefix format. Having them as
// methods on *model keeps access to shared state simple and avoids the need
// for a separate logger instance.

// LogInfo appends an informational message to the combined activity log.
func (m *model) LogInfo(format string, a ...interface{}) {
    m.appendLogLine("[INFO] " + fmt.Sprintf(format, a...))
}

// LogDebug appends a debug-level message to the combined activity log.
func (m *model) LogDebug(format string, a ...interface{}) {
    m.appendLogLine("[DEBUG] " + fmt.Sprintf(format, a...))
}

// LogWarn appends a warning message to the combined activity log.
func (m *model) LogWarn(format string, a ...interface{}) {
    m.appendLogLine("[WARN] " + fmt.Sprintf(format, a...))
}

// LogError appends an error message to the combined activity log.
func (m *model) LogError(format string, a ...interface{}) {
    m.appendLogLine("[ERROR] " + fmt.Sprintf(format, a...))
}

// appendLogLine is a small helper that performs the actual slice append and
// enforces the maxCombinedOutputLines invariant.
func (m *model) appendLogLine(line string) {
    if m == nil {
        return
    }
    m.combinedOutput = append(m.combinedOutput, line)
    if len(m.combinedOutput) > maxCombinedOutputLines {
        m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
    }
}

// AppendLogLines appends multiple preformatted lines (each treated as-is) to the
// combined log while keeping the slice length within bounds. Use this when you
// already have full log lines from external sources and do not wish to prefix
// them.
func (m *model) AppendLogLines(lines []string) {
    if m == nil || len(lines) == 0 {
        return
    }
    m.combinedOutput = append(m.combinedOutput, lines...)
    if len(m.combinedOutput) > maxCombinedOutputLines {
        m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
    }
} 