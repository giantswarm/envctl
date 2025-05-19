package tui

import (
	"envctl/internal/portforwarding"
	"fmt"
	"strings"
	"time"

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
                m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Failed to copy MCP config: %v", err))
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
            if err := clipboard.WriteAll(strings.Join(m.combinedOutput, "\n")); err != nil {
                m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Failed to copy logs: %v", err))
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
        order := getFocusOrder()
        if len(order) > 0 {
            idx := 0
            for i, k := range order {
                if k == m.focusedPanelKey {
                    idx = (i + 1) % len(order)
                    break
                }
            }
            m.focusedPanelKey = order[idx]
        }
        return m, nil

    case "shift+tab":
        order := getFocusOrder()
        if len(order) > 0 {
            idx := len(order) - 1
            for i, k := range order {
                if k == m.focusedPanelKey {
                    idx = (i - 1 + len(order)) % len(order)
                    break
                }
            }
            m.focusedPanelKey = order[idx]
        }
        return m, nil

    case "k", "up":
        order := getFocusOrder()
        if len(order) > 0 {
            idx := 0
            for i, k := range order {
                if k == m.focusedPanelKey {
                    idx = (i - 1 + len(order)) % len(order)
                    break
                }
            }
            m.focusedPanelKey = order[idx]
        }
        return m, nil

    case "j", "down":
        order := getFocusOrder()
        if len(order) > 0 {
            idx := 0
            for i, k := range order {
                if k == m.focusedPanelKey {
                    idx = (i + 1) % len(order)
                    break
                }
            }
            m.focusedPanelKey = order[idx]
        }
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
            m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Attempting restart...", pf.label))

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
            m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy] Manual restart requested via key.", m.focusedPanelKey))
            cmds = append(cmds, func() tea.Msg { return restartMcpServerMsg{Label: m.focusedPanelKey} })
        }

    case "s": // Context switch
        var identifier, pane string
        if m.focusedPanelKey == mcPaneFocusKey && m.managementCluster != "" {
            identifier, pane = m.getManagementClusterContextIdentifier(), "MC"
        } else if m.focusedPanelKey == wcPaneFocusKey && m.workloadCluster != "" {
            identifier, pane = m.getWorkloadClusterContextIdentifier(), "WC"
        }
        if identifier != "" {
            target := "teleport.giantswarm.io-" + identifier
            m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Attempting to switch Kubernetes context to: %s (Pane: %s)", target, pane))
            cmds = append(cmds, performSwitchKubeContextCmd(target))
        } else {
            m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Cannot switch context: Focus a valid MC/WC pane with a defined cluster name.")
        }
        if len(m.combinedOutput) > maxCombinedOutputLines {
            m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
        }
    }

    return m, tea.Batch(cmds...)
} 