package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
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

// Styles for the TUI, defined using the lipgloss library.
// These styles control the appearance of various UI elements like panels, text, borders, and backgrounds.
var (
	// appStyle defines the overall margin for the application view.
	// Use zero margin to ensure content spans the entire terminal width
	appStyle = lipgloss.NewStyle().Margin(0, 0)

	// headerStyle is for the main instruction header at the top of the TUI.
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Background(lipgloss.AdaptiveColor{Light: "#D0D0D0", Dark: "#303030"}).
			Padding(0, 2).
			MarginBottom(0)

	// panelStyle is the base style for all rectangular panels (e.g., port forwards, context info).
	// It defines default border and padding.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1) // Minimal vertical padding for more compact panels

	// focusedPanelStyle is the base for focused panels, adding a thick adaptive border.
	focusedPanelStyle = panelStyle.Copy().
				Border(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#0000CC", Dark: "#58A6FF"}). // Brighter blue in both modes
				BorderBackground(lipgloss.AdaptiveColor{Light: "#E8E8FF", Dark: "#1E293B"})  // Subtle background for border

	// Port Forward Panel Titles: Darker for light mode, White for dark mode
	portTitleStyle = lipgloss.NewStyle().Bold(true).MarginBottom(1).Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// statusStyle is a general-purpose style, currently not heavily specialized.
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// logLineStyle is for individual lines in the activity log - increased contrast for both modes
	logLineStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// New log level specific styles for the activity log. These will be applied on a per-line basis in
	// prepareLogContent() (see render_log.go).
	logInfoStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#E0E0E0"})
	logWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#A07000", Dark: "#FFD066"}).Bold(true)
	logErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B30000", Dark: "#FF6B6B"}).Bold(true)
	logDebugStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#909090"}).Italic(true)

	// Health related styles for the activity log
	logHealthGoodStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#006600", Dark: "#8AE234"}).Bold(true)
	logHealthWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#A07000", Dark: "#FFD066"}).Bold(true)
	logHealthErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B30000", Dark: "#FF6B6B"}).Bold(true)

	// errorStyle is a general style for error messages with high contrast in both modes
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B30000", Dark: "#FF6B6B"})

	// Log Panel Title: Black on light mode, White on dark mode for maximum contrast
	logPanelTitleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginBottom(1).Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})

	// --- Help Overlay Styles ---
	// Help overlay style for the main container
	helpOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
				Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#222222"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
				Padding(1, 2).
				Margin(2, 4)

	// Help overlay title style (re-added for bubbles/help container)
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E1E"}).
			MarginBottom(1).
			Align(lipgloss.Center)

	// Help section title style
	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
				MarginTop(1).
				MarginBottom(1)

	// Help key style for keyboard shortcut keys
	helpKeyStyle = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E1E"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Padding(0, 1).
			Margin(0, 1, 0, 0)

	// --- Log Overlay Styles (similar to Help Overlay) ---
	logOverlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2A2A3A"}). // Similar to combinedLogPanel background
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			Padding(1, 2)

	// --- MCP Config Overlay Style ---
	mcpConfigOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
				Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E1E"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
				Padding(1, 2)

	// --- Panel Background Styles based on Status ---
	// These define the background color of port-forward panels based on their operational status.
	// They derive from the base panelStyle. Text within these panels uses specific statusMsg...Styles with AdaptiveColor.
	panelStatusDefaultStyle      = panelStyle.Copy().BorderForeground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#909090"}).Background(lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#202020"})
	panelStatusInitializingStyle = panelStyle.Copy().Background(lipgloss.AdaptiveColor{Light: "#E0E8FF", Dark: "#2A384D"}).BorderForeground(lipgloss.AdaptiveColor{Light: "#5060A0", Dark: "#6A78AD"})
	panelStatusAttemptingStyle   = panelStatusInitializingStyle.Copy() // Typically same as initializing
	panelStatusRunningStyle      = panelStyle.Copy().Background(lipgloss.AdaptiveColor{Light: "#D4EFDF", Dark: "#1A3A1A"}).BorderForeground(lipgloss.AdaptiveColor{Light: "#307030", Dark: "#60A060"})
	panelStatusErrorStyle        = panelStyle.Copy().Background(lipgloss.AdaptiveColor{Light: "#FADBD8", Dark: "#4D2A2A"}).BorderForeground(lipgloss.AdaptiveColor{Light: "#A04040", Dark: "#B07070"})
	panelStatusExitedStyle       = panelStyle.Copy().Background(lipgloss.AdaptiveColor{Light: "#FCF3CF", Dark: "#4D4D2A"}).BorderForeground(lipgloss.AdaptiveColor{Light: "#A07030", Dark: "#B0A070"})

	// --- Focused Panel Background Styles based on Status ---
	// Similar to the above, but these apply when a panel is focused.
	focusedPanelStatusDefaultStyle = panelStatusDefaultStyle.Copy().
					Inherit(focusedPanelStyle).
					Background(lipgloss.AdaptiveColor{Light: "#F0F8FF", Dark: "#252525"})

	focusedPanelStatusInitializingStyle = panelStatusInitializingStyle.Copy().
						Inherit(focusedPanelStyle).
						Background(lipgloss.AdaptiveColor{Light: "#E0E8FF", Dark: "#2F3D54"})

	focusedPanelStatusAttemptingStyle = focusedPanelStatusInitializingStyle.Copy()

	focusedPanelStatusRunningStyle = panelStatusRunningStyle.Copy().
					Inherit(focusedPanelStyle).
					Background(lipgloss.AdaptiveColor{Light: "#D4FFDF", Dark: "#1F4420"})

	focusedPanelStatusErrorStyle = panelStatusErrorStyle.Copy().
					Inherit(focusedPanelStyle).
					Background(lipgloss.AdaptiveColor{Light: "#FFDBDB", Dark: "#582F2F"})

	focusedPanelStatusExitedStyle = panelStatusExitedStyle.Copy().
					Inherit(focusedPanelStyle).
					Background(lipgloss.AdaptiveColor{Light: "#FFF3CF", Dark: "#574F2F"})

	// --- Text Styles for Status Messages within Port Forward Panels ---
	// These define the foreground color for status messages with higher contrast in both modes
	statusMsgInitializingStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000080", Dark: "#82B0FF"}) // Adjusted for readability
	statusMsgRunningStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#004400", Dark: "#8AE234"}) // Adjusted for readability
	statusMsgErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#880000", Dark: "#FF8787"}) // Adjusted for readability
	statusMsgExitedStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#553300", Dark: "#FFB86C"}) // Adjusted for readability

	// --- Context Pane Styles (for MC and WC info panes) ---
	contextPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(0, 1).                                                               // Reduced from 1,2 to 0,1 for more compact panes
				BorderForeground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#A0A0A0"}). // Lighter border in dark mode
				Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2A2A3A"}).       // Added background - light in light mode, medium dark in dark mode
				Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})        // Maximum contrast

	activeContextPaneStyle = contextPaneStyle.Copy().
				BorderForeground(lipgloss.AdaptiveColor{Light: "#0000CC", Dark: "#A0A0FF"}). // Brighter border in dark mode
				Background(lipgloss.AdaptiveColor{Light: "#E8F4FF", Dark: "#2A3450"})        // Slightly different background to indicate active

	focusedContextPaneStyle = contextPaneStyle.Copy().
				Inherit(focusedPanelStyle).
				Background(lipgloss.AdaptiveColor{Light: "#F0F0FF", Dark: "#2A385D"}) // Slightly blueish background when focused

	focusedAndActiveContextPaneStyle = activeContextPaneStyle.Copy().
						Inherit(focusedPanelStyle).
						Background(lipgloss.AdaptiveColor{Light: "#E0E8FF", Dark: "#30406B"}) // More saturated blue background when focused and active

	// --- Health Status Text Styles (used within Context Panes) ---
	healthLoadingStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#303030", Dark: "#F0F0F0"}).Bold(true) // Bolder and brighter in dark mode
	healthGoodStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005500", Dark: "#90FF90"}).Bold(true) // Brighter green in dark mode
	healthWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#703000", Dark: "#FFFF00"}).Bold(true) // Bright yellow in dark mode
	healthErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#990000", Dark: "#FF9090"}).Bold(true) // Brighter red in dark mode

	// Quit key style for the quit confirmation message
	quitKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#990000", Dark: "#FF8787"}).Bold(true)

	centeredOverlayContainerStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.AdaptiveColor{Light: "#AAAAAA", Dark: "#666666"}).
		// Background for the container itself. The actual help bubble will have its own bg.
		// This style is for the box that lipgloss.Place will draw.
		Background(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#2A2A2A"}). // Re-enable solid background
		Padding(1, 2)

	// --- Status Bar Styles ---
	// Exported color constants for status bar backgrounds
	StatusBarDefaultBg = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#374151"} // Default dark grey/blue
	StatusBarSuccessBg = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#059669"} // Green
	StatusBarErrorBg   = lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#DC2626"} // Red
	StatusBarWarningBg = lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#D97706"} // Yellow/Amber
	StatusBarInfoBg    = lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#2563EB"} // Blue

	// Exported common style for status bar text
	StatusBarTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#F0F0F0"}).
				Padding(0, 1) // Ensure padding is within the colored background

	// Exported base style for the status bar container
	StatusBarBaseStyle = lipgloss.NewStyle().Height(1) // Base only, background set dynamically

	// Exported styles for the message text itself (primarily foreground)
	StatusMessageInfoStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#E0E0E0"})
	StatusMessageSuccessStyle = lipgloss.NewStyle().
					Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#E0E0E0"})
	StatusMessageErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#F0F0F0"}) // Brighter white on red
	StatusMessageWarningStyle = lipgloss.NewStyle().
					Foreground(lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#111827"}) // Darker text on yellow

	// Shared overlay colour palette (Help & MCP Config)
	HelpOverlayBg          = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E1E"}
	HelpOverlayDescFg      = lipgloss.Color("#C0C0C0")
	HelpOverlayEllipsisFg  = lipgloss.Color("#777777")
	HelpOverlaySeparatorFg = lipgloss.Color("#777777")
	HelpOverlayKeyFg       = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
)
