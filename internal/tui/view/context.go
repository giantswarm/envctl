package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"envctl/internal/utils"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// renderMcPane renders the Management Cluster (MC) information panel.
func renderMcPane(m *model.Model, paneWidth int) string {
	// controller.LogDebug is not callable from view. Logging from view should be minimized or avoided.
	// fmt.Printf("[ViewDebug][renderMcPane] Comparing for Active: currentKubeContext ('%s') vs built MC context ('%s') for mcName ('%s')\n",
	// 	m.CurrentKubeContext, utils.BuildMcContext(m.ManagementClusterName), m.ManagementClusterName)

	mcName := m.ManagementClusterName
	if mcName == "" {
		mcName = "N/A"
	}

	isMcActive := false
	if m.ManagementClusterName != "" && m.CurrentKubeContext == utils.BuildMcContext(m.ManagementClusterName) {
		isMcActive = true
	}

	mcActiveString := ""
	if isMcActive {
		mcActiveString = " (Active)"
	}

	mcPaneContent := fmt.Sprintf("%sMC: %s%s", SafeIcon(IconKubernetes), mcName, mcActiveString)

	var healthStatusText string
	var healthStyle lipgloss.Style

	if m.MCHealth.IsLoading {
		healthStatusText = RenderIconWithMessage(IconHourglass, "Nodes: Loading...")
		healthStyle = color.HealthLoadingStyle
	} else if m.MCHealth.StatusError != nil {
		// Ensure m.MCHealth.LastUpdated is accessible and exported from model.ClusterHealthInfo
		healthStatusText = RenderIconWithMessage(IconCross, fmt.Sprintf("Nodes: Error (%s)", m.MCHealth.LastUpdated.Format("15:04:05")))
		healthStyle = color.HealthErrorStyle
	} else {
		healthIcon := IconCheck
		if m.MCHealth.ReadyNodes < m.MCHealth.TotalNodes {
			healthIcon = IconWarning
			healthStatusText = RenderIconWithNodes(healthIcon, m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes, "[WARN]")
			healthStyle = color.HealthWarnStyle
		} else {
			healthStatusText = RenderIconWithNodes(healthIcon, m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes, "")
			healthStyle = color.HealthGoodStyle
		}
	}

	renderedHealthText := healthStyle.Render(healthStatusText)
	mcPaneContent += "\n" + renderedHealthText

	mcPaneStyleToUse := color.ContextPaneStyle
	if isMcActive {
		mcPaneStyleToUse = color.ActiveContextPaneStyle
	}
	if m.FocusedPanelKey == model.McPaneFocusKey { // Use model.McPaneFocusKey
		if isMcActive {
			mcPaneStyleToUse = color.FocusedAndActiveContextPaneStyle
		} else {
			mcPaneStyleToUse = color.FocusedContextPaneStyle
		}
	}
	return mcPaneStyleToUse.Copy().Width(paneWidth - mcPaneStyleToUse.GetHorizontalFrameSize()).Render(mcPaneContent)
}

// renderWcPane renders the Workload Cluster (WC) information panel.
func renderWcPane(m *model.Model, paneWidth int) string {
	if m.WorkloadClusterName == "" {
		return ""
	}

	// fmt.Printf("[ViewDebug][renderWcPane] Comparing for Active: currentKubeContext ('%s') vs built WC context ('%s') for mcName ('%s'), wcName ('%s')\n",
	// 	m.CurrentKubeContext, utils.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName), m.ManagementClusterName, m.WorkloadClusterName)

	wcName := m.WorkloadClusterName
	isWcActive := false
	if m.ManagementClusterName != "" && m.WorkloadClusterName != "" &&
		m.CurrentKubeContext == utils.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName) {
		isWcActive = true
	}

	wcActiveString := ""
	if isWcActive {
		wcActiveString = " (Active)"
	}

	wcPaneContent := fmt.Sprintf("%sWC: %s%s", SafeIcon(IconKubernetes), wcName, wcActiveString)

	var healthStatusText string
	var healthStyle lipgloss.Style

	if m.WCHealth.IsLoading {
		healthStatusText = RenderIconWithMessage(IconHourglass, "Nodes: Loading...")
		healthStyle = color.HealthLoadingStyle
	} else if m.WCHealth.StatusError != nil {
		healthStatusText = RenderIconWithMessage(IconCross, "Nodes: Error")
		healthStyle = color.HealthErrorStyle
	} else {
		healthIcon := IconCheck
		if m.WCHealth.ReadyNodes < m.WCHealth.TotalNodes {
			healthIcon = IconWarning
			healthStatusText = RenderIconWithNodes(healthIcon, m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes, "[WARN]")
			healthStyle = color.HealthWarnStyle
		} else {
			healthStatusText = RenderIconWithNodes(healthIcon, m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes, "")
			healthStyle = color.HealthGoodStyle
		}
	}

	renderedHealthText := healthStyle.Render(healthStatusText)
	wcPaneContent += "\n" + renderedHealthText

	wcPaneStyleToRender := color.ContextPaneStyle
	if isWcActive {
		wcPaneStyleToRender = color.ActiveContextPaneStyle
	}
	if m.FocusedPanelKey == model.WcPaneFocusKey { // Use model.WcPaneFocusKey
		if isWcActive {
			wcPaneStyleToRender = color.FocusedAndActiveContextPaneStyle
		} else {
			wcPaneStyleToRender = color.FocusedContextPaneStyle
		}
	}
	return wcPaneStyleToRender.Copy().Width(paneWidth - wcPaneStyleToRender.GetHorizontalFrameSize()).Render(wcPaneContent)
}

// renderContextPanesRow joins MC and WC panes.
func renderContextPanesRow(m *model.Model, contentWidth int, maxRowHeight int) string {
	var rowView string
	if m.WorkloadClusterName != "" {
		mcBorder := color.ContextPaneStyle.GetHorizontalFrameSize()
		wcBorder := color.ContextPaneStyle.GetHorizontalFrameSize()
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
