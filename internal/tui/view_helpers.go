package tui

import (
	"envctl/internal/mcpserver" // Added for mcpserver types
	"fmt"
	"strings"

	// For time.Format
	"github.com/charmbracelet/lipgloss"
)

// Will likely be needed for formatting LastUpdated times

// This file will contain helper functions for the View() method in model.go

// renderMcPane renders the Management Cluster (MC) information panel.
// It displays the MC name, its full Kubernetes context, health status (node readiness),
// and indicates if it's the currently active context and/or focused for navigation.
// - m: The current TUI model containing all state information.
// - paneWidth: The target outer width for this pane.
func renderMcPane(m model, paneWidth int) string {
	mcFullNameString := m.managementCluster
	if mcFullNameString == "" {
		mcFullNameString = "N/A"
	}
	// Use the helper method for consistency, though m.managementCluster should be the identifier.
	mcIdentifier := m.getManagementClusterContextIdentifier()
	mcFullContext := "N/A"
	if mcIdentifier != "" {
		mcFullContext = "teleport.giantswarm.io-" + mcIdentifier
	}

	mcActiveString := ""
	isMcActive := m.currentKubeContext == mcFullContext && mcIdentifier != ""
	if isMcActive {
		mcActiveString = " (Active)"
	}

	// Compact version with abbreviated context
	shortContext := strings.TrimPrefix(mcFullContext, "teleport.giantswarm.io-")
	mcPaneContent := fmt.Sprintf("%sMC: %s%s\nCtx: %s", SafeIcon(IconKubernetes), mcFullNameString, mcActiveString, shortContext)

	var healthStatusText string
	var healthStyle lipgloss.Style

	if m.MCHealth.IsLoading {
		healthStatusText = RenderIconWithMessage(IconHourglass, "Nodes: Loading...")
		healthStyle = healthLoadingStyle
	} else if m.MCHealth.StatusError != nil {
		healthStatusText = RenderIconWithMessage(IconCross, fmt.Sprintf("Nodes: Error (%s)", m.MCHealth.LastUpdated.Format("15:04:05")))
		healthStyle = healthErrorStyle
	} else {
		healthIcon := IconCheck
		if m.MCHealth.ReadyNodes < m.MCHealth.TotalNodes {
			healthIcon = IconWarning
			healthStatusText = RenderIconWithNodes(healthIcon, m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes, "[WARN]")
			healthStyle = healthWarnStyle
		} else {
			healthStatusText = RenderIconWithNodes(healthIcon, m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes, "")
			healthStyle = healthGoodStyle
		}
	}
	// Render the health status with appropriate style
	renderedHealthText := healthStyle.Render(healthStatusText)
	mcPaneContent += "\n" + renderedHealthText

	mcPaneStyleToUse := contextPaneStyle
	if isMcActive {
		mcPaneStyleToUse = activeContextPaneStyle
	}
	if m.focusedPanelKey == mcPaneFocusKey {
		if isMcActive {
			mcPaneStyleToUse = focusedAndActiveContextPaneStyle
		} else {
			mcPaneStyleToUse = focusedContextPaneStyle
		}
	}
	return mcPaneStyleToUse.Copy().Width(paneWidth - mcPaneStyleToUse.GetHorizontalFrameSize()).Render(mcPaneContent)
}

