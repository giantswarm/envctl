package model

// Ensure necessary imports like "time", "fmt" are NOT here if not used by AddRawLineToActivityLog directly

// AddRawLineToActivityLog adds a pre-formatted log entry to the model's activity log,
// ensuring it doesn't exceed MaxActivityLogLines and sets the dirty flag.
func AddRawLineToActivityLog(m *Model, entry string) {
	m.ActivityLog = append(m.ActivityLog, entry)
	if len(m.ActivityLog) > MaxActivityLogLines {
		m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-MaxActivityLogLines:]
	}
	m.ActivityLogDirty = true
}

// LogDebug, LogInfo, and AppendActivityLog are removed from this file.
// Logging logic is now centralized via the pkg/logging package and controller handlers.
