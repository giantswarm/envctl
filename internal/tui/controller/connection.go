package controller

import (
	"envctl/internal/mcpserver"
	"envctl/internal/tui/model"
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
func handleSubmitNewConnectionMsg(m *model.Model, msg model.SubmitNewConnectionMsg, existingCmds []tea.Cmd) (*model.Model, tea.Cmd) {
	m.IsLoading = true
	LogInfo(m, "Initiating new connection to MC: %s, WC: %s", msg.MC, msg.WC)
	LogInfo(m, "Step 0: Stopping all existing port-forwarding processes...")

	stoppedCount := 0
	for pfKey, pf := range m.PortForwards {
		if pf.StopChan != nil {
			LogInfo(m, "[%s] Sending stop signal...", pf.Label)
			safeCloseChan(pf.StopChan)
			pf.StopChan = nil
			pf.StatusMsg = "Stopped (new conn)"
			pf.Active = false
			m.PortForwards[pfKey] = pf
			stoppedCount++
		} else if pf.Active {
			pf.StatusMsg = "Stopped (new conn)"
			pf.Active = false
			m.PortForwards[pfKey] = pf
		}
	}

	if stoppedCount > 0 {
		LogInfo(m, "Finished stopping %d port-forwards.", stoppedCount)
	} else {
		LogInfo(m, "No active port-forwards to stop.")
	}

	mcpStopped := 0
	if m.McpServers != nil {
		for srvKey, srv := range m.McpServers {
			if srv.StopChan != nil {
				LogInfo(m, "[%s MCP Proxy] Sending stop signal...", srv.Label)
				safeCloseChan(srv.StopChan)
				srv.StopChan = nil
				srv.StatusMsg = "Stopped (new conn)"
				srv.Active = false
				m.McpServers[srvKey] = srv
				mcpStopped++
			} else if srv.Active {
				srv.StatusMsg = "Stopped (new conn)"
				srv.Active = false
				m.McpServers[srvKey] = srv
			}
		}
	}

	if mcpStopped > 0 {
		LogInfo(m, "Finished stopping %d MCP proxies.", mcpStopped)
	} else {
		LogInfo(m, "No active MCP proxies to stop.")
	}

	m.StashedMcName = msg.MC

	if msg.MC == "" {
		LogError(m, "Management Cluster name cannot be empty.")
		m.CurrentAppMode = model.ModeMainDashboard
		m.NewConnectionInput.Blur()
		m.NewConnectionInput.Reset()
		m.CurrentInputStep = model.McInputStep
		if len(m.PortForwardOrder) > 0 {
			m.FocusedPanelKey = m.PortForwardOrder[0]
		}
		m.IsLoading = false
		clearCmd := m.SetStatusMessage("MC name cannot be empty.", model.StatusBarError, 5*time.Second)
		return m, clearCmd
	}

	m.SetStatusMessage(fmt.Sprintf("Login to %s...", msg.MC), model.StatusBarInfo, 2*time.Second)

	LogInfo(m, "Step 1: Logging into Management Cluster: %s...", msg.MC)
	return m, PerformKubeLoginCmd(msg.MC, true, msg.WC)
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
func handleKubeLoginResultMsg(m *model.Model, msg model.KubeLoginResultMsg, cmds []tea.Cmd) (*model.Model, tea.Cmd) {
	LogStdout(m, "tsh", msg.LoginStdout)
	if strings.TrimSpace(msg.LoginStderr) != "" {
		LogStderr(m, "tsh", msg.LoginStderr)
	}

	if msg.Err != nil {
		LogError(m, "Login failed for %s: %v", msg.ClusterName, msg.Err)
		m.IsLoading = false
		clearCmd := m.SetStatusMessage(fmt.Sprintf("Login failed for %s", msg.ClusterName), model.StatusBarError, 5*time.Second)
		return m, clearCmd
	}
	LogInfo(m, "Login successful for: %s", msg.ClusterName)
	clearStatusCmd := m.SetStatusMessage(fmt.Sprintf("Login OK: %s", msg.ClusterName), model.StatusBarSuccess, 3*time.Second)

	var nextCmds []tea.Cmd
	if msg.IsMC {
		m.ManagementClusterName = msg.ClusterName
		desiredMcForNextStep := m.ManagementClusterName
		desiredWcForNextStep := msg.DesiredWCShortName

		if desiredWcForNextStep != "" {
			var wcIdentifierForLogin string
			if strings.HasPrefix(desiredWcForNextStep, desiredMcForNextStep+"-") {
				wcIdentifierForLogin = desiredWcForNextStep
			} else {
				wcIdentifierForLogin = desiredMcForNextStep + "-" + desiredWcForNextStep
			}
			LogInfo(m, "Step 2: Logging into Workload Cluster: %s...", wcIdentifierForLogin)
			nextCmds = append(nextCmds, PerformKubeLoginCmd(wcIdentifierForLogin, false, ""))
		} else {
			LogInfo(m, "Step 2: No Workload Cluster specified. Proceeding to context switch for MC.")
			targetKubeContext := utils.BuildMcContext(desiredMcForNextStep)
			nextCmds = append(nextCmds, PerformPostLoginOperationsCmd(targetKubeContext, desiredMcForNextStep, ""))
		}
	} else {
		finalMcName := m.ManagementClusterName
		var shortWcName string
		if finalMcName != "" && strings.HasPrefix(msg.ClusterName, finalMcName+"-") {
			shortWcName = strings.TrimPrefix(msg.ClusterName, finalMcName+"-")
		} else {
			shortWcName = msg.ClusterName
			LogWarn(m, "WC login name '%s' for MC '%s' did not have expected MC prefix; using '%s' as short WC name.", msg.ClusterName, finalMcName, shortWcName)
		}
		m.WorkloadClusterName = shortWcName

		LogInfo(m, "Step 3: Workload Cluster login successful. Proceeding to context switch for WC.")
		targetKubeContext := utils.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName)
		nextCmds = append(nextCmds, PerformPostLoginOperationsCmd(targetKubeContext, m.ManagementClusterName, m.WorkloadClusterName))
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
func handleContextSwitchAndReinitializeResultMsg(m *model.Model, msg model.ContextSwitchAndReinitializeResultMsg, existingCmds []tea.Cmd) (*model.Model, tea.Cmd) {
	if msg.DiagnosticLog != "" {
		LogInfo(m, "--- Diagnostic Log (Context Switch Phase) ---")
		for _, line := range strings.Split(strings.TrimSpace(msg.DiagnosticLog), "\n") {
			LogInfo(m, "%s", line)
		}
		LogInfo(m, "--- End Diagnostic Log ---")
	}
	if msg.Err != nil {
		LogError(m, "Context switch/re-init failed: %v. MCP PROXIES WILL NOT START.", msg.Err)
		m.IsLoading = false
		clearCmd := m.SetStatusMessage("Context switch/re-init failed.", model.StatusBarError, 5*time.Second)
		return m, clearCmd
	}

	LogDebug(m, "Entered handleContextSwitchAndReinitializeResultMsg (after error check).")
	LogInfo(m, "Successfully switched context. Target was: %s. Re-initializing TUI.", msg.SwitchedContext)
	clearStatusCmd := m.SetStatusMessage(fmt.Sprintf("Target: %s. Re-initializing...", msg.SwitchedContext), model.StatusBarSuccess, 3*time.Second)

	m.ManagementClusterName = msg.DesiredMCName
	m.WorkloadClusterName = msg.DesiredWCName

	m.MCHealth = model.ClusterHealthInfo{IsLoading: true}
	if m.WorkloadClusterName != "" {
		m.WCHealth = model.ClusterHealthInfo{IsLoading: true}
	} else {
		m.WCHealth = model.ClusterHealthInfo{}
	}

	SetupPortForwards(m, m.ManagementClusterName, m.WorkloadClusterName)
	m.DependencyGraph = BuildDependencyGraph(m)

	if len(m.PortForwardOrder) > 0 {
		m.FocusedPanelKey = m.PortForwardOrder[0]
	} else if m.ManagementClusterName != "" {
		m.FocusedPanelKey = model.McPaneFocusKey
	} else {
		m.FocusedPanelKey = ""
	}

	m.McpProxyOrder = nil
	for _, cfg := range mcpserver.PredefinedMcpServers {
		m.McpProxyOrder = append(m.McpProxyOrder, cfg.Name)
	}

	var newInitCmds []tea.Cmd
	newInitCmds = append(newInitCmds, GetCurrentKubeContextCmd(m.Services.Cluster))

	if m.ManagementClusterName != "" {
		mcTargetContext := utils.BuildMcContext(m.ManagementClusterName)
		newInitCmds = append(newInitCmds, FetchNodeStatusCmd(mcTargetContext, true, m.ManagementClusterName))
	}
	if m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
		wcTargetContext := utils.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName)
		newInitCmds = append(newInitCmds, FetchNodeStatusCmd(wcTargetContext, false, m.WorkloadClusterName))
	}

	initialPfCmds := GetInitialPortForwardCmds(m)
	newInitCmds = append(newInitCmds, initialPfCmds...)

	LogInfo(m, "Initializing predefined MCP proxies (kubernetes, prometheus, grafana)...")
	mcpProxyStartupCmds := StartMcpProxiesCmd(m.Services.Proxy, m.TUIChannel)
	if mcpProxyStartupCmds == nil {
		LogDebug(m, "StartMcpProxiesCmd returned nil. No MCP commands generated.")
	} else {
		LogDebug(m, "Generated %d MCP proxy startup commands.", len(mcpProxyStartupCmds))
		if len(mcpProxyStartupCmds) > 0 {
			newInitCmds = append(newInitCmds, mcpProxyStartupCmds...)
		}
	}

	tickCmd := tea.Tick(HealthUpdateInterval, func(t time.Time) tea.Msg {
		return model.RequestClusterHealthUpdate{}
	})
	newInitCmds = append(newInitCmds, tickCmd)

	finalCmdsToBatch := append(existingCmds, newInitCmds...)
	finalCmdsToBatch = append(finalCmdsToBatch, clearStatusCmd)
	m.IsLoading = false
	return m, tea.Batch(finalCmdsToBatch...)
}
