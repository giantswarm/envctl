package tui

import (
	"envctl/internal/utils"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// fetchNodeStatusCmd creates a tea.Cmd to asynchronously fetch the node status.
// - clusterIdentifier: The canonical cluster identifier part of the context name (e.g., "myinstallation" for MC, "myinstallation-myworkloadcluster" for WC).
// - isMC: Boolean indicating if the status is for a Management Cluster.
// - originalClusterShortName: The original short name of the cluster (e.g., "myinstallation" or "myworkloadcluster"), used for tagging the result message.
// Returns a tea.Cmd that, when run, will call utils.GetNodeStatusClientGo and send a nodeStatusMsg.
func fetchNodeStatusCmd(clusterIdentifier string, isMC bool, originalClusterShortName string) tea.Cmd {
	return func() tea.Msg {
		if clusterIdentifier == "" {
			return nodeStatusMsg{clusterShortName: originalClusterShortName, forMC: isMC, err: fmt.Errorf("cluster identifier for health check is empty")}
		}

		// clusterIdentifier is already the correct part, e.g., "myinstallation" or "myinstallation-myworkloadcluster".
		// We just need to prepend the teleport prefix if it's not already a full context name (though it shouldn't be).
		fullContextName := clusterIdentifier
		if !strings.HasPrefix(clusterIdentifier, "teleport.giantswarm.io-") {
			fullContextName = "teleport.giantswarm.io-" + clusterIdentifier
		} else {
			// If it somehow already has the prefix, ensure it doesn't get double prefixed by a mistake upstream.
			// However, the expectation is clusterIdentifier is just the cluster part.
		}

		// Ensure fullContextName is not just the prefix if clusterIdentifier was somehow empty and skipped previous checks.
		if fullContextName == "teleport.giantswarm.io-" && clusterIdentifier == "" { // defensive check
			return nodeStatusMsg{clusterShortName: originalClusterShortName, forMC: isMC, err: fmt.Errorf("malformed full context name (prefix only from empty identifier)")}
		}

		ready, total, err := utils.GetNodeStatusClientGo(fullContextName)
		return nodeStatusMsg{clusterShortName: originalClusterShortName, forMC: isMC, readyNodes: ready, totalNodes: total, err: err}
	}
}

// getCurrentKubeContextCmd creates a tea.Cmd to asynchronously fetch the current active Kubernetes context.
// Returns a tea.Cmd that, when run, will call utils.GetCurrentKubeContext and send a kubeContextResultMsg.
func getCurrentKubeContextCmd() tea.Cmd {
	return func() tea.Msg {
		// utils.GetCurrentKubeContext would eventually use client-go
		currentCtx, err := utils.GetCurrentKubeContext()
		return kubeContextResultMsg{context: currentCtx, err: err}
	}
}

// performSwitchKubeContextCmd creates a tea.Cmd to attempt switching the active Kubernetes context.
// - targetContextName: The full name of the Kubernetes context to switch to.
// Returns a tea.Cmd that, when run, will call utils.SwitchKubeContext and send a kubeContextSwitchedMsg.
func performSwitchKubeContextCmd(targetContextName string) tea.Cmd {
	return func() tea.Msg {
		// utils.SwitchKubeContext would eventually use client-go
		err := utils.SwitchKubeContext(targetContextName)
		return kubeContextSwitchedMsg{TargetContext: targetContextName, err: err}
	}
}

// performKubeLoginCmd creates a tea.Cmd to attempt a `tsh kube login` to the specified cluster.
// This is part of the new connection flow.
// - clusterName: The name of the cluster to log into (can be MC name or full WC name like "mc-wc").
// - isMC: True if this login attempt is for a Management Cluster.
// - desiredWcShortNameToCarry: If isMC is true, this holds the short name of the desired WC to be used in the next step.
// Returns a tea.Cmd that, when run, will call utils.LoginToKubeCluster and send a kubeLoginResultMsg.
func performKubeLoginCmd(clusterName string, isMC bool, desiredWcShortNameToCarry string) tea.Cmd {
	return func() tea.Msg {
		stdout, stderr, err := utils.LoginToKubeCluster(clusterName)
		return kubeLoginResultMsg{
			clusterName:        clusterName,
			isMC:               isMC,
			desiredWcShortName: desiredWcShortNameToCarry,
			loginStdout:        stdout,
			loginStderr:        stderr,
			err:                err,
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
func performPostLoginOperationsCmd(targetKubeContext, desiredMc, desiredWc string) tea.Cmd {
	return func() tea.Msg {
		var diagnosticLog strings.Builder
		diagnosticLog.WriteString(fmt.Sprintf("Attempting to switch context to: %s\n", targetKubeContext))

		// utils.SwitchKubeContext would eventually use client-go
		err := utils.SwitchKubeContext(targetKubeContext)
		if err != nil {
			diagnosticLog.WriteString(fmt.Sprintf("SwitchKubeContext error: %v\n", err))
			return contextSwitchAndReinitializeResultMsg{
				err:           fmt.Errorf("failed to switch context to %s: %w", targetKubeContext, err),
				desiredMcName: desiredMc,
				desiredWcName: desiredWc,
				diagnosticLog: diagnosticLog.String(),
			}
		}
		diagnosticLog.WriteString("SwitchKubeContext successful.\n")

		// utils.GetCurrentKubeContext would eventually use client-go
		actualCurrentContext, err := utils.GetCurrentKubeContext()
		if err != nil {
			diagnosticLog.WriteString(fmt.Sprintf("GetCurrentKubeContext error: %v\n", err))
			return contextSwitchAndReinitializeResultMsg{
				err:             fmt.Errorf("failed to get current context after switch: %w", err),
				switchedContext: targetKubeContext,
				desiredMcName:   desiredMc,
				desiredWcName:   desiredWc,
				diagnosticLog:   diagnosticLog.String(),
			}
		}
		diagnosticLog.WriteString(fmt.Sprintf("GetCurrentKubeContext successful: %s\n", actualCurrentContext))

		// This kubectl call would also ideally use client-go
		contextsListCmd := exec.Command("kubectl", "config", "get-contexts", "-o", "name")
		contextsListOutput, contextsListErr := contextsListCmd.Output()
		if contextsListErr != nil {
			diagnosticLog.WriteString(fmt.Sprintf("kubectl config get-contexts error: %v\nOutput: %s\n", contextsListErr, string(contextsListOutput)))
		} else {
			diagnosticLog.WriteString(fmt.Sprintf("kubectl config get-contexts output:\n%s\n", string(contextsListOutput)))
		}

		return contextSwitchAndReinitializeResultMsg{
			switchedContext: actualCurrentContext,
			desiredMcName:   desiredMc,
			desiredWcName:   desiredWc,
			diagnosticLog:   diagnosticLog.String(),
			err:             nil,
		}
	}
}

// fetchClusterListCmd creates a tea.Cmd to asynchronously fetch the list of available management and workload clusters.
// This is typically used to populate autocompletion suggestions for the new connection input.
// Returns a tea.Cmd that, when run, will call utils.GetClusterInfo and send a clusterListResultMsg.
func fetchClusterListCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := utils.GetClusterInfo()
		return clusterListResultMsg{info: info, err: err}
	}
}

// startPortForwardCmd creates a tea.Cmd to initiate a port-forwarding process using the client-go library.
// The actual port-forwarding is handled in a separate goroutine (launched by utils.StartPortForwardClientGo).
// This command function itself returns a portForwardSetupCompletedMsg once the synchronous part of the setup is done.
// Ongoing status updates from the port-forwarding goroutine are sent to the TUI via the provided tuiChan.
// - label: A user-friendly label for this port-forward (e.g., "Prometheus (MC)").
// - context: The Kubernetes context to use for this port-forward.
// - namespace: The Kubernetes namespace of the target service.
// - service: The name of the Kubernetes service to connect to.
// - port: The port mapping string (e.g., "localPort:remotePort").
// - tuiChan: The channel used by the port-forwarding goroutine to send portForwardStatusUpdateMsg messages back to the TUI.
// Returns a tea.Cmd that, when run, calls utils.StartPortForwardClientGo and then sends a portForwardSetupCompletedMsg.
func startPortForwardCmd(label, context, namespace, service, port string, tuiChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		sendUpdateFunc := func(status, outputLog string, isError, isReady bool) {
			// The fmt.Printf debug logs previously here were for console debugging.
			// We are now focusing on TUI-based logging for handler behavior.
			if tuiChan == nil {
				// This case should ideally not be reached if TUI is initialized correctly.
				// If it does, a console log is still valuable for critical failure.
				fmt.Printf("[CRITICAL ERROR] tuiChan is nil in sendUpdateFunc for label: %s. This is a bug.\n", label)
				return // Avoid panic
			}
			tuiChan <- portForwardStatusUpdateMsg{
				label:     label,
				status:    status,
				outputLog: outputLog,
				isError:   isError,
				isReady:   isReady,
			}
		}

		// utils.StartPortForwardClientGo now returns (chan struct{}, string, error)
		// The string is the initial status message if synchronous setup was successful.
		stopChan, initialStatus, initialError := utils.StartPortForwardClientGo(context, namespace, service, port, label, sendUpdateFunc)

		return portForwardSetupCompletedMsg{
			label:    label,
			stopChan: stopChan,
			status:   initialStatus, // Pass status from the setup function
			err:      initialError,
		}
	}
}
