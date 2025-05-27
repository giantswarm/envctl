package view

import (
	"envctl/internal/api"
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPortForwardingRowV2 renders the port forwarding row
func renderPortForwardingRowV2(m *model.ModelV2, width, maxHeight int) string {
	// Match v1 exactly - always use 3 columns
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
			} else if pf.State == "failed" {
				style = color.PanelStatusErrorStyle
			} else if pf.State == "running" {
				style = color.PanelStatusRunningStyle
			} else {
				style = color.PanelStatusInitializingStyle
			}
		} else {
			style = color.PanelStyle
		}
		totalBorder += style.GetHorizontalFrameSize()
	}

	innerWidth := width - totalBorder
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
			if pf.State == "failed" {
				bs = color.PanelStatusErrorStyle
			} else if pf.State == "running" {
				bs = color.PanelStatusRunningStyle
			} else {
				bs = color.PanelStatusInitializingStyle
			}
			borderSize = bs.GetHorizontalFrameSize()
			rendered = renderPortForwardPanelV2(m, keys[i], pf, inner+borderSize)
		} else {
			borderSize = color.PanelStyle.GetHorizontalFrameSize()
			rendered = color.PanelStyle.Copy().Width(inner).Render("")
		}
		cells[i] = rendered
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
}

// renderPortForwardPanelV2 renders a single port forward panel
func renderPortForwardPanelV2(m *model.ModelV2, label string, pf *api.PortForwardServiceInfo, targetWidth int) string {
	// Match v1 exactly
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	trimmedStatus := strings.TrimSpace(pf.State)
	statusToCheck := strings.ToLower(trimmedStatus)

	switch {
	case pf.State == "failed" || strings.HasPrefix(statusToCheck, "error"):
		baseStyleForPanel = color.PanelStatusErrorStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
	case pf.State == "running":
		baseStyleForPanel = color.PanelStatusRunningStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
	case pf.State == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		baseStyleForPanel = color.PanelStatusExitedStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
	default:
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if label == m.FocusedPanelKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	var contentFg lipgloss.Style
	switch {
	case pf.State == "failed" || strings.HasPrefix(statusToCheck, "error"):
		contentFg = color.StatusMsgErrorStyle
	case pf.State == "running":
		contentFg = color.StatusMsgRunningStyle
	case pf.State == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		contentFg = color.StatusMsgExitedStyle
	default:
		contentFg = color.StatusMsgInitializingStyle
	}

	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	var b strings.Builder
	icon := pf.Icon
	if icon == "" {
		icon = IconLink
	}
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(icon) + pf.Name))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Port: %d:%d", pf.LocalPort, pf.RemotePort))
	b.WriteString("\n")
	targetInfo := fmt.Sprintf("%s/%s", pf.TargetType, pf.TargetName)
	b.WriteString(fmt.Sprintf("Target: %s", targetInfo))
	b.WriteString("\n")

	var statusIcon string
	switch {
	case pf.State == "running":
		statusIcon = SafeIcon(IconPlay)
	case pf.State == "failed" || strings.HasPrefix(statusToCheck, "error"):
		statusIcon = SafeIcon(IconCross)
	case pf.State == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		statusIcon = SafeIcon(IconStop)
	default:
		statusIcon = SafeIcon(IconHourglass)
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(trimmedStatus))))

	// Add health indicator
	b.WriteString("\n")
	var healthIcon, healthText string
	if pf.State == "running" && (pf.Health == "healthy" || pf.Health == "Healthy") {
		healthIcon = SafeIcon(IconCheck)
		healthText = "Healthy"
	} else if pf.State == "failed" || pf.Health == "unhealthy" || pf.Health == "Unhealthy" {
		healthIcon = SafeIcon(IconCross)
		healthText = "Unhealthy"
	} else {
		healthIcon = SafeIcon(IconHourglass)
		healthText = "Checking..."
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))

	frame := finalPanelStyle.GetHorizontalFrameSize()
	contentWidth := targetWidth - frame
	if contentWidth < 0 {
		contentWidth = 0
	}
	return finalPanelStyle.Copy().Width(contentWidth).Render(b.String())
}
