package controller

import (
	"envctl/internal/portforwarding"
	"envctl/internal/tui/model"
	"envctl/internal/utils"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleKeyMsgGlobal processes global key presses when not in a specific input mode.
// It governs navigation, restart actions, context switching, etc.
func handleKeyMsgGlobal(m *model.Model, keyMsg tea.KeyMsg, existingCmds []tea.Cmd) (*model.Model, tea.Cmd) {
	cmds := existingCmds

	// Helper to build focus order list.
	getFocusOrder := func() []string {
		var order []string
		order = append(order, m.PortForwardOrder...)
		order = append(order, m.McpProxyOrder...)
		return order
	}

	// --- Overlay-specific key handling --------------------------------------
	if m.CurrentAppMode == model.ModeMcpConfigOverlay {
		switch keyMsg.String() {
		case "C", "esc":
			m.CurrentAppMode = model.ModeMainDashboard
			return m, nil
		case "y":
			configStr := GenerateMcpConfigJson(m.MCPServerConfig)
			if err := clipboard.WriteAll(configStr); err != nil {
				LogError(m, "Failed to copy MCP config: %v", err)
				return m, m.SetStatusMessage("Copy MCP config failed", model.StatusBarError, 3*time.Second)
			}
			return m, m.SetStatusMessage("MCP config copied", model.StatusBarSuccess, 3*time.Second)
		case "k", "up", "j", "down", "pgup", "pgdown", "home", "end":
			var vpCmd tea.Cmd
			m.McpConfigViewport, vpCmd = m.McpConfigViewport.Update(keyMsg)
			return m, vpCmd
		default:
			return m, nil
		}
	}

	if m.CurrentAppMode == model.ModeLogOverlay {
		switch keyMsg.String() {
		case "L", "esc":
			m.CurrentAppMode = model.ModeMainDashboard
			return m, nil
		case "y":
			if err := clipboard.WriteAll(strings.Join(m.ActivityLog, "\n")); err != nil {
				LogError(m, "Failed to copy logs: %v", err)
				return m, m.SetStatusMessage("Copy logs failed", model.StatusBarError, 3*time.Second)
			}
			return m, m.SetStatusMessage("Logs copied to clipboard", model.StatusBarSuccess, 3*time.Second)
		case "k", "up", "j", "down", "pgup", "pgdown", "home", "end":
			var vpCmd tea.Cmd
			m.LogViewport, vpCmd = m.LogViewport.Update(keyMsg)
			return m, vpCmd
		default:
			return m, nil
		}
	}

	if m.CurrentAppMode == model.ModeHelpOverlay {
		// If help overlay is active, Esc should close it.
		// The 'h' key (m.Keys.Help) is handled in the switch below to toggle.
		if key.Matches(keyMsg, m.Keys.Esc) {
			m.CurrentAppMode = model.ModeMainDashboard
			return m, nil
		}
		// Allow other keys (like 'h' itself) to be processed by the switch below.
	}

	// --- Check against m.Keys for bubbletea/keys standard handling ---
	switch {
	case key.Matches(keyMsg, m.Keys.Help):
		if m.CurrentAppMode == model.ModeHelpOverlay {
			m.CurrentAppMode = model.ModeMainDashboard
		} else {
			m.CurrentAppMode = model.ModeHelpOverlay
		}
		return m, nil
	case key.Matches(keyMsg, m.Keys.ToggleDark):
		// This should ideally call a service or the color package to toggle
		// and then lipgloss will re-render. For now, direct manipulation for simplicity.
		currentIsDark := lipgloss.HasDarkBackground()
		lipgloss.SetHasDarkBackground(!currentIsDark)
		// Update ColorMode string in model for display purposes
		colorProfile := lipgloss.ColorProfile().String()
		m.ColorMode = fmt.Sprintf("%s (Dark: %v)", colorProfile, !currentIsDark)
		return m, nil
	case key.Matches(keyMsg, m.Keys.ToggleDebug):
		m.DebugMode = !m.DebugMode
		return m, nil
	case key.Matches(keyMsg, m.Keys.ToggleLog):
		if m.CurrentAppMode == model.ModeLogOverlay {
			m.CurrentAppMode = model.ModeMainDashboard
		} else {
			m.CurrentAppMode = model.ModeLogOverlay
		}
		return m, nil // tea.Batch(cmds...) if LogViewport.GotoBottom() cmd is added
	case key.Matches(keyMsg, m.Keys.ToggleMcpConfig):
		if m.CurrentAppMode == model.ModeMcpConfigOverlay {
			m.CurrentAppMode = model.ModeMainDashboard
		} else {
			m.CurrentAppMode = model.ModeMcpConfigOverlay
			// Populate the viewport content when entering the mode
			configJSON := GenerateMcpConfigJson(m.MCPServerConfig)
			m.McpConfigViewport.SetContent(configJSON)
			m.McpConfigViewport.GotoTop() // Reset scroll position
		}
		return m, nil
	}

	// --- Normal global shortcuts -------------------------------------------
	switch keyMsg.String() {
	case "ctrl+c", "q":
		return m, nil // quit handled in Update

	case "n": // new connection flow
		if m.CurrentAppMode != model.ModeNewConnectionInput {
			m.CurrentAppMode = model.ModeNewConnectionInput
			m.CurrentInputStep = model.McInputStep
			m.NewConnectionInput.Prompt = "Enter Management Cluster (Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): "
			m.NewConnectionInput.Focus()
			return m, textinput.Blink
		}

	case "tab":
		m.FocusedPanelKey = nextFocus(getFocusOrder(), m.FocusedPanelKey, 1)
		return m, nil

	case "shift+tab":
		m.FocusedPanelKey = nextFocus(getFocusOrder(), m.FocusedPanelKey, -1)
		return m, nil

	case "k", "up":
		m.FocusedPanelKey = nextFocus(getFocusOrder(), m.FocusedPanelKey, -1)
		return m, nil

	case "j", "down":
		m.FocusedPanelKey = nextFocus(getFocusOrder(), m.FocusedPanelKey, 1)
		return m, nil

	case "r": // restart PF or MCP depending on focus
		if m.FocusedPanelKey == "" {
			return m, nil
		}
		if pf, ok := m.PortForwards[m.FocusedPanelKey]; ok {
			// stop current PF
			safeCloseChan(pf.StopChan)

			pf.StatusMsg = "Restarting..."
			pf.Log = nil
			pf.Err = nil
			pf.Running = false
			pf.Active = true

			m.IsLoading = true
			LogInfo(m, "[%s] Attempting restart...", pf.Label)

			if m.TUIChannel != nil {
				currentCfg := pf.Config
				cmds = append(cmds, func() tea.Msg {
					tuiCb := func(update portforwarding.PortForwardProcessUpdate) {
						m.TUIChannel <- model.PortForwardCoreUpdateMsg{Update: update}
					}
					stop, err := m.Services.PF.Start(currentCfg, tuiCb)
					return model.PortForwardSetupResultMsg{InstanceKey: currentCfg.InstanceKey, StopChan: stop, Err: err}
				})
			}
		} else if _, ok := m.McpServers[m.FocusedPanelKey]; ok {
			m.IsLoading = true
			LogInfo(m, "[%s MCP Proxy] Manual restart requested via key.", m.FocusedPanelKey)
			cmds = append(cmds, func() tea.Msg { return model.RestartMcpServerMsg{Label: m.FocusedPanelKey} })
		}

	case "s": // Context switch
		if m.FocusedPanelKey == model.McPaneFocusKey && m.ManagementClusterName != "" {
			target := utils.BuildMcContext(m.ManagementClusterName)
			LogInfo(m, "Attempting to switch Kubernetes context to: %s (Pane: MC, Target: %s)", target, m.ManagementClusterName)
			cmds = append(cmds, PerformSwitchKubeContextCmd(m.Services.Cluster, target))
		} else if m.FocusedPanelKey == model.WcPaneFocusKey && m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
			target := utils.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName)
			LogInfo(m, "Attempting to switch Kubernetes context to: %s (Pane: WC, Target: %s-%s)", target, m.ManagementClusterName, m.WorkloadClusterName)
			cmds = append(cmds, PerformSwitchKubeContextCmd(m.Services.Cluster, target))
		} else {
			LogWarn(m, "Cannot switch context: Focus a valid MC/WC pane with defined cluster names or ensure clusters are set via (n)ew connection.")
		}
	}

	return m, tea.Batch(cmds...)
}

// nextFocus returns the next element from order based on the current element
// and a delta (+1 for forward, -1 for backward). It safely handles edge cases:
//   - If order is empty it returns current unchanged.
//   - If current is not found it returns the first (delta>0) or last (delta<0) element.
//   - It wraps around when reaching either end of the slice.
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
