package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

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

// renderHeaderV2 renders the header
func renderHeaderV2(m *model.ModelV2, width int) string {
	// Match v1 exactly
	if width < 40 {
		title := "envctl TUI"
		if m.IsLoading {
			title = m.Spinner.View() + " " + title
		}
		return color.HeaderStyle.Copy().Width(width).Render(title)
	}
	title := "envctl TUI - Press h for Help | Tab to Navigate | q to Quit"
	if m.IsLoading {
		title = m.Spinner.View() + " " + title
	}
	if m.DebugMode {
		title += fmt.Sprintf(" | Mode: %s | Toggle Dark: D | Debug: z", m.ColorMode)
	}
	frame := color.HeaderStyle.GetHorizontalFrameSize()
	if width <= frame {
		return "envctl TUI"
	}
	return color.HeaderStyle.Copy().Width(width - frame).Render(title)
}

// renderCombinedLogPanelV2 renders the combined log panel
func renderCombinedLogPanelV2(m *model.ModelV2, width, height int) string {
	// Match v1 exactly
	if height <= 0 {
		return ""
	}

	border := color.PanelStatusDefaultStyle.GetHorizontalFrameSize()
	innerWidth := width - border
	if innerWidth < 0 {
		innerWidth = 0
	}

	titleView := color.LogPanelTitleStyle.Render(SafeIcon(IconScroll) + "Combined Activity Log")
	viewportView := m.MainLogViewport.View()
	panelContent := lipgloss.JoinVertical(lipgloss.Left, titleView, viewportView)

	base := color.PanelStatusDefaultStyle.Copy().
		Width(innerWidth).
		MaxHeight(0).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#A0A0A0"}).
		Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2A2A3A"})
	rendered := base.Render(panelContent)

	// ensure min size
	if h := lipgloss.Height(rendered); h < height {
		return lipgloss.NewStyle().Width(width).Height(height).Render(rendered)
	}
	return rendered
}

// renderStatusBarV2 renders the status bar
func renderStatusBarV2(m *model.ModelV2, width int) string {
	// Match v1 exactly - need to calculate overall status first
	overallStatus := model.AppStatusUp // Default
	var bg lipgloss.AdaptiveColor

	// For v2, we'll use a simplified status calculation
	// Check if any services are failed
	hasFailures := false
	hasWarnings := false
	isConnecting := false

	for _, k8s := range m.K8sConnections {
		if k8s.State == "failed" {
			hasFailures = true
		} else if k8s.State == "starting" {
			isConnecting = true
		} else if k8s.ReadyNodes < k8s.TotalNodes {
			hasWarnings = true
		}
	}

	for _, pf := range m.PortForwards {
		if pf.State == "failed" {
			hasFailures = true
		} else if pf.State == "starting" {
			isConnecting = true
		}
	}

	for _, mcp := range m.MCPServers {
		if mcp.State == "failed" {
			hasFailures = true
		} else if mcp.State == "starting" {
			isConnecting = true
		}
	}

	if hasFailures {
		overallStatus = model.AppStatusFailed
	} else if isConnecting || m.IsLoading {
		overallStatus = model.AppStatusConnecting
	} else if hasWarnings {
		overallStatus = model.AppStatusDegraded
	}

	switch overallStatus {
	case model.AppStatusUp:
		bg = color.StatusBarSuccessBg
	case model.AppStatusConnecting:
		bg = color.StatusBarInfoBg
	case model.AppStatusDegraded:
		bg = color.StatusBarWarningBg
	case model.AppStatusFailed:
		bg = color.StatusBarErrorBg
	default:
		bg = color.StatusBarDefaultBg
	}

	leftW := int(float64(width) * 0.25)
	rightW := int(float64(width) * 0.35)
	centerW := width - leftW - rightW
	if centerW < 0 {
		centerW = 0
	}

	// left
	var leftStr string
	if m.IsLoading {
		leftStr = lipgloss.NewStyle().Background(bg).Width(leftW).Render(m.Spinner.View())
	} else {
		icon := ""
		switch overallStatus {
		case model.AppStatusUp:
			icon = SafeIcon(IconCheck)
		case model.AppStatusConnecting:
			icon = SafeIcon(IconHourglass)
		case model.AppStatusDegraded:
			icon = SafeIcon(IconWarning)
		case model.AppStatusFailed:
			icon = SafeIcon(IconCross)
		default:
			icon = SafeIcon(IconInfo)
		}
		leftStr = color.StatusBarTextStyle.Copy().Background(bg).Width(leftW).Render(icon + overallStatus.String())
	}

	// right
	mcDisplay := m.ManagementClusterName
	if mcDisplay == "" {
		mcDisplay = "N/A"
	}
	mcWc := fmt.Sprintf("%s MC: %s", SafeIcon(IconKubernetes), mcDisplay)

	if m.WorkloadClusterName != "" {
		wcDisplay := m.WorkloadClusterName
		mcWc += fmt.Sprintf(" / %s WC: %s", SafeIcon(IconKubernetes), wcDisplay)
	}
	rightStr := color.StatusBarTextStyle.Copy().Background(bg).Width(rightW).Align(lipgloss.Right).Render(mcWc)

	// center transient
	var centerStr string
	if m.StatusBarMessage != "" {
		var msgStyle lipgloss.Style
		var icon string
		switch m.StatusBarMessageType {
		case model.StatusBarSuccess:
			msgStyle = color.StatusMessageSuccessStyle.Copy()
			icon = SafeIcon(IconSparkles)
		case model.StatusBarError:
			msgStyle = color.StatusMessageErrorStyle.Copy()
			icon = SafeIcon(IconCross)
		case model.StatusBarWarning:
			msgStyle = color.StatusMessageWarningStyle.Copy()
			icon = SafeIcon(IconLightbulb)
		default:
			msgStyle = color.StatusMessageInfoStyle.Copy()
			icon = SafeIcon(IconInfo)
		}
		centerStr = msgStyle.Background(bg).Width(centerW).Align(lipgloss.Center).Render(icon + m.StatusBarMessage)
	} else {
		centerStr = lipgloss.NewStyle().Background(bg).Width(centerW).Render("")
	}

	final := lipgloss.JoinHorizontal(lipgloss.Bottom, leftStr, centerStr, rightStr)
	return color.StatusBarBaseStyle.Copy().Width(width).Render(final)
}
