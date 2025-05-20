package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"envctl/internal/utils"
)

// renderMcPane renders the Management Cluster (MC) information panel.
func renderMcPane(m model, paneWidth int) string {
	m.LogDebug("[renderMcPane] Comparing for Active: currentKubeContext ('%s') vs built MC context ('%s') for mcName ('%s')", 
		m.currentKubeContext, utils.BuildMcContext(m.managementClusterName), m.managementClusterName)

	mcName := m.managementClusterName
	if mcName == "" {
		mcName = "N/A"
	}

	isMcActive := false
	if m.managementClusterName != "" && m.currentKubeContext == utils.BuildMcContext(m.managementClusterName) {
		isMcActive = true
	}

	mcActiveString := ""
	if isMcActive {
		mcActiveString = " (Active)"
	}
	
	// Removed "Ctx:" line
	mcPaneContent := fmt.Sprintf("%sMC: %s%s", SafeIcon(IconKubernetes), mcName, mcActiveString)

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
func renderWcPane(m model, paneWidth int) string {
	if m.workloadClusterName == "" { 
		return "" 
	}

	m.LogDebug("[renderWcPane] Comparing for Active: currentKubeContext ('%s') vs built WC context ('%s') for mcName ('%s'), wcName ('%s')",
		m.currentKubeContext, utils.BuildWcContext(m.managementClusterName, m.workloadClusterName), m.managementClusterName, m.workloadClusterName)
	
	wcName := m.workloadClusterName 
	isWcActive := false
	if m.managementClusterName != "" && m.workloadClusterName != "" && 
	   m.currentKubeContext == utils.BuildWcContext(m.managementClusterName, m.workloadClusterName) {
		isWcActive = true
	}

	wcActiveString := ""
	if isWcActive {
		wcActiveString = " (Active)"
	}

	// Removed "Ctx:" line
	wcPaneContent := fmt.Sprintf("%sWC: %s%s", SafeIcon(IconKubernetes), wcName, wcActiveString)

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

// renderContextPanesRow joins MC and WC panes.
func renderContextPanesRow(m model, contentWidth int, maxRowHeight int) string {
	var rowView string
	if m.workloadClusterName != "" { // Use new name
		mcBorder := contextPaneStyle.GetHorizontalFrameSize()
		wcBorder := contextPaneStyle.GetHorizontalFrameSize()
		innerWidth := contentWidth - mcBorder - wcBorder
		mcInner := innerWidth / 2
		wcInner := innerWidth - mcInner
		mcPane := renderMcPane(m, mcInner+mcBorder)
		wcPane := renderWcPane(m, wcInner+wcBorder)
		rowView = lipgloss.JoinHorizontal(lipgloss.Top, mcPane, wcPane)
	} else {
		rowView = renderMcPane(m, contentWidth)
	}

	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		MaxHeight(maxRowHeight).
		Render(rowView)
} 