// renderWcPane renders the Workload Cluster (WC) information panel.
// Similar to renderMcPane, it displays the WC name, context, health, and active/focused status.
// If no workload cluster is defined in the model, it returns an empty string.
// - m: The current TUI model.
// - paneWidth: The target outer width for this pane.
func renderWcPane(m model, paneWidth int) string {
	if m.workloadCluster == "" {
		return ""
	}

	wcNameString := m.workloadCluster // This is the short WC name, e.g., "myworkloadcluster"

	// Correctly form the full WC context name for display and active check
	// using the model's helper method.
	wcClusterIdentifier := m.getWorkloadClusterContextIdentifier()
	var wcFullContext string
	if wcClusterIdentifier != "" {
		wcFullContext = "teleport.giantswarm.io-" + wcClusterIdentifier
	} else {
		// This case should ideally not be hit if m.workloadCluster was non-empty above,
		// as getWorkloadClusterContextIdentifier should return something.
		// However, as a fallback or if logic changes:
		wcFullContext = "N/A"
	}

	wcActiveString := ""
	isWcActive := m.currentKubeContext == wcFullContext && wcClusterIdentifier != ""
	if isWcActive {
		wcActiveString = " (Active)"
	}

	// Compact version with abbreviated context
	shortContext := wcClusterIdentifier // Use the identifier directly
	if wcFullContext == "N/A" {         // if identifier was empty
		shortContext = "N/A"
	}
	wcPaneContent := fmt.Sprintf("%sWC: %s%s\nCtx: %s", SafeIcon(IconKubernetes), wcNameString, wcActiveString, shortContext)

	var healthStatusText string
	var healthStyle lipgloss.Style

	if m.WCHealth.IsLoading {
		healthStatusText = RenderIconWithMessage(IconHourglass, "Nodes: Loading...")
		healthStyle = healthLoadingStyle
	} else if m.WCHealth.StatusError != nil {
		healthStatusText = RenderIconWithMessage(IconCross, "Nodes: Error")
		healthStyle = healthErrorStyle
	} else {
		healthIcon := IconCheck
		if m.WCHealth.ReadyNodes < m.WCHealth.TotalNodes {
			healthIcon = IconWarning
			healthStatusText = RenderIconWithNodes(healthIcon, m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes, "[WARN]")
			healthStyle = healthWarnStyle
		} else {
			healthStatusText = RenderIconWithNodes(healthIcon, m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes, "")
			healthStyle = healthGoodStyle
		}
	}
	// Render the health status with appropriate style
	renderedHealthText := healthStyle.Render(healthStatusText)
	wcPaneContent += "\n" + renderedHealthText

	wcPaneStyleToRender := contextPaneStyle
	if isWcActive {
		wcPaneStyleToRender = activeContextPaneStyle
	}
	if m.focusedPanelKey == wcPaneFocusKey {
		if isWcActive {
			wcPaneStyleToRender = focusedAndActiveContextPaneStyle
		} else {
			wcPaneStyleToRender = focusedContextPaneStyle
		}
	}
	return wcPaneStyleToRender.Copy().Width(paneWidth - wcPaneStyleToRender.GetHorizontalFrameSize()).Render(wcPaneContent)
}

// renderLogOverlay renders the scrollable activity log in an overlay.
// - m: The current TUI model.
// - width: The target width for the overlay (e.g., 80% of screen).
// - height: The target height for the overlay (e.g., 70% of screen).
func renderLogOverlay(m model, width, height int) string {
	// Ensure viewport has latest content, sized correctly (already done in Update for WindowSizeMsg)
	// Viewport.View() will render its current content within its set dimensions.
	viewportView := m.logViewport.View()

	// Apply the overlay style to the viewport's rendered content.
	// The viewport itself doesn't have a border/padding, so logOverlayStyle provides that.
	// We use the raw width and height passed, assuming they are the desired *outer* dimensions for the overlay box.
	return logOverlayStyle.Copy().
		Width(width - logOverlayStyle.GetHorizontalFrameSize()).
		Height(height - logOverlayStyle.GetVerticalFrameSize()).
		Render(viewportView)
}

