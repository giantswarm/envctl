package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"envctl/internal/api"

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
	// Look for MC and WC connections
	var mcConn, wcConn *api.K8sConnectionInfo
	
	for label, conn := range m.K8sConnections {
		if strings.Contains(label, "mc-") && strings.Contains(label, m.ManagementClusterName) {
			mcConn = conn
		} else if strings.Contains(label, "wc-") && strings.Contains(label, m.WorkloadClusterName) {
			wcConn = conn
		}
	}

	// Render MC pane
	mcPane := renderMcPaneV2(m, mcConn, width/2)
	
	// If we have a WC, render both side by side
	if wcConn != nil && m.WorkloadClusterName != "" {
		wcPane := renderWcPaneV2(m, wcConn, width/2)
		return lipgloss.JoinHorizontal(lipgloss.Top, mcPane, wcPane)
	}
	
	// Otherwise just render MC pane full width
	return renderMcPaneV2(m, mcConn, width)
}

func renderMcPaneV2(m *model.ModelV2, conn *api.K8sConnectionInfo, paneWidth int) string {
	mcName := m.ManagementClusterName
	if mcName == "" {
		mcName = "N/A"
	}

	// Check if MC is active
	isMcActive := m.CurrentKubeContext != "" && strings.Contains(m.CurrentKubeContext, m.ManagementClusterName)
	
	activeString := ""
	if isMcActive {
		activeString = " (Active)"
	}

	content := fmt.Sprintf("%s MC: %s%s", SafeIcon(IconKubernetes), mcName, activeString)
	
	// Add health status
	if conn != nil {
		healthIcon := SafeIcon(IconCheck)
		healthStyle := color.HealthGoodStyle
		
		if conn.Health != "healthy" {
			healthIcon = SafeIcon(IconWarning)
			healthStyle = color.HealthWarnStyle
		}
		
		healthText := fmt.Sprintf("%s Nodes: %d/%d", healthIcon, conn.ReadyNodes, conn.TotalNodes)
		content += "\n" + healthStyle.Render(healthText)
	} else {
		content += "\n" + color.HealthLoadingStyle.Render(SafeIcon(IconHourglass) + " Nodes: Loading...")
	}

	// Determine pane style
	paneStyle := color.ContextPaneStyle
	if isMcActive {
		paneStyle = color.ActiveContextPaneStyle
	}
	if m.FocusedPanelKey == "k8s-mc-"+m.ManagementClusterName {
		if isMcActive {
			paneStyle = color.FocusedAndActiveContextPaneStyle
		} else {
			paneStyle = color.FocusedContextPaneStyle
		}
	}
	
	return paneStyle.Width(paneWidth - paneStyle.GetHorizontalFrameSize()).Render(content)
}

func renderWcPaneV2(m *model.ModelV2, conn *api.K8sConnectionInfo, paneWidth int) string {
	wcName := m.WorkloadClusterName
	if wcName == "" {
		return ""
	}

	// Check if WC is active
	isWcActive := m.CurrentKubeContext != "" && strings.Contains(m.CurrentKubeContext, m.WorkloadClusterName)
	
	activeString := ""
	if isWcActive {
		activeString = " (Active)"
	}

	content := fmt.Sprintf("%s WC: %s%s", SafeIcon(IconKubernetes), wcName, activeString)
	
	// Add health status
	if conn != nil {
		healthIcon := SafeIcon(IconCheck)
		healthStyle := color.HealthGoodStyle
		
		if conn.Health != "healthy" {
			healthIcon = SafeIcon(IconWarning)
			healthStyle = color.HealthWarnStyle
		}
		
		healthText := fmt.Sprintf("%s Nodes: %d/%d", healthIcon, conn.ReadyNodes, conn.TotalNodes)
		content += "\n" + healthStyle.Render(healthText)
	} else {
		content += "\n" + color.HealthLoadingStyle.Render(SafeIcon(IconHourglass) + " Nodes: Loading...")
	}

	// Determine pane style
	paneStyle := color.ContextPaneStyle
	if isWcActive {
		paneStyle = color.ActiveContextPaneStyle
	}
	if m.FocusedPanelKey == "k8s-wc-"+m.WorkloadClusterName {
		if isWcActive {
			paneStyle = color.FocusedAndActiveContextPaneStyle
		} else {
			paneStyle = color.FocusedContextPaneStyle
		}
	}
	
	return paneStyle.Width(paneWidth - paneStyle.GetHorizontalFrameSize()).Render(content)
}

