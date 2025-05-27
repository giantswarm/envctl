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

// renderMcpProxiesRowV2 renders the MCP proxies row
func renderMcpProxiesRowV2(m *model.ModelV2, width, maxHeight int) string {
	// Match v1 exactly - no title above, just the panels
	const cols = 3

	// Get MCP server definitions from config
	numServers := len(m.MCPServerConfig)
	if numServers == 0 {
		// V1 returns empty panels when no servers configured
		var cells []string
		for i := 0; i < cols; i++ {
			cells = append(cells, color.PanelStyle.Copy().Width(width/cols).Render(""))
		}
		row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
	}

	// Calculate styles and borders like v1
	totalBorder := 0
	styles := make([]lipgloss.Style, numServers)
	for i, def := range m.MCPServerConfig {
		proc := m.MCPServers[def.Name]
		st := ""
		if proc != nil {
			st = strings.ToLower(proc.State)
		}
		var s lipgloss.Style
		switch {
		case proc != nil && (proc.State == "failed" || strings.Contains(st, "error")):
			s = color.PanelStatusErrorStyle
		case proc != nil && proc.State == "running":
			s = color.PanelStatusRunningStyle
		default:
			s = color.PanelStatusInitializingStyle
		}
		styles[i] = s
		if i < cols {
			totalBorder += s.GetHorizontalFrameSize()
		}
	}

	displayCols := numServers
	if displayCols > cols {
		displayCols = cols
	}
	innerWidth := width - totalBorder
	if innerWidth < 0 {
		innerWidth = 0
	}
	baseInner := 0
	if displayCols > 0 {
		baseInner = innerWidth / displayCols
	}
	remainder := 0
	if displayCols > 0 {
		remainder = innerWidth % displayCols
	}

	var cells []string
	for i := 0; i < displayCols; i++ {
		def := m.MCPServerConfig[i]
		proc := m.MCPServers[def.Name]
		w := baseInner
		if i < remainder {
			w++
		}
		rendered := renderMcpProxyPanelV2(def.Name, def, proc, m, w+styles[i].GetHorizontalFrameSize())
		cells = append(cells, rendered)
	}
	// Fill remaining columns with empty panels
	for i := numServers; i < cols; i++ {
		w := baseInner
		if i < remainder {
			w++
		}
		cells = append(cells, color.PanelStyle.Copy().Width(w).Render(""))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).MaxHeight(maxHeight).Render(row)
}

// renderMcpProxyPanelV2 renders one MCP proxy status panel to match v1 exactly
func renderMcpProxyPanelV2(serverName string, predefinedData config.MCPServerDefinition, proc *api.MCPServerInfo, m *model.ModelV2, targetOuterWidth int) string {
	var baseStyle lipgloss.Style
	var contentFg lipgloss.Style
	statusMsg := "Not Started"
	pidStr := "PID: N/A"
	portStr := "Port: N/A"

	if proc != nil {
		statusMsg = proc.State
		if proc.PID > 0 {
			pidStr = fmt.Sprintf("PID: %d", proc.PID)
		}
		if proc.Port > 0 {
			portStr = fmt.Sprintf("Port: %d", proc.Port)
		}
		st := strings.ToLower(proc.State)
		switch {
		case proc.State == "failed" || strings.Contains(st, "error"):
			baseStyle = color.PanelStatusErrorStyle
			contentFg = color.StatusMsgErrorStyle
		case proc.State == "running":
			baseStyle = color.PanelStatusRunningStyle
			contentFg = color.StatusMsgRunningStyle
		case proc.State == "stopped":
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
	b.WriteString(pidStr)
	b.WriteString("\n")
	b.WriteString(portStr)
	b.WriteString("\n")

	var iconStr string
	if proc == nil {
		iconStr = SafeIcon(IconWarning)
	} else {
		st := strings.ToLower(proc.State)
		switch {
		case proc.State == "failed" || strings.Contains(st, "error"):
			iconStr = SafeIcon(IconCross)
		case proc.State == "running":
			iconStr = SafeIcon(IconPlay)
		case proc.State == "stopped":
			iconStr = SafeIcon(IconStop)
		default:
			iconStr = SafeIcon(IconHourglass)
		}
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", iconStr, trimStatusMessage(statusMsg))))

	// Add health indicator
	if proc != nil {
		b.WriteString("\n")
		var healthIcon, healthText string
		if proc.State == "running" && proc.Health == "healthy" {
			healthIcon = SafeIcon(IconCheck)
			healthText = "Healthy"
		} else if proc.State == "failed" || proc.Health == "unhealthy" {
			healthIcon = SafeIcon(IconCross)
			healthText = "Unhealthy"
		} else {
			healthIcon = SafeIcon(IconHourglass)
			healthText = "Checking..."
		}
		b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))
	}

	frame := final.GetHorizontalFrameSize()
	width := targetOuterWidth - frame
	if width < 0 {
		width = 0
	}

	return final.Copy().Width(width).Render(b.String())
}
