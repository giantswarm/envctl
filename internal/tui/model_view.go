package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI according to the current model state.
func (m model) View() string {
	switch m.currentAppMode {
	case ModeQuitting:
		return statusStyle.Render(m.quittingMessage)
	case ModeInitializing:
		if m.width == 0 || m.height == 0 {
			return statusStyle.Render("Initializing... (waiting for window size)")
		}
		return statusStyle.Render("Initializing...")
	case ModeMainDashboard:
		contentWidth := m.width - appStyle.GetHorizontalFrameSize()
		totalAvailableHeight := m.height - appStyle.GetVerticalFrameSize()

		headerView := renderHeader(m, contentWidth)
		headerHeight := lipgloss.Height(headerView)

		maxRow1Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
		if maxRow1Height < 5 {
			maxRow1Height = 5
		} else if maxRow1Height > 7 {
			maxRow1Height = 7
		}
		row1View := renderContextPanesRow(m, contentWidth, maxRow1Height)
		row1Height := lipgloss.Height(row1View)

		maxRow2Height := int(float64(totalAvailableHeight-headerHeight) * 0.30)
		if maxRow2Height < 7 {
			maxRow2Height = 7
		} else if maxRow2Height > 9 {
			maxRow2Height = 9
		}
		row2View := renderPortForwardingRow(m, contentWidth, maxRow2Height)
		row2Height := lipgloss.Height(row2View)

		maxRow3Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
		if maxRow3Height < 5 {
			maxRow3Height = 5
		}
		row3View := renderMcpProxiesRow(m, contentWidth, maxRow3Height)
		row3Height := lipgloss.Height(row3View)

		logPanelView := ""
		var logSectionHeight int
		if m.height >= minHeightForMainLogView {
			numGaps := 4
			heightConsumed := headerHeight + row1Height + row2Height + row3Height + numGaps
			logSectionHeight = totalAvailableHeight - heightConsumed
			if logSectionHeight < 0 {
				logSectionHeight = 0
			}

			m.mainLogViewport.Width = contentWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()
			m.mainLogViewport.Height = logSectionHeight - panelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(logPanelTitleStyle.Render(" ")) - 1
			if m.mainLogViewport.Height < 0 {
				m.mainLogViewport.Height = 0
			}

			// Refresh content only when new lines arrived or width changed.
			mlvWidthChanged := m.mainLogViewportLastWidth != m.mainLogViewport.Width
			if m.activityLogDirty || mlvWidthChanged {
				trunc := prepareLogContent(m.activityLog, m.mainLogViewport.Width)
				m.mainLogViewport.SetContent(trunc)
				m.activityLogDirty = false
				m.mainLogViewportLastWidth = m.mainLogViewport.Width
			}

			logPanelView = renderCombinedLogPanel(&m, contentWidth, logSectionHeight)
		}

		statusBar := renderStatusBar(m, m.width)

		bodyParts := []string{headerView, row1View, row2View, row3View}
		if logPanelView != "" {
			bodyParts = append(bodyParts, logPanelView)
		}
		bodyParts = append(bodyParts, statusBar)

		mainView := lipgloss.JoinVertical(lipgloss.Left, bodyParts...)
		return appStyle.Width(m.width).Render(mainView)

	case ModeHelpOverlay:
		titleView := helpTitleStyle.Render("KEYBOARD SHORTCUTS")
		helpContentView := m.help.View(m.keys)
		content := lipgloss.JoinVertical(lipgloss.Left, titleView, helpContentView)
		style := mcpConfigOverlayStyle.Copy().Padding(1, 2)
		container := style.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, container, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))

	case ModeLogOverlay:
		overlayWidth := int(float64(m.width) * 0.8)
		overlayHeight := int(float64(m.height) * 0.7)
		m.logViewport.Width = overlayWidth - logOverlayStyle.GetHorizontalFrameSize()
		m.logViewport.Height = overlayHeight - logOverlayStyle.GetVerticalFrameSize()
		logOverlay := renderLogOverlay(m, overlayWidth, overlayHeight)
		overlayCanvas := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, logOverlay, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
		statusBar := renderStatusBar(m, m.width)
		return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)

	case ModeMcpConfigOverlay:
		cfgW := int(float64(m.width) * 0.8)
		cfgH := int(float64(m.height) * 0.7)
		m.mcpConfigViewport.Width = cfgW - mcpConfigOverlayStyle.GetHorizontalFrameSize()
		m.mcpConfigViewport.Height = cfgH - mcpConfigOverlayStyle.GetVerticalFrameSize()
		if m.mcpConfigViewport.TotalLineCount() == 0 {
			m.mcpConfigViewport.SetContent(generateMcpConfigJson())
		}
		cfgOverlay := renderMcpConfigOverlay(m, cfgW, cfgH)
		overlayCanvas := lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, cfgOverlay, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
		statusBar := renderStatusBar(m, m.width)
		return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
	default:
		return statusStyle.Render(fmt.Sprintf("Unhandled application mode: %s", m.currentAppMode.String()))
	}
}

