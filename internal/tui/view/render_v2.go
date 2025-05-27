package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderV2 renders the UI for ModelV2
func RenderV2(m *model.ModelV2) string {
	switch m.CurrentAppMode {
	case model.ModeQuitting:
		return color.StatusStyle.Render(m.QuittingMessage)
	case model.ModeInitializing:
		if m.Width == 0 || m.Height == 0 {
			return color.StatusStyle.Render("Initializing... (waiting for window size)")
		}
		return color.StatusStyle.Render(fmt.Sprintf("%s Initializing...", m.Spinner.View()))
	case model.ModeNewConnectionInput:
		return renderNewConnectionInputViewV2(m, m.Width)
	case model.ModeMainDashboard:
		return renderMainDashboardV2(m)
	case model.ModeHelpOverlay:
		return renderHelpOverlayV2(m)
	case model.ModeLogOverlay:
		return renderLogOverlayV2(m)
	case model.ModeMcpConfigOverlay:
		return renderMcpConfigOverlayV2(m)
	case model.ModeMcpToolsOverlay:
		return renderMcpToolsOverlayV2(m)
	default:
		return color.StatusStyle.Render(fmt.Sprintf("Unhandled application mode: %s", m.CurrentAppMode.String()))
	}
}

// renderMainDashboardV2 renders the main dashboard for ModelV2
func renderMainDashboardV2(m *model.ModelV2) string {
	contentWidth := m.Width - color.AppStyle.GetHorizontalFrameSize()
	totalAvailableHeight := m.Height - color.AppStyle.GetVerticalFrameSize()

	// Render header
	headerView := renderHeaderV2(m, contentWidth)
	headerHeight := lipgloss.Height(headerView)

	// Calculate heights for each section
	maxRow1Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
	if maxRow1Height < 5 {
		maxRow1Height = 5
	} else if maxRow1Height > 7 {
		maxRow1Height = 7
	}
	row1View := renderContextPanesRowV2(m, contentWidth, maxRow1Height)
	row1Height := lipgloss.Height(row1View)

	maxRow2Height := int(float64(totalAvailableHeight-headerHeight) * 0.30)
	if maxRow2Height < 7 {
		maxRow2Height = 7
	} else if maxRow2Height > 9 {
		maxRow2Height = 9
	}
	row2View := renderPortForwardingRowV2(m, contentWidth, maxRow2Height)
	row2Height := lipgloss.Height(row2View)

	maxRow3Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
	if maxRow3Height < 5 {
		maxRow3Height = 5
	}
	row3View := renderMcpProxiesRowV2(m, contentWidth, maxRow3Height)
	row3Height := lipgloss.Height(row3View)

	// Log panel
	logPanelView := ""
	if m.Height >= minHeightForMainLogView {
		numGaps := 4
		heightConsumed := headerHeight + row1Height + row2Height + row3Height + numGaps
		logSectionHeight := totalAvailableHeight - heightConsumed
		if logSectionHeight < 0 {
			logSectionHeight = 0
		}

		m.MainLogViewport.Width = contentWidth - color.PanelStatusDefaultStyle.GetHorizontalFrameSize()
		m.MainLogViewport.Height = logSectionHeight - color.PanelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(color.LogPanelTitleStyle.Render(" ")) - 1
		if m.MainLogViewport.Height < 0 {
			m.MainLogViewport.Height = 0
		}

		if m.ActivityLogDirty || m.MainLogViewportLastWidth != m.MainLogViewport.Width {
			trunc := PrepareLogContent(m.ActivityLog, m.MainLogViewport.Width)
			m.MainLogViewport.SetContent(trunc)
			m.ActivityLogDirty = false
			m.MainLogViewportLastWidth = m.MainLogViewport.Width
		}

		logPanelView = renderCombinedLogPanelV2(m, contentWidth, logSectionHeight)
	}

	statusBar := renderStatusBarV2(m, m.Width)

	bodyParts := []string{headerView, row1View, row2View, row3View}
	if logPanelView != "" {
		bodyParts = append(bodyParts, logPanelView)
	}
	bodyParts = append(bodyParts, statusBar)

	mainView := lipgloss.JoinVertical(lipgloss.Left, bodyParts...)
	return color.AppStyle.Width(m.Width).Render(mainView)
}

// Adapter functions to reuse existing view components
func renderHeaderV2(m *model.ModelV2, width int) string {
	// For now, create a simple header
	title := fmt.Sprintf("envctl v2 - %s", m.ManagementClusterName)
	if m.WorkloadClusterName != "" {
		title += fmt.Sprintf(" / %s", m.WorkloadClusterName)
	}
	return color.HeaderStyle.Width(width).Render(title)
}

func renderContextPanesRowV2(m *model.ModelV2, width, maxHeight int) string {
	// Render K8s connections
	var panes []string

	for label, conn := range m.K8sConnections {
		status := "●"
		statusColor := color.ErrorStyle
		if conn.Health == "healthy" {
			statusColor = color.HealthGoodStyle
		}
		status = statusColor.Render("●")

		content := fmt.Sprintf("%s %s\nNodes: %d/%d", status, label, conn.ReadyNodes, conn.TotalNodes)
		pane := color.PanelStatusDefaultStyle.Width(width/2 - 1).Height(maxHeight).Render(content)
		panes = append(panes, pane)
	}

	if len(panes) == 0 {
		return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render("No K8s connections")
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, panes...)
}

