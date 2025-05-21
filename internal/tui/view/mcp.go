package view

import (
	"envctl/internal/color"
	"envctl/internal/mcpserver"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderMcpProxyPanel renders one MCP proxy status panel.
func renderMcpProxyPanel(serverName string, predefinedData mcpserver.MCPServerConfig, proc *model.McpServerProcess, m *model.Model, targetOuterWidth int) string {
	var baseStyle lipgloss.Style
	var contentFg lipgloss.Style
	statusMsg := "Not Started"
	pidStr := "PID: N/A"

	if proc != nil {
		statusMsg = strings.TrimSpace(proc.StatusMsg)
		if proc.Pid > 0 {
			pidStr = fmt.Sprintf("PID: %d", proc.Pid)
		}
		st := strings.ToLower(statusMsg)
		switch {
		case proc.Err != nil || strings.Contains(st, "error") || strings.Contains(st, "failed"):
			baseStyle = color.PanelStatusErrorStyle
			contentFg = color.StatusMsgErrorStyle
		case strings.Contains(st, "running"):
			baseStyle = color.PanelStatusRunningStyle
			contentFg = color.StatusMsgRunningStyle
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
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(IconGear) + strings.TrimSpace(predefinedData.Name) + " MCP"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Port: %d (SSE)", predefinedData.ProxyPort))
	b.WriteString("\n")
	b.WriteString(pidStr)
	b.WriteString("\n")

	var icon string
	if proc == nil {
		icon = SafeIcon(IconWarning)
	} else {
		st := strings.ToLower(statusMsg)
		switch {
		case proc.Err != nil || strings.Contains(st, "error") || strings.Contains(st, "failed"):
			icon = SafeIcon(IconCross)
		case strings.Contains(st, "running"):
			icon = SafeIcon(IconPlay)
		case strings.Contains(st, "stopped"):
			icon = SafeIcon(IconStop)
		default:
			icon = SafeIcon(IconHourglass)
		}
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", icon, trimStatusMessage(statusMsg))))

	frame := final.GetHorizontalFrameSize()
	width := targetOuterWidth - frame
	if width < 0 {
		width = 0
	}

	return final.Copy().Width(width).Render(b.String())
}

// renderMcpProxiesRow lays out up to 3 MCP proxy panels.
func renderMcpProxiesRow(m *model.Model, contentWidth int, maxRowHeight int) string {
	const cols = 3
	totalBorder := 0
	styles := make([]lipgloss.Style, len(m.MCPServerConfig))
	for i, def := range m.MCPServerConfig {
		proc := m.McpServers[def.Name]
		st := ""
		if proc != nil {
			st = strings.ToLower(proc.StatusMsg)
		}
		var s lipgloss.Style
		switch {
		case proc != nil && (proc.Err != nil || strings.Contains(st, "error") || strings.Contains(st, "failed")):
			s = color.PanelStatusErrorStyle
		case proc != nil && strings.Contains(st, "running"):
			s = color.PanelStatusRunningStyle
		default:
			s = color.PanelStatusInitializingStyle
		}
		styles[i] = s
		if i < cols {
			totalBorder += s.GetHorizontalFrameSize()
		}
	}

	displayCols := len(m.MCPServerConfig)
	if displayCols > cols {
		displayCols = cols
	}
	innerWidth := contentWidth - totalBorder
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
		proc := m.McpServers[def.Name]
		w := baseInner
		if i < remainder {
			w++
		}
		rendered := renderMcpProxyPanel(def.Name, def, proc, m, w+styles[i].GetHorizontalFrameSize())
		cells = append(cells, rendered)
	}
	for i := len(m.MCPServerConfig); i < cols; i++ {
		w := baseInner
		if i < remainder {
			w++
		}
		cells = append(cells, color.PanelStyle.Copy().Width(w).Render(""))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).MaxHeight(maxRowHeight).Render(row)
}
