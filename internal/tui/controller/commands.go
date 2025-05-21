package controller

import (
	// "bufio" // No longer used directly here

	"envctl/internal/kube"
	"envctl/internal/mcpserver"
	"envctl/internal/service"
	"envctl/internal/tui/model"
	"envctl/internal/utils"
	"fmt"

	// "io" // No longer used directly here
	// "os" // No longer used directly here
	"strings" // Keep for performPostLoginOperationsCmd and potentially others

	// "syscall" // No longer used directly here

	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd" // Added for kubeconfig handling
)

// fetchNodeStatusCmd creates a tea.Cmd to asynchronously fetch the node status.
// - fullTargetContextName: The fully constructed Kubernetes context name to use for the API call.
// - isMC: Boolean indicating if the status is for a Management Cluster.
// - originalClusterShortName: The original short name of the cluster (e.g., "myinstallation" or "myworkloadcluster"), used for tagging the result message.
// Returns a tea.Cmd that, when run, will create a clientset, call kube.GetNodeStatusClientGo, and send a model.NodeStatusMsg.
func FetchNodeStatusCmd(fullTargetContextName string, isMC bool, originalClusterShortName string) tea.Cmd {
	return func() tea.Msg {
		var debugStr strings.Builder
		debugStr.WriteString(fmt.Sprintf("[DEBUG] fetchNodeStatusCmd started: origShort=%s, isMC=%v, targetCtx=%s\n", originalClusterShortName, isMC, fullTargetContextName))

		defer func() {
			if r := recover(); r != nil {
				panicDebugInfo := fmt.Sprintf("PANIC in fetchNodeStatusCmd for '%s' (target ctx '%s'): %v\n", originalClusterShortName, fullTargetContextName, r)
				// Print to terminal for immediate visibility.
				fmt.Printf("[TERMINAL_DEBUG] [fetchNodeStatusCmd] %s", panicDebugInfo)
				debugStr.WriteString("CRITICAL_PANIC: " + panicDebugInfo)

				// Send a message back to the TUI so it can clear the loading state and surface the error.
				// IMPORTANT: We recover the panic and convert it into a nodeStatusMsg with an error.
				// Because we're inside a defer, we cannot simply "return"; instead, we schedule the send
				// via a goroutine on the TUI's message channel (tea.NewCallback).
				tea.Printf("[DEBUG] Recovered panic in fetchNodeStatusCmd for %s – sending error nodeStatusMsg", originalClusterShortName)
				// We cannot directly return a message from inside the defer. Instead, we rely on the fact that
				// the outer function will continue execution after the defer completes. The final debug string
				// now contains the panic information, so the function will fall through and return an error
				// nodeStatusMsg a few lines below (ready == 0, total == 0, err != nil).
			}
		}()

		if originalClusterShortName == "" {
			debugStr.WriteString(fmt.Sprintf("Health check failed: Empty originalClusterShortName (isMC: %v, targetCtx: %s)\n", isMC, fullTargetContextName))
			return model.NodeStatusMsg{ClusterShortName: originalClusterShortName, ForMC: isMC, Err: fmt.Errorf("originalClusterShortName for health check is empty"), DebugInfo: debugStr.String()}
		}

		if strings.TrimSpace(fullTargetContextName) == "" {
			debugStr.WriteString(fmt.Sprintf("Health check failed: Empty fullTargetContextName for %s (isMC: %v)\n", originalClusterShortName, isMC))
			return model.NodeStatusMsg{ClusterShortName: originalClusterShortName, ForMC: isMC, Err: fmt.Errorf("fullTargetContextName for health check is empty"), DebugInfo: debugStr.String()}
		}

		{
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			cfg, errLoad := loadingRules.Load()
			if errLoad != nil {
				ds := fmt.Sprintf("Health check for %s (target ctx %s): Failed to load kubeconfig: %v\n", originalClusterShortName, fullTargetContextName, errLoad)
				debugStr.WriteString("Kubeconfig load error: " + ds)
				return model.NodeStatusMsg{ClusterShortName: originalClusterShortName, ForMC: isMC, Err: fmt.Errorf("kubeconfig error: %v", errLoad), DebugInfo: debugStr.String()}
			}
			debugStr.WriteString(fmt.Sprintf("Checking health for cluster %s (isMC: %v)\n", originalClusterShortName, isMC))
			debugStr.WriteString(fmt.Sprintf("Target context: %s (will use this context explicitly)\n", fullTargetContextName))
			debugStr.WriteString(fmt.Sprintf("Actual current context in kubeconfig: %s (will NOT use this for health check)\n", cfg.CurrentContext))
			debugStr.WriteString("Available contexts in kubeconfig:\n")
			for ctxName := range cfg.Contexts {
				debugStr.WriteString(fmt.Sprintf("- %s\n", ctxName))
			}
			if _, ok := cfg.Contexts[fullTargetContextName]; !ok {
				debugStr.WriteString(fmt.Sprintf("WARNING: Target context %s not found in kubeconfig. Health check may fail.\n",
					fullTargetContextName))
			}
		}

		// Create clientset here
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{CurrentContext: fullTargetContextName}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		restConfig, err := kubeConfig.ClientConfig()
		if err != nil {
			debugStr.WriteString(fmt.Sprintf("Failed to get REST config for context %q: %v\n", fullTargetContextName, err))
			return model.NodeStatusMsg{ClusterShortName: originalClusterShortName, ForMC: isMC, Err: fmt.Errorf("REST config error: %v", err), DebugInfo: debugStr.String()}
		}
		restConfig.Timeout = 15 * time.Second
		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			debugStr.WriteString(fmt.Sprintf("Failed to create Kubernetes clientset for context %q: %v\n", fullTargetContextName, err))
			return model.NodeStatusMsg{ClusterShortName: originalClusterShortName, ForMC: isMC, Err: fmt.Errorf("clientset error: %v", err), DebugInfo: debugStr.String()}
		}

		debugStr.WriteString(fmt.Sprintf("\nAttempting to fetch node status for %s using context: %s\n", originalClusterShortName, fullTargetContextName))
		ready, total, errGetStatus := kube.GetNodeStatusClientGo(clientset)
		debugStr.WriteString(fmt.Sprintf("Node status result for %s: targetContext=%s, ready=%d, total=%d, err=%v\n",
			originalClusterShortName, fullTargetContextName, ready, total, errGetStatus))

		if errGetStatus != nil && strings.Contains(errGetStatus.Error(), "does not exist in kubeconfig") {
			debugStr.WriteString(fmt.Sprintf("Context %s (for cluster %s) does not exist in kubeconfig. This is expected if you haven't logged in to this cluster yet.\n", fullTargetContextName, originalClusterShortName))
			debugStr.WriteString(fmt.Sprintf("Health check will continue to show 'loading' until a valid login to %s is completed.\n", originalClusterShortName))
		}

		debugStr.WriteString(fmt.Sprintf("[fetchNodeStatusCmd] FINISHING for %s. Error: %v\n", originalClusterShortName, errGetStatus))
		debugStr.WriteString(fmt.Sprintf("[DEBUG] fetchNodeStatusCmd completed: origShort=%s ready=%d total=%d err=%v\n", originalClusterShortName, ready, total, errGetStatus))
		finalDebugInfo := debugStr.String()

		return model.NodeStatusMsg{ClusterShortName: originalClusterShortName, ForMC: isMC, ReadyNodes: ready, TotalNodes: total, Err: errGetStatus, DebugInfo: finalDebugInfo}
	}
}

