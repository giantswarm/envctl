package view

import (
	"envctl/internal/api"
	"envctl/internal/color"
	"envctl/internal/kube"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

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

	// Determine panel style based on state (like port forwards and MCP servers)
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	var contentFg lipgloss.Style

	if conn != nil {
		stateLower := strings.ToLower(conn.State)
		switch {
		case stateLower == "failed":
			baseStyleForPanel = color.PanelStatusErrorStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
			contentFg = color.StatusMsgErrorStyle
		case stateLower == "running":
			baseStyleForPanel = color.PanelStatusRunningStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
			contentFg = color.StatusMsgRunningStyle
		case stateLower == "starting":
			baseStyleForPanel = color.PanelStatusInitializingStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		case stateLower == "stopped" || stateLower == "stopping":
			baseStyleForPanel = color.PanelStatusExitedStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
			contentFg = color.StatusMsgExitedStyle
		default:
			baseStyleForPanel = color.PanelStatusInitializingStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		}
	} else {
		// No connection info yet
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
		contentFg = color.StatusMsgInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if m.FocusedPanelKey == model.McPaneFocusKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	// Override foreground color for content
	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	// Build content
	var b strings.Builder

	// Title
	activeStr := ""
	if isMcActive {
		activeStr = " (Active)"
	}
	b.WriteString(color.PortTitleStyle.Render(fmt.Sprintf("%sMC: %s%s", SafeIcon(IconKubernetes), mcName, activeStr)))
	b.WriteString("\n")

	// Status line (like port forwards)
	var statusIcon string
	var statusText string
	if conn != nil {
		stateLower := strings.ToLower(conn.State)
		statusText = strings.TrimSpace(conn.State)
		switch {
		case stateLower == "running":
			statusIcon = SafeIcon(IconPlay)
		case stateLower == "failed":
			statusIcon = SafeIcon(IconCross)
		case stateLower == "starting":
			statusIcon = SafeIcon(IconHourglass)
		case stateLower == "stopped" || stateLower == "stopping":
			statusIcon = SafeIcon(IconStop)
		default:
			statusIcon = SafeIcon(IconHourglass)
		}
	} else {
		statusIcon = SafeIcon(IconHourglass)
		statusText = "Initializing"
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, statusText)))
	b.WriteString("\n")

	// Health line (like port forwards)
	var healthIcon, healthText string
	if conn != nil {
		stateLower := strings.ToLower(conn.State)
		healthLower := strings.ToLower(conn.Health)
		if stateLower == "running" && healthLower == "healthy" {
			healthIcon = SafeIcon(IconCheck)
			healthText = "Healthy"
		} else if stateLower == "failed" || healthLower == "unhealthy" {
			healthIcon = SafeIcon(IconCross)
			healthText = "Unhealthy"
		} else {
			healthIcon = SafeIcon(IconHourglass)
			healthText = "Checking..."
		}
	} else {
		healthIcon = SafeIcon(IconHourglass)
		healthText = "Checking..."
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))

	// Add version if available
	if conn != nil && conn.Version != "" {
		b.WriteString("\n")
		b.WriteString(contentFg.Render(fmt.Sprintf("Version: %s", conn.Version)))
	}

	// Add node status if available
	if conn != nil && conn.State == "Running" {
		b.WriteString("\n")
		nodeIcon := IconCheck
		if conn.ReadyNodes < conn.TotalNodes {
			nodeIcon = IconWarning
		}
		b.WriteString(contentFg.Render(fmt.Sprintf("Nodes: %s%d/%d", SafeIcon(nodeIcon), conn.ReadyNodes, conn.TotalNodes)))
	}

	frame := finalPanelStyle.GetHorizontalFrameSize()
	contentWidth := paneWidth - frame
	if contentWidth < 0 {
		contentWidth = 0
	}
	return finalPanelStyle.Copy().Width(contentWidth).Render(b.String())
}