func renderPortForwardingRowV2(m *model.ModelV2, width, maxHeight int) string {
	title := color.PortTitleStyle.Render("Port Forwards")

	var content []string
	for label, pf := range m.PortForwards {
		status := "○"
		if pf.State == "running" {
			status = color.HealthGoodStyle.Render("●")
		}
		line := fmt.Sprintf("%s %s → localhost:%d", status, label, pf.LocalPort)
		content = append(content, line)
	}

	if len(content) == 0 {
		content = append(content, "No port forwards configured")
	}

	body := strings.Join(content, "\n")
	return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render(title + "\n" + body)
}

func renderMcpProxiesRowV2(m *model.ModelV2, width, maxHeight int) string {
	title := color.PortTitleStyle.Render("MCP Servers")

	var content []string
	for label, mcp := range m.MCPServers {
		status := "○"
		if mcp.State == "running" {
			status = color.HealthGoodStyle.Render("●")
		}
		line := fmt.Sprintf("%s %s (port: %d)", status, label, mcp.Port)
		content = append(content, line)
	}

	if len(content) == 0 {
		content = append(content, "No MCP servers configured")
	}

	body := strings.Join(content, "\n")
	return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render(title + "\n" + body)
}

func renderCombinedLogPanelV2(m *model.ModelV2, width, height int) string {
	title := color.LogPanelTitleStyle.Render("Activity Log")
	viewport := m.MainLogViewport.View()
	return color.PanelStatusDefaultStyle.Width(width).Height(height).Render(title + "\n" + viewport)
}

func renderStatusBarV2(m *model.ModelV2, width int) string {
	left := m.StatusBarMessage
	if left == "" {
		left = "Ready"
	}
	right := "? help • q quit"

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := width - leftWidth - rightWidth - 2
	if padding < 0 {
		padding = 0
	}

	// Determine background color based on message type
	bgColor := color.StatusBarDefaultBg
	if m.StatusBarMessageType == model.StatusBarSuccess {
		bgColor = color.StatusBarSuccessBg
	} else if m.StatusBarMessageType == model.StatusBarError {
		bgColor = color.StatusBarErrorBg
	} else if m.StatusBarMessageType == model.StatusBarWarning {
		bgColor = color.StatusBarWarningBg
	}

	return color.StatusBarBaseStyle.
		Background(bgColor).
		Width(width).
		Render(color.StatusBarTextStyle.Render(left + strings.Repeat(" ", padding) + right))
}

// Placeholder functions for other modes
func renderNewConnectionInputViewV2(m *model.ModelV2, width int) string {
	return renderNewConnectionInputView(&model.Model{
		NewConnectionInput: m.NewConnectionInput,
		Width:              width,
	}, width)
}

func renderHelpOverlayV2(m *model.ModelV2) string {
	// Reuse existing help overlay logic
	titleView := color.HelpTitleStyle.Render("KEYBOARD SHORTCUTS")
	helpContent := "Help content here..." // TODO: Generate from m.Keys
	container := color.CenteredOverlayContainerStyle.Render(titleView + "\n" + helpContent)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, container)
}

func renderLogOverlayV2(m *model.ModelV2) string {
	titleText := SafeIcon(IconScroll) + " Activity Log  (↑/↓ scroll  •  y copy  •  Esc close)"
	titleView := color.LogPanelTitleStyle.Render(titleText)

	overlayTotalWidth := int(float64(m.Width) * 0.8)
	overlayTotalHeight := int(float64(m.Height) * 0.7)

	content := lipgloss.JoinVertical(lipgloss.Left, titleView, m.LogViewport.View())
	overlay := color.LogOverlayStyle.Width(overlayTotalWidth).Height(overlayTotalHeight).Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, overlay)
}

func renderMcpConfigOverlayV2(m *model.ModelV2) string {
	titleText := SafeIcon(IconGear) + " MCP Configuration  (↑/↓ scroll  •  y copy  •  Esc close)"
	titleView := color.LogPanelTitleStyle.Render(titleText)

	overlayTotalWidth := int(float64(m.Width) * 0.8)
	overlayTotalHeight := int(float64(m.Height) * 0.7)

	content := lipgloss.JoinVertical(lipgloss.Left, titleView, m.McpConfigViewport.View())
	overlay := color.McpConfigOverlayStyle.Width(overlayTotalWidth).Height(overlayTotalHeight).Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, overlay)
}

func renderMcpToolsOverlayV2(m *model.ModelV2) string {
	titleText := SafeIcon(IconGear) + " MCP Server Tools  (↑/↓ scroll  •  Esc close)"
	titleView := color.LogPanelTitleStyle.Render(titleText)

	overlayTotalWidth := int(float64(m.Width) * 0.8)
	overlayTotalHeight := int(float64(m.Height) * 0.7)

	// Generate tools content
	toolsContent := GenerateMcpToolsContentV2(m)
	m.McpToolsViewport.SetContent(toolsContent)

	content := lipgloss.JoinVertical(lipgloss.Left, titleView, m.McpToolsViewport.View())
	overlay := color.McpConfigOverlayStyle.Width(overlayTotalWidth).Height(overlayTotalHeight).Render(content)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, overlay)
}

// GenerateMcpToolsContentV2 generates MCP tools content for ModelV2
func GenerateMcpToolsContentV2(m *model.ModelV2) string {
	var content []string

	for serverName, tools := range m.MCPTools {
		content = append(content, fmt.Sprintf("=== %s ===", serverName))
		if len(tools) == 0 {
			content = append(content, "  No tools available")
		} else {
			for _, tool := range tools {
				content = append(content, fmt.Sprintf("  • %s: %s", tool.Name, tool.Description))
			}
		}
		content = append(content, "")
	}

	if len(content) == 0 {
		return "No MCP servers with tools available"
	}

	return strings.Join(content, "\n")
}