// getCurrentKubeContextCmd creates a tea.Cmd to asynchronously fetch the current active Kubernetes context.
// Returns a tea.Cmd that, when run, will call kube.GetCurrentKubeContext and send a kubeContextResultMsg.
func GetCurrentKubeContextCmd(clusterSvc service.ClusterService) tea.Cmd {
	return func() tea.Msg {
		currentCtx, err := clusterSvc.CurrentContext()
		return model.KubeContextResultMsg{Context: currentCtx, Err: err}
	}
}

// performSwitchKubeContextCmd creates a tea.Cmd to attempt switching the active Kubernetes context.
// - targetContextName: The full name of the Kubernetes context to switch to.
// Returns a tea.Cmd that, when run, will call kube.SwitchKubeContext and send a kubeContextSwitchedMsg.
func PerformSwitchKubeContextCmd(clusterSvc service.ClusterService, targetContextName string) tea.Cmd {
	return func() tea.Msg {
		oldCtx, _ := clusterSvc.CurrentContext()

		err := clusterSvc.SwitchContext(targetContextName, "")

		result := model.KubeContextSwitchedMsg{
			TargetContext: targetContextName,
			Err:           err,
			DebugInfo: fmt.Sprintf("Context switch: %s -> %s, Result: %v",
				oldCtx, targetContextName, err == nil),
		}
		return result
	}
}

