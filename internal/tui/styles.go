package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Constants for TUI behavior and internal logic.
const (
	// mcPaneFocusKey and wcPaneFocusKey are special string keys used to identify
	// the Management Cluster and Workload Cluster info panes for focus management in navigation.
	mcPaneFocusKey       = "__MC_PANE_FOCUS_KEY__"
	wcPaneFocusKey       = "__WC_PANE_FOCUS_KEY__"
	// healthUpdateInterval defines how often cluster health information (node status) is refreshed.
	healthUpdateInterval = 30 * time.Second
)

// Styles for the TUI, defined using the lipgloss library.
// These styles control the appearance of various UI elements like panels, text, borders, and backgrounds.
var (
	// appStyle defines the overall margin for the application view.
	appStyle = lipgloss.NewStyle().Margin(1, 2)

	// headerStyle is for the main instruction header at the top of the TUI.
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1).MarginBottom(1)

	// panelStyle is the base style for all rectangular panels (e.g., port forwards, context info).
	// It defines default border and padding.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)

	// focusedPanelStyle is applied to a panel when it has focus, typically making the border thicker or changing its color.
	focusedPanelStyle = panelStyle.Copy().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("62"))

	// portTitleStyle is for the title text within each port-forward panel.
	portTitleStyle = lipgloss.NewStyle().Bold(true).MarginBottom(1)

	// statusStyle is a general-purpose style, currently not heavily specialized.
	statusStyle    = lipgloss.NewStyle()

	// logLineStyle is for individual lines in the activity log, typically making them less prominent (faint).
	logLineStyle   = lipgloss.NewStyle().Faint(true)

	// errorStyle is a general style for error messages, primarily setting a red foreground color.
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// logPanelTitleStyle is for the title of the combined activity log panel.
	logPanelTitleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginBottom(1)

	// --- Panel Background Styles based on Status ---
	// These styles define the background color of port-forward panels based on their operational status.
	// They derive from the base panelStyle.
	panelStatusDefaultStyle      = panelStyle.Copy()                                   // Default, no specific background
	panelStatusInitializingStyle = panelStyle.Copy().Background(lipgloss.Color("238")) // Dark Gray for initializing/attempting states
	panelStatusAttemptingStyle   = panelStyle.Copy().Background(lipgloss.Color("238")) // Dark Gray, same as initializing
	panelStatusRunningStyle      = panelStyle.Copy().Background(lipgloss.Color("28"))  // Dark Green for successfully running port forwards
	panelStatusErrorStyle        = panelStyle.Copy().Background(lipgloss.Color("124")) // Dark Red for port forwards in an error state
	panelStatusExitedStyle       = panelStyle.Copy().Background(lipgloss.Color("130")) // Dark Orange for port forwards that have exited

	// --- Focused Panel Background Styles based on Status ---
	// Similar to the above, but these apply when a panel is focused, inheriting from focusedPanelStyle.
	focusedPanelStatusDefaultStyle      = focusedPanelStyle.Copy()
	focusedPanelStatusInitializingStyle = focusedPanelStyle.Copy().Background(lipgloss.Color("238"))
	focusedPanelStatusAttemptingStyle   = focusedPanelStyle.Copy().Background(lipgloss.Color("238"))
	focusedPanelStatusRunningStyle      = focusedPanelStyle.Copy().Background(lipgloss.Color("28"))
	focusedPanelStatusErrorStyle        = focusedPanelStyle.Copy().Background(lipgloss.Color("124"))
	focusedPanelStatusExitedStyle       = focusedPanelStyle.Copy().Background(lipgloss.Color("130"))

	// --- Text Styles for Status Messages ---
	// These define the foreground color for status messages within port-forward panels,
	// ensuring contrast against the panel backgrounds defined above.
	statusMsgInitializingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Light Gray/White for initializing/attempting messages
	statusMsgRunningStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("154")) // Light Green for running status messages
	statusMsgErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Light Red for error messages
	statusMsgExitedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Light Yellow for exited status messages

	// --- Context Pane Styles (for MC and WC info panes) ---
	// These styles are for the panes displaying Management and Workload Cluster information.
	contextPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1, 2).
				BorderForeground(lipgloss.Color("240")) // Default border color for context panes

	// activeContextPaneStyle is applied to a context pane when its corresponding Kubernetes context is active.
	activeContextPaneStyle = contextPaneStyle.Copy().Background(lipgloss.Color("235")) // Subtle dark gray background for active context

	// focusedContextPaneStyle is applied when a context pane is focused for navigation.
	focusedContextPaneStyle          = contextPaneStyle.Copy().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("62"))
	// focusedAndActiveContextPaneStyle is applied when a context pane is both focused and its context is active.
	focusedAndActiveContextPaneStyle = activeContextPaneStyle.Copy().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("62"))

	// --- Health Status Text Styles ---
	// These define foreground colors for text indicating cluster health status (e.g., node readiness).
	healthLoadingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Dim gray for "Loading..." text
	healthGoodStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("154")) // Light Green for good health status
	healthWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Light Yellow for warning health status
	healthErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Light Red for error health status
)
