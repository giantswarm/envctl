package tui

import (
	"envctl/internal/utils"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func handleSubmitNewConnectionMsg(m model, msg submitNewConnectionMsg, cmds []tea.Cmd) (model, tea.Cmd) {
	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Initiating new connection to MC: %s, WC: %s", msg.mc, msg.wc))

	// 1. Stop existing port forwards
	for _, pfKey := range m.portForwardOrder {
		if pf, ok := m.portForwards[pfKey]; ok {
			if pf.cmd != nil && pf.cmd.Process != nil {
				pf.cmd.Process.Kill() //nolint:errcheck // Best effort
			}
		}
	}

	m.stashedMcName = msg.mc // Used to reconstruct WC name if needed later

	if msg.mc == "" {
		m.combinedOutput = append(m.combinedOutput, "[SYSTEM ERROR] Management Cluster name cannot be empty.")
		// Consider how to provide feedback to the user or reset state
		return m, nil
	}

	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Step 1: Logging into Management Cluster: %s...", msg.mc))
	updatedCmds := append(cmds, performKubeLoginCmd(msg.mc, true, msg.wc)) // Pass desired WC to carry through
	return m, tea.Batch(updatedCmds...)
}

func handleKubeLoginResultMsg(m model, msg kubeLoginResultMsg, cmds []tea.Cmd) (model, tea.Cmd) {
	// Append login output to the combined log first, regardless of error
	if strings.TrimSpace(msg.loginStdout) != "" {
		m.combinedOutput = append(m.combinedOutput, strings.Split(strings.TrimRight(msg.loginStdout, "\n"), "\n")...)
	}
	if strings.TrimSpace(msg.loginStderr) != "" {
		for _, line := range strings.Split(strings.TrimRight(msg.loginStderr, "\n"), "\n") {
			m.combinedOutput = append(m.combinedOutput, "[tsh stderr] "+line)
		}
	}

	if msg.err != nil {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Login failed for %s: %v", msg.clusterName, msg.err))
		// Potentially reset isConnectingNew = false here or offer retry to user?
		// For now, just log and return.
		return m, nil
	}
	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Login successful for: %s", msg.clusterName))

	var nextCmds []tea.Cmd
	if msg.isMC {
		// MC Login was successful. Now, check if WC login is needed.
		desiredMcForNextStep := msg.clusterName        // This is the confirmed MC name
		desiredWcForNextStep := msg.desiredWcShortName // WC name from original user input

		if desiredWcForNextStep != "" {
			fullDesiredWcName := desiredMcForNextStep + "-" + desiredWcForNextStep
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Step 2: Logging into Workload Cluster: %s...", fullDesiredWcName))
			nextCmds = append(nextCmds, performKubeLoginCmd(fullDesiredWcName, false, "")) // For WC login, desiredWcShortNameToCarry is ""
		} else {
			// No WC specified, proceed to context switch and re-initialize for MC only.
			m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Step 2: No Workload Cluster specified. Proceeding to context switch for MC.")
			targetKubeContext := "teleport.giantswarm.io-" + desiredMcForNextStep
			nextCmds = append(nextCmds, performPostLoginOperationsCmd(targetKubeContext, desiredMcForNextStep, ""))
		}
	} else {
		// WC Login was successful. Proceed to context switch and re-initialize for MC + WC.
		// msg.clusterName is the full WC name (e.g., "mc-wc").
		// m.stashedMcName should hold the MC name from the initial submitNewConnectionMsg.
		finalMcName := m.stashedMcName
		shortWcName := ""
		if strings.HasPrefix(msg.clusterName, finalMcName+"-") {
			shortWcName = strings.TrimPrefix(msg.clusterName, finalMcName+"-")
		} else {
			// This case implies wc name didn't need mc prefix, or stashedMcName was not set as expected.
			// For robustness, if shortWcName is still empty, we might infer it or handle error.
			// However, performPostLoginOperationsCmd expects a short WC name.
			// If msg.clusterName is just "wc" and finalMcName is "mc", shortWcName calculation above will be correct.
			// If msg.clusterName is "mc-wc" and finalMcName is "mc", it's also correct.
			// What if user enters full "mc-wc" for WC and "mc" for MC? desiredWcShortName in kubeLoginResultMsg would be "mc-wc"
			// Then fullDesiredWcName = "mc-mc-wc". This needs care in performKubeLoginCmd and how names are passed.
			// For now, assume current logic for shortWcName is mostly okay.
		}

		m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Step 3: Workload Cluster login successful. Proceeding to context switch for WC.")
		targetKubeContext := "teleport.giantswarm.io-" + msg.clusterName // Full WC name for context
		nextCmds = append(nextCmds, performPostLoginOperationsCmd(targetKubeContext, finalMcName, shortWcName))
	}
	return m, tea.Batch(append(cmds, nextCmds...)...)
}

func handleContextSwitchAndReinitializeResultMsg(m model, msg contextSwitchAndReinitializeResultMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
	if msg.diagnosticLog != "" {
		m.combinedOutput = append(m.combinedOutput, "--- Diagnostic Log (Context Switch Phase) ---")
		m.combinedOutput = append(m.combinedOutput, strings.Split(strings.TrimSpace(msg.diagnosticLog), "\n")...)
		m.combinedOutput = append(m.combinedOutput, "--- End Diagnostic Log ---")
	}
	if msg.err != nil {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Context switch/re-init failed: %v", msg.err))
		// Consider how to provide feedback or allow user to retry/cancel
		return m, nil
	}

	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Successfully switched context to: %s. Re-initializing TUI.", msg.switchedContext))

	// Apply new cluster names to the model
	m.managementCluster = msg.desiredMcName
	m.workloadCluster = msg.desiredWcName
	m.currentKubeContext = msg.switchedContext // Update the current context based on successful switch

	// Reset health info
	m.MCHealth = clusterHealthInfo{IsLoading: true}
	if m.workloadCluster != "" {
		m.WCHealth = clusterHealthInfo{IsLoading: true}
	} else {
		m.WCHealth = clusterHealthInfo{} // Clear WC health if no WC
	}

	// Reset and set up new port forwards
	m.setupPortForwards(m.managementCluster, m.workloadCluster) // This clears and rebuilds

	// Reset focus
	if len(m.portForwardOrder) > 0 {
		m.focusedPanelKey = m.portForwardOrder[0]
	} else if m.managementCluster != "" {
		m.focusedPanelKey = mcPaneFocusKey
	} else {
		m.focusedPanelKey = "" // No items to focus
	}

	// --- Re-initialize essential parts of the TUI (similar to Init, but after connection change) ---
	var newInitCmds []tea.Cmd
	newInitCmds = append(newInitCmds, getCurrentKubeContextCmd()) // Verify/update displayed current context

	if m.managementCluster != "" {
		newInitCmds = append(newInitCmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
	}
	if m.workloadCluster != "" {
		newInitCmds = append(newInitCmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
	}

	// Restart port-forwarding processes for the new setup
	for _, label := range m.portForwardOrder {
		pf, isActualPortForward := m.portForwards[label]
		if isActualPortForward && pf.active { // Only if it's an actual, active port-forward config
			pf_loop := pf // Capture loop variable
			startCmd, stdout, stderr, err := utils.StartPortForward(pf_loop.context, pf_loop.namespace, pf_loop.service, pf_loop.port, pf_loop.label)
			if err != nil {
				m.portForwards[pf_loop.label].err = err
				m.portForwards[pf_loop.label].statusMsg = "Failed to start"
				m.portForwards[pf_loop.label].stdoutClosed = true
				m.portForwards[pf_loop.label].stderrClosed = true
				// Send an error message for this specific port-forward
				newInitCmds = append(newInitCmds, func() tea.Msg {
					return portForwardErrorMsg{label: pf_loop.label, streamType: "general", err: fmt.Errorf("failed to start %s: %w", pf_loop.label, err)}
				})
			} else {
				processID := startCmd.Process.Pid
				m.portForwards[pf_loop.label].cmd = startCmd
				m.portForwards[pf_loop.label].stdout = stdout
				m.portForwards[pf_loop.label].stderr = stderr
				m.portForwards[pf_loop.label].statusMsg = "Starting..." // Will be updated by portForwardStartedMsg
				newInitCmds = append(newInitCmds,
					waitForPortForwardActivity(pf_loop.label, "stdout", stdout),
					waitForPortForwardActivity(pf_loop.label, "stderr", stderr),
					func() tea.Msg { return portForwardStartedMsg{label: pf_loop.label, pid: processID} },
				)
			}
		}
	}

	// Re-add ticker for periodic health updates
	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	})
	newInitCmds = append(newInitCmds, tickCmd)

	return m, tea.Batch(append(existingCmds, newInitCmds...)...)
}
