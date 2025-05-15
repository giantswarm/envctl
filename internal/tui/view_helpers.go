package tui

import (
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
	mcFullContext := "teleport.giantswarm.io-" + m.managementCluster
	mcActiveString := ""
	isMcActive := m.currentKubeContext == mcFullContext
	if isMcActive {
		mcActiveString = " (Active)"
	}
	mcPaneContent := fmt.Sprintf("MC: %s%s\nCtx: %s", mcFullNameString, mcActiveString, mcFullContext)
	var mcHealthStr string
	if m.MCHealth.IsLoading {
		mcHealthStr = healthLoadingStyle.Render("Nodes: Loading...")
	} else if m.MCHealth.StatusError != nil {
		mcHealthStr = healthErrorStyle.Render(fmt.Sprintf("Nodes: Error (%s)", m.MCHealth.LastUpdated.Format("15:04:05")))
	} else {
		nodeStyleToUse := healthGoodStyle
		if m.MCHealth.ReadyNodes < m.MCHealth.TotalNodes {
			nodeStyleToUse = healthWarnStyle
		}
		mcHealthStr = nodeStyleToUse.Render(fmt.Sprintf("Nodes: %d/%d (%s)", m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes, m.MCHealth.LastUpdated.Format("15:04:05")))
	}
	mcPaneContent += "\n" + mcHealthStr

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

	wcNameString := m.workloadCluster // This is the short WC name, e.g., "deu01"

	// Correctly form the full WC context name for display and active check
	var wcFullContext string
	if m.managementCluster != "" && m.workloadCluster != "" {
		// If m.workloadCluster already contains m.managementCluster (e.g. from initial connect command)
		// we should avoid double prefixing. However, after a new connection, m.workloadCluster is the short name.
		// The fetchNodeStatusCmd and setupPortForwards handle this logic for their needs.
		// For display here, we assume m.managementCluster and m.workloadCluster are the definitive short names.
		wcFullContext = "teleport.giantswarm.io-" + m.managementCluster + "-" + m.workloadCluster
	} else if m.workloadCluster != "" {
		// This case is less ideal, implies MC is empty. If wcNameString somehow has MC prefix, it would be doubled by teleport.
		// However, StartPortForward and GetNodeStatus have their own prefix logic based on what they receive.
		// For display consistency, if MC is empty, just use WC. The 'active' check might be off.
		wcFullContext = "teleport.giantswarm.io-" + wcNameString
	} else {
		wcFullContext = "N/A"
	}

	wcActiveString := ""
	isWcActive := m.currentKubeContext == wcFullContext
	if isWcActive {
		wcActiveString = " (Active)"
	}
	wcPaneContent := fmt.Sprintf("WC: %s%s\nCtx: %s", wcNameString, wcActiveString, wcFullContext)

	var wcHealthStr string
	if m.WCHealth.IsLoading {
		wcHealthStr = healthLoadingStyle.Render("Nodes: Loading...")
	} else if m.WCHealth.StatusError != nil {
		wcHealthStr = healthErrorStyle.Render(fmt.Sprintf("Nodes: Error (%s)", m.WCHealth.LastUpdated.Format("15:04:05")))
	} else {
		nodeStyleToUse := healthGoodStyle
		if m.WCHealth.ReadyNodes < m.WCHealth.TotalNodes {
			nodeStyleToUse = healthWarnStyle
		}
		wcHealthStr = nodeStyleToUse.Render(fmt.Sprintf("Nodes: %d/%d (%s)", m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes, m.WCHealth.LastUpdated.Format("15:04:05")))
	}
	wcPaneContent += "\n" + wcHealthStr

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
	statusToCheck := strings.ToLower(pf.statusMsg)

	if pf.err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed") {
		baseStyleForPanel = panelStatusErrorStyle
		focusedBaseStyleForPanel = focusedPanelStatusErrorStyle
	} else if pf.forwardingEstablished {
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
	} else if pf.forwardingEstablished {
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
	pfContentBuilder.WriteString(portTitleStyle.Render(pf.label))
	pfContentBuilder.WriteString("\n")

	// Info lines: Inherit foreground from finalPanelStyle.
	displayedContext := strings.TrimPrefix(pf.context, "teleport.giantswarm.io-")
	displayedService := strings.TrimPrefix(pf.service, "service/")
	pfContentBuilder.WriteString(fmt.Sprintf("Port: %s\n", pf.port))
	pfContentBuilder.WriteString(fmt.Sprintf("Ctx: %s\n", displayedContext))
	pfContentBuilder.WriteString(fmt.Sprintf("Svc: %s/%s\n", pf.namespace, displayedService))

	// Status line: Explicitly rendered with contentFgTextStyle for emphasis, matching the overall panel text color.
	pfContentBuilder.WriteString(contentFgTextStyle.Render("Status: " + pf.statusMsg))

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

// renderCombinedLogPanel renders the activity log panel at the bottom of the TUI.
// It displays a capped number of recent log entries from the model's combinedOutput.
// The panel has a title and styles log lines, ensuring they wrap within the available width.
// - m: The current TUI model, used to access the combinedOutput log lines.
// - availableWidth: The target outer width for the log panel.
// - logSectionHeight: The target total outer height for the log panel, including its border and title.
func renderCombinedLogPanel(m model, availableWidth int, logSectionHeight int) string {
	var combinedLogContent strings.Builder
	combinedLogContent.WriteString(logPanelTitleStyle.Render("Combined Activity Log") + "\n")

	// Calculate how many lines of logs can be shown in the allocated height
	logContentAreaHeight := logSectionHeight - lipgloss.Height(logPanelTitleStyle.Render("")) - panelStatusDefaultStyle.GetVerticalBorderSize()
	if logContentAreaHeight < 1 {
		logContentAreaHeight = 1
	}

	numLogLinesToShow := logContentAreaHeight
	startIdx := len(m.combinedOutput) - numLogLinesToShow
	if startIdx < 0 {
		startIdx = 0
	}
	displayableLogs := m.combinedOutput[startIdx:]

	// Calculate content width allowing for frame size
	// panelStatusDefaultStyle is used for the log panel's border/padding
	frameSize := panelStatusDefaultStyle.GetHorizontalFrameSize()
	contentWidth := availableWidth - frameSize
	if contentWidth < 0 {
		contentWidth = 0
	}
	
	logLineStyleToUse := logLineStyle.Copy().Width(contentWidth) // Ensure log lines wrap

	for _, line := range displayableLogs {
		// Truncation logic can be removed if Width() handles wrapping well.
		// Lipgloss's Width() on a style applied at render time should handle wrapping.
		combinedLogContent.WriteString(logLineStyleToUse.Render(line) + "\n")
	}
	
	// Apply panel style with the exact width and height
	// availableWidth is the target total outer width for the log panel.
	// panelStatusDefaultStyle is used for the log panel, calculate its frame.
	logPanelFrameSize := panelStatusDefaultStyle.GetHorizontalFrameSize()
	actualLogPanelContentWidth := availableWidth - logPanelFrameSize
	if actualLogPanelContentWidth < 0 {
		actualLogPanelContentWidth = 0
	}
	
	logPanelStyle := panelStatusDefaultStyle.Copy().
		Width(actualLogPanelContentWidth). // Set content width
		Height(logSectionHeight)
		
	// The content string is already styled per line.
	// logPanelStyle's job is to add border/padding around this.
	// If logPanelStyle had a background, it would apply.
	return logPanelStyle.Render(strings.TrimRight(combinedLogContent.String(), "\n"))
}