// renderPortForwardPanel renders a single panel for a port-forwarding process.
// It dynamically styles the panel (border, background, text color) based on the process's status
// (e.g., running, error, initializing) and whether the panel is currently focused.
// The panel displays the port-forward label, port, context, service, and current status message.
// - pf: The portForwardProcess struct containing details of the specific port forward.
// - m: The current TUI model (used to check for focus).
// - targetOuterWidth: The target outer width for this panel. The function calculates the inner content width.
func renderPortForwardPanel(pf *portForwardProcess, m model, targetOuterWidth int) string {
	// --- 1. Determine panel style based on status and focus ---
	// Selects base and focused styles (border, background) according to the port forward's current state (error, running, exited, initializing).
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	// Trim status message from data source before further processing
	trimmedStatusFromSource := strings.TrimSpace(pf.statusMsg)
	statusToCheck := strings.ToLower(trimmedStatusFromSource)

	if pf.err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed") {
		baseStyleForPanel = panelStatusErrorStyle
		focusedBaseStyleForPanel = focusedPanelStatusErrorStyle
	} else if pf.running && pf.err == nil {
		baseStyleForPanel = panelStatusRunningStyle
		focusedBaseStyleForPanel = focusedPanelStatusRunningStyle
	} else if strings.HasPrefix(statusToCheck, "running (pid:") {
		baseStyleForPanel = panelStatusAttemptingStyle
		focusedBaseStyleForPanel = focusedPanelStatusAttemptingStyle
	} else if strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed") {
		baseStyleForPanel = panelStatusExitedStyle
		focusedBaseStyleForPanel = focusedPanelStatusExitedStyle
	} else { // Covers "Initializing...", "Starting...", "Restarting..."
		baseStyleForPanel = panelStatusInitializingStyle
		focusedBaseStyleForPanel = focusedPanelStatusInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if pf.label == m.focusedPanelKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	// --- 2. Determine foreground text color based on status ---
	// Sets the color of the text content within the panel, distinct from the panel's background or border color.
	var contentFgTextStyle lipgloss.Style
	if pf.err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed") {
		contentFgTextStyle = statusMsgErrorStyle
	} else if pf.running && pf.err == nil {
		contentFgTextStyle = statusMsgRunningStyle
	} else if strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed") {
		contentFgTextStyle = statusMsgExitedStyle
	} else {
		contentFgTextStyle = statusMsgInitializingStyle
	}

	// Apply the determined foreground color to the panel's overall style.
	// This ensures all text within the panel (title, info, status) inherits this color by default,
	// unless overridden by a more specific style (like a bold title).
	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFgTextStyle.GetForeground())

	// --- 3. Construct the textual content of the panel ---
	// Builds the string containing title, port, context, service, and status message.
	var pfContentBuilder strings.Builder

	// Title: Uses a specific bold style but inherits the foreground color from finalPanelStyle.
	pfContentBuilder.WriteString(portTitleStyle.Render(SafeIcon(IconLink) + pf.label))
	pfContentBuilder.WriteString("\n")

	// Info lines: Inherit foreground from finalPanelStyle.
	// Construct port string from config
	portString := fmt.Sprintf("%s:%s", pf.config.LocalPort, pf.config.RemotePort)
	pfContentBuilder.WriteString(fmt.Sprintf("Port: %s", portString))
	pfContentBuilder.WriteString("\n")

	// Add Service information from config
	serviceName := strings.TrimPrefix(pf.config.ServiceName, "service/")
	pfContentBuilder.WriteString(fmt.Sprintf("Svc: %s", serviceName))
	pfContentBuilder.WriteString("\n")

	// Compact status line
	statusIcon := ""
	if pf.running && pf.err == nil {
		statusIcon = SafeIcon(IconPlay)
	} else if pf.err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") {
		statusIcon = SafeIcon(IconCross)
	} else if strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed") {
		statusIcon = SafeIcon(IconStop)
	} else {
		statusIcon = SafeIcon(IconHourglass) // Initializing, Starting, Restarting, Awaiting Setup
	}

	pfContentBuilder.WriteString(contentFgTextStyle.Render(
		fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(trimmedStatusFromSource)),
	))

	textForPanel := pfContentBuilder.String()

	// --- 4. Calculate actual content width for the panel ---
	// The actual width available for text inside the panel is the targetOuterWidth minus
	// the horizontal space taken by the panel's border and padding (finalPanelStyle.GetHorizontalFrameSize()).
	actualFrameSize := finalPanelStyle.GetHorizontalFrameSize()
	actualContentWidth := targetOuterWidth - actualFrameSize
	if actualContentWidth < 0 {
		actualContentWidth = 0
	}

	// --- 5. Render the text content using the fully configured finalPanelStyle ---
	// finalPanelStyle handles border, padding, background, overall foreground, and content wrapping.
	return finalPanelStyle.Copy().Width(actualContentWidth).Render(textForPanel)
}

// trimStatusMessage trims long status messages to make panels more compact
func trimStatusMessage(status string) string {
	// Shorten "Running (PID: 12345)" to just "Running"
	if strings.HasPrefix(status, "Running (PID:") {
		return "Running"
	}

	// Abbreviate "Forwarding from 127.0.0.1:8080 to pod port 8080"
	if strings.HasPrefix(status, "Forwarding from") {
		return "Forwarding"
	}

	// Keep error messages but limit length
	if len(status) > 15 && (strings.Contains(status, "Error") || strings.Contains(status, "Failed")) {
		return status[:12] + "..."
	}

	return status
}

