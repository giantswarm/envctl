package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	// Assuming utils is in "envctl/internal/utils" based on model.go
	// We might need to adjust this if utils is not directly accessible or causes import cycle
)

func handleWindowSizeMsg(m model, msg tea.WindowSizeMsg) (model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true
	return m, nil
}

func handleKeyMsgInputMode(m model, keyMsg tea.KeyMsg) (model, tea.Cmd) {
	switch keyMsg.String() {
	case "ctrl+s": // Submit new connection (MC or WC)
		if m.currentInputStep == mcInputStep {
			mcName := m.newConnectionInput.Value()
			if mcName == "" {
				return m, nil // Optionally set an error on m.newConnectionInput
			}
			m.stashedMcName = mcName
			m.currentInputStep = wcInputStep
			m.newConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName)
			m.newConnectionInput.SetValue("")
			m.newConnectionInput.Focus()
			return m, nil
		} else if m.currentInputStep == wcInputStep {
			wcName := m.newConnectionInput.Value()
			m.isConnectingNew = false
			m.newConnectionInput.Blur()
			m.newConnectionInput.Reset()
			if len(m.portForwardOrder) > 0 {
				m.focusedPanelKey = m.portForwardOrder[0]
			}
			return m, func() tea.Msg { return submitNewConnectionMsg{mc: m.stashedMcName, wc: wcName} }
		}

	case "enter": // Confirm MC input and move to WC, or submit WC input
		if m.currentInputStep == mcInputStep {
			mcName := m.newConnectionInput.Value()
			if mcName == "" {
				return m, nil // Optionally set an error
			}
			m.stashedMcName = mcName
			m.currentInputStep = wcInputStep
			m.newConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName)
			m.newConnectionInput.SetValue("")
			m.newConnectionInput.Focus()
			return m, nil
		} else if m.currentInputStep == wcInputStep {
			wcName := m.newConnectionInput.Value()
			m.isConnectingNew = false
			m.newConnectionInput.Blur()
			m.newConnectionInput.Reset()
			if len(m.portForwardOrder) > 0 {
				m.focusedPanelKey = m.portForwardOrder[0]
			}
			return m, func() tea.Msg { return submitNewConnectionMsg{mc: m.stashedMcName, wc: wcName} }
		}

	case "esc": // Cancel new connection input
		m.isConnectingNew = false
		m.newConnectionInput.Blur()
		m.newConnectionInput.Reset()
		m.currentInputStep = mcInputStep // Reset for next time
		m.stashedMcName = ""
		if len(m.portForwardOrder) > 0 {
			m.focusedPanelKey = m.portForwardOrder[0]
		}
		return m, nil

	case "tab": // Autocompletion
		currentInput := m.newConnectionInput.Value()
		if m.clusterInfo != nil && currentInput != "" {
			var suggestions []string
			normalizedCurrentInput := strings.ToLower(currentInput)
			if m.currentInputStep == mcInputStep {
				for _, mcSuggestion := range m.clusterInfo.ManagementClusters {
					if strings.HasPrefix(strings.ToLower(mcSuggestion), normalizedCurrentInput) {
						suggestions = append(suggestions, mcSuggestion)
					}
				}
			} else if m.currentInputStep == wcInputStep && m.stashedMcName != "" {
				if wcs, ok := m.clusterInfo.WorkloadClusters[m.stashedMcName]; ok {
					for _, wcSuggestion := range wcs {
						if strings.HasPrefix(strings.ToLower(wcSuggestion), normalizedCurrentInput) {
							suggestions = append(suggestions, wcSuggestion)
						}
					}
				}
			}
			if len(suggestions) > 0 {
				m.newConnectionInput.SetValue(suggestions[0])
				m.newConnectionInput.SetCursor(len(suggestions[0]))
			}
		}
		return m, nil // Tab consumed

	default:
		// Let the textinput handle other keys
		var inputCmd tea.Cmd
		m.newConnectionInput, inputCmd = m.newConnectionInput.Update(keyMsg)
		return m, inputCmd
	}
	return m, nil // Should not be reached
}