// calculateOverallStatus determines a high-level status to display in the header.
func (m *model) calculateOverallStatus() (OverallAppStatus, string) {
	if m.isLoading {
		return AppStatusConnecting, "Ongoing operation..."
	}
	if m.currentAppMode == ModeInitializing {
		return AppStatusConnecting, "Initializing UI..."
	}
	if m.MCHealth.IsLoading {
		return AppStatusConnecting, "MC Health..."
	}
	if m.workloadClusterName != "" && m.WCHealth.IsLoading {
		return AppStatusConnecting, "WC Health..."
	}

	for _, pf := range m.portForwards {
		if strings.Contains(pf.statusMsg, "Initial") || strings.Contains(pf.statusMsg, "Awaiting") || strings.Contains(pf.statusMsg, "Restarting") {
			return AppStatusConnecting, fmt.Sprintf("%s starting...", pf.label)
		}
	}
	for _, mcp := range m.mcpServers {
		if strings.Contains(mcp.statusMsg, "Initial") || strings.Contains(mcp.statusMsg, "Restarting") {
			return AppStatusConnecting, fmt.Sprintf("%s starting...", mcp.label)
		}
	}

	if m.MCHealth.StatusError != nil {
		return AppStatusFailed, fmt.Sprintf("MC: %s", m.MCHealth.StatusError.Error())
	}
	if m.workloadClusterName != "" && m.WCHealth.StatusError != nil {
		return AppStatusFailed, fmt.Sprintf("WC: %s", m.WCHealth.StatusError.Error())
	}

	var degraded []string
	if m.MCHealth.TotalNodes > 0 && m.MCHealth.ReadyNodes < m.MCHealth.TotalNodes {
		degraded = append(degraded, fmt.Sprintf("MC nodes: %d/%d", m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes))
	}
	if m.workloadClusterName != "" && m.WCHealth.TotalNodes > 0 && m.WCHealth.ReadyNodes < m.WCHealth.TotalNodes {
		degraded = append(degraded, fmt.Sprintf("WC nodes: %d/%d", m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes))
	}
	for _, pf := range m.portForwards {
		if pf.active && (!pf.running || pf.err != nil) && !strings.Contains(pf.statusMsg, "Initial") {
			degraded = append(degraded, fmt.Sprintf("PF %s error", pf.label))
		}
	}
	for _, mcp := range m.mcpServers {
		if mcp.active && (mcp.err != nil || (!strings.Contains(mcp.statusMsg, "Running") && !strings.Contains(mcp.statusMsg, "Initial"))) {
			degraded = append(degraded, fmt.Sprintf("MCP %s error", mcp.label))
		}
	}
	if len(degraded) > 0 {
		return AppStatusDegraded, strings.Join(degraded, ", ")
	}
	return AppStatusUp, "All systems operational"
}
