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

	// Help overlay title style
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
			MarginBottom(1).
			Underline(true)

	// Help section title style
	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}).
				MarginTop(1).
				MarginBottom(1)

	// Help key style for keyboard shortcut keys
	helpKeyStyle = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#D0D0D0", Dark: "#505050"}).
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
	statusMsgInitializingStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000080", Dark: "#B0D8FF"}) // Darker blue / Lighter blue
	statusMsgRunningStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#004400", Dark: "#C0FFC0"}) // Darker green / Lighter green
	statusMsgErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#880000", Dark: "#FFABAB"}) // Darker red / Lighter red
	statusMsgExitedStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#553300", Dark: "#FFE0B0"}) // Darker orange / Lighter orange

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
)