// performKubeLoginCmd creates a tea.Cmd to attempt a `tsh kube login` to the specified cluster.
// This is part of the new connection flow.
// - clusterName: The name of the cluster to log into (can be MC name or full WC name like "mc-wc").
// - isMC: True if this login attempt is for a Management Cluster.
// - desiredWcShortNameToCarry: If isMC is true, this holds the short name of the desired WC to be used in the next step.
// Returns a tea.Cmd that, when run, will call utils.LoginToKubeCluster and send a kubeLoginResultMsg.
func PerformKubeLoginCmd(clusterName string, isMC bool, desiredWcShortNameToCarry string) tea.Cmd {
	return func() tea.Msg {
		stdout, stderr, err := utils.LoginToKubeCluster(clusterName)
		return model.KubeLoginResultMsg{
			ClusterName:        clusterName,
			IsMC:               isMC,
			DesiredWCShortName: desiredWcShortNameToCarry,
			LoginStdout:        stdout,
			LoginStderr:        stderr,
			Err:                err,
		}
	}
}

// performPostLoginOperationsCmd creates a tea.Cmd to perform operations after successful Kubernetes logins,
// primarily switching to the target Kubernetes context and gathering diagnostic information.
// This is a key step in finalizing a new connection sequence.
// - targetKubeContext: The full Kubernetes context name to switch to (e.g., "teleport.giantswarm.io-mc-wc").
// - desiredMc: The short name of the Management Cluster for the new connection.
// - desiredWc: The short name of the Workload Cluster for the new connection (can be empty).
// Returns a tea.Cmd that, when run, attempts the context switch, gets the current context, gathers diagnostics,
// and then sends a contextSwitchAndReinitializeResultMsg back to the TUI.
func PerformPostLoginOperationsCmd(targetKubeContext, desiredMc, desiredWc string) tea.Cmd {
	return func() tea.Msg {
		var diagnosticLog strings.Builder

		// Get initial context for better debug info
		initialContext, initialErr := kube.GetCurrentKubeContext()
		if initialErr != nil {
			diagnosticLog.WriteString(fmt.Sprintf("Initial GetCurrentKubeContext error: %v\n", initialErr))
		} else {
			diagnosticLog.WriteString(fmt.Sprintf("Initial context before switch: %s\n", initialContext))
		}

		diagnosticLog.WriteString(fmt.Sprintf("Attempting to switch context to: %s\n", targetKubeContext))
		diagnosticLog.WriteString(fmt.Sprintf("Desired MC: %s, Desired WC: %s\n", desiredMc, desiredWc))

		err := kube.SwitchKubeContext(targetKubeContext)
		if err != nil {
			diagnosticLog.WriteString(fmt.Sprintf("SwitchKubeContext error: %v\n", err))
			// Try to get more diagnostic information about kubeconfig state
			pathOptions := clientcmd.NewDefaultPathOptions()
			if config, configErr := pathOptions.GetStartingConfig(); configErr == nil {
				diagnosticLog.WriteString(fmt.Sprintf("Current context after failed switch: %s\n", config.CurrentContext))
				if _, exists := config.Contexts[targetKubeContext]; !exists {
					diagnosticLog.WriteString(fmt.Sprintf("Target context '%s' does not exist in kubeconfig\n", targetKubeContext))
				}
			}

			return model.ContextSwitchAndReinitializeResultMsg{
				Err:           fmt.Errorf("failed to switch context to %s: %v", targetKubeContext, err),
				DesiredMCName: desiredMc,
				DesiredWCName: desiredWc,
				DiagnosticLog: diagnosticLog.String(),
			}
		}
		diagnosticLog.WriteString("SwitchKubeContext successful.\n")

		actualCurrentContext, err := kube.GetCurrentKubeContext()
		if err != nil {
			diagnosticLog.WriteString(fmt.Sprintf("GetCurrentKubeContext error: %v\n", err))
			return model.ContextSwitchAndReinitializeResultMsg{
				Err:             fmt.Errorf("failed to get current context after switch: %v", err),
				SwitchedContext: actualCurrentContext,
				DesiredMCName:   desiredMc,
				DesiredWCName:   desiredWc,
				DiagnosticLog:   diagnosticLog.String(),
			}
		}
		diagnosticLog.WriteString(fmt.Sprintf("GetCurrentKubeContext successful: %s\n", actualCurrentContext))

		// Get all contexts using client-go
		pathOptions := clientcmd.NewDefaultPathOptions()
		config, err := pathOptions.GetStartingConfig()
		if err != nil {
			diagnosticLog.WriteString(fmt.Sprintf("Failed to load kubeconfig for getting all contexts: %v\n", err))
		} else {
			diagnosticLog.WriteString("Available Kubernetes contexts (from client-go):\n")
			for contextName := range config.Contexts {
				if contextName == actualCurrentContext {
					diagnosticLog.WriteString(fmt.Sprintf("- %s (CURRENT)\n", contextName))
				} else {
					diagnosticLog.WriteString(fmt.Sprintf("- %s\n", contextName))
				}
			}

			// Add additional diagnostic info about expected contexts
			mcContextName := utils.BuildMcContext(desiredMc)
			diagnosticLog.WriteString(fmt.Sprintf("\nDiagnostic Information:\n"))
			diagnosticLog.WriteString(fmt.Sprintf("- Expected MC context: %s (exists: %v)\n",
				mcContextName, config.Contexts[mcContextName] != nil))

			if desiredWc != "" {
				wcContextName := utils.BuildWcContext(desiredMc, desiredWc)
				diagnosticLog.WriteString(fmt.Sprintf("- Expected WC context: %s (exists: %v)\n",
					wcContextName, config.Contexts[wcContextName] != nil))
			}
		}

		return model.ContextSwitchAndReinitializeResultMsg{
			SwitchedContext: actualCurrentContext,
			DesiredMCName:   desiredMc,
			DesiredWCName:   desiredWc,
			DiagnosticLog:   diagnosticLog.String(),
			Err:             nil,
		}
	}
}

