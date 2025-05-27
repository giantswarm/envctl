package view

import "time"

const (
	// healthUpdateInterval defines how often cluster health information (node status) is refreshed.
	healthUpdateInterval = 30 * time.Second
	// minHeightForMainLogView defines the minimum terminal height (in lines)
	// required to display the activity log in the main view.
	// If the terminal is shorter, the log is hidden from the main view and accessible via overlay.
	minHeightForMainLogView = 28
)