// renderCombinedLogPanel renders the activity log panel at the bottom of the TUI.
// It displays a capped number of recent log entries from the model's combinedOutput.
// The panel has a title and styles log lines, ensuring they wrap within the available width.
// - m: The current TUI model, used to access the combinedOutput log lines.
// - availableWidth: The target outer width for the log panel.
// - logSectionHeight: The target total outer height for the log panel, including its border and title.
func renderCombinedLogPanel(m *model, availableWidth int, logSectionHeight int) string {
	// Return early if no height available
	if logSectionHeight <= 0 {
		return ""
	}

	// Calculate inner content area dimensions after accounting for border
	borderSize := panelStatusDefaultStyle.GetHorizontalFrameSize()
	innerWidth := availableWidth - borderSize
	if innerWidth < 0 {
		innerWidth = 0
	}

	// Always use the original title for the log panel
	title := SafeIcon(IconScroll) + "Combined Activity Log"

	// Debug information will be added to the log content instead of the title

	titleView := logPanelTitleStyle.Render(title)

	// The viewport is already sized correctly and content set in the main View function
	// Simply get its rendered view
	viewportView := m.mainLogViewport.View()

	// Create inner content by joining title and viewport vertically
	panelContent := lipgloss.JoinVertical(lipgloss.Left, titleView, viewportView)

	// Create a panel with specific styling
	// Make sure we apply NO height limit to the panel
	basePanel := panelStatusDefaultStyle.Copy().
		Width(innerWidth).
		MaxHeight(0). // No max height limit!
		BorderForeground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#A0A0A0"}).
		Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2A2A3A"})

	// Render the panel with our content inside
	renderedPanel := basePanel.Render(panelContent)

	// Calculate the actual height of the rendered panel
	actualHeight := lipgloss.Height(renderedPanel)
	actualWidth := lipgloss.Width(renderedPanel)

	// If we're short on height or width, add padding
	if actualHeight < logSectionHeight || actualWidth < availableWidth {
		// Log discrepancies if in debug mode
		if m.debugMode {
			if actualHeight < logSectionHeight {
				heightDiffMsg := fmt.Sprintf("[Panel height: %d/%d]", actualHeight, logSectionHeight)
				m.combinedOutput = append([]string{heightDiffMsg}, m.combinedOutput...)
			}

			if actualWidth < availableWidth {
				widthDiffMsg := fmt.Sprintf("[Panel width: %d/%d]", actualWidth, availableWidth)
				m.combinedOutput = append([]string{widthDiffMsg}, m.combinedOutput...)
			}
		}

		// Create final wrapped panel with exact dimensions
		finalPanel := lipgloss.NewStyle().
			Width(availableWidth).
			Height(logSectionHeight).
			Align(lipgloss.Left, lipgloss.Top).
			Render(renderedPanel)

		return finalPanel
	}

	return renderedPanel
}

// renderNewConnectionInputView renders the UI when the application is in new connection input mode.
func renderNewConnectionInputView(m model, width int) string {
	var inputPrompt strings.Builder
	inputPrompt.WriteString("Enter new cluster information (ESC to cancel, Enter to confirm/next)\n\n")
	inputPrompt.WriteString(m.newConnectionInput.View()) // Renders the text input bubble
	if m.currentInputStep == mcInputStep {
		inputPrompt.WriteString("\n\n[Input: Management Cluster Name]")
	} else {
		inputPrompt.WriteString(fmt.Sprintf("\n\n[Input: Workload Cluster Name for MC: %s (optional)]", m.stashedMcName))
	}
	inputViewStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).Width(width - 4).Align(lipgloss.Center)
	return inputViewStyle.Render(inputPrompt.String())
}

// renderHeader renders the global header for the TUI.
func renderHeader(m model, contentWidth int) string {
	// Use a simplified header when width is very small
	if contentWidth < 40 {
		headerTitleString := "envctl TUI"
		if m.isLoading {
			headerTitleString = m.spinner.View() + " envctl TUI"
		}

		// Ensure there's no possible negative width when applying style
		headerStyle := headerStyle.Copy().Width(contentWidth)
		return headerStyle.Render(headerTitleString)
	}

	// Regular header with more information
	headerTitleString := "envctl TUI - Press h for Help | Tab to Navigate | q to Quit"

	// Add spinner if loading
	if m.isLoading {
		headerTitleString = m.spinner.View() + " " + headerTitleString
	}

	// Add color mode debug info if debugMode is enabled
	if m.debugMode {
		headerTitleString += fmt.Sprintf(" | Mode: %s | Toggle Dark: D | Debug: z", m.colorMode)
	}

	// Make sure we leave enough space for the header content by not over-subtracting frame size
	headerWidth := contentWidth
	frameSize := headerStyle.GetHorizontalFrameSize()

	if headerWidth <= frameSize {
		// If available width is too small, use minimal style and content
		return "envctl TUI"
	}

	// Otherwise use styled header with full content
	return headerStyle.Copy().
		Width(headerWidth - frameSize).
		Render(headerTitleString)
}

