package model

// AddRawLineToActivityLog adds a pre-formatted log entry to the model's activity log,
// ensuring it doesn't exceed MaxActivityLogLines and sets the dirty flag.
func AddRawLineToActivityLog(m *Model, entry string) {
	m.ActivityLog = append(m.ActivityLog, entry)
	if len(m.ActivityLog) > MaxActivityLogLines {
		m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-MaxActivityLogLines:]
	}
	m.ActivityLogDirty = true
}