func renderPortForwardingRowV2(m *model.ModelV2, width, maxHeight int) string {
	title := color.PortTitleStyle.Render(SafeIcon(IconLink) + " Port Forwards")

	var content []string
	
	// Use the ordered list to maintain consistent ordering
	for _, label := range m.PortForwardOrder {
		pf, exists := m.PortForwards[label]
		if !exists {
			continue
		}
		
		statusIcon := SafeIcon(IconStop)
		statusStyle := color.HealthLoadingStyle
		statusText := "Starting"
		
		switch pf.State {
		case "running":
			statusIcon = SafeIcon(IconCheck)
			statusStyle = color.HealthGoodStyle
			statusText = "Running"
		case "failed":
			statusIcon = SafeIcon(IconCross)
			statusStyle = color.HealthErrorStyle
			statusText = "Failed"
		case "stopped":
			statusIcon = SafeIcon(IconStop)
			statusStyle = color.HealthWarnStyle
			statusText = "Stopped"
		}
		
		// Format: icon name
		// Port: localPort → targetType/targetName
		// Status: statusText • Health: healthStatus
		line1 := fmt.Sprintf("%s %s", pf.Icon, pf.Name)
		line2 := fmt.Sprintf("Port: %d:%d → %s/%s", pf.LocalPort, pf.RemotePort, pf.TargetType, pf.TargetName)
		line3 := fmt.Sprintf("Status: %s %s • Health: %s %s", 
			statusStyle.Render(statusIcon), statusText,
			statusStyle.Render(statusIcon), pf.Health)
		
		content = append(content, line1, line2, line3, "")
	}

	if len(content) == 0 {
		content = append(content, "No port forwards configured")
	}

	body := strings.Join(content, "\n")
	return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render(title + "\n\n" + body)
}

func renderMcpProxiesRowV2(m *model.ModelV2, width, maxHeight int) string {
	// Split width into 3 columns for MCP servers
	colWidth := width / 3
	var columns []string
	
	// Use the ordered list to maintain consistent ordering
	for i, name := range m.MCPServerOrder {
		mcp, exists := m.MCPServers[name]
		if !exists {
			continue
		}
		
		// Determine which column this MCP server goes in
		colIndex := i % 3
		if colIndex >= len(columns) {
			columns = append(columns, "")
		}
		
		statusIcon := SafeIcon(IconStop)
		pidText := "N/A"
		portText := "N/A"
		statusText := "Starting"
		healthText := "Checking..."
		
		switch mcp.State {
		case "running":
			statusIcon = SafeIcon(IconPlay)
			statusText = "Running"
			if mcp.PID > 0 {
				pidText = fmt.Sprintf("%d", mcp.PID)
			}
			if mcp.Port > 0 {
				portText = fmt.Sprintf("%d", mcp.Port)
			}
			healthText = mcp.Health
		case "failed":
			statusIcon = SafeIcon(IconCross)
			statusText = "Failed"
			healthText = "Error"
		case "stopped":
			statusIcon = SafeIcon(IconStop)
			statusText = "Stopped"
			healthText = "N/A"
		}
		
		// Format the MCP server info
		content := fmt.Sprintf("%s %s %s\n", statusIcon, mcp.Icon, mcp.Name)
		content += fmt.Sprintf("PID: %s\n", pidText)
		content += fmt.Sprintf("Port: %s\n", portText)
		content += fmt.Sprintf("Status: %s\n", statusText)
		content += fmt.Sprintf("Health: %s", healthText)
		
		// Style based on state and focus
		paneStyle := color.PanelStatusDefaultStyle
		if mcp.State == "running" {
			paneStyle = color.PanelStatusRunningStyle
		} else if mcp.State == "failed" {
			paneStyle = color.PanelStatusErrorStyle
		}
		
		if m.FocusedPanelKey == name {
			if mcp.State == "running" {
				paneStyle = color.FocusedPanelStatusRunningStyle
			} else if mcp.State == "failed" {
				paneStyle = color.FocusedPanelStatusErrorStyle
			} else {
				paneStyle = color.FocusedPanelStatusDefaultStyle
			}
		}
		
		pane := paneStyle.Width(colWidth - 2).Render(content)
		columns[colIndex] += pane + "\n"
	}
	
	if len(columns) == 0 {
		return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render("No MCP servers configured")
	}
	
	// Join columns horizontally
	mcpRow := lipgloss.JoinHorizontal(lipgloss.Top, columns...)
	
	// Add title
	title := color.PortTitleStyle.Render(SafeIcon(IconGear) + " MCP Servers")
	return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render(title + "\n" + mcpRow)
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
