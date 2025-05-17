package tui

import (
	"envctl/internal/portforwarding"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	// Assuming utils is in "envctl/internal/utils" based on model.go
	// We might need to adjust this if utils is not directly accessible or causes import cycle
)

// handleWindowSizeMsg updates the model with the new terminal dimensions when the window is resized.
// It also sets the `ready` flag to true, indicating the TUI can perform its initial full render.
func handleWindowSizeMsg(m model, msg tea.WindowSizeMsg) (model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true
	return m, nil
}

// handleKeyMsgInputMode processes key presses when the TUI is in the 'new connection input' mode.
// It handles keys for submitting input (Enter, Ctrl+S), canceling (Esc), and autocompletion (Tab).
// - For Enter/Ctrl+S: If entering MC name, it stores it and moves to WC input. If entering WC name, it submits both.
// - For Esc: Cancels the input mode and resets state.
// - For Tab: Attempts to autocomplete the current input based on fetched cluster lists.
// Other keys are passed to the textinput component for standard text editing.
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

// handleKeyMsgGlobal processes global key presses when not in a specific input mode.
// It handles actions like:
// - Quitting the application ('q', Ctrl+C): Closes active port-forward stop channels and sends tea.Quit.
// - Initiating a new connection ('n'): Switches to input mode.
// - Navigating panels (Tab, Shift+Tab, 'j'/Down, 'k'/Up): Cycles focus through UI panels.
// - Restarting a focused port-forward ('r'): Stops and starts the selected port-forward process.
// - Switching Kubernetes context ('s'): Attempts to switch to the context of the focused MC or WC pane.
// - Toggling Log Overlay ('L') is handled in model.Update's KeyMsg block.
func handleKeyMsgGlobal(m model, keyMsg tea.KeyMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
	var cmds = existingCmds // Start with existing commands

	// If log overlay is visible, prioritize its controls
	if m.logOverlayVisible {
		switch keyMsg.String() {
		case "L", "esc": // Close log overlay
			m.logOverlayVisible = false
			return m, nil
		case "k", "up", "j", "down", "pgup", "pgdown", "home", "end": // Pass scrolling keys to viewport
			var viewportCmd tea.Cmd
			m.logViewport, viewportCmd = m.logViewport.Update(keyMsg)
			return m, viewportCmd
		default: // Other keys are ignored when log overlay is active
			return m, nil
		}
	}

	// If help overlay is visible, only Esc or h work (handled in model.Update's KeyMsg block)
	if m.helpVisible {
		// Key handling for when help is visible is done in model.Update
		// We shouldn't process global keys here to avoid conflicts.
		return m, nil
	}

	switch keyMsg.String() {
	case "ctrl+c", "q":
		// Don't do anything here - quitting is handled in model.Update
		// Just return to allow model.Update to handle the quit sequence
		return m, nil

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

				// Update TUI state for restart
				pf.statusMsg = "Restarting..."
				pf.output = []string{} // Clear old specific output
				pf.err = nil
				pf.active = true   // Mark as attempting to be active
				pf.running = false // It's not running yet
				pf.pid = 0         // Reset PID
				pf.cmd = nil       // Reset command reference

				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Attempting restart...", pf.label))
				// Trim log directly if needed, or rely on model.Update

				// Create a new command to start this port forward
				if m.TUIChannel != nil {
					currentPfConfig := pf.config // Get the config from the TUI process state

					restartCmdFunc := func() tea.Msg {
						// Define the update callback function for the core package
						tuiUpdateFn := func(update portforwarding.PortForwardProcessUpdate) {
							m.TUIChannel <- portForwardCoreUpdateMsg{update: update}
						}

						// Call the core function to start and manage the port forward
						cmd, stopChan, err := portforwarding.StartAndManageIndividualPortForward(currentPfConfig, tuiUpdateFn, nil)

						initialPID := 0
						if cmd != nil && cmd.Process != nil {
							initialPID = cmd.Process.Pid
						}

						return portForwardSetupResultMsg{
							InstanceKey: currentPfConfig.InstanceKey,
							Cmd:         cmd,
							StopChan:    stopChan,
							InitialPID:  initialPID,
							Err:         err,
						}
					}
					cmds = append(cmds, restartCmdFunc)
				} else {
					m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s ERROR] TUIChannel is nil. Cannot restart.", pf.label))
					pf.statusMsg = "Restart Failed (Internal Error)"
					pf.active = false
				}
			} else {
				// Focused key does not correspond to a known port-forward (e.g. MC/WC pane)
				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Panel '%s' is not a restartable port-forward.", m.focusedPanelKey))
			}
		}

	case "s": // Switch Kubernetes context to focused MC/WC pane
		var targetContextToSwitch string
		var clusterIdentifier string // Renamed from clusterShortNameForContext
		var paneNameForLog string

		if m.focusedPanelKey == mcPaneFocusKey && m.managementCluster != "" {
			clusterIdentifier = m.getManagementClusterContextIdentifier()
			paneNameForLog = "MC"
		} else if m.focusedPanelKey == wcPaneFocusKey && m.workloadCluster != "" {
			clusterIdentifier = m.getWorkloadClusterContextIdentifier()
			paneNameForLog = "WC"
		}

		if clusterIdentifier != "" {
			// The getManagementClusterContextIdentifier/getWorkloadClusterContextIdentifier methods return
			// the part of the context name *after* "teleport.giantswarm.io-".
			// So, we always prepend the prefix here.
			targetContextToSwitch = "teleport.giantswarm.io-" + clusterIdentifier
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Attempting to switch Kubernetes context to: %s (Pane: %s)", targetContextToSwitch, paneNameForLog))
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

// handleKubeContextResultMsg updates the model with the current Kubernetes context or an error if fetching failed.
// This is typically called after startup or a context switch to reflect the actual current context.
func handleKubeContextResultMsg(m model, msg kubeContextResultMsg) model {
	if msg.err != nil {
		m.currentKubeContext = "Error fetching context"
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Error getting current Kubernetes context: %s", msg.err.Error()))
	} else {
		m.currentKubeContext = msg.context
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Current Kubernetes context: %s", msg.context))
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m
}

// handleRequestClusterHealthUpdate is triggered by a ticker or after certain operations to refresh cluster health.
// It sets the IsLoading flag for relevant clusters and issues fetchNodeStatusCmd for both MC and WC (if defined).
// It also re-schedules the next health update tick.
func handleRequestClusterHealthUpdate(m model) (model, tea.Cmd) {
	var cmds []tea.Cmd
	logMsg := fmt.Sprintf("[SYSTEM] Requesting cluster health updates at %s", time.Now().Format("15:04:05"))
	m.combinedOutput = append(m.combinedOutput, logMsg)
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}

	if m.managementCluster != "" {
		m.MCHealth.IsLoading = true
		mcIdentifier := m.getManagementClusterContextIdentifier()
		if mcIdentifier != "" {
			cmds = append(cmds, fetchNodeStatusCmd(mcIdentifier, true, m.managementCluster))
		}
	}
	if m.workloadCluster != "" {
		m.WCHealth.IsLoading = true
		wcIdentifier := m.getWorkloadClusterContextIdentifier()
		if wcIdentifier != "" {
			cmds = append(cmds, fetchNodeStatusCmd(wcIdentifier, false, m.workloadCluster))
		}
	}
	// Re-tick for next update
	cmds = append(cmds, tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	}))
	return m, tea.Batch(cmds...)
}

