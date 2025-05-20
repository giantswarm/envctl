package view

import (
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

// Render renders the UI according to the current model state.
func Render(m *model.Model) string {
	switch m.CurrentAppMode {
	case model.ModeQuitting:
		return statusStyle.Render(m.QuittingMessage)
	case model.ModeInitializing:
		if m.Width == 0 || m.Height == 0 {
			return statusStyle.Render("Initializing... (waiting for window size)")
		}
		return statusStyle.Render("Initializing...")
	case model.ModeMainDashboard:
		contentWidth := m.Width - appStyle.GetHorizontalFrameSize()
		totalAvailableHeight := m.Height - appStyle.GetVerticalFrameSize()

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
		if m.Height >= minHeightForMainLogView {
			numGaps := 4
			heightConsumed := headerHeight + row1Height + row2Height + row3Height + numGaps
			logSectionHeight = totalAvailableHeight - heightConsumed
			if logSectionHeight < 0 {
				logSectionHeight = 0
			}

			m.MainLogViewport.Width = contentWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()
			m.MainLogViewport.Height = logSectionHeight - panelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(logPanelTitleStyle.Render(" ")) - 1
			if m.MainLogViewport.Height < 0 {
				m.MainLogViewport.Height = 0
			}

			// Refresh content only when new lines arrived or width changed.
			mlvWidthChanged := m.MainLogViewportLastWidth != m.MainLogViewport.Width
			if m.ActivityLogDirty || mlvWidthChanged {
				trunc := prepareLogContent(m.ActivityLog, m.MainLogViewport.Width)
				m.MainLogViewport.SetContent(trunc)
				m.ActivityLogDirty = false
				m.MainLogViewportLastWidth = m.MainLogViewport.Width
			}

			logPanelView = renderCombinedLogPanel(m, contentWidth, logSectionHeight)
		}

		statusBar := renderStatusBar(m, m.Width)

		bodyParts := []string{headerView, row1View, row2View, row3View}
		if logPanelView != "" {
			bodyParts = append(bodyParts, logPanelView)
		}
		bodyParts = append(bodyParts, statusBar)

		mainView := lipgloss.JoinVertical(lipgloss.Left, bodyParts...)
		return appStyle.Width(m.Width).Render(mainView)

	case model.ModeHelpOverlay:
		// Configure help styles just before rendering it
		m.Help.Styles = help.Styles{
			Ellipsis:       lipgloss.NewStyle().Foreground(HelpOverlayEllipsisFg).Background(HelpOverlayBg),
			FullDesc:       lipgloss.NewStyle().Foreground(HelpOverlayDescFg).Background(HelpOverlayBg),
			FullKey:        lipgloss.NewStyle().Foreground(HelpOverlayKeyFg).Bold(true),
			ShortDesc:      lipgloss.NewStyle().Foreground(HelpOverlayDescFg).Background(HelpOverlayBg),
			ShortKey:       lipgloss.NewStyle().Foreground(HelpOverlayKeyFg).Bold(true),
			ShortSeparator: lipgloss.NewStyle().Foreground(HelpOverlaySeparatorFg).Background(HelpOverlayBg).SetString(" • "),
		}
		titleView := helpTitleStyle.Render("KEYBOARD SHORTCUTS")
		helpContentView := m.Help.View(m.Keys)
		content := lipgloss.JoinVertical(lipgloss.Left, titleView, helpContentView)
		style := mcpConfigOverlayStyle.Copy().Padding(1, 2)
		container := style.Render(content)
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, container, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))

	case model.ModeLogOverlay:
		overlayWidth := int(float64(m.Width) * 0.8)
		overlayHeight := int(float64(m.Height) * 0.7)
		m.LogViewport.Width = overlayWidth - logOverlayStyle.GetHorizontalFrameSize()
		m.LogViewport.Height = overlayHeight - logOverlayStyle.GetVerticalFrameSize()
		logOverlay := renderLogOverlay(m, overlayWidth, overlayHeight)
		overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, logOverlay, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
		statusBar := renderStatusBar(m, m.Width)
		return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)

	case model.ModeMcpConfigOverlay:
		cfgW := int(float64(m.Width) * 0.8)
		cfgH := int(float64(m.Height) * 0.7)
		m.McpConfigViewport.Width = cfgW - mcpConfigOverlayStyle.GetHorizontalFrameSize()
		m.McpConfigViewport.Height = cfgH - mcpConfigOverlayStyle.GetVerticalFrameSize()
		if m.McpConfigViewport.TotalLineCount() == 0 {
			m.McpConfigViewport.SetContent(GenerateMcpConfigJson())
		}
		cfgOverlay := renderMcpConfigOverlay(m, cfgW, cfgH)
		overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, cfgOverlay, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
		statusBar := renderStatusBar(m, m.Width)
		return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
	default:
		return statusStyle.Render(fmt.Sprintf("Unhandled application mode: %s", m.CurrentAppMode.String()))
	}
}

