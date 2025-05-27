package view

import (
	"envctl/internal/color"
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
		baseStyleForPanel = color.PanelStatusErrorStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
	case pf.Running && pf.Err == nil:
		baseStyleForPanel = color.PanelStatusRunningStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
	case strings.HasPrefix(statusToCheck, "running"):
		baseStyleForPanel = color.PanelStatusRunningStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
	case strings.HasPrefix(statusToCheck, "stopped") || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		baseStyleForPanel = color.PanelStatusExitedStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
	default:
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if pf.Label == m.FocusedPanelKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	var contentFg lipgloss.Style
	switch {
	case pf.Err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed"):
		contentFg = color.StatusMsgErrorStyle
	case pf.Running && pf.Err == nil:
		contentFg = color.StatusMsgRunningStyle
	case strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		contentFg = color.StatusMsgExitedStyle
	default:
		contentFg = color.StatusMsgInitializingStyle
	}

	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	var b strings.Builder
	icon := pf.Config.Icon
	if icon == "" {
		icon = IconLink
	}
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(icon) + pf.Label))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Port: %s:%s", pf.Config.LocalPort, pf.Config.RemotePort))
	b.WriteString("\n")
	targetInfo := fmt.Sprintf("%s/%s", pf.Config.TargetType, pf.Config.TargetName)
	b.WriteString(fmt.Sprintf("Target: %s", targetInfo))
	b.WriteString("\n")

	var statusIcon string
	switch {
	case pf.Running && pf.Err == nil:
		statusIcon = SafeIcon(IconPlay)
	case pf.Err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error"):
		statusIcon = SafeIcon(IconCross)
	case strings.HasPrefix(statusToCheck, "stopped") || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		statusIcon = SafeIcon(IconStop)
	default:
		statusIcon = SafeIcon(IconHourglass)
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(trimmedStatus))))

	// Add health indicator
	if pf.Active {
		b.WriteString("\n")
		var healthIcon, healthText string
		if pf.Running && pf.Err == nil {
			healthIcon = SafeIcon(IconCheck)
			healthText = "Healthy"
		} else if pf.Err != nil {
			healthIcon = SafeIcon(IconCross)
			healthText = "Unhealthy"
		} else {
			healthIcon = SafeIcon(IconHourglass)
			healthText = "Checking..."
		}
		b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))
	}

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
				style = color.PanelStyle
			} else if pf.Err != nil || strings.HasPrefix(strings.ToLower(pf.StatusMsg), "failed") {
				style = color.PanelStatusErrorStyle
			} else if pf.Running && pf.Err == nil {
				style = color.PanelStatusRunningStyle
			} else {
				style = color.PanelStatusInitializingStyle
			}
		} else {
			style = color.PanelStyle
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
				bs = color.PanelStatusErrorStyle
			} else if pf.Running && pf.Err == nil {
				bs = color.PanelStatusRunningStyle
			} else {
				bs = color.PanelStatusInitializingStyle
			}
			borderSize = bs.GetHorizontalFrameSize()
			rendered = renderPortForwardPanel(pf, m, inner+borderSize)
		} else {
			borderSize = color.PanelStyle.GetHorizontalFrameSize()
			rendered = color.PanelStyle.Copy().Width(inner).Render("")
		}
		cells[i] = rendered
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).MaxHeight(maxRowHeight).Render(row)
}
