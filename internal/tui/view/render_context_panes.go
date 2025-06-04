package view

import (
	"envctl/internal/api"
	"envctl/internal/color"
	"envctl/internal/kube"
	"envctl/internal/tui/model"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderContextPanesRow(renders the context panes row
func renderContextPanesRow(m *model.Model, width, maxHeight int) string {
	// Match v1 exactly
	var rowView string
	if m.WorkloadClusterName != "" {
		mcBorder := color.ContextPaneStyle.GetHorizontalFrameSize()
		wcBorder := color.ContextPaneStyle.GetHorizontalFrameSize()
		innerWidth := width - mcBorder - wcBorder
		mcInner := innerWidth / 2
		wcInner := innerWidth - mcInner
		mcPane := renderMcPane(m, nil, mcInner+mcBorder)
		wcPane := renderWcPane(m, nil, wcInner+wcBorder)
		rowView = lipgloss.JoinHorizontal(lipgloss.Top, mcPane, wcPane)
	} else {
		rowView = renderMcPane(m, nil, width)
	}

	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		MaxHeight(maxHeight).
		Render(rowView)
}

// renderMcPane(renders the management cluster pane
func renderMcPane(m *model.Model, conn *api.K8sConnectionInfo, paneWidth int) string {
	// Match v1 exactly
	mcName := m.ManagementClusterName
	if mcName == "" {
		mcName = "N/A"
	}

	// Need to get kube manager to build context name
	kubeMgr := kube.NewManager(nil)
	isMcActive := false
	if m.ManagementClusterName != "" && m.CurrentKubeContext == kubeMgr.BuildMcContextName(m.ManagementClusterName) {
		isMcActive = true
	}

	mcActiveString := ""
	if isMcActive {
		mcActiveString = " (Active)"
	}

	mcPaneContent := fmt.Sprintf("%sMC: %s%s", SafeIcon(IconKubernetes), mcName, mcActiveString)

	var healthStatusText string
	var healthStyle lipgloss.Style

	// Get connection info if not passed
	if conn == nil {
		expectedLabel := fmt.Sprintf("mc-%s", m.ManagementClusterName)
		for label, c := range m.K8sConnections {
			if label == expectedLabel {
				conn = c
				break
			}
		}
	}

	if conn == nil || conn.State == "starting" {
		healthStatusText = RenderIconWithMessage(IconHourglass, "Nodes: Loading...")
		healthStyle = color.HealthLoadingStyle
	} else if conn.State == "failed" || conn.Error != "" {
		healthStatusText = RenderIconWithMessage(IconCross, fmt.Sprintf("Nodes: Error (%s)", time.Now().Format("15:04:05")))
		healthStyle = color.HealthErrorStyle
	} else {
		healthIcon := IconCheck
		if conn.ReadyNodes < conn.TotalNodes {
			healthIcon = IconWarning
			healthStatusText = RenderIconWithNodes(healthIcon, conn.ReadyNodes, conn.TotalNodes, "[WARN]")
			healthStyle = color.HealthWarnStyle
		} else {
			healthStatusText = RenderIconWithNodes(healthIcon, conn.ReadyNodes, conn.TotalNodes, "")
			healthStyle = color.HealthGoodStyle
		}
	}

	renderedHealthText := healthStyle.Render(healthStatusText)
	mcPaneContent += "\n" + renderedHealthText

	mcPaneStyleToUse := color.ContextPaneStyle
	if isMcActive {
		mcPaneStyleToUse = color.ActiveContextPaneStyle
	}
	if m.FocusedPanelKey == model.McPaneFocusKey {
		if isMcActive {
			mcPaneStyleToUse = color.FocusedAndActiveContextPaneStyle
		} else {
			mcPaneStyleToUse = color.FocusedContextPaneStyle
		}
	}
	return mcPaneStyleToUse.Copy().Width(paneWidth - mcPaneStyleToUse.GetHorizontalFrameSize()).Render(mcPaneContent)
}

// renderWcPane(renders the workload cluster pane
func renderWcPane(m *model.Model, conn *api.K8sConnectionInfo, paneWidth int) string {
	// Match v1 exactly
	if m.WorkloadClusterName == "" {
		return ""
	}

	wcName := m.WorkloadClusterName
	kubeMgr := kube.NewManager(nil)
	isWcActive := false
	if m.ManagementClusterName != "" && m.WorkloadClusterName != "" &&
		m.CurrentKubeContext == kubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName) {
		isWcActive = true
	}

	wcActiveString := ""
	if isWcActive {
		wcActiveString = " (Active)"
	}

	wcPaneContent := fmt.Sprintf("%sWC: %s%s", SafeIcon(IconKubernetes), wcName, wcActiveString)

	var healthStatusText string
	var healthStyle lipgloss.Style

	// Get connection info if not passed
	if conn == nil {
		expectedLabel := fmt.Sprintf("wc-%s", m.WorkloadClusterName)
		for label, c := range m.K8sConnections {
			if label == expectedLabel {
				conn = c
				break
			}
		}
	}

	if conn == nil || conn.State == "starting" {
		healthStatusText = RenderIconWithMessage(IconHourglass, "Nodes: Loading...")
		healthStyle = color.HealthLoadingStyle
	} else if conn.State == "failed" || conn.Error != "" {
		healthStatusText = RenderIconWithMessage(IconCross, "Nodes: Error")
		healthStyle = color.HealthErrorStyle
	} else {
		healthIcon := IconCheck
		if conn.ReadyNodes < conn.TotalNodes {
			healthIcon = IconWarning
			healthStatusText = RenderIconWithNodes(healthIcon, conn.ReadyNodes, conn.TotalNodes, "[WARN]")
			healthStyle = color.HealthWarnStyle
		} else {
			healthStatusText = RenderIconWithNodes(healthIcon, conn.ReadyNodes, conn.TotalNodes, "")
			healthStyle = color.HealthGoodStyle
		}
	}

	renderedHealthText := healthStyle.Render(healthStatusText)
	wcPaneContent += "\n" + renderedHealthText

	wcPaneStyleToRender := color.ContextPaneStyle
	if isWcActive {
		wcPaneStyleToRender = color.ActiveContextPaneStyle
	}
	if m.FocusedPanelKey == model.WcPaneFocusKey {
		if isWcActive {
			wcPaneStyleToRender = color.FocusedAndActiveContextPaneStyle
		} else {
			wcPaneStyleToRender = color.FocusedContextPaneStyle
		}
	}
	return wcPaneStyleToRender.Copy().Width(paneWidth - wcPaneStyleToRender.GetHorizontalFrameSize()).Render(wcPaneContent)
}
