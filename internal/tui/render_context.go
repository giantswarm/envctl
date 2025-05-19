package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderMcPane renders the Management Cluster (MC) information panel.
// (moved verbatim from view_helpers.go)
func renderMcPane(m model, paneWidth int) string {
    mcFullNameString := m.managementCluster
    if mcFullNameString == "" {
        mcFullNameString = "N/A"
    }
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
// (moved verbatim from view_helpers.go)
func renderWcPane(m model, paneWidth int) string {
    if m.workloadCluster == "" {
        return ""
    }

    wcNameString := m.workloadCluster
    wcClusterIdentifier := m.getWorkloadClusterContextIdentifier()
    var wcFullContext string
    if wcClusterIdentifier != "" {
        wcFullContext = "teleport.giantswarm.io-" + wcClusterIdentifier
    } else {
        wcFullContext = "N/A"
    }

    wcActiveString := ""
    isWcActive := m.currentKubeContext == wcFullContext && wcClusterIdentifier != ""
    if isWcActive {
        wcActiveString = " (Active)"
    }

    shortContext := wcClusterIdentifier
    if wcFullContext == "N/A" {
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
    if m.workloadCluster != "" {
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