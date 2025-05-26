package controller

import (
	"envctl/internal/kube"
	"envctl/internal/tui/model" // For LogError if we create new errors
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const keyGlobalSubsystem = "KeyGlobal"

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
		// Pass all unhandled keys to McpConfigViewport for its own handling (like scrolling)
		var vpCmd tea.Cmd
		switch keyMsg.String() {
		case "C", "esc":
			m.CurrentAppMode = model.ModeMainDashboard
			return m, nil
		case "y":
			configStr := GenerateMcpConfigJson(m.MCPServerConfig, m.McpServers)
			if err := clipboard.WriteAll(configStr); err != nil {
				LogError(keyGlobalSubsystem, err, "Failed to copy MCP config: %v", err)
				return m, m.SetStatusMessage("Copy MCP config failed", model.StatusBarError, 3*time.Second)
			}
			return m, m.SetStatusMessage("MCP config copied", model.StatusBarSuccess, 3*time.Second)
		// k, up, j, down, etc. are handled by the default case below for this overlay
		default:
			// TEMPORARY DEBUG for McpConfigOverlay key presses
			LogDebug(m, keyGlobalSubsystem, "McpConfigOverlay KeyPress: Type=%v, String='%s', Runes=%+q", keyMsg.Type, keyMsg.String(), keyMsg.Runes)
			m.McpConfigViewport, vpCmd = m.McpConfigViewport.Update(keyMsg)
			return m, vpCmd
		}
	}

	if m.CurrentAppMode == model.ModeLogOverlay {
		// TEMPORARY TEST FOR HORIZONTAL SCROLLING
		if keyMsg.Type == tea.KeyRunes && keyMsg.String() == "H" { // Using Shift+H for test trigger
			longLine := "1234567890abcdefghijklmnopqrstuvwxyz" +
				"1234567890abcdefghijklmnopqrstuvwxyz" +
				"1234567890abcdefghijklmnopqrstuvwxyz" +
				"1234567890abcdefghijklmnopqrstuvwxyz"
			m.LogViewport.SetContent(longLine + "\n" + "short line")
			m.LogViewport.GotoTop()
			LogInfo(keyGlobalSubsystem, "LogViewport content set to test string. VP Width: %d, VP Height: %d, Content Line0 Len: %d", m.LogViewport.Width, m.LogViewport.Height, len(longLine))
			return m, nil
		}

		// Original logic for ModeLogOverlay
		switch keyMsg.String() {
		case "L", "esc":
			m.CurrentAppMode = model.ModeMainDashboard
			return m, nil
		case "y":
			if err := clipboard.WriteAll(strings.Join(m.ActivityLog, "\n")); err != nil {
				LogError(keyGlobalSubsystem, err, "Failed to copy logs: %v", err)
				return m, m.SetStatusMessage("Copy logs failed", model.StatusBarError, 3*time.Second)
			}
			return m, m.SetStatusMessage("Logs copied to clipboard", model.StatusBarSuccess, 3*time.Second)
		// Let other keys, including arrows for scrolling, be handled by the viewport's Update method.
		default:
			// TEMPORARY DEBUG for LogOverlay key presses
			LogDebug(m, keyGlobalSubsystem, "LogOverlay KeyPress: Type=%v, String='%s', Runes=%+q", keyMsg.Type, keyMsg.String(), keyMsg.Runes)
			var vpCmd tea.Cmd
			m.LogViewport, vpCmd = m.LogViewport.Update(keyMsg)
			return m, vpCmd
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
			configJSON := GenerateMcpConfigJson(m.MCPServerConfig, m.McpServers)
			m.McpConfigViewport.SetContent(configJSON)
			m.McpConfigViewport.GotoTop() // Reset scroll position
		}
		return m, nil
	case key.Matches(keyMsg, m.Keys.Restart): // RESTART focused PF or MCP service
		if m.FocusedPanelKey == "" {
			return m, nil
		}
		var serviceLabelToRestart string
		var serviceType string // To help with logging or specific actions if needed

		if pf, ok := m.PortForwards[m.FocusedPanelKey]; ok {
			serviceLabelToRestart = pf.Label // Use the label from the PortForwardProcess struct
			serviceType = "Port Forward"
		} else if mcp, ok := m.McpServers[m.FocusedPanelKey]; ok {
			serviceLabelToRestart = mcp.Label // Use the label from the McpServerProcess struct
			serviceType = "MCP Server"
		} else {
			LogDebug(m, keyGlobalSubsystem, "'r' pressed but no known service focused: %s", m.FocusedPanelKey)
			return m, nil
		}

		if serviceLabelToRestart != "" {
			LogInfo(keyGlobalSubsystem, "User requested restart for %s: %s", serviceType, serviceLabelToRestart)
			// Logging of this action will now happen via pkg/logging, which the TUI will pick up.
			restartCmd := func() tea.Msg {
				if m.Orchestrator == nil {
					return model.ServiceErrorMsg{Label: serviceLabelToRestart, Err: fmt.Errorf("Orchestrator not available")}
				}
				err := m.Orchestrator.RestartService(serviceLabelToRestart)
				if err != nil {
					return model.ServiceErrorMsg{Label: serviceLabelToRestart, Err: fmt.Errorf("failed to initiate restart: %w", err)}
				}
				return model.NopMsg{}
			}
			cmds = append(cmds, restartCmd)
			cmds = append(cmds, m.SetStatusMessage(fmt.Sprintf("Restart initiated for %s...", serviceLabelToRestart), model.StatusBarInfo, 3*time.Second))
		} else {
			LogDebug(m, keyGlobalSubsystem, "'r' pressed but could not determine service label for focused key: %s", m.FocusedPanelKey)
		}
		return m, tea.Batch(cmds...)

	case key.Matches(keyMsg, m.Keys.Stop): // STOP focused PF or MCP service
		if m.FocusedPanelKey == "" {
			return m, nil
		}
		var serviceLabelToStop string
		var serviceTypeToStop string
		stoppable := false

		if pf, ok := m.PortForwards[m.FocusedPanelKey]; ok {
			if pf.Running { // Only allow stopping if it's considered running
				serviceLabelToStop = pf.Label
				serviceTypeToStop = "Port Forward"
				stoppable = true
				pf.StatusMsg = "Stopping..." // Immediate visual feedback
			}
		} else if mcp, ok := m.McpServers[m.FocusedPanelKey]; ok {
			if mcp.Active { // Or a more specific check if MCP has a distinct Running state
				serviceLabelToStop = mcp.Label
				serviceTypeToStop = "MCP Server"
				stoppable = true
				mcp.StatusMsg = "Stopping..." // Immediate visual feedback
			}
		}

		if serviceLabelToStop != "" && stoppable {
			LogInfo(keyGlobalSubsystem, "User requested to stop %s: %s", serviceTypeToStop, serviceLabelToStop)

			stopCmd := func() tea.Msg {
				if m.Orchestrator == nil {
					return model.ServiceErrorMsg{Label: serviceLabelToStop, Err: fmt.Errorf("Orchestrator not available")}
				}

				// Let the orchestrator handle stopping with proper dependency management
				err := m.Orchestrator.StopService(serviceLabelToStop)

				return model.ServiceStopResultMsg{Label: serviceLabelToStop, Err: err}
			}
			cmds = append(cmds, stopCmd)
			cmds = append(cmds, m.SetStatusMessage(fmt.Sprintf("Stopping %s...", serviceLabelToStop), model.StatusBarInfo, 3*time.Second))
		} else if serviceLabelToStop != "" && !stoppable {
			LogInfo(keyGlobalSubsystem, "Service %s is not in a stoppable state.", serviceLabelToStop)
			cmds = append(cmds, m.SetStatusMessage(fmt.Sprintf("%s not running/active.", serviceLabelToStop), model.StatusBarWarning, 3*time.Second))
		} else {
			LogDebug(m, keyGlobalSubsystem, "'x' pressed but no known stoppable service focused: %s", m.FocusedPanelKey)
		}
		return m, tea.Batch(cmds...)
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

	case "s": // Context switch
		if m.FocusedPanelKey == model.McPaneFocusKey && m.ManagementClusterName != "" {
			target := kube.BuildMcContext(m.ManagementClusterName)
			LogInfo(keyGlobalSubsystem, "Attempting to switch Kubernetes context to: %s (Pane: MC, Target: %s)", target, m.ManagementClusterName)
			cmds = append(cmds, PerformSwitchKubeContextCmd(target))
		} else if m.FocusedPanelKey == model.WcPaneFocusKey && m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
			target := kube.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName)
			LogInfo(keyGlobalSubsystem, "Attempting to switch Kubernetes context to: %s (Pane: WC, Target: %s-%s)", target, m.ManagementClusterName, m.WorkloadClusterName)
			cmds = append(cmds, PerformSwitchKubeContextCmd(target))
		} else {
			LogWarn(keyGlobalSubsystem, "Cannot switch context: Focus a valid MC/WC pane with defined cluster names or ensure clusters are set via (n)ew connection.")
			cmds = append(cmds, m.SetStatusMessage("Cannot switch context: Focus a valid MC/WC pane with defined cluster names.", model.StatusBarWarning, 3*time.Second))
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
