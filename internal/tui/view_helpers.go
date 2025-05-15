package tui

import (
	"fmt"
	"strings"

	// For time.Format
	"github.com/charmbracelet/lipgloss"
)

// Will likely be needed for formatting LastUpdated times

// This file will contain helper functions for the View() method in model.go

// renderMcPane renders the Management Cluster information pane.
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

// renderWcPane renders the Workload Cluster information pane if a WC is defined.
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

// renderPortForwardPanel renders a single port-forward panel.
func renderPortForwardPanel(pf *portForwardProcess, m model, panelOuterWidth int, panelContentWidth int) string {
	var pfContent strings.Builder

	pfContent.WriteString(portTitleStyle.Render(pf.label) + "\n")
	pfContent.WriteString(fmt.Sprintf("Port: %s | Ctx: %s\nSvc: %s/%s\n", pf.port, pf.context, pf.namespace, pf.service))

	currentStatusMsgText := pf.statusMsg
	currentStatusTextStyle := statusStyle // Default

	// Determine text style based on status
	if pf.err != nil || strings.HasPrefix(pf.statusMsg, "Failed") || strings.HasPrefix(pf.statusMsg, "Error") || strings.HasPrefix(pf.statusMsg, "Restart failed") {
		currentStatusTextStyle = errorStyle
	} else if pf.forwardingEstablished {
		currentStatusTextStyle = statusMsgRunningStyle
	} else if strings.HasPrefix(pf.statusMsg, "Exited") || strings.HasPrefix(pf.statusMsg, "Killed") {
		currentStatusTextStyle = statusMsgExitedStyle
	} else if strings.HasPrefix(pf.statusMsg, "Running (PID:") {
		// This was a bit ambiguous before, let's use Initializing for "Running (PID:" as it's pre-Forwarding Established
		currentStatusTextStyle = statusMsgInitializingStyle
	} else { // Covers "Initializing...", "Starting...", "Restarting..."
		currentStatusTextStyle = statusMsgInitializingStyle
	}
	pfContent.WriteString(currentStatusTextStyle.Render("Status: "+currentStatusMsgText) + "\n")

	pfContent.WriteString(lipgloss.NewStyle().Bold(true).Render("Last output:") + "\n")
	effectiveLogLineWidth := panelContentWidth
	if effectiveLogLineWidth <= 0 {
		effectiveLogLineWidth = 1 // Ensure it's at least 1 to avoid panic with slicing
	}

	for i, line := range pf.output {
		displayLine := line
		if len(line) > effectiveLogLineWidth {
			if effectiveLogLineWidth > 3 {
				displayLine = line[:effectiveLogLineWidth-3] + "..."
			} else {
				displayLine = line[:effectiveLogLineWidth] // Truncate directly if not enough space for ellipsis
			}
		}
		pfContent.WriteString(logLineStyle.Render("  " + displayLine))
		if i < len(pf.output)-1 {
			pfContent.WriteString("\n")
		}
	}

	// Determine panel background style based on status
	var currentBasePanelStyle, currentFocusedBasePanelStyle lipgloss.Style
	statusToCheck := strings.ToLower(pf.statusMsg)

	if pf.err != nil || strings.HasPrefix(statusToCheck, "failed") || strings.HasPrefix(statusToCheck, "error") || strings.HasPrefix(statusToCheck, "restart failed") {
		currentBasePanelStyle = panelStatusErrorStyle
		currentFocusedBasePanelStyle = focusedPanelStatusErrorStyle
	} else if pf.forwardingEstablished {
		currentBasePanelStyle = panelStatusRunningStyle
		currentFocusedBasePanelStyle = focusedPanelStatusRunningStyle
	} else if strings.HasPrefix(statusToCheck, "running (pid:") { // Indicates process is running but not yet fully established, or just started
		currentBasePanelStyle = panelStatusAttemptingStyle
		currentFocusedBasePanelStyle = focusedPanelStatusAttemptingStyle
	} else if strings.HasPrefix(statusToCheck, "exited") || strings.HasPrefix(statusToCheck, "killed") {
		currentBasePanelStyle = panelStatusExitedStyle
		currentFocusedBasePanelStyle = focusedPanelStatusExitedStyle
	} else if strings.HasPrefix(statusToCheck, "initializing") || strings.HasPrefix(statusToCheck, "starting") || strings.HasPrefix(statusToCheck, "restarting") {
		currentBasePanelStyle = panelStatusInitializingStyle
		currentFocusedBasePanelStyle = focusedPanelStatusInitializingStyle
	} else {
		currentBasePanelStyle = panelStatusDefaultStyle
		currentFocusedBasePanelStyle = focusedPanelStatusDefaultStyle
	}

	finalPanelStyleToUse := currentBasePanelStyle
	if pf.label == m.focusedPanelKey {
		finalPanelStyleToUse = currentFocusedBasePanelStyle
	}

	// Calculate the target width for the content area of the panel
	contentWidthForStyle := panelOuterWidth - finalPanelStyleToUse.GetHorizontalFrameSize()
	if contentWidthForStyle < 0 {
		contentWidthForStyle = 0
	}

	// 1. Pre-process the text to ensure it conforms to contentWidthForStyle (truncate/wrap long lines).
	// This style does not need background or other visual properties, only width.
	textProcessingStyle := lipgloss.NewStyle().Width(contentWidthForStyle)
	processedText := textProcessingStyle.Render(pfContent.String())

	// 2. Apply the final panel style (border, padding, background) AND set its content width.
	// This style is now responsible for ensuring the content area is `contentWidthForStyle` wide
	// (hopefully by space-padding `processedText` if it's narrower) and then adding its frame.
	finalPanelStyleToUse = finalPanelStyleToUse.Copy().Width(contentWidthForStyle)

	return finalPanelStyleToUse.Render(processedText)
}

// renderCombinedLogPanel renders the combined activity log panel.
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

	// Ensure log lines fit within the panel width
	availableWidthForLog := availableWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()
	if availableWidthForLog <= 0 {
		availableWidthForLog = 1
	}

	for _, line := range displayableLogs {
		displayLine := line
		if len(line) > availableWidthForLog {
			if availableWidthForLog > 3 {
				displayLine = line[:availableWidthForLog-3] + "..."
			} else {
				displayLine = line[:availableWidthForLog]
			}
		}
		combinedLogContent.WriteString(logLineStyle.Render(displayLine) + "\n")
	}

	combinedLogPanelStyle := panelStatusDefaultStyle.Copy().Width(availableWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()).Height(logSectionHeight)
	return combinedLogPanelStyle.Render(strings.TrimRight(combinedLogContent.String(), "\n"))
}
