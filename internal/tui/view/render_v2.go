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
	// Render port forwards as a grid of panels, up to 3 columns
	const maxCols = 3
	
	// Get number of port forwards
	numPFs := len(m.PortForwardOrder)
	if numPFs == 0 {
		return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render(
			color.PortTitleStyle.Render(SafeIcon(IconLink) + " Port Forwards") + "\n\nNo port forwards configured")
	}
	
	// Determine number of columns to display
	displayCols := numPFs
	if displayCols > maxCols {
		displayCols = maxCols
	}
	
	// Calculate total border width
	totalBorderWidth := 0
	for i := 0; i < displayCols && i < numPFs; i++ {
		totalBorderWidth += color.PanelStatusDefaultStyle.GetHorizontalFrameSize()
	}
	
	// Calculate inner width for content
	innerWidth := width - totalBorderWidth
	if innerWidth < 0 {
		innerWidth = 0
	}
	
	// Distribute width among columns
	baseInnerWidth := 0
	if displayCols > 0 {
		baseInnerWidth = innerWidth / displayCols
	}
	remainder := 0
	if displayCols > 0 {
		remainder = innerWidth % displayCols
	}
	
	// Render each port forward as a panel
	var panels []string
	for i := 0; i < displayCols && i < numPFs; i++ {
		label := m.PortForwardOrder[i]
		pf, exists := m.PortForwards[label]
		if !exists {
			continue
		}
		
		// Calculate width for this panel
		panelInnerWidth := baseInnerWidth
		if i < remainder {
			panelInnerWidth++
		}
		
		// Render the panel
		panel := renderPortForwardPanelV2(m, label, pf, panelInnerWidth + color.PanelStatusDefaultStyle.GetHorizontalFrameSize())
		panels = append(panels, panel)
	}
	
	// Add empty panels to fill up to 3 columns
	for i := len(panels); i < maxCols; i++ {
		panelInnerWidth := baseInnerWidth
		if i < remainder {
			panelInnerWidth++
		}
		emptyPanel := color.PanelStyle.Width(panelInnerWidth).Render("")
		panels = append(panels, emptyPanel)
	}
	
	// Join panels horizontally
	pfRow := lipgloss.JoinHorizontal(lipgloss.Top, panels...)
	
	// Wrap with proper height constraint
	return lipgloss.NewStyle().
		Width(width).
		MaxHeight(maxHeight).
		Align(lipgloss.Left).
		Render(pfRow)
}