func handleKeyMsgGlobal(m model, keyMsg tea.KeyMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
	var cmds = existingCmds // Start with existing commands

	switch keyMsg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		var quitCmds []tea.Cmd
		for _, pf := range m.portForwards {
			if pf.stopChan != nil {
				// Safely close the stopChan
				// The goroutine managing the port-forward is expected to handle this signal
				// and perform cleanup.
				close(pf.stopChan)
				pf.stopChan = nil // Avoid reusing a closed channel
				pf.statusMsg = "Stopping..."
			}
		}
		quitCmds = append(quitCmds, tea.Quit)
		return m, tea.Batch(quitCmds...)

	case "n": // Start new connection
		if !m.isConnectingNew {
			m.isConnectingNew = true
			m.currentInputStep = mcInputStep
			m.newConnectionInput.Prompt = "Enter Management Cluster (Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): "
			m.newConnectionInput.Focus()
			return m, textinput.Blink
		}

	case "tab": // Panel focus
		if len(m.portForwardOrder) > 0 {
			currentIndex := -1
			for i, key := range m.portForwardOrder {
				if key == m.focusedPanelKey {
					currentIndex = i
					break
				}
			}
			if currentIndex != -1 {
				nextIndex := (currentIndex + 1) % len(m.portForwardOrder)
				m.focusedPanelKey = m.portForwardOrder[nextIndex]
			} else {
				m.focusedPanelKey = m.portForwardOrder[0]
			}
		}
		return m, nil

	case "shift+tab": // Panel focus (reverse)
		if len(m.portForwardOrder) > 0 {
			currentIndex := -1
			for i, key := range m.portForwardOrder {
				if key == m.focusedPanelKey {
					currentIndex = i
					break
				}
			}
			if currentIndex != -1 {
				nextIndex := (currentIndex - 1 + len(m.portForwardOrder)) % len(m.portForwardOrder)
				m.focusedPanelKey = m.portForwardOrder[nextIndex]
			} else {
				m.focusedPanelKey = m.portForwardOrder[len(m.portForwardOrder)-1]
			}
		}
		return m, nil

	case "k", "up":
		if len(m.portForwardOrder) > 0 {
			currentIndex := -1
			for i, key := range m.portForwardOrder {
				if key == m.focusedPanelKey {
					currentIndex = i
					break
				}
			}
			if currentIndex != -1 {
				nextIndex := (currentIndex - 1 + len(m.portForwardOrder)) % len(m.portForwardOrder)
				m.focusedPanelKey = m.portForwardOrder[nextIndex]
			} else if len(m.portForwardOrder) > 0 {
				m.focusedPanelKey = m.portForwardOrder[len(m.portForwardOrder)-1]
			}
		}
		return m, nil

	case "j", "down":
		if len(m.portForwardOrder) > 0 {
			currentIndex := -1
			for i, key := range m.portForwardOrder {
				if key == m.focusedPanelKey {
					currentIndex = i
					break
				}
			}
			if currentIndex != -1 {
				nextIndex := (currentIndex + 1) % len(m.portForwardOrder)
				m.focusedPanelKey = m.portForwardOrder[nextIndex]
			} else if len(m.portForwardOrder) > 0 {
				m.focusedPanelKey = m.portForwardOrder[0]
			}
		}
		return m, nil

	case "r": // Restart focused port-forward
		if m.focusedPanelKey != "" {
			if pf, ok := m.portForwards[m.focusedPanelKey]; ok {
				// Stop the existing port-forward if it's running
				if pf.stopChan != nil {
					m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Sending stop signal...", pf.label))
					close(pf.stopChan)
					pf.stopChan = nil
				}

				// Update UI immediately to reflect that a restart is in progress
				pf.statusMsg = "Restarting..."
				pf.output = []string{} // Clear old specific output for this PF
				pf.err = nil
				pf.active = true // It is attempting to become active
				pf.forwardingEstablished = false
				// Fields like cmd, stdout, stderr, stdoutClosed, stderrClosed are removed from portForwardProcess

				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Attempting restart...", pf.label))
				if len(m.combinedOutput) > maxCombinedOutputLines {
					m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
				}

				// Start the new port-forward using startPortForwardCmd
				if m.TUIChannel != nil {
					restartCmd := startPortForwardCmd(pf.label, pf.context, pf.namespace, pf.service, pf.port, m.TUIChannel)
					cmds = append(cmds, restartCmd)
				} else {
					m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s ERROR] TUIChannel is nil. Cannot restart.", pf.label))
					pf.statusMsg = "Restart Failed (Internal Error)"
					pf.active = false
				}
			}
		}

	case "s": // Switch kubectl context to focused MC/WC pane
		var targetContextToSwitch string
		var clusterShortNameForContext string
		var paneNameForLog string

		if m.focusedPanelKey == mcPaneFocusKey && m.managementCluster != "" {
			clusterShortNameForContext = m.managementCluster
			paneNameForLog = "MC"
		} else if m.focusedPanelKey == wcPaneFocusKey && m.workloadCluster != "" {
			if m.managementCluster != "" {
				clusterShortNameForContext = m.managementCluster + "-" + m.workloadCluster
			} else {
				clusterShortNameForContext = m.workloadCluster
			}
			paneNameForLog = "WC"
		}

		if clusterShortNameForContext != "" {
			if !strings.HasPrefix(clusterShortNameForContext, "teleport.giantswarm.io-") {
				targetContextToSwitch = "teleport.giantswarm.io-" + clusterShortNameForContext
			} else {
				targetContextToSwitch = clusterShortNameForContext
			}
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Attempting to switch kubectl context to: %s (Pane: %s)", targetContextToSwitch, paneNameForLog))
			cmds = append(cmds, performSwitchKubeContextCmd(targetContextToSwitch))
		} else {
			m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Cannot switch context: Focus a valid MC/WC pane with a defined cluster name.")
		}
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
	}
	return m, tea.Batch(cmds...)
}

