package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	// "github.com/charmbracelet/bubbles/help" // No longer needed
	"github.com/charmbracelet/lipgloss"
)

// max helper function (consider moving to a utility package if used elsewhere)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Render renders the UI according to the current model state.
func Render(m *model.Model) string {
	switch m.CurrentAppMode {
	case model.ModeQuitting:
		return color.StatusStyle.Render(m.QuittingMessage)
	case model.ModeInitializing:
		if m.Width == 0 || m.Height == 0 {
			return color.StatusStyle.Render("Initializing... (waiting for window size)")
		}
		return color.StatusStyle.Render("Initializing...")
	case model.ModeMainDashboard:
		contentWidth := m.Width - color.AppStyle.GetHorizontalFrameSize()
		totalAvailableHeight := m.Height - color.AppStyle.GetVerticalFrameSize()

		// Render the header section of the main dashboard.
		headerView := renderHeader(m, contentWidth)
		headerHeight := lipgloss.Height(headerView)

		// Calculate and constrain the height for the first row (context panes).
		// It takes a small percentage of the available height, with min/max caps.
		maxRow1Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
		if maxRow1Height < 5 {
			maxRow1Height = 5
		} else if maxRow1Height > 7 {
			maxRow1Height = 7
		}
		row1View := renderContextPanesRow(m, contentWidth, maxRow1Height)
		row1Height := lipgloss.Height(row1View)

		// Calculate and constrain the height for the second row (port forwarding).
		// It takes a moderate percentage of the available height, with min/max caps.
		maxRow2Height := int(float64(totalAvailableHeight-headerHeight) * 0.30)
		if maxRow2Height < 7 {
			maxRow2Height = 7
		} else if maxRow2Height > 9 {
			maxRow2Height = 9
		}
		row2View := renderPortForwardingRow(m, contentWidth, maxRow2Height)
		row2Height := lipgloss.Height(row2View)

		// Calculate and constrain the height for the third row (MCP proxies).
		// It takes a small percentage of the available height, with a min cap.
		maxRow3Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
		if maxRow3Height < 5 {
			maxRow3Height = 5
		}
		row3View := renderMcpProxiesRow(m, contentWidth, maxRow3Height)
		row3Height := lipgloss.Height(row3View)

		logPanelView := ""
		var logSectionHeight int
		if m.Height >= minHeightForMainLogView {
			numGaps := 4 // Account for gaps between sections when calculating remaining height.
			heightConsumed := headerHeight + row1Height + row2Height + row3Height + numGaps
			logSectionHeight = totalAvailableHeight - heightConsumed
			if logSectionHeight < 0 {
				logSectionHeight = 0
			}

			m.MainLogViewport.Width = contentWidth - color.PanelStatusDefaultStyle.GetHorizontalFrameSize()
			m.MainLogViewport.Height = logSectionHeight - color.PanelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(color.LogPanelTitleStyle.Render(" ")) - 1
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
		return color.AppStyle.Width(m.Width).Render(mainView)

	case model.ModeHelpOverlay:
		titleView := color.HelpTitleStyle.Render("KEYBOARD SHORTCUTS")

		var helpLines []string

		columnSeparator := "  "
		interColumnGap := "   " // Space between logical columns (key+desc pairs)
		descColumnWidth := 20      // Approximate width for description text for inter-column padding

		keyBindingColumns := m.Keys.FullHelp() // [][]key.Binding, outer slice is columns

		if len(keyBindingColumns) == 0 {
			helpLines = append(helpLines, "No keybindings configured.")
		} else {
			// Pre-calculate the maximum width needed for keys IN EACH COLUMN
			maxKeyWidths := make([]int, len(keyBindingColumns))
			for c, column := range keyBindingColumns {
				currentMax := 0
				for _, binding := range column {
					keyWidth := lipgloss.Width(binding.Help().Key)
					if keyWidth > currentMax {
						currentMax = keyWidth
					}
				}
				maxKeyWidths[c] = currentMax
			}

			maxRows := 0
			for _, column := range keyBindingColumns {
				if len(column) > maxRows {
					maxRows = len(column)
				}
			}

			for r := 0; r < maxRows; r++ { // Iterate down the visual rows
				var currentLineStrBuilder strings.Builder
				for c := 0; c < len(keyBindingColumns); c++ { // Iterate across the columns
					if r < len(keyBindingColumns[c]) { // Check if current column has a binding for this row
						binding := keyBindingColumns[c][r]
						keyText := binding.Help().Key
						descText := binding.Help().Desc

						currentColKeyDisplayWidth := maxKeyWidths[c]
						currentKeyActualWidth := lipgloss.Width(keyText) 
						paddingForKey := ""
						if currentKeyActualWidth < currentColKeyDisplayWidth {
							paddingForKey = strings.Repeat(" ", currentColKeyDisplayWidth-currentKeyActualWidth)
						}
						currentLineStrBuilder.WriteString(keyText)
						currentLineStrBuilder.WriteString(paddingForKey)
						currentLineStrBuilder.WriteString(columnSeparator)
						currentLineStrBuilder.WriteString(descText)
						
						if c < len(keyBindingColumns)-1 {
							currentDescActualWidth := lipgloss.Width(descText)
							paddingForDesc := ""
							if currentDescActualWidth < descColumnWidth {
								paddingForDesc = strings.Repeat(" ", descColumnWidth-currentDescActualWidth)
							}
							currentLineStrBuilder.WriteString(paddingForDesc)
							currentLineStrBuilder.WriteString(interColumnGap)
						}
					} else {
						if c < len(keyBindingColumns)-1 {
							fullCellWidthEstimate := maxKeyWidths[c] + len(columnSeparator) + descColumnWidth + len(interColumnGap)
							currentLineStrBuilder.WriteString(strings.Repeat(" ", fullCellWidthEstimate))
						}
					}
				}
				helpLines = append(helpLines, currentLineStrBuilder.String())
			}
		}

		helpContent := strings.Join(helpLines, "\n")
		
		finalContentString := titleView + "\n" + helpContent
		
		container := color.CenteredOverlayContainerStyle.Render(finalContentString)

		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, container, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))

	case model.ModeLogOverlay:
		overlayWidth := int(float64(m.Width) * 0.8)
		overlayHeight := int(float64(m.Height) * 0.7)
		m.LogViewport.Width = overlayWidth - color.LogOverlayStyle.GetHorizontalFrameSize()
		m.LogViewport.Height = overlayHeight - color.LogOverlayStyle.GetVerticalFrameSize()
		logOverlay := renderLogOverlay(m, overlayWidth, overlayHeight)
		overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, logOverlay, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
		statusBar := renderStatusBar(m, m.Width)
		return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)

	case model.ModeMcpConfigOverlay:
		cfgW := int(float64(m.Width) * 0.8)
		cfgH := int(float64(m.Height) * 0.7)
		m.McpConfigViewport.Width = cfgW - color.McpConfigOverlayStyle.GetHorizontalFrameSize()
		m.McpConfigViewport.Height = cfgH - color.McpConfigOverlayStyle.GetVerticalFrameSize()
		if m.McpConfigViewport.TotalLineCount() == 0 {
			m.McpConfigViewport.SetContent(GenerateMcpConfigJson())
		}
		cfgOverlay := renderMcpConfigOverlay(m, cfgW, cfgH)
		overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, cfgOverlay, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
		statusBar := renderStatusBar(m, m.Width)
		return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
	default:
		return color.StatusStyle.Render(fmt.Sprintf("Unhandled application mode: %s", m.CurrentAppMode.String()))
	}
}

// calculateOverallStatus determines a high-level status to display in the header.
// It aggregates health from clusters, port forwards, and MCP servers.
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