func renderPortForwardPanelV2(m *model.ModelV2, label string, pf *api.PortForwardServiceInfo, targetWidth int) string {
	// Determine panel style based on state
	var baseStyle lipgloss.Style
	var contentStyle lipgloss.Style
	
	switch pf.State {
	case "running":
		baseStyle = color.PanelStatusRunningStyle
		contentStyle = color.StatusMsgRunningStyle
	case "failed":
		baseStyle = color.PanelStatusErrorStyle
		contentStyle = color.StatusMsgErrorStyle
	case "stopped":
		baseStyle = color.PanelStatusExitedStyle
		contentStyle = color.StatusMsgExitedStyle
	default:
		baseStyle = color.PanelStatusInitializingStyle
		contentStyle = color.StatusMsgInitializingStyle
	}
	
	// Apply focus style
	if m.FocusedPanelKey == label {
		switch pf.State {
		case "running":
			baseStyle = color.FocusedPanelStatusRunningStyle
		case "failed":
			baseStyle = color.FocusedPanelStatusErrorStyle
		case "stopped":
			baseStyle = color.FocusedPanelStatusExitedStyle
		default:
			baseStyle = color.FocusedPanelStatusInitializingStyle
		}
	}
	
	// Build content
	var content strings.Builder
	
	// Title with icon
	icon := pf.Icon
	if icon == "" {
		icon = SafeIcon(IconLink)
	}
	content.WriteString(color.PortTitleStyle.Render(icon + " " + pf.Name))
	content.WriteString("\n")
	
	// Port info
	content.WriteString(fmt.Sprintf("Port: %d:%d", pf.LocalPort, pf.RemotePort))
	content.WriteString("\n")
	
	// Target info
	content.WriteString(fmt.Sprintf("Target: %s/%s", pf.TargetType, pf.TargetName))
	content.WriteString("\n")
	
	// Status with icon
	statusIcon := SafeIcon(IconHourglass)
	statusText := "Starting"
	switch pf.State {
	case "running":
		statusIcon = SafeIcon(IconPlay)
		statusText = "Running"
	case "failed":
		statusIcon = SafeIcon(IconCross)
		statusText = "Failed"
	case "stopped":
		statusIcon = SafeIcon(IconStop)
		statusText = "Stopped"
	}
	content.WriteString(contentStyle.Render(fmt.Sprintf("Status: %s %s", statusIcon, statusText)))
	
	// Health indicator
	if pf.State == "running" || pf.State == "starting" {
		content.WriteString("\n")
		healthIcon := SafeIcon(IconHourglass)
		healthText := "Checking..."
		if pf.State == "running" && pf.Health == "healthy" {
			healthIcon = SafeIcon(IconCheck)
			healthText = "Healthy"
		} else if pf.Health == "unhealthy" {
			healthIcon = SafeIcon(IconCross)
			healthText = "Unhealthy"
		}
		content.WriteString(contentStyle.Render(fmt.Sprintf("Health: %s %s", healthIcon, healthText)))
	}
	
	// Calculate actual width for content
	frameSize := baseStyle.GetHorizontalFrameSize()
	contentWidth := targetWidth - frameSize
	if contentWidth < 0 {
		contentWidth = 0
	}
	
	// Render the panel
	return baseStyle.Width(contentWidth).Render(content.String())
}

func renderMcpProxiesRowV2(m *model.ModelV2, width, maxHeight int) string {
	// Render MCP servers as individual panels, up to 3 columns
	const maxCols = 3
	
	// Calculate how many servers we have
	numServers := len(m.MCPServerOrder)
	if numServers == 0 {
		return color.PanelStatusDefaultStyle.Width(width).Height(maxHeight).Render("No MCP servers configured")
	}
	
	// Determine number of columns to display
	displayCols := numServers
	if displayCols > maxCols {
		displayCols = maxCols
	}
	
	// Calculate width for each column
	// First, account for borders of each panel
	totalBorderWidth := 0
	for i := 0; i < displayCols && i < numServers; i++ {
		// Each panel has a border
		totalBorderWidth += color.PanelStatusDefaultStyle.GetHorizontalFrameSize()
	}
	
	// Calculate inner width available for content
	innerWidth := width - totalBorderWidth
	if innerWidth < 0 {
		innerWidth = 0
	}
	
	// Distribute width evenly among columns
	baseInnerWidth := 0
	if displayCols > 0 {
		baseInnerWidth = innerWidth / displayCols
	}
	remainder := 0
	if displayCols > 0 {
		remainder = innerWidth % displayCols
	}
	
	// Render each MCP server as a panel
	var panels []string
	for i := 0; i < displayCols && i < numServers; i++ {
		name := m.MCPServerOrder[i]
		mcp, exists := m.MCPServers[name]
		if !exists {
			continue
		}
		
		// Calculate width for this panel
		panelInnerWidth := baseInnerWidth
		if i < remainder {
			panelInnerWidth++ // Distribute remainder
		}
		
		// Render the panel
		panel := renderMcpPanelV2(m, name, mcp, panelInnerWidth + color.PanelStatusDefaultStyle.GetHorizontalFrameSize())
		panels = append(panels, panel)
	}
	
	// Add empty panels if needed to fill up to 3 columns
	for i := len(panels); i < maxCols; i++ {
		panelInnerWidth := baseInnerWidth
		if i < remainder {
			panelInnerWidth++
		}
		emptyPanel := color.PanelStyle.Width(panelInnerWidth).Render("")
		panels = append(panels, emptyPanel)
	}
	
	// Join panels horizontally
	mcpRow := lipgloss.JoinHorizontal(lipgloss.Top, panels...)
	
	// Add title above the panels
	title := color.PortTitleStyle.Render(SafeIcon(IconGear) + " MCP Servers")
	fullContent := lipgloss.JoinVertical(lipgloss.Left, title, mcpRow)
	
	// Wrap in a container with proper height
	return lipgloss.NewStyle().
		Width(width).
		MaxHeight(maxHeight).
		Align(lipgloss.Left).
		Render(fullContent)
}