// renderContextPanesRow renders the row containing MC and WC info panes.
func renderContextPanesRow(m model, contentWidth int, maxRowHeight int) string {
	var rowView string
	if m.workloadCluster != "" {
		// Ensure the panes together exactly match contentWidth by accounting for borders
		// First calculate the needed inner widths
		mcPaneStyle := contextPaneStyle
		wcPaneStyle := contextPaneStyle

		mcBorderSize := mcPaneStyle.GetHorizontalFrameSize()
		wcBorderSize := wcPaneStyle.GetHorizontalFrameSize()

		// Subtract border sizes from total width to get distributable content space
		innerWidth := contentWidth - mcBorderSize - wcBorderSize

		// Split the available inner width evenly between MC and WC panes
		mcInnerWidth := innerWidth / 2
		wcInnerWidth := innerWidth - mcInnerWidth // Use remainder for WC to avoid rounding issues

		// The full width includes borders for each pane
		mcPaneWidth := mcInnerWidth + mcBorderSize
		wcPaneWidth := wcInnerWidth + wcBorderSize

		renderedMcPane := renderMcPane(m, mcPaneWidth)
		renderedWcPane := renderWcPane(m, wcPaneWidth)
		rowView = lipgloss.JoinHorizontal(lipgloss.Top, renderedMcPane, renderedWcPane)
	} else {
		// If only MC pane, it should take full width
		rowView = renderMcPane(m, contentWidth)
	}

	// Ensure rowView itself is exactly contentWidth wide, aligning its internal content left.
	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		MaxHeight(maxRowHeight). // Limit height
		Render(rowView)
}

// renderPortForwardingRow renders the row containing port forwarding panels.
func renderPortForwardingRow(m model, contentWidth int, maxRowHeight int) string {
	numFixedColumns := 3 // From original View logic for port forward panels

	// Get panels to show
	pfPanelKeysToShow := []string{}
	for _, key := range m.portForwardOrder {
		if key != mcPaneFocusKey && key != wcPaneFocusKey { // mcPaneFocusKey and wcPaneFocusKey are constants
			pfPanelKeysToShow = append(pfPanelKeysToShow, key)
		}
	}

	// Calculate total border size for all panels
	totalBorderSize := 0
	for i := 0; i < numFixedColumns; i++ {
		if i < len(pfPanelKeysToShow) {
			// Use actual panel style for real panels
			pfKey := pfPanelKeysToShow[i]
			pf := m.portForwards[pfKey]

			var borderStyle lipgloss.Style
			if pf.err != nil || strings.HasPrefix(strings.ToLower(pf.statusMsg), "failed") {
				borderStyle = panelStatusErrorStyle
			} else if pf.running && pf.err == nil {
				borderStyle = panelStatusRunningStyle
			} else {
				borderStyle = panelStatusInitializingStyle
			}

			totalBorderSize += borderStyle.GetHorizontalFrameSize()
		} else {
			// Use default panel style for empty panels
			totalBorderSize += panelStyle.GetHorizontalFrameSize()
		}
	}

	// Calculate distributable inner width
	innerWidth := contentWidth - totalBorderSize
	if innerWidth < 0 {
		innerWidth = 0
	}

	// Base width for each panel's content area
	innerPanelBaseWidth := innerWidth / numFixedColumns

	// Remainder to distribute one extra character per panel from left to right
	innerRemainder := innerWidth % numFixedColumns

	// Render panels with exact widths
	cellsRendered := make([]string, numFixedColumns)

	for i := 0; i < numFixedColumns; i++ {
		// Calculate inner content width for this panel
		innerPanelWidth := innerPanelBaseWidth
		if i < innerRemainder {
			innerPanelWidth++
		}

		// Get the border size for this specific panel
		var panelBorderSize int
		if i < len(pfPanelKeysToShow) {
			pfKey := pfPanelKeysToShow[i]
			pf := m.portForwards[pfKey]

			var borderStyle lipgloss.Style
			if pf.err != nil || strings.HasPrefix(strings.ToLower(pf.statusMsg), "failed") {
				borderStyle = panelStatusErrorStyle
			} else if pf.running && pf.err == nil {
				borderStyle = panelStatusRunningStyle
			} else {
				borderStyle = panelStatusInitializingStyle
			}

			panelBorderSize = borderStyle.GetHorizontalFrameSize()
		} else {
			panelBorderSize = panelStyle.GetHorizontalFrameSize()
		}

		// Calculate the full panel width including its border
		currentPanelWidth := innerPanelWidth + panelBorderSize

		if i < len(pfPanelKeysToShow) {
			pfKey := pfPanelKeysToShow[i]
			pf := m.portForwards[pfKey]
			renderedPfCell := renderPortForwardPanel(pf, m, currentPanelWidth)
			cellsRendered[i] = renderedPfCell
		} else {
			// Render an empty placeholder panel with exact width
			emptyCell := panelStyle.Copy().Width(innerPanelWidth).Render("")
			cellsRendered[i] = emptyCell
		}
	}

	joinedPanels := lipgloss.JoinHorizontal(lipgloss.Top, cellsRendered...)

	// Ensure row is exactly contentWidth wide
	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		MaxHeight(maxRowHeight).
		Render(joinedPanels)
}

