package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	mcPaneFocusKey  = "__MC_PANE_FOCUS_KEY__"
	wcPaneFocusKey  = "__WC_PANE_FOCUS_KEY__"
	healthUpdateInterval = 30 * time.Second
)

// Styles
var (
	appStyle = lipgloss.NewStyle().Margin(1, 2) // Margin around the entire app

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1).MarginBottom(1)
	// infoStyle is no longer used directly for header, but could be reused elsewhere
	// infoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Padding(0, 1).MarginBottom(1)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	focusedPanelStyle = panelStyle.Copy().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("62"))

	portTitleStyle = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	statusStyle    = lipgloss.NewStyle()
	logLineStyle   = lipgloss.NewStyle().Faint(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Consistent with statusMsgErrorStyle text
	// portRunningStyle and portExitedStyle are effectively replaced by statusMsg styles
	// portRunningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	// portExitedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow

	logPanelTitleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginBottom(1)

	// Panel background styles based on status - Revised for better contrast
	panelStatusDefaultStyle      = panelStyle.Copy() // Default, no specific background
	panelStatusInitializingStyle = panelStyle.Copy().Background(lipgloss.Color("238")) // Darker Gray
	panelStatusAttemptingStyle   = panelStyle.Copy().Background(lipgloss.Color("238")) // Darker Gray (for "Establishing...")
	panelStatusRunningStyle      = panelStyle.Copy().Background(lipgloss.Color("28"))  // Dark Green
	panelStatusErrorStyle        = panelStyle.Copy().Background(lipgloss.Color("124")) // Dark Red
	panelStatusExitedStyle       = panelStyle.Copy().Background(lipgloss.Color("130")) // Dark Orange

	focusedPanelStatusDefaultStyle      = focusedPanelStyle.Copy()
	focusedPanelStatusInitializingStyle = focusedPanelStyle.Copy().Background(lipgloss.Color("238"))
	focusedPanelStatusAttemptingStyle   = focusedPanelStyle.Copy().Background(lipgloss.Color("238"))
	focusedPanelStatusRunningStyle      = focusedPanelStyle.Copy().Background(lipgloss.Color("28"))
	focusedPanelStatusErrorStyle        = focusedPanelStyle.Copy().Background(lipgloss.Color("124"))
	focusedPanelStatusExitedStyle       = focusedPanelStyle.Copy().Background(lipgloss.Color("130"))

	// Text styles for status messages, ensuring contrast with new backgrounds
	statusMsgInitializingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Light Gray/White
	statusMsgRunningStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("154")) // Light Green
	statusMsgErrorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Light Red
	statusMsgExitedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Light Yellow

	contextPaneStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		BorderForeground(lipgloss.Color("240")) // Light gray border for context panes

	activeContextPaneStyle = contextPaneStyle.Copy().Background(lipgloss.Color("235")) // Very subtle dark gray for active context background

	focusedContextPaneStyle = contextPaneStyle.Copy().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("62"))
	focusedAndActiveContextPaneStyle = activeContextPaneStyle.Copy().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("62"))

	healthLoadingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Dim gray for loading text
	healthGoodStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("154")) // Light Green
	healthWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Light Yellow
	healthErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Light Red
) 