func renderMcpPanelV2(m *model.ModelV2, name string, mcp *api.MCPServerInfo, targetWidth int) string {
	// Determine panel style based on state
	var baseStyle lipgloss.Style
	var contentStyle lipgloss.Style
	
	switch mcp.State {
	case "running":
		baseStyle = color.PanelStatusRunningStyle
		contentStyle = color.StatusMsgRunningStyle
	case "failed":
		baseStyle = color.PanelStatusErrorStyle
		contentStyle = color.StatusMsgErrorStyle
	case "stopped":
		baseStyle = color.PanelStatusExitedStyle
		contentStyle = color.StatusMsgExitedStyle
	default:
		baseStyle = color.PanelStatusInitializingStyle
		contentStyle = color.StatusMsgInitializingStyle
	}
	
	// Apply focus style if this panel is focused
	if m.FocusedPanelKey == name {
		if mcp.State == "running" {
			baseStyle = color.FocusedPanelStatusRunningStyle
		} else if mcp.State == "failed" {
			baseStyle = color.FocusedPanelStatusErrorStyle
		} else {
			baseStyle = color.FocusedPanelStatusDefaultStyle
		}
	}
	
	// Build content
	var content strings.Builder
	
	// Title with icon
	icon := mcp.Icon
	if icon == "" {
		icon = SafeIcon(IconGear)
	}
	content.WriteString(color.PortTitleStyle.Render(icon + " " + name))
	content.WriteString("\n")
	
	// PID
	pidStr := "PID: N/A"
	if mcp.PID > 0 {
		pidStr = fmt.Sprintf("PID: %d", mcp.PID)
	}
	content.WriteString(pidStr)
	content.WriteString("\n")
	
	// Port
	portStr := "Port: N/A"
	if mcp.Port > 0 {
		portStr = fmt.Sprintf("Port: %d", mcp.Port)
	}
	content.WriteString(portStr)
	content.WriteString("\n")
	
	// Status with icon
	statusIcon := SafeIcon(IconHourglass)
	statusText := "Starting"
	switch mcp.State {
	case "running":
		statusIcon = SafeIcon(IconPlay)
		statusText = "Running"
	case "failed":
		statusIcon = SafeIcon(IconCross)
		statusText = "Failed"
	case "stopped":
		statusIcon = SafeIcon(IconStop)
		statusText = "Stopped"
	}
	content.WriteString(contentStyle.Render(fmt.Sprintf("Status: %s %s", statusIcon, statusText)))
	
	// Health
	healthIcon := SafeIcon(IconHourglass)
	healthText := "Checking..."
	if mcp.State == "running" && mcp.Health == "healthy" {
		healthIcon = SafeIcon(IconCheck)
		healthText = "Healthy"
	} else if mcp.State == "failed" || mcp.Health == "unhealthy" {
		healthIcon = SafeIcon(IconCross)
		healthText = "Unhealthy"
	}
	content.WriteString(contentStyle.Render(fmt.Sprintf("Health: %s %s", healthIcon, healthText)))
	
	// Calculate actual width for content
	frameSize := baseStyle.GetHorizontalFrameSize()
	contentWidth := targetWidth - frameSize
	if contentWidth < 0 {
		contentWidth = 0
	}
	
	// Render the panel
	return baseStyle.Width(contentWidth).Render(content.String())
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