// renderWcPane(renders the workload cluster pane
func renderWcPane(m *model.Model, conn *api.K8sConnectionInfo, paneWidth int) string {
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

	// Determine panel style based on state (like port forwards and MCP servers)
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	var contentFg lipgloss.Style

	if conn != nil {
		stateLower := strings.ToLower(conn.State)
		switch {
		case stateLower == "failed":
			baseStyleForPanel = color.PanelStatusErrorStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
			contentFg = color.StatusMsgErrorStyle
		case stateLower == "running":
			baseStyleForPanel = color.PanelStatusRunningStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
			contentFg = color.StatusMsgRunningStyle
		case stateLower == "starting":
			baseStyleForPanel = color.PanelStatusInitializingStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		case stateLower == "stopped" || stateLower == "stopping":
			baseStyleForPanel = color.PanelStatusExitedStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
			contentFg = color.StatusMsgExitedStyle
		default:
			baseStyleForPanel = color.PanelStatusInitializingStyle
			focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
			contentFg = color.StatusMsgInitializingStyle
		}
	} else {
		// No connection info yet
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
		contentFg = color.StatusMsgInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if m.FocusedPanelKey == model.WcPaneFocusKey {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	// Override foreground color for content
	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	// Build content
	var b strings.Builder

	// Title
	activeStr := ""
	if isWcActive {
		activeStr = " (Active)"
	}
	b.WriteString(color.PortTitleStyle.Render(fmt.Sprintf("%sWC: %s%s", SafeIcon(IconKubernetes), wcName, activeStr)))
	b.WriteString("\n")

	// Status line (like port forwards)
	var statusIcon string
	var statusText string
	if conn != nil {
		stateLower := strings.ToLower(conn.State)
		statusText = strings.TrimSpace(conn.State)
		switch {
		case stateLower == "running":
			statusIcon = SafeIcon(IconPlay)
		case stateLower == "failed":
			statusIcon = SafeIcon(IconCross)
		case stateLower == "starting":
			statusIcon = SafeIcon(IconHourglass)
		case stateLower == "stopped" || stateLower == "stopping":
			statusIcon = SafeIcon(IconStop)
		default:
			statusIcon = SafeIcon(IconHourglass)
		}
	} else {
		statusIcon = SafeIcon(IconHourglass)
		statusText = "Initializing"
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, statusText)))
	b.WriteString("\n")

	// Health line (like port forwards)
	var healthIcon, healthText string
	if conn != nil {
		stateLower := strings.ToLower(conn.State)
		healthLower := strings.ToLower(conn.Health)
		if stateLower == "running" && healthLower == "healthy" {
			healthIcon = SafeIcon(IconCheck)
			healthText = "Healthy"
		} else if stateLower == "failed" || healthLower == "unhealthy" {
			healthIcon = SafeIcon(IconCross)
			healthText = "Unhealthy"
		} else {
			healthIcon = SafeIcon(IconHourglass)
			healthText = "Checking..."
		}
	} else {
		healthIcon = SafeIcon(IconHourglass)
		healthText = "Checking..."
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))

	// Add version if available
	if conn != nil && conn.Version != "" {
		b.WriteString("\n")
		b.WriteString(contentFg.Render(fmt.Sprintf("Version: %s", conn.Version)))
	}

	// Add node status if available
	if conn != nil && conn.State == "Running" {
		b.WriteString("\n")
		nodeIcon := IconCheck
		if conn.ReadyNodes < conn.TotalNodes {
			nodeIcon = IconWarning
		}
		b.WriteString(contentFg.Render(fmt.Sprintf("Nodes: %s%d/%d", SafeIcon(nodeIcon), conn.ReadyNodes, conn.TotalNodes)))
	}

	frame := finalPanelStyle.GetHorizontalFrameSize()
	contentWidth := paneWidth - frame
	if contentWidth < 0 {
		contentWidth = 0
	}
	return finalPanelStyle.Copy().Width(contentWidth).Render(b.String())
}
