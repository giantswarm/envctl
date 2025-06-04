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

// renderMcpProxiesRow renders the MCP proxies row
func renderMcpProxiesRow(m *model.Model, width, maxHeight int) string {
	// Match v1 exactly - no title above, just the panels
	const cols = 3

	// Get MCP server definitions from config
	numServers := len(m.MCPServerConfig)

	if numServers == 0 {
		// Return empty panels when no servers configured
		var cells []string
		for i := 0; i < cols; i++ {
			cells = append(cells, color.PanelStyle.Copy().Width(width/cols).Render(""))
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
	}

	// Calculate how many items to display (max 3)
	displayItems := numServers
	if displayItems > cols {
		displayItems = cols
	}

	// Calculate width for each panel
	baseWidth := width / cols
	remainder := width % cols

	var cells []string

	// Add MCP server panels
	for i := 0; i < numServers && i < cols; i++ {
		def := m.MCPServerConfig[i]
		proc := m.MCPServers[def.Name]

		w := baseWidth
		if i < remainder {
			w++
		}

		// Determine style based on state
		var panelStyle lipgloss.Style
		st := ""
		if proc != nil {
			st = strings.ToLower(proc.State)
		}
		switch {
		case proc != nil && (st == "failed" || strings.Contains(st, "error")):
			panelStyle = color.PanelStatusErrorStyle
		case proc != nil && st == "running":
			panelStyle = color.PanelStatusRunningStyle
		default:
			panelStyle = color.PanelStatusInitializingStyle
		}

		// Adjust width for panel border
		adjustedWidth := w
		if panelStyle.GetHorizontalFrameSize() > 0 {
			// Panel has border, ensure we account for it
			adjustedWidth = w
		}

		rendered := renderMcpProxyPanel(def.Name, def, proc, m, adjustedWidth)
		cells = append(cells, rendered)
	}

	// Fill remaining columns with empty panels
	for len(cells) < cols {
		w := baseWidth
		if len(cells) < remainder {
			w++
		}
		cells = append(cells, color.PanelStyle.Copy().Width(w).Render(""))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
}

// renderMcpProxyPanel renders one MCP proxy status panel to match v1 exactly
func renderMcpProxyPanel(serverName string, predefinedData config.MCPServerDefinition, proc *api.MCPServerInfo, m *model.Model, targetOuterWidth int) string {
	var baseStyle lipgloss.Style
	var contentFg lipgloss.Style
	statusMsg := "Not Started"

	if proc != nil {
		statusMsg = proc.State
		st := strings.ToLower(proc.State)
		switch {
		case st == "failed" || strings.Contains(st, "error"):
			baseStyle = color.PanelStatusErrorStyle
			contentFg = color.StatusMsgErrorStyle
		case st == "running":
			baseStyle = color.PanelStatusRunningStyle
			contentFg = color.StatusMsgRunningStyle
		case st == "waiting":
			baseStyle = color.PanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		case st == "stopped":
			baseStyle = color.PanelStatusExitedStyle
			contentFg = color.StatusMsgExitedStyle
		default:
			baseStyle = color.PanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		}
	} else {
		baseStyle = color.PanelStatusExitedStyle
		contentFg = color.StatusMsgExitedStyle
	}

	final := baseStyle.Copy().Foreground(contentFg.GetForeground())
	if m.FocusedPanelKey == serverName {
		final = final.Copy().Border(lipgloss.DoubleBorder()).Bold(true)
	}

	var b strings.Builder
	icon := predefinedData.Icon
	if icon == "" {
		icon = IconGear
	}
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(icon) + strings.TrimSpace(predefinedData.Name) + " MCP"))
	b.WriteString("\n")

	var iconStr string
	if proc == nil {
		iconStr = SafeIcon(IconWarning)
	} else {
		st := strings.ToLower(proc.State)
		switch {
		case st == "failed" || strings.Contains(st, "error"):
			iconStr = SafeIcon(IconCross)
		case st == "running":
			iconStr = SafeIcon(IconPlay)
		case st == "waiting":
			iconStr = SafeIcon(IconHourglass)
		case st == "stopped":
			iconStr = SafeIcon(IconStop)
		default:
			iconStr = SafeIcon(IconHourglass)
		}
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", iconStr, trimStatusMessage(statusMsg))))

	// Add health indicator - always show it like port forwards do
	b.WriteString("\n")
	var healthIcon, healthText string
	if proc != nil {
		stateLower := strings.ToLower(proc.State)
		healthLower := strings.ToLower(proc.Health)
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

	frame := final.GetHorizontalFrameSize()
	width := targetOuterWidth - frame
	if width < 0 {
		width = 0
	}

	return final.Copy().Width(width).Render(b.String())
}