// handleNodeStatusMsg processes the results of a fetchNodeStatusCmd.
// It updates the health information (ready/total nodes, error state, last updated time) for the specific cluster (MC or WC).
// It discards stale or mismatched status messages (e.g., if the cluster context changed since the request was made).
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

// handleClusterListResultMsg updates the model with the fetched list of management and workload clusters.
// This information (m.clusterInfo) is used for autocompletion in the new connection input mode.
// If fetching fails, an error is logged.
func handleClusterListResultMsg(m model, msg clusterListResultMsg) model {
	if msg.err != nil {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Failed to fetch cluster list: %v", msg.err))
	} else {
		m.clusterInfo = msg.info
		// m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Cluster list fetched for autocompletion.") // Optional: too verbose?
	}
	return m
}

// handleKubeContextSwitchedMsg processes the result of an attempt to switch the Kubernetes context (performSwitchKubeContextCmd).
// If successful, it logs the success and triggers commands to refresh the current kube context display and cluster health data.
// If failed, it logs the error.
func handleKubeContextSwitchedMsg(m model, msg kubeContextSwitchedMsg) (model, tea.Cmd) {
	var cmds []tea.Cmd
	if msg.err != nil {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Failed to switch Kubernetes context to '%s': %s", msg.TargetContext, msg.err.Error()))
	} else {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Successfully switched Kubernetes context. Target was: %s", msg.TargetContext))
		cmds = append(cmds, getCurrentKubeContextCmd())
		if m.managementCluster != "" {
			m.MCHealth.IsLoading = true
			mcIdentifier := m.getManagementClusterContextIdentifier()
			if mcIdentifier != "" {
				cmds = append(cmds, fetchNodeStatusCmd(mcIdentifier, true, m.managementCluster))
			}
		}
		if m.workloadCluster != "" {
			m.WCHealth.IsLoading = true
			wcIdentifier := m.getWorkloadClusterContextIdentifier()
			if wcIdentifier != "" {
				cmds = append(cmds, fetchNodeStatusCmd(wcIdentifier, false, m.workloadCluster))
			}
		}
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m, tea.Batch(cmds...)
}

// MCP Server Message Handlers are now in mcpserver_handlers.go
