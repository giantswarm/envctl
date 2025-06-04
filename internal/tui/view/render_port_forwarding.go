package view

import (
	"envctl/internal/api"
	"envctl/internal/color"
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPortForwardingRow(renders the port forwarding row
func renderPortForwardingRow(m *model.Model, width, maxHeight int) string {
	// Match v1 exactly - always use 3 columns
	const numCols = 3

	// Get port forward definitions from config (like MCP servers do)
	numPortForwards := len(m.PortForwardingConfig)

	if numPortForwards == 0 {
		// Return empty panels when no port forwards configured
		var cells []string
		for i := 0; i < numCols; i++ {
			cells = append(cells, color.PanelStyle.Copy().Width(width/numCols).Render(""))
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
	}

	// Calculate how many items to display (max 3)
	displayItems := numPortForwards
	if displayItems > numCols {
		displayItems = numCols
	}

	// Calculate width for each panel
	baseWidth := width / numCols
	remainder := width % numCols

	var cells []string

	// Add port forward panels
	for i := 0; i < numPortForwards && i < numCols; i++ {
		pfConfig := m.PortForwardingConfig[i]
		// Look up the actual service state
		pfService := m.PortForwards[pfConfig.Name]

		w := baseWidth
		if i < remainder {
			w++
		}

		// Determine style based on state
		var panelStyle lipgloss.Style
		if pfService != nil {
			stateLower := strings.ToLower(pfService.State)
			if stateLower == "failed" {
				panelStyle = color.PanelStatusErrorStyle
			} else if stateLower == "running" {
				panelStyle = color.PanelStatusRunningStyle
			} else {
				panelStyle = color.PanelStatusInitializingStyle
			}
		} else {
			panelStyle = color.PanelStatusExitedStyle
		}

		// Adjust width for panel border
		adjustedWidth := w
		if panelStyle.GetHorizontalFrameSize() > 0 {
			adjustedWidth = w
		}

		rendered := renderPortForwardPanelFromConfig(m, pfConfig, pfService, adjustedWidth)
		cells = append(cells, rendered)
	}

	// Fill remaining columns with empty panels
	for len(cells) < numCols {
		w := baseWidth
		if len(cells) < remainder {
			w++
		}
		cells = append(cells, color.PanelStyle.Copy().Width(w).Render(""))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
}

// renderPortForwardPanel(renders a single port forward panel
func renderPortForwardPanel(m *model.Model, label string, pf *api.PortForwardServiceInfo, targetWidth int) string {
	// Match v1 exactly
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	trimmedStatus := strings.TrimSpace(pf.State)
	statusToCheck := strings.ToLower(trimmedStatus)
	stateLower := strings.ToLower(pf.State)

	switch {
	case stateLower == "failed" || strings.HasPrefix(statusToCheck, "error"):
		baseStyleForPanel = color.PanelStatusErrorStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
	case stateLower == "running":
		baseStyleForPanel = color.PanelStatusRunningStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
	case stateLower == "waiting":
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
	case stateLower == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
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
	case stateLower == "failed" || strings.HasPrefix(statusToCheck, "error"):
		contentFg = color.StatusMsgErrorStyle
	case stateLower == "running":
		contentFg = color.StatusMsgRunningStyle
	case stateLower == "waiting":
		contentFg = color.StatusMsgInitializingStyle
	case stateLower == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
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
	case stateLower == "running":
		statusIcon = SafeIcon(IconPlay)
	case stateLower == "failed" || strings.HasPrefix(statusToCheck, "error"):
		statusIcon = SafeIcon(IconCross)
	case stateLower == "waiting":
		statusIcon = SafeIcon(IconHourglass)
	case stateLower == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
		statusIcon = SafeIcon(IconStop)
	default:
		statusIcon = SafeIcon(IconHourglass)
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(trimmedStatus))))

	// Add health indicator
	b.WriteString("\n")
	var healthIcon, healthText string
	healthLower := strings.ToLower(pf.Health)
	if stateLower == "running" && healthLower == "healthy" {
		healthIcon = SafeIcon(IconCheck)
		healthText = "Healthy"
	} else if stateLower == "failed" || healthLower == "unhealthy" {
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

// renderPortForwardPanelFromConfig renders a port forward panel from config with optional service data
func renderPortForwardPanelFromConfig(m *model.Model, pfConfig config.PortForwardDefinition, pfService *api.PortForwardServiceInfo, targetWidth int) string {
	// Determine style based on service state or default if no service
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	var contentFg lipgloss.Style

	if pfService != nil {
		// Use service state if available
		trimmedStatus := strings.TrimSpace(pfService.State)
		statusToCheck := strings.ToLower(trimmedStatus)
		stateLower := strings.ToLower(pfService.State)

		switch {
		case stateLower == "failed" || strings.HasPrefix(statusToCheck, "error"):
			baseStyleForPanel = color.PanelStatusErrorStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
			contentFg = color.StatusMsgErrorStyle
		case stateLower == "running":
			baseStyleForPanel = color.PanelStatusRunningStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
			contentFg = color.StatusMsgRunningStyle
		case stateLower == "waiting":
			baseStyleForPanel = color.PanelStatusInitializingStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		case stateLower == "stopped" || strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed"):
			baseStyleForPanel = color.PanelStatusExitedStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
			contentFg = color.StatusMsgExitedStyle
		default:
			baseStyleForPanel = color.PanelStatusInitializingStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		}
	} else {
		// No service data yet - show as not started
		baseStyleForPanel = color.PanelStatusExitedStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
		contentFg = color.StatusMsgExitedStyle
	}

	finalPanelStyle := baseStyleForPanel
	if pfConfig.Name == m.FocusedPanelKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	var b strings.Builder

	// Use icon from config or service data
	icon := pfConfig.Icon
	if icon == "" && pfService != nil && pfService.Icon != "" {
		icon = pfService.Icon
	}
	if icon == "" {
		icon = IconLink
	}

	// Display name from config
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(icon) + pfConfig.Name))
	b.WriteString("\n")

	// Port info - use service data if available, otherwise config
	if pfService != nil && pfService.LocalPort > 0 && pfService.RemotePort > 0 {
		b.WriteString(fmt.Sprintf("Port: %d:%d", pfService.LocalPort, pfService.RemotePort))
	} else {
		// Use port strings from config
		b.WriteString(fmt.Sprintf("Port: %s:%s", pfConfig.LocalPort, pfConfig.RemotePort))
	}
	b.WriteString("\n")

	// Target info - use service data if available, otherwise config
	if pfService != nil && pfService.TargetType != "" && pfService.TargetName != "" {
		targetInfo := fmt.Sprintf("%s/%s", pfService.TargetType, pfService.TargetName)
		b.WriteString(fmt.Sprintf("Target: %s", targetInfo))
	} else {
		targetInfo := fmt.Sprintf("%s/%s", pfConfig.TargetType, pfConfig.TargetName)
		b.WriteString(fmt.Sprintf("Target: %s", targetInfo))
	}
	b.WriteString("\n")

	// Status
	var statusIcon string
	var statusText string
	if pfService != nil {
		stateLower := strings.ToLower(pfService.State)
		statusText = strings.TrimSpace(pfService.State)
		switch {
		case stateLower == "running":
			statusIcon = SafeIcon(IconPlay)
		case stateLower == "failed" || strings.HasPrefix(stateLower, "error"):
			statusIcon = SafeIcon(IconCross)
		case stateLower == "waiting":
			statusIcon = SafeIcon(IconHourglass)
		case stateLower == "stopped" || strings.HasPrefix(stateLower, "exited") || strings.HasPrefix(stateLower, "killed"):
			statusIcon = SafeIcon(IconStop)
		default:
			statusIcon = SafeIcon(IconHourglass)
		}
	} else {
		statusIcon = SafeIcon(IconWarning)
		statusText = "Not Started"
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(statusText))))

	// Add health indicator
	b.WriteString("\n")
	var healthIcon, healthText string
	if pfService != nil {
		stateLower := strings.ToLower(pfService.State)
		healthLower := strings.ToLower(pfService.Health)
		if stateLower == "running" && healthLower == "healthy" {
			healthIcon = SafeIcon(IconCheck)
			healthText = "Healthy"
		} else if stateLower == "failed" || healthLower == "unhealthy" {
			healthIcon = SafeIcon(IconCross)
			healthText = "Unhealthy"
		} else {
			healthIcon = SafeIcon(IconHourglass)
			healthText = "Checking..."
		}
	} else {
		healthIcon = SafeIcon(IconWarning)
		healthText = "Not Started"
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))

	frame := finalPanelStyle.GetHorizontalFrameSize()
	contentWidth := targetWidth - frame
	if contentWidth < 0 {
		contentWidth = 0
	}
	return finalPanelStyle.Copy().Width(contentWidth).Render(b.String())
}