// renderMcpProxyPanel renders a single panel for an MCP proxy process.
// It styles the panel based on status (running, error, initializing) and focus (not implemented yet for MCP proxies).
// Displays proxy name, listening port, status message, and PID.
func renderMcpProxyPanel(serverName string, predefinedData mcpserver.PredefinedMcpServer, proc *mcpServerProcess, m model, targetOuterWidth int) string {
	var baseStyleForPanel lipgloss.Style // Focused style not used yet, so no focusedBaseStyleForPanel
	var contentFgTextStyle lipgloss.Style
	statusMsgData := "Not Started"
	pidStr := "PID: N/A"

	if proc != nil {
		statusMsgData = strings.TrimSpace(proc.statusMsg) // Trim status message from data source
		if proc.pid > 0 {
			pidStr = fmt.Sprintf("PID: %d", proc.pid)
		}
		statusToCheck := strings.ToLower(statusMsgData)

		if proc.err != nil || strings.Contains(statusToCheck, "error") || strings.Contains(statusToCheck, "failed") {
			baseStyleForPanel = panelStatusErrorStyle // Reuse port-forward styles for now
			contentFgTextStyle = statusMsgErrorStyle
		} else if strings.Contains(statusToCheck, "running") {
			baseStyleForPanel = panelStatusRunningStyle
			contentFgTextStyle = statusMsgRunningStyle
		} else { // Covers "Initializing...", "Starting...", "Stopped", etc.
			baseStyleForPanel = panelStatusInitializingStyle
			contentFgTextStyle = statusMsgInitializingStyle
		}
	} else { // Process not found in model, treat as not started or an issue
		baseStyleForPanel = panelStatusExitedStyle // Or a new "unknown/not found" style
		contentFgTextStyle = statusMsgExitedStyle
	}

	finalPanelStyle := baseStyleForPanel
	// TODO: Add focus logic if MCP panels become focusable: if m.focusedPanelKey == serverName (or a new key format) ...

	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFgTextStyle.GetForeground())

	var contentBuilder strings.Builder
	trimmedName := strings.TrimSpace(predefinedData.Name) // Trim MCP name from data source
	contentBuilder.WriteString(portTitleStyle.Render(SafeIcon(IconGear) + trimmedName + " MCP")) // e.g. "Kubernetes MCP"
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString(fmt.Sprintf("Port: %d (SSE)", predefinedData.ProxyPort))
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString(pidStr)
	contentBuilder.WriteString("\n")

	statusIcon := ""
	if proc != nil { // Determine icon based on actual proc state
		if proc.err != nil || strings.Contains(strings.ToLower(proc.statusMsg), "error") || strings.Contains(strings.ToLower(proc.statusMsg), "failed") {
			statusIcon = SafeIcon(IconCross)
		} else if strings.Contains(strings.ToLower(proc.statusMsg), "running") {
			statusIcon = SafeIcon(IconPlay)
		} else if strings.Contains(strings.ToLower(proc.statusMsg), "stopped") {
			statusIcon = SafeIcon(IconStop)
		} else {
			statusIcon = SafeIcon(IconHourglass) // Initializing, Starting, Restarting
		}
	} else { // No proc, means not started or issue
		statusIcon = SafeIcon(IconWarning) // Or IconCross if considered an error not to find it
	}

	contentBuilder.WriteString(contentFgTextStyle.Render(fmt.Sprintf("Status: %s%s", statusIcon, trimStatusMessage(statusMsgData))))

	textForPanel := contentBuilder.String()
	actualFrameSize := finalPanelStyle.GetHorizontalFrameSize()
	actualContentWidth := targetOuterWidth - actualFrameSize
	if actualContentWidth < 0 {
		actualContentWidth = 0
	}
	return finalPanelStyle.Copy().Width(actualContentWidth).Render(textForPanel)
}

