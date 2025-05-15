package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func handleSubmitNewConnectionMsg(m model, msg submitNewConnectionMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Initiating new connection to MC: %s, WC: %s", msg.mc, msg.wc))
	m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Step 0: Stopping all existing port-forwarding processes...")

	stoppedCount := 0
	for pfKey, pf := range m.portForwards {
		if pf.stopChan != nil {
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Sending stop signal...", pf.label))
			close(pf.stopChan)
			pf.stopChan = nil
			pf.statusMsg = "Stopped (new conn)"
			pf.active = false // Mark as inactive, setupPortForwards will re-evaluate
			m.portForwards[pfKey] = pf // Ensure changes are written back if pf is a copy
			stoppedCount++
		} else if pf.active { // If it was supposed to be active but had no stopChan (e.g. setup failed before chan was set)
			pf.statusMsg = "Stopped (new conn)"
			pf.active = false
			m.portForwards[pfKey] = pf
			// No stopChan to close, but still log it as conceptually stopped.
			// stoppedCount++; // Optionally count these as well, or only count those actively stopped.
		}
	}

	if stoppedCount > 0 {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Finished stopping %d port-forwards.", stoppedCount))
	} else {
		m.combinedOutput = append(m.combinedOutput, "[SYSTEM] No active port-forwards to stop.")
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}

	// Proceed with the new connection logic.
	m.stashedMcName = msg.mc // Used to reconstruct WC name if needed later

	if msg.mc == "" {
		m.combinedOutput = append(m.combinedOutput, "[SYSTEM ERROR] Management Cluster name cannot be empty.")
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
		// Reset input mode
		m.isConnectingNew = false
		m.newConnectionInput.Blur()
		m.newConnectionInput.Reset()
		m.currentInputStep = mcInputStep
		if len(m.portForwardOrder) > 0 {
			m.focusedPanelKey = m.portForwardOrder[0]
		}
		return m, nil // No command, user needs to try 'n' again or quit.
	}

	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Step 1: Logging into Management Cluster: %s...", msg.mc))
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	// Return a new command to start the login process.
	// We are not batching with existingCmds here as this handler starts a new logical flow.
	return m, performKubeLoginCmd(msg.mc, true, msg.wc)
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
	m.setupPortForwards(m.managementCluster, m.workloadCluster) // This clears and rebuilds portForwards map and order

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

	// Start port-forwarding processes for the new setup using the centralized function
	initialPfCmds := getInitialPortForwardCmds(&m)
	newInitCmds = append(newInitCmds, initialPfCmds...)

	// Re-add ticker for periodic health updates
	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	})
	newInitCmds = append(newInitCmds, tickCmd)

	return m, tea.Batch(append(existingCmds, newInitCmds...)...)
}