func handleKubeContextResultMsg(m model, msg kubeContextResultMsg) model {
	if msg.err != nil {
		m.currentKubeContext = "Error fetching context"
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Error getting current kube context: %s", msg.err.Error()))
	} else {
		m.currentKubeContext = msg.context
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Current kubectl context: %s", msg.context))
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m
}

func handleRequestClusterHealthUpdate(m model) (model, tea.Cmd) {
	var cmds []tea.Cmd
	logMsg := fmt.Sprintf("[SYSTEM] Requesting cluster health updates at %s", time.Now().Format("15:04:05"))
	m.combinedOutput = append(m.combinedOutput, logMsg)
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}

	if m.managementCluster != "" {
		m.MCHealth.IsLoading = true
		cmds = append(cmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
	}
	if m.workloadCluster != "" {
		m.WCHealth.IsLoading = true
		cmds = append(cmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
	}
	// Re-tick for next update
	cmds = append(cmds, tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	}))
	return m, tea.Batch(cmds...)
}

func handleNodeStatusMsg(m model, msg nodeStatusMsg) model {
	var targetHealth *clusterHealthInfo
	clusterNameForLog := ""

	if msg.forMC && msg.clusterShortName == m.managementCluster {
		targetHealth = &m.MCHealth
		clusterNameForLog = m.managementCluster
	} else if !msg.forMC && msg.clusterShortName == m.workloadCluster {
		targetHealth = &m.WCHealth
		clusterNameForLog = m.workloadCluster
	} else {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[HEALTH STALE/MISMATCH] Received status for '%s' (isMC: %v), current MC: '%s', WC: '%s'. Discarding.", msg.clusterShortName, msg.forMC, m.managementCluster, m.workloadCluster))
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
		return m // No further processing for this stale/mismatched message
	}

	targetHealth.IsLoading = false
	targetHealth.LastUpdated = time.Now()
	if msg.err != nil {
		targetHealth.StatusError = msg.err
		targetHealth.ReadyNodes = 0
		targetHealth.TotalNodes = 0
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[HEALTH %s] Error: %s", clusterNameForLog, msg.err.Error()))
	} else {
		targetHealth.StatusError = nil
		targetHealth.ReadyNodes = msg.readyNodes
		targetHealth.TotalNodes = msg.totalNodes
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[HEALTH %s] Nodes: %d/%d", clusterNameForLog, msg.readyNodes, msg.totalNodes))
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m
}

func handleClusterListResultMsg(m model, msg clusterListResultMsg) model {
	if msg.err != nil {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Failed to fetch cluster list: %v", msg.err))
	} else {
		m.clusterInfo = msg.info
		// m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Cluster list fetched for autocompletion.") // Optional: too verbose?
	}
	return m
}

func handleKubeContextSwitchedMsg(m model, msg kubeContextSwitchedMsg) (model, tea.Cmd) {
	var cmds []tea.Cmd
	if msg.err != nil {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Failed to switch kubectl context to '%s': %s", msg.TargetContext, msg.err.Error()))
	} else {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Successfully switched kubectl context. Target was: %s", msg.TargetContext))
		cmds = append(cmds, getCurrentKubeContextCmd())
		if m.managementCluster != "" {
			m.MCHealth.IsLoading = true
			cmds = append(cmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
		}
		if m.workloadCluster != "" {
			m.WCHealth.IsLoading = true
			cmds = append(cmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
		}
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m, tea.Batch(cmds...)
}