// renderMcpProxiesRow renders the row containing MCP proxy status panels.
func renderMcpProxiesRow(m model, contentWidth int, maxRowHeight int) string {
	numFixedColumns := 3 // Display up to 3 MCP proxies per row, similar to port forwards
	var cellsRendered []string

	if len(mcpserver.PredefinedMcpServers) == 0 { // Use mcpserver.PredefinedMcpServers
		return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render("No MCP Proxies Defined")
	}

	// Calculate total border size and distribute width (similar to renderPortForwardingRow)
	totalBorderSize := 0
	panelStyles := make([]lipgloss.Style, len(mcpserver.PredefinedMcpServers)) // Use mcpserver.PredefinedMcpServers

	for i, serverDef := range mcpserver.PredefinedMcpServers { // Use mcpserver.PredefinedMcpServers
		proc := m.mcpServers[serverDef.Name]
		statusToCheck := "not started"
		if proc != nil {
			statusToCheck = strings.ToLower(proc.statusMsg)
		}

		var currentStyle lipgloss.Style
		if proc != nil && (proc.err != nil || strings.Contains(statusToCheck, "error") || strings.Contains(statusToCheck, "failed")) {
			currentStyle = panelStatusErrorStyle
		} else if proc != nil && strings.Contains(statusToCheck, "running") {
			currentStyle = panelStatusRunningStyle
		} else {
			currentStyle = panelStatusInitializingStyle // Default/Exited/Initializing
		}
		panelStyles[i] = currentStyle
		if i < numFixedColumns { // Only count borders for panels that will be in the first row if more than numFixedColumns
			totalBorderSize += currentStyle.GetHorizontalFrameSize()
		}
	}

	// Calculate distributable inner width for the displayable columns
	displayableColumnCount := len(mcpserver.PredefinedMcpServers) // Use mcpserver.PredefinedMcpServers
	if displayableColumnCount > numFixedColumns {
		displayableColumnCount = numFixedColumns
		// Recalculate totalBorderSize for only the displayable columns
		totalBorderSize = 0
		for i := 0; i < numFixedColumns; i++ {
			totalBorderSize += panelStyles[i].GetHorizontalFrameSize()
		}
	}

	innerWidth := contentWidth - totalBorderSize
	if innerWidth < 0 {
		innerWidth = 0
	}

	innerPanelBaseWidth := 0
	if displayableColumnCount > 0 {
		innerPanelBaseWidth = innerWidth / displayableColumnCount
	}
	innerRemainder := 0
	if displayableColumnCount > 0 {
		innerRemainder = innerWidth % displayableColumnCount
	}

	for i := 0; i < displayableColumnCount; i++ {
		serverDef := mcpserver.PredefinedMcpServers[i] // Use mcpserver.PredefinedMcpServers
		proc, _ := m.mcpServers[serverDef.Name]        // Ok is false if not found, proc will be nil

		innerPanelWidth := innerPanelBaseWidth
		if i < innerRemainder {
			innerPanelWidth++
		}

		renderedPanel := renderMcpProxyPanel(serverDef.Name, serverDef, proc, m, innerPanelWidth+panelStyles[i].GetHorizontalFrameSize())
		cellsRendered = append(cellsRendered, renderedPanel)
	}

	// If there are fewer predefined servers than columns, fill with empty panels
	for i := len(mcpserver.PredefinedMcpServers); i < numFixedColumns; i++ { // Use mcpserver.PredefinedMcpServers
		innerPanelWidth := innerPanelBaseWidth
		if i < innerRemainder { // This applies if numFixedColumns > len(PredefinedMcpServers)
			innerPanelWidth++
		}
		emptyCell := panelStyle.Copy().Width(innerPanelWidth).Render("")
		cellsRendered = append(cellsRendered, emptyCell)
	}

	joinedPanels := lipgloss.JoinHorizontal(lipgloss.Top, cellsRendered...)

	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		MaxHeight(maxRowHeight).
		Render(joinedPanels)
}

