package model

import (
	"fmt"
	"time"
)

// AppendActivityLog adds a new entry to the model's activity log,
// ensuring it doesn't exceed MaxActivityLogLines and sets the dirty flag.
func AppendActivityLog(m *Model, entry string) {
	// Prepend timestamp (optional, but common for logs)
	timestamp := time.Now().Format("15:04:05")
	fullEntry := fmt.Sprintf("%s %s", timestamp, entry)

	m.ActivityLog = append(m.ActivityLog, fullEntry)
	if len(m.ActivityLog) > MaxActivityLogLines {
		m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-MaxActivityLogLines:]
	}
	m.ActivityLogDirty = true
}

// LogDebug is a placeholder for where your existing LogDebug might be defined or moved.
// If it appends to ActivityLog, it should also use MaxActivityLogLines and set ActivityLogDirty.
func LogDebug(m *Model, format string, args ...interface{}) {
	// Example implementation, replace with your actual one or integrate its logic.
	if m.DebugMode {
		entry := fmt.Sprintf("[DEBUG] "+format, args...)
		AppendActivityLog(m, entry)
	}
}

// LogInfo is a placeholder for where your existing LogInfo might be defined or moved.
func LogInfo(m *Model, format string, args ...interface{}) {
	// Example implementation, replace with your actual one or integrate its logic.
	entry := fmt.Sprintf("[INFO] "+format, args...)
	AppendActivityLog(m, entry)
} 