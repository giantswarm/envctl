package tui

import (
	"fmt"
	"strings"

	"envctl/internal/mcpserver"

	"github.com/charmbracelet/lipgloss"
)

// renderMcpProxyPanel renders one MCP proxy status panel (moved from view_helpers.go).
func renderMcpProxyPanel(serverName string, predefinedData mcpserver.PredefinedMcpServer, proc *mcpServerProcess, m model, targetOuterWidth int) string {
	var baseStyle lipgloss.Style
	var contentFg lipgloss.Style
	statusMsg := "Not Started"
	pidStr := "PID: N/A"

	if proc != nil {
		statusMsg = strings.TrimSpace(proc.statusMsg)
		if proc.pid > 0 {
			pidStr = fmt.Sprintf("PID: %d", proc.pid)
		}
		st := strings.ToLower(statusMsg)
		switch {
		case proc.err != nil || strings.Contains(st, "error") || strings.Contains(st, "failed"):
			baseStyle = panelStatusErrorStyle
			contentFg = statusMsgErrorStyle
		case strings.Contains(st, "running"):
			baseStyle = panelStatusRunningStyle
			contentFg = statusMsgRunningStyle
		default:
			baseStyle = panelStatusInitializingStyle
			contentFg = statusMsgInitializingStyle
		}
	} else {
		baseStyle = panelStatusExitedStyle
		contentFg = statusMsgExitedStyle
	}

	final := baseStyle.Copy().Foreground(contentFg.GetForeground())
	if m.focusedPanelKey == serverName {
		final = final.Copy().Border(lipgloss.DoubleBorder()).Bold(true)
	}

	var b strings.Builder
	b.WriteString(portTitleStyle.Render(SafeIcon(IconGear) + strings.TrimSpace(predefinedData.Name) + " MCP"))
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
		case proc.err != nil || strings.Contains(st, "error") || strings.Contains(st, "failed"):
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
func renderMcpProxiesRow(m model, contentWidth int, maxRowHeight int) string {
	const cols = 3
	totalBorder := 0
	styles := make([]lipgloss.Style, len(mcpserver.PredefinedMcpServers))
	for i, def := range mcpserver.PredefinedMcpServers {
		proc := m.mcpServers[def.Name]
		st := ""
		if proc != nil {
			st = strings.ToLower(proc.statusMsg)
		}
		var s lipgloss.Style
		switch {
		case proc != nil && (proc.err != nil || strings.Contains(st, "error") || strings.Contains(st, "failed")):
			s = panelStatusErrorStyle
		case proc != nil && strings.Contains(st, "running"):
			s = panelStatusRunningStyle
		default:
			s = panelStatusInitializingStyle
		}
		styles[i] = s
		if i < cols {
			totalBorder += s.GetHorizontalFrameSize()
		}
	}

	displayCols := len(mcpserver.PredefinedMcpServers)
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
		def := mcpserver.PredefinedMcpServers[i]
		proc := m.mcpServers[def.Name]
		w := baseInner
		if i < remainder {
			w++
		}
		rendered := renderMcpProxyPanel(def.Name, def, proc, m, w+styles[i].GetHorizontalFrameSize())
		cells = append(cells, rendered)
	}
	for i := len(mcpserver.PredefinedMcpServers); i < cols; i++ {
		w := baseInner
		if i < remainder {
			w++
		}
		cells = append(cells, panelStyle.Copy().Width(w).Render(""))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).MaxHeight(maxRowHeight).Render(row)
}