// renderStatusBar constructs the status bar string.
// It includes loading spinner, app status, cluster info, and temporary messages.
func renderStatusBar(m model, width int) string {
	// Determine overall status bar background based on OverallAppStatus
	overallStatus, _ := m.calculateOverallStatus()
	var currentStatusBarBg lipgloss.AdaptiveColor
	switch overallStatus {
	case AppStatusUp:
		currentStatusBarBg = StatusBarSuccessBg
	case AppStatusConnecting:
		currentStatusBarBg = StatusBarInfoBg
	case AppStatusDegraded:
		currentStatusBarBg = StatusBarWarningBg
	case AppStatusFailed:
		currentStatusBarBg = StatusBarErrorBg
	default:
		currentStatusBarBg = StatusBarDefaultBg
	}

	// --- Define Column Widths ---
	// Let's try more fixed proportions for stability, ensuring they sum to width.
	leftColW := int(float64(width) * 0.25)  // Max width for left status/spinner
	rightColW := int(float64(width) * 0.35) // Max width for right MC/WC info
	centerColW := width - leftColW - rightColW
	if centerColW < 0 { // Should not happen if width is reasonable
		centerColW = 0
		// Adjust others if center is squeezed out, e.g. take from widest of left/right
		if leftColW + rightColW > width {
			if rightColW > leftColW {
				rightColW = width - leftColW
			} else {
				leftColW = width - rightColW
			}
		}
	}
	// Ensure sum is exact, giving remainder to largest flexible part (center)
	if leftColW + rightColW + centerColW != width {
		centerColW = width - leftColW - rightColW
	}

	// --- Left Part: Spinner or Overall App Status ---
	var leftFinalStr string
	if m.isLoading {
		// Spinner view. It should ideally be shorter than leftColW.
		// We render it inside a block of leftColW.
		leftFinalStr = lipgloss.NewStyle().Background(currentStatusBarBg).Width(leftColW).Align(lipgloss.Left).Render(m.spinner.View())
	} else {
		icon := ""
		switch overallStatus {
		case AppStatusUp:
			icon = SafeIcon(IconCheck)
		case AppStatusConnecting:
			icon = SafeIcon(IconHourglass)
		case AppStatusDegraded:
			icon = SafeIcon(IconWarning)
		case AppStatusFailed:
			icon = SafeIcon(IconCross)
		case AppStatusUnknown:
			icon = SafeIcon(IconInfo)
		}
		leftContent := icon + overallStatus.String()
		// StatusBarTextStyle has Padding(0,1). The Width(leftColW) will truncate content if too long for the column.
		leftFinalStr = StatusBarTextStyle.Copy().Background(currentStatusBarBg).Width(leftColW).Align(lipgloss.Left).Render(leftContent)
	}

	// --- Right Part: MC/WC Info ---
	mcWcText := fmt.Sprintf("%s MC: %s", SafeIcon(IconKubernetes), m.managementCluster)
	if m.workloadCluster != "" {
		mcWcText += fmt.Sprintf(" / %s WC: %s", SafeIcon(IconKubernetes), m.workloadCluster)
	}
	rightFinalStr := StatusBarTextStyle.Copy().Background(currentStatusBarBg).Width(rightColW).Align(lipgloss.Right).Render(mcWcText)

	// --- Center Part: Transient Status Message ---
	var centerFinalStr string
	rawTransientMessage := m.statusBarMessage
	if rawTransientMessage != "" {
		var msgStyle lipgloss.Style
		var msgIcon string
		switch m.statusBarMessageType {
		case StatusBarSuccess:
			msgStyle = StatusMessageSuccessStyle.Copy()
			msgIcon = SafeIcon(IconSparkles)
		case StatusBarError:
			msgStyle = StatusMessageErrorStyle.Copy()
			msgIcon = SafeIcon(IconCross)
		case StatusBarWarning:
			msgStyle = StatusMessageWarningStyle.Copy()
			msgIcon = SafeIcon(IconLightbulb)
		case StatusBarInfo:
			msgStyle = StatusMessageInfoStyle.Copy()
			msgIcon = SafeIcon(IconInfo)
		default:
			msgStyle = StatusMessageInfoStyle.Copy()
			msgIcon = SafeIcon(IconInfo) // Fallback for any other case
		}
		centerContent := msgIcon + rawTransientMessage
		// Apply background from currentStatusBarBg, width from centerColW, specific text color from msgStyle
		centerFinalStr = msgStyle.Copy().Background(currentStatusBarBg).Width(centerColW).Align(lipgloss.Center).Render(centerContent)
	} else {
		// Render an empty block with the correct background and width to maintain column structure
		centerFinalStr = lipgloss.NewStyle().Background(currentStatusBarBg).Width(centerColW).Render("")
	}

	finalStatusContent := lipgloss.JoinHorizontal(lipgloss.Bottom,
		leftFinalStr,
		centerFinalStr,
		rightFinalStr,
	)

	// The outer style now mainly ensures height and applies an overall width for final clipping if JoinHorizontal misbehaved.
	// Background is effectively handled by the parts.
	return StatusBarBaseStyle.Copy().Width(width).Render(finalStatusContent)
}
