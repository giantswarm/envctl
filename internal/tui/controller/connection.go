package controller

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/tui/model"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const connectionControllerSubsystem = "ConnectionController"

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
	LogInfo(connectionControllerSubsystem, "Initiating new connection to MC: %s, WC: %s", msg.MC, msg.WC)
	LogInfo(connectionControllerSubsystem, "Step 0: Stopping all existing port-forwarding processes...")

	stoppedCount := 0
	for pfKey, pf := range m.PortForwards {
		if pf.StopChan != nil {
			LogInfo(connectionControllerSubsystem, "[%s] Sending stop signal...", pf.Label)
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
		LogInfo(connectionControllerSubsystem, "Finished stopping %d port-forwards.", stoppedCount)
	} else {
		LogInfo(connectionControllerSubsystem, "No active port-forwards to stop.")
	}

	mcpStopped := 0
	if m.McpServers != nil {
		for srvKey, srv := range m.McpServers {
			if srv.StopChan != nil {
				LogInfo(connectionControllerSubsystem, "[%s MCP Proxy] Sending stop signal...", srv.Label)
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
		LogInfo(connectionControllerSubsystem, "Finished stopping %d MCP proxies.", mcpStopped)
	} else {
		LogInfo(connectionControllerSubsystem, "No active MCP proxies to stop.")
	}

	m.StashedMcName = msg.MC

	if msg.MC == "" {
		LogError(connectionControllerSubsystem, errors.New("management cluster name cannot be empty"), "Management Cluster name cannot be empty.")
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

	LogInfo(connectionControllerSubsystem, "Step 1: Logging into Management Cluster: %s...", msg.MC)
	if m.KubeMgr == nil {
		LogInfo(connectionControllerSubsystem, "KubeManager not available in handleSubmitNewConnectionMsg")
		return m, m.SetStatusMessage("KubeManager error", model.StatusBarError, 5*time.Second)
	}
	return m, PerformKubeLoginCmd(m.KubeMgr, msg.MC, true, msg.WC)
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
	LogStdout("tsh", msg.LoginStdout)
	if strings.TrimSpace(msg.LoginStderr) != "" {
		LogStderr("tsh", msg.LoginStderr)
	}

	if msg.Err != nil {
		// Login failed
		LogError(connectionControllerSubsystem, msg.Err, "kube login error for %s", msg.ClusterName)
		setStatusBarMessage := m.SetStatusMessage(fmt.Sprintf("❌ Login failed: %v", msg.Err), model.StatusBarError, 5*time.Second)
		cmds = append(cmds, setStatusBarMessage)

		// Update k8s state to not authenticated
		if m.K8sStateManager != nil {
			context := m.KubeMgr.BuildMcContextName(msg.ClusterName)
			if !msg.IsMC && m.ManagementClusterName != "" {
				context = m.KubeMgr.BuildWcContextName(m.ManagementClusterName, msg.ClusterName)
			}
			m.K8sStateManager.SetAuthenticated(context, false)
		}

		return m, tea.Batch(cmds...)
	}

	// Success
	LogInfo(connectionControllerSubsystem, "kube login success for %s", msg.ClusterName)

	// Update k8s state to authenticated
	if m.K8sStateManager != nil {
		context := m.KubeMgr.BuildMcContextName(msg.ClusterName)
		if !msg.IsMC && m.ManagementClusterName != "" {
			context = m.KubeMgr.BuildWcContextName(m.ManagementClusterName, msg.ClusterName)
		}
		m.K8sStateManager.SetAuthenticated(context, true)

		// Start health monitoring for this context
		m.K8sStateManager.StartHealthMonitor(context, 30*time.Second)
	}

	// Clear success message from status bar after a delay
	setStatusBarMessage := m.SetStatusMessage(fmt.Sprintf("✅ Login successful to %s", msg.ClusterName), model.StatusBarSuccess, 3*time.Second)
	cmds = append(cmds, setStatusBarMessage)

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
			LogInfo(connectionControllerSubsystem, "Step 2: Logging into Workload Cluster: %s...", wcIdentifierForLogin)
			nextCmds = append(nextCmds, PerformKubeLoginCmd(m.KubeMgr, wcIdentifierForLogin, false, ""))
		} else {
			LogInfo(connectionControllerSubsystem, "Step 2: No Workload Cluster specified. Proceeding to context switch for MC.")
			if m.KubeMgr == nil {
				return m, m.SetStatusMessage("KubeManager error", model.StatusBarError, 5*time.Second)
			}
			targetKubeContext := m.KubeMgr.BuildMcContextName(desiredMcForNextStep)
			nextCmds = append(nextCmds, PerformPostLoginOperationsCmd(m.KubeMgr, targetKubeContext, desiredMcForNextStep, ""))
		}
	} else {
		finalMcName := m.ManagementClusterName
		var shortWcName string
		if finalMcName != "" && strings.HasPrefix(msg.ClusterName, finalMcName+"-") {
			shortWcName = strings.TrimPrefix(msg.ClusterName, finalMcName+"-")
		} else {
			shortWcName = msg.ClusterName
			LogWarn(connectionControllerSubsystem, "WC login name '%s' for MC '%s' did not have expected MC prefix; using '%s' as short WC name.", msg.ClusterName, finalMcName, shortWcName)
		}
		m.WorkloadClusterName = shortWcName

		LogInfo(connectionControllerSubsystem, "Step 3: Workload Cluster login successful. Proceeding to context switch for WC.")
		if m.KubeMgr == nil {
			return m, m.SetStatusMessage("KubeManager error", model.StatusBarError, 5*time.Second)
		}
		targetKubeContext := m.KubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName)
		nextCmds = append(nextCmds, PerformPostLoginOperationsCmd(m.KubeMgr, targetKubeContext, m.ManagementClusterName, m.WorkloadClusterName))
	}
	finalCmds := append(cmds, nextCmds...)
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
		LogInfo(connectionControllerSubsystem, "--- Diagnostic Log (Context Switch Phase) ---")
		for _, line := range strings.Split(strings.TrimSpace(msg.DiagnosticLog), "\n") {
			LogInfo(connectionControllerSubsystem, "%s", line)
		}
		LogInfo(connectionControllerSubsystem, "--- End Diagnostic Log ---")
	}
	if msg.Err != nil {
		LogError(connectionControllerSubsystem, msg.Err, "Context switch/re-init failed: %v.", msg.Err)
		m.IsLoading = false
		return m, m.SetStatusMessage("Context switch/re-init failed.", model.StatusBarError, 5*time.Second)
	}

	LogInfo(connectionControllerSubsystem, "Successfully switched context to: %s. Re-initializing TUI services.", msg.SwitchedContext)
	m.IsLoading = true

	// Let the orchestrator handle all service lifecycle management
	if m.Orchestrator != nil {
		// Stop the old orchestrator
		m.Orchestrator.Stop()

		// Load the new configuration based on the new MC/WC names
		newEnvctlConfig, err := config.LoadConfig(msg.DesiredMCName, msg.DesiredWCName)
		if err != nil {
			LogError(connectionControllerSubsystem, err, "Failed to load new envctl configuration for MC: %s, WC: %s", msg.DesiredMCName, msg.DesiredWCName)
			m.IsLoading = false
			return m, m.SetStatusMessage("Failed to load new configuration.", model.StatusBarError, 5*time.Second)
		}

		// Update model with new cluster names and configuration
		m.ManagementClusterName = msg.DesiredMCName
		m.WorkloadClusterName = msg.DesiredWCName
		m.CurrentKubeContext = msg.SwitchedContext
		m.MCHealth = model.ClusterHealthInfo{IsLoading: true}
		if m.WorkloadClusterName != "" {
			m.WCHealth = model.ClusterHealthInfo{IsLoading: true}
		} else {
			m.WCHealth = model.ClusterHealthInfo{}
		}

		// Update model with new loaded configs
		m.PortForwardingConfig = newEnvctlConfig.PortForwards
		m.MCPServerConfig = newEnvctlConfig.MCPServers

		// Reset UI state
		m.PortForwards = make(map[string]*model.PortForwardProcess)
		m.McpServers = make(map[string]*model.McpServerProcess)

		SetupPortForwards(m, m.ManagementClusterName, m.WorkloadClusterName)

		m.McpProxyOrder = nil
		for _, cfg := range m.MCPServerConfig {
			if !cfg.Enabled {
				continue
			}
			m.McpProxyOrder = append(m.McpProxyOrder, cfg.Name)
			m.McpServers[cfg.Name] = &model.McpServerProcess{
				Label:     cfg.Name,
				Active:    true,
				StatusMsg: "Awaiting Setup...",
				Config:    cfg,
				ProxyPort: cfg.ProxyPort,
				Pid:       0,
			}
		}

		// Create new orchestrator with updated configuration
		newOrch := orchestrator.New(
			m.KubeMgr,
			m.ServiceManager,
			m.Reporter,
			orchestrator.Config{
				MCName:              m.ManagementClusterName,
				WCName:              m.WorkloadClusterName,
				PortForwards:        m.PortForwardingConfig,
				MCPServers:          m.MCPServerConfig,
				HealthCheckInterval: 15 * time.Second,
			},
		)

		// Start the new orchestrator in a goroutine
		go func() {
			ctx := context.Background()
			if err := newOrch.Start(ctx); err != nil {
				LogError(connectionControllerSubsystem, err, "Failed to start orchestrator after context switch")
			} else {
				LogInfo(connectionControllerSubsystem, "Orchestrator started with MC: %s, WC: %s", m.ManagementClusterName, m.WorkloadClusterName)
			}
		}()

		m.Orchestrator = newOrch
		m.DependencyGraph = newOrch.GetDependencyGraph()

		// Rebuild the dependency graph for the UI
		m.DependencyGraph = BuildDependencyGraph(m)
	} else {
		LogInfo(connectionControllerSubsystem, "Orchestrator is nil, cannot manage services during re-initialize.")
	}

	if len(m.PortForwardOrder) > 0 {
		m.FocusedPanelKey = m.PortForwardOrder[0]
	} else if m.ManagementClusterName != "" {
		m.FocusedPanelKey = model.McPaneFocusKey
	} else {
		m.FocusedPanelKey = ""
	}

	var newInitCmds []tea.Cmd
	if m.KubeMgr != nil {
		newInitCmds = append(newInitCmds, GetCurrentKubeContextCmd(m.KubeMgr))
		if m.ManagementClusterName != "" {
			mcTargetContext := m.KubeMgr.BuildMcContextName(m.ManagementClusterName)
			newInitCmds = append(newInitCmds, FetchNodeStatusCmd(m.KubeMgr, mcTargetContext, true, m.ManagementClusterName))
		}
		if m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
			wcTargetContext := m.KubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName)
			newInitCmds = append(newInitCmds, FetchNodeStatusCmd(m.KubeMgr, wcTargetContext, false, m.WorkloadClusterName))
		}

		// The orchestrator will handle starting services, not the TUI
	}
	statusCmd := m.SetStatusMessage(fmt.Sprintf("Context: %s. Initializing...", msg.SwitchedContext), model.StatusBarSuccess, 3*time.Second)
	finalCmdsToBatch := append(existingCmds, newInitCmds...)
	finalCmdsToBatch = append(finalCmdsToBatch, statusCmd)
	return m, tea.Batch(finalCmdsToBatch...)
}
