package tui

import (
	"fmt"
	"time"

	// We need to ensure imports for key and spinner are present if they were in the original,
	// but the diff implies they might have been added by the faulty edit.
	// For now, assuming they are not strictly needed for *just* the logging additions to compile
	// if the surrounding code is correct.
	// "github.com/charmbracelet/bubbles/key"
	// "github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// setStatusMessage updates the status bar message and schedules clearing it after the given duration.
func (m *model) setStatusMessage(message string, msgType MessageType, clearAfter time.Duration) tea.Cmd {
    m.statusBarMessage = message
    m.statusBarMessageType = msgType

    if m.statusBarClearCancel != nil {
        close(m.statusBarClearCancel)
    }

    m.statusBarClearCancel = make(chan struct{})
    captured := m.statusBarClearCancel

    return tea.Tick(clearAfter, func(t time.Time) tea.Msg {
        select {
        case <-captured:
            return nil
        default:
            return clearStatusBarMsg{}
        }
    })
}

// Update is the heart of the Bubbletea program – handling all incoming messages.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Record message type sampling for realistic workload generation (no-op unless built with the msgsample tag).
    recordMsgSample(msg)
    var cmds []tea.Cmd
    var cmd tea.Cmd

    // Only log if not a spinner.TickMsg or tea.MouseMsg, to reduce noise.
    switch msg.(type) {
    case spinner.TickMsg, tea.MouseMsg:
        // Do nothing, just let these common/noisy messages pass through without generic logging.
    default:
        // Only perform expensive formatting when debug mode is enabled.
        if m.debugMode {
            m.LogDebug("[Main Update] Received msg: %T -- Value: %v", msg, msg)
        }
    }

    // --- Global quit shortcuts ------------------------------------------------
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            m.currentAppMode = ModeQuitting
            m.quittingMessage = "Shutting down..."

            for _, pf := range m.portForwards {
                if pf.stopChan != nil {
                    close(pf.stopChan)
                    pf.stopChan = nil
                    pf.statusMsg = "Stopping..."
                }
            }

            if m.mcpServers != nil {
                for name, proc := range m.mcpServers {
                    if proc.active && proc.stopChan != nil {
                        m.LogInfo("[%s MCP Proxy] Sending stop signal...", name)
                        close(proc.stopChan)
                        proc.stopChan = nil
                        proc.statusMsg = "Stopping..."
                        proc.active = false
                    }
                }
            }

            finalizeMsgSampling()
            cmds = append(cmds, tea.Quit)
            return m, tea.Batch(cmds...)
        case "ctrl+c":
            finalizeMsgSampling()
            return m, tea.Quit
        }
    }

    // --- Mode specific handling ------------------------------------------------
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if m.currentAppMode == ModeNewConnectionInput && m.newConnectionInput.Focused() {
            m, cmd = handleKeyMsgInputMode(m, msg)
        } else {
            // Overlay toggles & debug shortcuts.
            switch msg.String() {
            case "h":
                if m.currentAppMode == ModeHelpOverlay {
                    m.currentAppMode = ModeMainDashboard
                } else {
                    m.currentAppMode = ModeHelpOverlay
                }
                return m, channelReaderCmd(m.TUIChannel)
            case "D":
                dark := lipgloss.HasDarkBackground()
                lipgloss.SetHasDarkBackground(!dark)
                m.colorMode = fmt.Sprintf("%s (Dark: %v)", lipgloss.ColorProfile().String(), !dark)
                return m, channelReaderCmd(m.TUIChannel)
            case "z":
                m.debugMode = !m.debugMode
                return m, channelReaderCmd(m.TUIChannel)
            case "esc":
                if m.currentAppMode == ModeHelpOverlay {
                    m.currentAppMode = ModeMainDashboard
                    return m, channelReaderCmd(m.TUIChannel)
                }
            }

            // Log overlay toggle.
            if m.currentAppMode != ModeHelpOverlay && msg.String() == "L" {
                if m.currentAppMode == ModeLogOverlay {
                    m.currentAppMode = ModeMainDashboard
                } else {
                    m.currentAppMode = ModeLogOverlay
                    overlayW := int(float64(m.width) * 0.8)
                    overlayH := int(float64(m.height) * 0.7)
                    m.logViewport.Width = overlayW - logOverlayStyle.GetHorizontalFrameSize()
                    m.logViewport.Height = overlayH - logOverlayStyle.GetVerticalFrameSize()
                    trunc := prepareLogContent(m.activityLog, m.logViewport.Width)
                    m.logViewport.SetContent(trunc)
                }
                return m, channelReaderCmd(m.TUIChannel)
            }

            // MCP config overlay toggle.
            if m.currentAppMode != ModeHelpOverlay && msg.String() == "C" {
                if m.currentAppMode == ModeMcpConfigOverlay {
                    m.currentAppMode = ModeMainDashboard
                } else {
                    m.currentAppMode = ModeMcpConfigOverlay
                    cfgW := int(float64(m.width) * 0.8)
                    cfgH := int(float64(m.height) * 0.7)
                    m.mcpConfigViewport.Width = cfgW - mcpConfigOverlayStyle.GetHorizontalFrameSize()
                    m.mcpConfigViewport.Height = cfgH - mcpConfigOverlayStyle.GetVerticalFrameSize()
                    if m.mcpConfigViewport.TotalLineCount() == 0 {
                        m.mcpConfigViewport.SetContent(generateMcpConfigJson())
                    }
                }
                return m, channelReaderCmd(m.TUIChannel)
            }

            // Fallback to global handler.
            m, cmd = handleKeyMsgGlobal(m, msg, []tea.Cmd{})
        }
        cmds = append(cmds, cmd)

    case tea.WindowSizeMsg:
        // Skip expensive layout recalculations when the dimensions haven't actually changed.
        if msg.Width == m.width && msg.Height == m.height {
            // No-op size event – ignore.
            return m, nil
        }

        m, cmd = handleWindowSizeMsg(m, msg)
        // Help width update.
        helpWidth := int(float64(msg.Width) * 0.8)
        if helpWidth > 100 {
            helpWidth = 100
        }
        m.help.Width = helpWidth

        // Resize active overlay viewport.
        if m.currentAppMode == ModeLogOverlay {
            w := int(float64(m.width) * 0.8)
            h := int(float64(m.height) * 0.7)
            m.logViewport.Width = w - logOverlayStyle.GetHorizontalFrameSize()
            m.logViewport.Height = h - logOverlayStyle.GetVerticalFrameSize()
        } else if m.currentAppMode == ModeMcpConfigOverlay {
            w := int(float64(m.width) * 0.8)
            h := int(float64(m.height) * 0.7)
            m.mcpConfigViewport.Width = w - mcpConfigOverlayStyle.GetHorizontalFrameSize()
            m.mcpConfigViewport.Height = h - mcpConfigOverlayStyle.GetVerticalFrameSize()
        } else {
            // Delegate to View() calculations.
            if m.width > 0 && m.height > 0 {
                contentWidth := m.width - appStyle.GetHorizontalFrameSize()
                totalAvailableHeight := m.height - appStyle.GetVerticalFrameSize()
                headerHeight := lipgloss.Height(renderHeader(m, contentWidth))
                maxRow1Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
                if maxRow1Height < 5 {
                    maxRow1Height = 5
                } else if maxRow1Height > 7 {
                    maxRow1Height = 7
                }
                row1Height := lipgloss.Height(renderContextPanesRow(m, contentWidth, maxRow1Height))

                maxRow2Height := int(float64(totalAvailableHeight-headerHeight) * 0.30)
                if maxRow2Height < 7 {
                    maxRow2Height = 7
                } else if maxRow2Height > 9 {
                    maxRow2Height = 9
                }
                row2Height := lipgloss.Height(renderPortForwardingRow(m, contentWidth, maxRow2Height))

                if m.height >= minHeightForMainLogView {
                    numGaps := 3
                    consumed := headerHeight + row1Height + row2Height + numGaps
                    logSectionHeight := totalAvailableHeight - consumed
                    if logSectionHeight < 0 {
                        logSectionHeight = 0
                    }
                    m.mainLogViewport.Width = contentWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()
                    m.mainLogViewport.Height = logSectionHeight - panelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(logPanelTitleStyle.Render(" ")) - 1
                    if m.mainLogViewport.Height < 0 {
                        m.mainLogViewport.Height = 0
                    }
                }
            }
        }
        cmds = append(cmds, cmd)

    // Remaining message types are delegated to specialised handlers in other files.
    case portForwardSetupResultMsg:
        m, cmd = handlePortForwardSetupResultMsg(m, msg)
        cmds = append(cmds, cmd)
    case portForwardCoreUpdateMsg:
        m, cmd = handlePortForwardCoreUpdateMsg(m, msg)
        cmds = append(cmds, cmd)

    case submitNewConnectionMsg:
        m, cmd = handleSubmitNewConnectionMsg(m, msg, cmds)
    case kubeLoginResultMsg:
        m.LogDebug("[Main Update] Matched kubeLoginResultMsg. Routing to handleKubeLoginResultMsg...")
        m, cmd = handleKubeLoginResultMsg(m, msg, cmds)
    case contextSwitchAndReinitializeResultMsg:
        m.LogDebug("[Main Update] Matched contextSwitchAndReinitializeResultMsg. Routing to handleContextSwitchAndReinitializeResultMsg...")
        m, cmd = handleContextSwitchAndReinitializeResultMsg(m, msg, cmds)

    case kubeContextResultMsg:
        m.LogDebug("[Main Update] Matched kubeContextResultMsg. Routing to handleKubeContextResultMsg...")
        m = handleKubeContextResultMsg(m, msg)
        cmds = append(cmds, channelReaderCmd(m.TUIChannel))
    case requestClusterHealthUpdate:
        m, cmd = handleRequestClusterHealthUpdate(m)
    case kubeContextSwitchedMsg:
        m.LogDebug("[Main Update] Matched kubeContextSwitchedMsg. Routing to handleKubeContextSwitchedMsg...")
        m, cmd = handleKubeContextSwitchedMsg(m, msg)
        cmds = append(cmds, cmd)
    case nodeStatusMsg:
        m.LogDebug("[Main Update] Matched nodeStatusMsg. Routing to handleNodeStatusMsg...")
        m = handleNodeStatusMsg(m, msg)
        // Temporarily comment out to test if this is blocking other messages
        // cmds = append(cmds, channelReaderCmd(m.TUIChannel)) 
    case clusterListResultMsg:
        m = handleClusterListResultMsg(m, msg)
        cmds = append(cmds, channelReaderCmd(m.TUIChannel))

    case clearStatusBarMsg:
        m.statusBarMessage = ""
        if m.statusBarClearCancel != nil {
            close(m.statusBarClearCancel)
            m.statusBarClearCancel = nil
        }
        cmds = append(cmds, channelReaderCmd(m.TUIChannel))

    case mcpServerSetupCompletedMsg:
        m, cmd = handleMcpServerSetupCompletedMsg(m, msg)
        cmds = append(cmds, cmd)
    case mcpServerStatusUpdateMsg:
        m, cmd = handleMcpServerStatusUpdateMsg(m, msg)
        cmds = append(cmds, cmd)
    case restartMcpServerMsg:
        m, cmd = handleRestartMcpServerMsg(m, msg)
        cmds = append(cmds, cmd)

    case tea.MouseMsg:
        if m.currentAppMode == ModeLogOverlay {
            m.logViewport, cmd = m.logViewport.Update(msg)
        } else if m.currentAppMode == ModeMcpConfigOverlay {
            m.mcpConfigViewport, cmd = m.mcpConfigViewport.Update(msg)
        } else {
            m.mainLogViewport, cmd = m.mainLogViewport.Update(msg)
        }
        cmds = append(cmds, cmd)

    case spinner.TickMsg:
        var spinCmd tea.Cmd
        m.spinner, spinCmd = m.spinner.Update(msg)
        cmds = append(cmds, spinCmd)

    default:
        m.LogDebug("[Main Update] Unhandled msg type in default case: %T -- Value: %v", msg, msg)
        if m.currentAppMode == ModeNewConnectionInput && m.newConnectionInput.Focused() {
            m.newConnectionInput, cmd = m.newConnectionInput.Update(msg)
        } else if m.currentAppMode == ModeLogOverlay {
            m.logViewport, cmd = m.logViewport.Update(msg)
        } else if m.currentAppMode == ModeMcpConfigOverlay {
            m.mcpConfigViewport, cmd = m.mcpConfigViewport.Update(msg)
        } else {
            m.spinner, cmd = m.spinner.Update(msg)
        }
        cmds = append(cmds, cmd)
    }

    // Lazily refresh the log viewport only when new lines were added or the
    // viewport width changed (i.e. after a resize). This avoids redundant
    // O(n) work on every single Update cycle.
    widthChanged := m.logViewportLastWidth != m.logViewport.Width
    if m.activityLogDirty || widthChanged {
        prepared := prepareLogContent(m.activityLog, m.logViewport.Width)
        m.logViewport.SetContent(prepared)
        if m.currentAppMode == ModeLogOverlay && m.logViewport.YOffset == 0 {
            m.logViewport.GotoBottom()
        }
        m.activityLogDirty = false
        m.logViewportLastWidth = m.logViewport.Width
    }

    // cmds = append(cmds, channelReaderCmd(m.TUIChannel)) // Also consider commenting out the one at the very end if the problem persists
    return m, tea.Batch(cmds...)
} 