// calculateOverallStatus determines a high-level status to display in the header.
func calculateOverallStatus(m *model.Model) (model.OverallAppStatus, string) {
	if m.IsLoading {
		return model.AppStatusConnecting, "Ongoing operation..."
	}
	if m.CurrentAppMode == model.ModeInitializing {
		return model.AppStatusConnecting, "Initializing UI..."
	}
	if m.MCHealth.IsLoading {
		return model.AppStatusConnecting, "MC Health..."
	}
	if m.WorkloadClusterName != "" && m.WCHealth.IsLoading {
		return model.AppStatusConnecting, "WC Health..."
	}

	for _, pf := range m.PortForwards {
		if strings.Contains(pf.StatusMsg, "Initial") || strings.Contains(pf.StatusMsg, "Awaiting") || strings.Contains(pf.StatusMsg, "Restarting") {
			return model.AppStatusConnecting, fmt.Sprintf("%s starting...", pf.Label)
		}
	}
	for _, mcp := range m.McpServers {
		if strings.Contains(mcp.StatusMsg, "Initial") || strings.Contains(mcp.StatusMsg, "Restarting") {
			return model.AppStatusConnecting, fmt.Sprintf("%s starting...", mcp.Label)
		}
	}

	if m.MCHealth.StatusError != nil {
		return model.AppStatusFailed, fmt.Sprintf("MC: %s", m.MCHealth.StatusError.Error())
	}
	if m.WorkloadClusterName != "" && m.WCHealth.StatusError != nil {
		return model.AppStatusFailed, fmt.Sprintf("WC: %s", m.WCHealth.StatusError.Error())
	}

	var degraded []string
	if m.MCHealth.TotalNodes > 0 && m.MCHealth.ReadyNodes < m.MCHealth.TotalNodes {
		degraded = append(degraded, fmt.Sprintf("MC nodes: %d/%d", m.MCHealth.ReadyNodes, m.MCHealth.TotalNodes))
	}
	if m.WorkloadClusterName != "" && m.WCHealth.TotalNodes > 0 && m.WCHealth.ReadyNodes < m.WCHealth.TotalNodes {
		degraded = append(degraded, fmt.Sprintf("WC nodes: %d/%d", m.WCHealth.ReadyNodes, m.WCHealth.TotalNodes))
	}
	for _, pf := range m.PortForwards {
		if pf.Active && (!pf.Running || pf.Err != nil) && !strings.Contains(pf.StatusMsg, "Initial") {
			degraded = append(degraded, fmt.Sprintf("PF %s error", pf.Label))
		}
	}
	for _, mcp := range m.McpServers {
		if mcp.Active && (mcp.Err != nil || (!strings.Contains(mcp.StatusMsg, "Running") && !strings.Contains(mcp.StatusMsg, "Initial"))) {
			degraded = append(degraded, fmt.Sprintf("MCP %s error", mcp.Label))
		}
	}
	if len(degraded) > 0 {
		return model.AppStatusDegraded, strings.Join(degraded, ", ")
	}
	return model.AppStatusUp, "All systems operational"
}
