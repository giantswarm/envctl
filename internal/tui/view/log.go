package view

import (
	// "envctl/internal/color" // color styles will be bypassed for this test
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// var rawStyleForDebug = lipgloss.NewStyle() // Remove this if no longer needed

// renderLogOverlay (moved from view_helpers.go)
func renderLogOverlay(m *model.Model, width, height int) string {
	title := color.LogPanelTitleStyle.Render(SafeIcon(IconScroll) + " Activity Log  (↑/↓ scroll  •  y copy  •  Esc close)")
	viewportView := m.LogViewport.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, viewportView)
	return color.LogOverlayStyle.Copy().
		Width(width - color.LogOverlayStyle.GetHorizontalFrameSize()).
		Height(height - color.LogOverlayStyle.GetVerticalFrameSize()).
		Render(content)
}

// renderCombinedLogPanel renders the activity log panel at bottom.
func renderCombinedLogPanel(m *model.Model, availableWidth int, logSectionHeight int) string {
	if logSectionHeight <= 0 {
		return ""
	}

	border := color.PanelStatusDefaultStyle.GetHorizontalFrameSize()
	innerWidth := availableWidth - border
	if innerWidth < 0 {
		innerWidth = 0
	}

	titleView := color.LogPanelTitleStyle.Render(SafeIcon(IconScroll) + "Combined Activity Log")
	viewportView := m.MainLogViewport.View()
	panelContent := lipgloss.JoinVertical(lipgloss.Left, titleView, viewportView)

	base := color.PanelStatusDefaultStyle.Copy().
		Width(innerWidth).
		MaxHeight(0).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#A0A0A0"}).
		Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2A2A3A"})
	rendered := base.Render(panelContent)

	// ensure min size
	if h := lipgloss.Height(rendered); h < logSectionHeight {
		return lipgloss.NewStyle().Width(availableWidth).Height(logSectionHeight).Render(rendered)
	}
	return rendered
}

// PrepareLogContent applies color styles based on log level keywords.
func PrepareLogContent(lines []string, maxWidth int) string {
	// maxWidth is conceptually available if styling needed to be width-aware,
	// but styling itself shouldn't truncate. Viewport handles overflow.
	out := make([]string, len(lines))
	for i, rawLine := range lines {
		out[i] = styleLogLine(rawLine)
	}
	return strings.Join(out, "\n")
}

// styleLogLine returns the line wrapped in appropriate lipgloss style depending
// on markers contained in the text.
func styleLogLine(l string) string {

	switch {
	case strings.Contains(l, "[ERROR]"):
		return color.LogErrorStyle.Render(l)
	case strings.Contains(l, "[WARN]"):
		return color.LogWarnStyle.Render(l)
	case strings.Contains(l, "[DEBUG]"):
		return color.LogDebugStyle.Render(l)
	case strings.Contains(l, "[HEALTH"):
		switch {
		case strings.Contains(l, "Error"):
			return color.LogHealthErrStyle.Render(l)
		case strings.Contains(l, "Nodes"):
			return color.LogHealthGoodStyle.Render(l)
		default:
			return color.LogHealthWarnStyle.Render(l)
		}
	case strings.Contains(l, "[INFO]"):
		return color.LogInfoStyle.Render(l)
	default:
		return color.LogInfoStyle.Render(l)
	}
}

// applyStyling is a helper to map all lines through styleLogLine.
// func applyStyling(lines []string) []string { // This function is not directly called anymore by PrepareLogContent if styleLogLine is inlined
// 	styled := make([]string, len(lines))
// 	for i, l := range lines {
// 		styled[i] = styleLogLine(l)
// 	}
// 	return styled
// }

// generateMcpConfigJson has been moved to internal/tui/controller/mcpserver.go

func renderMcpConfigOverlay(m *model.Model, width, height int) string {
	title := color.LogPanelTitleStyle.Render(SafeIcon(IconGear) + " MCP Configuration  (↑/↓ scroll  •  y copy  •  Esc close)")
	viewportView := m.McpConfigViewport.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, viewportView)
	return color.McpConfigOverlayStyle.Copy().
		Width(width - color.McpConfigOverlayStyle.GetHorizontalFrameSize()).
		Height(height - color.McpConfigOverlayStyle.GetVerticalFrameSize()).
		Render(content)
}
