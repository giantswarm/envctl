package tui

import (
	"envctl/internal/portforwarding"
	"strings"
	"time"

	"envctl/internal/utils"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsgGlobal processes global key presses when not in a specific input mode.
// It governs navigation, restart actions, context switching, etc.
func handleKeyMsgGlobal(m model, keyMsg tea.KeyMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
    cmds := existingCmds

    // Helper to build focus order list.
    getFocusOrder := func() []string {
        var order []string
        order = append(order, m.portForwardOrder...)
        order = append(order, m.mcpProxyOrder...)
        return order
    }

    // --- Overlay-specific key handling --------------------------------------
    if m.currentAppMode == ModeMcpConfigOverlay {
        switch keyMsg.String() {
        case "C", "esc":
            m.currentAppMode = ModeMainDashboard
            return m, nil
        case "y":
            if err := clipboard.WriteAll(generateMcpConfigJson()); err != nil {
                m.LogError("Failed to copy MCP config: %v", err)
                return m, m.setStatusMessage("Copy MCP config failed", StatusBarError, 3*time.Second)
            }
            return m, m.setStatusMessage("MCP config copied", StatusBarSuccess, 3*time.Second)
        case "k", "up", "j", "down", "pgup", "pgdown", "home", "end":
            var vpCmd tea.Cmd
            m.mcpConfigViewport, vpCmd = m.mcpConfigViewport.Update(keyMsg)
            return m, vpCmd
        default:
            return m, nil
        }
    }

    if m.currentAppMode == ModeLogOverlay {
        switch keyMsg.String() {
        case "L", "esc":
            m.currentAppMode = ModeMainDashboard
            return m, nil
        case "y":
            if err := clipboard.WriteAll(strings.Join(m.activityLog, "\n")); err != nil {
                m.LogError("Failed to copy logs: %v", err)
                return m, m.setStatusMessage("Copy logs failed", StatusBarError, 3*time.Second)
            }
            return m, m.setStatusMessage("Logs copied to clipboard", StatusBarSuccess, 3*time.Second)
        case "k", "up", "j", "down", "pgup", "pgdown", "home", "end":
            var vpCmd tea.Cmd
            m.logViewport, vpCmd = m.logViewport.Update(keyMsg)
            return m, vpCmd
        default:
            return m, nil
        }
    }

    if m.currentAppMode == ModeHelpOverlay {
        return m, nil // Help overlay handled elsewhere
    }

    // --- Normal global shortcuts -------------------------------------------
    switch keyMsg.String() {
    case "ctrl+c", "q":
        return m, nil // quit handled in Update

    case "n": // new connection flow
        if m.currentAppMode != ModeNewConnectionInput {
            m.currentAppMode = ModeNewConnectionInput
            m.currentInputStep = mcInputStep
            m.newConnectionInput.Prompt = "Enter Management Cluster (Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): "
            m.newConnectionInput.Focus()
            return m, textinput.Blink
        }

    case "tab":
        m.focusedPanelKey = nextFocus(getFocusOrder(), m.focusedPanelKey, 1)
        return m, nil

    case "shift+tab":
        m.focusedPanelKey = nextFocus(getFocusOrder(), m.focusedPanelKey, -1)
        return m, nil

    case "k", "up":
        m.focusedPanelKey = nextFocus(getFocusOrder(), m.focusedPanelKey, -1)
        return m, nil

    case "j", "down":
        m.focusedPanelKey = nextFocus(getFocusOrder(), m.focusedPanelKey, 1)
        return m, nil

    case "r": // restart PF or MCP depending on focus
        if m.focusedPanelKey == "" {
            return m, nil
        }
        if pf, ok := m.portForwards[m.focusedPanelKey]; ok {
            // stop current PF
            safeCloseChan(pf.stopChan)

            pf.statusMsg = "Restarting..."
            pf.output = nil
            pf.err = nil
            pf.running = false
            pf.pid = 0
            pf.cmd = nil
            pf.active = true

            m.isLoading = true
            m.LogInfo("[%s] Attempting restart...", pf.label)

            if m.TUIChannel != nil {
                currentCfg := pf.config
                cmds = append(cmds, func() tea.Msg {
                    tuiCb := func(update portforwarding.PortForwardProcessUpdate) {
                        m.TUIChannel <- portForwardCoreUpdateMsg{update: update}
                    }
                    cmd, stop, err := portforwarding.StartAndManageIndividualPortForward(currentCfg, tuiCb)
                    pid := 0
                    if cmd != nil && cmd.Process != nil {
                        pid = cmd.Process.Pid
                    }
                    return portForwardSetupResultMsg{InstanceKey: currentCfg.InstanceKey, Cmd: cmd, StopChan: stop, InitialPID: pid, Err: err}
                })
            }
        } else if _, ok := m.mcpServers[m.focusedPanelKey]; ok {
            m.isLoading = true
            m.LogInfo("[%s MCP Proxy] Manual restart requested via key.", m.focusedPanelKey)
            cmds = append(cmds, func() tea.Msg { return restartMcpServerMsg{Label: m.focusedPanelKey} })
        }

    case "s": // Context switch
        if m.focusedPanelKey == mcPaneFocusKey && m.managementClusterName != "" {
            target := utils.BuildMcContext(m.managementClusterName)
            m.LogInfo("Attempting to switch Kubernetes context to: %s (Pane: MC, Target: %s)", target, m.managementClusterName)
            cmds = append(cmds, performSwitchKubeContextCmd(target)) 
        } else if m.focusedPanelKey == wcPaneFocusKey && m.workloadClusterName != "" && m.managementClusterName != "" {
            target := utils.BuildWcContext(m.managementClusterName, m.workloadClusterName)
            m.LogInfo("Attempting to switch Kubernetes context to: %s (Pane: WC, Target: %s-%s)", target, m.managementClusterName, m.workloadClusterName)
            cmds = append(cmds, performSwitchKubeContextCmd(target))
        } else {
            m.LogWarn("Cannot switch context: Focus a valid MC/WC pane with defined cluster names or ensure clusters are set via (n)ew connection.")
        }
    }

    return m, tea.Batch(cmds...)
}

// nextFocus returns the next element from order based on the current element
// and a delta (+1 for forward, -1 for backward). It safely handles edge cases:
//   * If order is empty it returns current unchanged.
//   * If current is not found it returns the first (delta>0) or last (delta<0) element.
//   * It wraps around when reaching either end of the slice.
func nextFocus(order []string, current string, delta int) string {
    if len(order) == 0 {
        return current
    }

    // Clamp delta to +/-1 so that unexpected values do not lead to panics.
    if delta > 0 {
        delta = 1
    } else if delta < 0 {
        delta = -1
    }

    // Locate current index.
    idx := -1
    for i, v := range order {
        if v == current {
            idx = i
            break
        }
    }

    if idx == -1 {
        // Current not found – pick start/end based on direction.
        if delta >= 0 {
            return order[0]
        }
        return order[len(order)-1]
    }

    n := len(order)
    idx = (idx + delta + n) % n
    return order[idx]
} 