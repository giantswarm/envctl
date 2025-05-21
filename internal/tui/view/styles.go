package view

import (
	"time"
)

// Constants for TUI behavior and internal logic.
const (
	// mcPaneFocusKey and wcPaneFocusKey are special string keys used to identify
	// the Management Cluster and Workload Cluster info panes for focus management in navigation.
	mcPaneFocusKey = "__MC_PANE_FOCUS_KEY__"
	wcPaneFocusKey = "__WC_PANE_FOCUS_KEY__"
	// healthUpdateInterval defines how often cluster health information (node status) is refreshed.
	healthUpdateInterval = 30 * time.Second
	// minHeightForMainLogView defines the minimum terminal height (in lines)
	// required to display the activity log in the main view.
	// If the terminal is shorter, the log is hidden from the main view and accessible via overlay.
	minHeightForMainLogView = 28
)

// Nerd Font Icons
// Ensure your terminal is configured with a Nerd Font to see these correctly.
const (
	IconCheck      = "✔" // U+2714
	IconCross      = "❌" // U+274C
	IconWarning    = "⚠" // U+26A0 without VS16
	IconHourglass  = "⏳" // U+23F3 (keep)
	IconSpinner    = "🔄" // maybe leave
	IconFire       = "🔥" // U+1F525 (for very critical errors)
	IconSparkles   = "✨" // U+2728 (for success messages)
	IconThumbsUp   = "👍" // U+1F44D
	IconThumbsDown = "👎" // U+1F44E
	IconLightbulb  = "💡" // U+1F4A1
	IconKubernetes = "☸" // U+2638
	IconDesktop    = "💻" // U+1F4BB
	IconLink       = "🔗" // U+1F517
	IconPlay       = "▶" // U+25B6 without VS16
	IconStop       = "⏹" // U+23F9 without VS16
	IconServer     = "🖥" // U+1F5A5 without VS16
	IconGear       = "⚙" // U+2699 without VS16
	IconScroll     = "📜" // U+1F4DC
	IconInfo       = "ℹ" // U+2139 without VS16
)
