package view

import (
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPortForwardPanel renders a single port-forward panel.
func renderPortForwardPanel(pf *model.PortForwardProcess, m *model.Model, targetOuterWidth int) string {
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	trimmedStatus := strings.TrimSpace(pf.StatusMsg)
	statusToCheck := strings.ToLower(trimmedStatus)

	switch {
	case pf.Err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed"):
		baseStyleForPanel = panelStatusErrorStyle
		focusedBaseStyleForPanel = focusedPanelStatusErrorStyle
	case pf.Running && pf.Err == nil:
		baseStyleForPanel = panelStatusRunningStyle
		focusedBaseStyleForPanel = focusedPanelStatusRunningStyle
	case strings.HasPrefix(statusToCheck, "running (pid:"):
		baseStyleForPanel = panelStatusAttemptingStyle
		focusedBaseStyleForPanel = focusedPanelStatusAttemptingStyle
	case strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		baseStyleForPanel = panelStatusExitedStyle
		focusedBaseStyleForPanel = focusedPanelStatusExitedStyle
	default:
		baseStyleForPanel = panelStatusInitializingStyle
		focusedBaseStyleForPanel = focusedPanelStatusInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if pf.Label == m.FocusedPanelKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	var contentFg lipgloss.Style
	switch {
	case pf.Err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed"):
		contentFg = statusMsgErrorStyle
	case pf.Running && pf.Err == nil:
		contentFg = statusMsgRunningStyle
	case strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		contentFg = statusMsgExitedStyle
	default:
		contentFg = statusMsgInitializingStyle
	}

	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	var b strings.Builder
	b.WriteString(portTitleStyle.Render(SafeIcon(IconLink) + pf.Label))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Port: %s:%s", pf.Config.LocalPort, pf.Config.RemotePort))
	b.WriteString("\n")
	svc := strings.TrimPrefix(pf.Config.ServiceName, "service/")
	b.WriteString(fmt.Sprintf("Svc: %s", svc))
	b.WriteString("\n")

	var statusIcon string
	switch {
	case pf.Running && pf.Err == nil:
		statusIcon = SafeIcon(IconPlay)
	case pf.Err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error"):
		statusIcon = SafeIcon(IconCross)
	case strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		statusIcon = SafeIcon(IconStop)
	default:
		statusIcon = SafeIcon(IconHourglass)
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(trimmedStatus))))

	frame := finalPanelStyle.GetHorizontalFrameSize()
	contentWidth := targetOuterWidth - frame
	if contentWidth < 0 {
		contentWidth = 0
	}
	return finalPanelStyle.Copy().Width(contentWidth).Render(b.String())
}

// renderPortForwardingRow lays out up to 3 port-forward panels.
func renderPortForwardingRow(m *model.Model, contentWidth int, maxRowHeight int) string {
	const numCols = 3
	var keys []string
	for _, k := range m.PortForwardOrder {
		if k != model.McPaneFocusKey && k != model.WcPaneFocusKey {
			keys = append(keys, k)
		}
	}

	totalBorder := 0
	for i := 0; i < numCols; i++ {
		var style lipgloss.Style
		if i < len(keys) {
			pf := m.PortForwards[keys[i]]
			if pf == nil {
				style = panelStyle
			} else if pf.Err != nil || strings.HasPrefix(strings.ToLower(pf.StatusMsg), "failed") {
				style = panelStatusErrorStyle
			} else if pf.Running && pf.Err == nil {
				style = panelStatusRunningStyle
			} else {
				style = panelStatusInitializingStyle
			}
		} else {
			style = panelStyle
		}
		totalBorder += style.GetHorizontalFrameSize()
	}

	innerWidth := contentWidth - totalBorder
	if innerWidth < 0 {
		innerWidth = 0
	}
	baseInner := innerWidth / numCols
	remainder := innerWidth % numCols

	cells := make([]string, numCols)
	for i := 0; i < numCols; i++ {
		inner := baseInner
		if i < remainder {
			inner++
		}
		var borderSize int
		var rendered string
		if i < len(keys) {
			pf := m.PortForwards[keys[i]]
			var bs lipgloss.Style
			if pf.Err != nil || strings.HasPrefix(strings.ToLower(pf.StatusMsg), "failed") {
				bs = panelStatusErrorStyle
			} else if pf.Running && pf.Err == nil {
				bs = panelStatusRunningStyle
			} else {
				bs = panelStatusInitializingStyle
			}
			borderSize = bs.GetHorizontalFrameSize()
			rendered = renderPortForwardPanel(pf, m, inner+borderSize)
		} else {
			borderSize = panelStyle.GetHorizontalFrameSize()
			rendered = panelStyle.Copy().Width(inner).Render("")
		}
		cells[i] = rendered
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).MaxHeight(maxRowHeight).Render(row)
}
