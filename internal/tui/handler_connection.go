package tui

import (
	"envctl/internal/mcpserver"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleSubmitNewConnectionMsg handles the initial request to establish a new connection.
// It performs the first part of the new connection sequence:
// 1. Logs the intent to connect.
// 2. Stops all currently active port-forwarding processes to prepare for the new setup.
// 3. Validates that a management cluster name is provided.
// 4. If valid, initiates the Kubernetes login process for the Management Cluster by returning a performKubeLoginCmd.
// - m: The current TUI model.
// - msg: The submitNewConnectionMsg containing the target MC and WC names.
// - existingCmds: A slice of commands that might have been accumulated (though typically not used here as this starts a new flow).
// Returns the updated model and a command to begin the login sequence or nil if validation fails.
func handleSubmitNewConnectionMsg(m model, msg submitNewConnectionMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
	m.isLoading = true // Start loading for new connection sequence
	m.LogInfo("Initiating new connection to MC: %s, WC: %s", msg.mc, msg.wc)
	m.LogInfo("Step 0: Stopping all existing port-forwarding processes...")

	stoppedCount := 0
	for pfKey, pf := range m.portForwards {
		if pf.stopChan != nil {
			m.LogInfo("[%s] Sending stop signal...", pf.label)
			close(pf.stopChan)
			pf.stopChan = nil
			pf.statusMsg = "Stopped (new conn)"
			pf.active = false          // Mark as inactive, setupPortForwards will re-evaluate
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

	// Summarize port-forward shutdown
	if stoppedCount > 0 {
		m.LogInfo("Finished stopping %d port-forwards.", stoppedCount)
	} else {
		m.LogInfo("No active port-forwards to stop.")
	}

	// --- Stop existing MCP proxy processes to free their ports ---
	mcpStopped := 0
	if m.mcpServers != nil {
		for srvKey, srv := range m.mcpServers {
			if srv.stopChan != nil {
				m.LogInfo("[%s MCP Proxy] Sending stop signal...", srv.label)
				safeCloseChan(srv.stopChan)
				srv.stopChan = nil
				srv.statusMsg = "Stopped (new conn)"
				srv.active = false
				m.mcpServers[srvKey] = srv
				mcpStopped++
			} else if srv.active {
				srv.statusMsg = "Stopped (new conn)"
				srv.active = false
				m.mcpServers[srvKey] = srv
			}
		}
	}

	if mcpStopped > 0 {
		m.LogInfo("Finished stopping %d MCP proxies.", mcpStopped)
	} else {
		m.LogInfo("No active MCP proxies to stop.")
	}
	if len(m.combinedOutput) > maxCombinedOutputLines {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}

	// Proceed with the new connection logic.
	m.stashedMcName = msg.mc // Used to reconstruct WC name if needed later

	if msg.mc == "" {
		m.LogError("Management Cluster name cannot be empty.")
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
		// Reset input mode
		m.currentAppMode = ModeMainDashboard
		m.newConnectionInput.Blur()
		m.newConnectionInput.Reset()
		m.currentInputStep = mcInputStep
		if len(m.portForwardOrder) > 0 {
			m.focusedPanelKey = m.portForwardOrder[0]
		}
		m.isLoading = false // Failed validation, stop loading
		clearCmd := m.setStatusMessage("MC name cannot be empty.", StatusBarError, 5*time.Second)
		return m, clearCmd
	}

	// Set a status message for starting the login process
	// This message might be quickly overwritten by login results, but good for feedback.
	// No need to batch a clear command here as the next message will likely set its own.
	m.setStatusMessage(fmt.Sprintf("Login to %s...", msg.mc), StatusBarInfo, 2*time.Second)

	m.LogInfo("Step 1: Logging into Management Cluster: %s...", msg.mc)
	// Return a new command to start the login process.
	// We are not batching with existingCmds here as this handler starts a new logical flow.
	return m, performKubeLoginCmd(msg.mc, true, msg.wc)
}

// handleKubeLoginResultMsg processes the outcome of a `tsh kube login` attempt (performKubeLoginCmd).
// It logs the stdout/stderr from the login command.
// If the login was successful:
//   - If it was for an MC and a WC is also desired, it triggers the login for the WC.
//   - If it was for an MC and no WC is desired, or if it was for a WC (meaning MC login was already done),
//     it proceeds to the post-login operations (context switching, TUI re-initialization) by returning a performPostLoginOperationsCmd.
//
// If the login failed, it logs the error and takes no further action in the connection sequence.
// - m: The current TUI model.
// - msg: The kubeLoginResultMsg containing details of the login attempt (cluster name, success/failure, output).
// - cmds: A slice of commands that might have been accumulated.
// Returns the updated model and a command for the next step in the connection flow or nil if login failed or no next step is taken from here.
func handleKubeLoginResultMsg(m model, msg kubeLoginResultMsg, cmds []tea.Cmd) (model, tea.Cmd) {
	// Append login output to the combined log first, regardless of error
	m.AppendLogLines(strings.Split(strings.TrimRight(msg.loginStdout, "\n"), "\n"))
	if strings.TrimSpace(msg.loginStderr) != "" {
		for _, line := range strings.Split(strings.TrimRight(msg.loginStderr, "\n"), "\n") {
			m.AppendLogLines([]string{"[tsh stderr] " + line})
		}
	}

	if msg.err != nil {
		m.LogError("Login failed for %s: %v", msg.clusterName, msg.err)
		m.isLoading = false // Login failed, stop loading
		clearCmd := m.setStatusMessage(fmt.Sprintf("Login failed for %s", msg.clusterName), StatusBarError, 5*time.Second)
		return m, clearCmd
	}
	m.LogInfo("Login successful for: %s", msg.clusterName)
	clearStatusCmd := m.setStatusMessage(fmt.Sprintf("Login OK: %s", msg.clusterName), StatusBarSuccess, 3*time.Second)

	var nextCmds []tea.Cmd
	if msg.isMC {
		// MC Login was successful. Now, check if WC login is needed.
		desiredMcForNextStep := msg.clusterName        // This is the confirmed MC name (e.g., "myinstallation")
		desiredWcForNextStep := msg.desiredWcShortName // WC name from original user input (e.g., "mycluster")

		if desiredWcForNextStep != "" {
			// Construct the canonical WC identifier for the tsh login command.
			// Ensures that if desiredWcForNextStep was already "mc-wc", it's not prefixed again.
			var wcIdentifierForLogin string
			if strings.HasPrefix(desiredWcForNextStep, desiredMcForNextStep+"-") {
				wcIdentifierForLogin = desiredWcForNextStep
			} else {
				wcIdentifierForLogin = desiredMcForNextStep + "-" + desiredWcForNextStep
			}
			m.LogInfo("Step 2: Logging into Workload Cluster: %s...", wcIdentifierForLogin)
			nextCmds = append(nextCmds, performKubeLoginCmd(wcIdentifierForLogin, false, "")) // For WC login, desiredWcShortNameToCarry is ""
		} else {
			// No WC specified, proceed to context switch and re-initialize for MC only.
			m.LogInfo("Step 2: No Workload Cluster specified. Proceeding to context switch for MC.")
			// desiredMcForNextStep is the MC identifier (e.g., "myinstallation")
			targetKubeContext := "teleport.giantswarm.io-" + desiredMcForNextStep
			nextCmds = append(nextCmds, performPostLoginOperationsCmd(targetKubeContext, desiredMcForNextStep, ""))
		}
	} else {
		// WC Login was successful. msg.clusterName here is the WC identifier (e.g., "myinstallation-mycluster") used for login.
		// Proceed to context switch and re-initialize for MC + WC.
		// m.stashedMcName should hold the MC name from the initial submitNewConnectionMsg.
		finalMcName := m.stashedMcName // This is the short MC name (e.g., "myinstallation")

		// msg.clusterName is the WC identifier (e.g., "myinstallation-mycluster") that was successfully logged into.
		// We need the short WC name for desiredWcName in performPostLoginOperationsCmd.
		var shortWcName string
		if finalMcName != "" && strings.HasPrefix(msg.clusterName, finalMcName+"-") {
			shortWcName = strings.TrimPrefix(msg.clusterName, finalMcName+"-")
		} else {
			// This implies msg.clusterName might be a short WC name itself (if finalMcName was empty or no prefix match)
			// or some other naming convention was used for login that we need to adapt.
			// For now, assume if it doesn't have the MC prefix, it *is* the short WC name.
			// This matches the behavior of m.getWorkloadClusterContextIdentifier if MC is empty.
			shortWcName = msg.clusterName // This might be problematic if msg.clusterName is complex and not just short WC
		}

		m.LogInfo("Step 3: Workload Cluster login successful. Proceeding to context switch for WC.")
		// msg.clusterName is the WC identifier (e.g., "myinstallation-mycluster") that was successfully logged into.
		// This is the correct identifier to form the targetKubeContext.
		targetKubeContext := "teleport.giantswarm.io-" + msg.clusterName
		nextCmds = append(nextCmds, performPostLoginOperationsCmd(targetKubeContext, finalMcName, shortWcName))
	}
	finalCmds := append(cmds, nextCmds...)
	finalCmds = append(finalCmds, clearStatusCmd)
	return m, tea.Batch(finalCmds...)
}

// handleContextSwitchAndReinitializeResultMsg processes the result of the performPostLoginOperationsCmd.
// This command attempts to switch the Kubernetes context and then prepares the TUI for re-initialization.
// If successful, this handler will:
// 1. Log diagnostic information and the successful context switch.
// 2. Update the model with the new MC and WC names, and the current Kubernetes context.
// 3. Reset health information for the clusters.
// 4. Re-configure port forwards for the new cluster setup using m.setupPortForwards().
// 5. Reset the focused panel in the TUI.
// 6. Trigger a series of commands to re-initialize the TUI state, similar to model.Init(), including:
//   - Fetching current kube context (to confirm the switch).
//   - Fetching initial node statuses for the new clusters.
//   - Starting all newly configured port-forwarding processes (getInitialPortForwardCmd).
//   - Restarting the health update ticker.
//
// If the context switch or preparation fails, it logs the error.
// - m: The current TUI model.
// - msg: The contextSwitchAndReinitializeResultMsg containing the outcome, new cluster names, and diagnostics.
// - existingCmds: A slice of commands that might have been accumulated.
// Returns the updated model and a batch of commands to re-initialize the TUI or nil if an error occurred.
func handleContextSwitchAndReinitializeResultMsg(m model, msg contextSwitchAndReinitializeResultMsg, existingCmds []tea.Cmd) (model, tea.Cmd) {
	// Log moved after the error check to see if error is the cause of no further logs
	if msg.diagnosticLog != "" {
		m.AppendLogLines([]string{"--- Diagnostic Log (Context Switch Phase) ---"})
		m.AppendLogLines(strings.Split(strings.TrimSpace(msg.diagnosticLog), "\n"))
		m.AppendLogLines([]string{"--- End Diagnostic Log ---"})
	}
	if msg.err != nil {
		m.LogError("Context switch/re-init failed: %v. MCP PROXIES WILL NOT START.", msg.err)
		m.isLoading = false // Context switch/re-init failed, stop loading
		clearCmd := m.setStatusMessage("Context switch/re-init failed.", StatusBarError, 5*time.Second)
		return m, clearCmd
	}

	if m.debugMode {
		m.LogDebug("Entered handleContextSwitchAndReinitializeResultMsg (after error check).")
	}

	m.LogInfo("Successfully switched context to: %s. Re-initializing TUI.", msg.switchedContext)
	clearStatusCmd := m.setStatusMessage(fmt.Sprintf("Context: %s. Re-initializing...", msg.switchedContext), StatusBarSuccess, 3*time.Second)

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
	setupPortForwards(&m, m.managementCluster, m.workloadCluster) // This clears and rebuilds portForwards map and order

	// Rebuild dependency graph because the set of port forwards may have changed
	m.dependencyGraph = buildDependencyGraph(&m)

	// Reset focus
	if len(m.portForwardOrder) > 0 {
		m.focusedPanelKey = m.portForwardOrder[0]
	} else if m.managementCluster != "" {
		m.focusedPanelKey = mcPaneFocusKey
	} else {
		m.focusedPanelKey = "" // No items to focus
	}

	// Rebuild MCP proxy order (predefined list is static)
	m.mcpProxyOrder = nil
	for _, cfg := range mcpserver.PredefinedMcpServers {
		m.mcpProxyOrder = append(m.mcpProxyOrder, cfg.Name)
	}

	// --- Re-initialize essential parts of the TUI (similar to Init, but after connection change) ---
	var newInitCmds []tea.Cmd
	newInitCmds = append(newInitCmds, getCurrentKubeContextCmd()) // Verify/update displayed current context

	if m.managementCluster != "" {
		mcIdentifier := m.getManagementClusterContextIdentifier()
		if mcIdentifier != "" {
			newInitCmds = append(newInitCmds, fetchNodeStatusCmd(mcIdentifier, true, m.managementCluster))
		}
	}
	if m.workloadCluster != "" {
		wcIdentifier := m.getWorkloadClusterContextIdentifier()
		if wcIdentifier != "" {
			newInitCmds = append(newInitCmds, fetchNodeStatusCmd(wcIdentifier, false, m.workloadCluster))
		}
	}

	// Start port-forwarding processes for the new setup using the centralized function
	initialPfCmds := getInitialPortForwardCmds(&m)
	newInitCmds = append(newInitCmds, initialPfCmds...)

	// Start MCP proxy processes
	m.LogInfo("Initializing predefined MCP proxies (kubernetes, prometheus, grafana)...")
	mcpProxyStartupCmds := startMcpProxiesCmd(m.TUIChannel)
	if mcpProxyStartupCmds == nil {
		if m.debugMode {
			m.LogDebug("startMcpProxiesCmd returned nil. No MCP commands generated.")
		}
	} else {
		if m.debugMode {
			m.LogDebug("Generated %d MCP proxy startup commands.", len(mcpProxyStartupCmds))
		}
		if len(mcpProxyStartupCmds) > 0 {
			newInitCmds = append(newInitCmds, mcpProxyStartupCmds...)
		}
	}

	// Re-add ticker for periodic health updates
	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	})
	newInitCmds = append(newInitCmds, tickCmd)

	finalCmdsToBatch := append(existingCmds, newInitCmds...)
	finalCmdsToBatch = append(finalCmdsToBatch, clearStatusCmd)
	return m, tea.Batch(finalCmdsToBatch...)
}