// fetchClusterListCmd creates a tea.Cmd to asynchronously fetch the list of available management and workload clusters.
// This is typically used to populate autocompletion suggestions for the new connection input.
// Returns a tea.Cmd that, when run, will call utils.GetClusterInfo and send a clusterListResultMsg.
func FetchClusterListCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := utils.GetClusterInfo()
		return model.ClusterListResultMsg{Info: info, Err: err}
	}
}

// PredefinedMcpServer struct, PredefinedMcpServers variable, and StartAndManageMcpProcess, StartAllMcpServersNonTUI
// have been moved to the internal/mcpserver package.

// startMcpProxiesCmd creates a slice of tea.Cmds, one for each predefined MCP proxy.
func StartMcpProxiesCmd(mcpServerConfig []mcpserver.MCPServerConfig, proxySvc service.MCPProxyService, tuiChan chan tea.Msg) []tea.Cmd {
	var commandsToBatch []tea.Cmd

	if len(mcpServerConfig) == 0 {
		cmd := func() tea.Msg {
			return model.McpServerStatusUpdateMsg{Label: "MCP Proxies", Status: "Info", OutputLog: "No predefined MCP servers configured."}
		}
		commandsToBatch = append(commandsToBatch, cmd)
		return commandsToBatch
	}

	for _, serverCfg := range mcpServerConfig {
		// Capture range variable for the closure
		capturedServerCfg := serverCfg

		proxyStartCmd := func() tea.Msg {
			label := capturedServerCfg.Name

			// Define the McpUpdateFunc for TUI mode
			tuiUpdateFn := func(update mcpserver.McpProcessUpdate) {
				if tuiChan != nil {
					tuiChan <- model.McpServerStatusUpdateMsg{
						Label:     update.Label,
						PID:       update.PID,
						Status:    update.Status,
						OutputLog: update.OutputLog,
						Err:       update.Err,
					}
				}
			}

			// mcpserver.StartAndManageIndividualMcpServer prepares and starts the exec.Cmd.
			// For TUI mode, WaitGroup is not used, so pass nil.
			stopChan, pid, startErr := proxySvc.Start(capturedServerCfg, tuiUpdateFn)

			initialStatusMsg := fmt.Sprintf("Initializing proxy for %s...", label)
			if startErr != nil {
				initialStatusMsg = fmt.Sprintf("Failed to start %s: %v", label, startErr)
			}

			return model.McpServerSetupCompletedMsg{
				Label:    label,
				StopChan: stopChan,
				PID:      pid,
				Status:   initialStatusMsg,
				Err:      startErr,
			}
		}
		commandsToBatch = append(commandsToBatch, proxyStartCmd)
	}

	if len(commandsToBatch) == 0 {
		// This case should not be hit if PredefinedMcpServers is not empty,
		// as a cmd is created for each.
		return nil
	}
	return commandsToBatch
}
