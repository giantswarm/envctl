package tui

import (
	"envctl/internal/mcpserver"
	"envctl/internal/utils"
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
	// Removed direct activityLog manipulation - this is now handled by the logging interface

	// Proceed with the new connection logic.
	m.stashedMcName = msg.mc // Used to reconstruct WC name if needed later

	if msg.mc == "" {
		m.LogError("Management Cluster name cannot be empty.")
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
	// Log the login command output first, regardless of error
	m.LogStdout("tsh", msg.loginStdout)
	if strings.TrimSpace(msg.loginStderr) != "" {
		m.LogStderr("tsh", msg.loginStderr)
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
		// MC Login was successful. msg.clusterName is the MC short name.
		// m.stashedMcName should have been set to this MC short name from the input UI.
		// For consistency, ensure desiredMcName is updated if it wasn't already.
		m.managementClusterName = msg.clusterName

		desiredMcForNextStep := m.managementClusterName
		desiredWcForNextStep := msg.desiredWcShortName // This comes from the initial UI input, carried by msg

		if desiredWcForNextStep != "" {
			var wcIdentifierForLogin string
			if strings.HasPrefix(desiredWcForNextStep, desiredMcForNextStep+"-") {
				wcIdentifierForLogin = desiredWcForNextStep
			} else {
				wcIdentifierForLogin = desiredMcForNextStep + "-" + desiredWcForNextStep
			}
			m.LogInfo("Step 2: Logging into Workload Cluster: %s...", wcIdentifierForLogin)
			nextCmds = append(nextCmds, performKubeLoginCmd(wcIdentifierForLogin, false, ""))
		} else {
			m.LogInfo("Step 2: No Workload Cluster specified. Proceeding to context switch for MC.")
			targetKubeContext := utils.BuildMcContext(desiredMcForNextStep)
			// Pass the desired MC and empty WC short names to post login operations
			nextCmds = append(nextCmds, performPostLoginOperationsCmd(targetKubeContext, desiredMcForNextStep, ""))
		}
	} else {
		// WC Login was successful. msg.clusterName is the WC identifier (e.g., "myinstallation-mycluster").
		// m.managementClusterName should hold the MC short name from the previous step or initial UI.
		finalMcName := m.managementClusterName

		// Determine the short WC name from the logged-in WC identifier (msg.clusterName)
		// and the known MC name (m.managementClusterName).
		var shortWcName string
		if finalMcName != "" && strings.HasPrefix(msg.clusterName, finalMcName+"-") {
			shortWcName = strings.TrimPrefix(msg.clusterName, finalMcName+"-")
		} else {
			// This case implies that msg.clusterName might be just the short WC name itself,
			// or the MC prefix part was not as expected. This should be less common if logins use full identifiers.
			// Or, if finalMcName was somehow empty at this stage.
			shortWcName = msg.clusterName
			m.LogWarn("WC login name '%s' for MC '%s' did not have expected MC prefix; using '%s' as short WC name.", msg.clusterName, finalMcName, shortWcName)
		}

		// Update the model's workloadClusterName based on successful WC login.
		m.workloadClusterName = shortWcName

		m.LogInfo("Step 3: Workload Cluster login successful. Proceeding to context switch for WC.")
		targetKubeContext := utils.BuildWcContext(m.managementClusterName, m.workloadClusterName) // Use the now updated m.workloadClusterName
		nextCmds = append(nextCmds, performPostLoginOperationsCmd(targetKubeContext, m.managementClusterName, m.workloadClusterName))
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
	if msg.diagnosticLog != "" {
		m.LogInfo("--- Diagnostic Log (Context Switch Phase) ---")
		for _, line := range strings.Split(strings.TrimSpace(msg.diagnosticLog), "\n") {
			m.LogInfo("%s", line)
		}
		m.LogInfo("--- End Diagnostic Log ---")
	}
	if msg.err != nil {
		m.LogError("Context switch/re-init failed: %v. MCP PROXIES WILL NOT START.", msg.err)
		m.isLoading = false
		clearCmd := m.setStatusMessage("Context switch/re-init failed.", StatusBarError, 5*time.Second)
		return m, clearCmd
	}

	m.LogDebug("Entered handleContextSwitchAndReinitializeResultMsg (after error check).")

	m.LogInfo("Successfully switched context. Target was: %s. Re-initializing TUI.", msg.switchedContext)
	clearStatusCmd := m.setStatusMessage(fmt.Sprintf("Target: %s. Re-initializing...", msg.switchedContext), StatusBarSuccess, 3*time.Second)

	// Apply new cluster names to the model from the successful operation
	m.managementClusterName = msg.desiredMcName
	m.workloadClusterName = msg.desiredWcName

	// m.currentKubeContext will be updated by the getCurrentKubeContextCmd called below.
	// No parsing logic needed here anymore.

	// Reset health info
	m.MCHealth = clusterHealthInfo{IsLoading: true}
	if m.workloadClusterName != "" {
		m.WCHealth = clusterHealthInfo{IsLoading: true}
	} else {
		m.WCHealth = clusterHealthInfo{}
	}

	setupPortForwards(&m, m.managementClusterName, m.workloadClusterName)
	m.dependencyGraph = buildDependencyGraph(&m)

	if len(m.portForwardOrder) > 0 {
		m.focusedPanelKey = m.portForwardOrder[0]
	} else if m.managementClusterName != "" {
		m.focusedPanelKey = mcPaneFocusKey
	} else {
		m.focusedPanelKey = ""
	}

	m.mcpProxyOrder = nil
	for _, cfg := range mcpserver.PredefinedMcpServers {
		m.mcpProxyOrder = append(m.mcpProxyOrder, cfg.Name)
	}

	var newInitCmds []tea.Cmd
	newInitCmds = append(newInitCmds, getCurrentKubeContextCmd(m.services.Cluster))

	if m.managementClusterName != "" {
		mcTargetContext := utils.BuildMcContext(m.managementClusterName)
		newInitCmds = append(newInitCmds, fetchNodeStatusCmd(mcTargetContext, true, m.managementClusterName))
	}
	if m.workloadClusterName != "" && m.managementClusterName != "" {
		wcTargetContext := utils.BuildWcContext(m.managementClusterName, m.workloadClusterName)
		newInitCmds = append(newInitCmds, fetchNodeStatusCmd(wcTargetContext, false, m.workloadClusterName))
	}

	initialPfCmds := getInitialPortForwardCmds(&m)
	newInitCmds = append(newInitCmds, initialPfCmds...)

	m.LogInfo("Initializing predefined MCP proxies (kubernetes, prometheus, grafana)...")
	mcpProxyStartupCmds := startMcpProxiesCmd(m.services.Proxy, m.TUIChannel)
	if mcpProxyStartupCmds == nil {
		m.LogDebug("startMcpProxiesCmd returned nil. No MCP commands generated.")
	} else {
		m.LogDebug("Generated %d MCP proxy startup commands.", len(mcpProxyStartupCmds))
		if len(mcpProxyStartupCmds) > 0 {
			newInitCmds = append(newInitCmds, mcpProxyStartupCmds...)
		}
	}

	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	})
	newInitCmds = append(newInitCmds, tickCmd)

	finalCmdsToBatch := append(existingCmds, newInitCmds...)
	finalCmdsToBatch = append(finalCmdsToBatch, clearStatusCmd)
	m.isLoading = false
	return m, tea.Batch(finalCmdsToBatch...)
}
