package tui

import (
	"bufio"
	"envctl/internal/utils"
	"fmt"
	"io"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// waitForPortForwardActivity is DEPRECATED for client-go based port-forwarding.
// Output and status will be handled by messages sent via TUIChannel from utils.StartPortForwardClientGo.
func waitForPortForwardActivity(label string, streamType string, rc io.ReadCloser) tea.Cmd {
	return func() tea.Msg {
		if rc == nil {
			return portForwardStreamEndedMsg{label: label, streamType: streamType} // This msg type might also be deprecated
		}
		scanner := bufio.NewScanner(rc)
		if scanner.Scan() {
			// This message type needs review; direct output might not be the primary way info is conveyed.
			return portForwardOutputMsg{label: label, streamType: streamType, line: scanner.Text()}
		}
		err := scanner.Err()
		if err != nil {
			// This message type needs review for client-go.
			return portForwardErrorMsg{label: label, err: err}
		}
		return portForwardStreamEndedMsg{label: label, streamType: streamType} // This msg type might also be deprecated
	}
}

// fetchNodeStatusCmd creates a tea.Cmd to get node status for a given cluster.
// clusterNameToFetchStatusFor: The short name of the MC (e.g., "alba") or WC (e.g., "deu01").
//
//	For WCs, if initially coming from CLI, it might be "mc-wc" (e.g. "alba-deu01").
//
// isMC: True if fetching for the MC.
// mcNameIfWC: The short name of the MC, used only if isMC is false to help form the full WC context.
func fetchNodeStatusCmd(clusterNameToFetchStatusFor string, isMC bool, mcNameIfWC string) tea.Cmd {
	return func() tea.Msg {
		var contextClusterPart string

		if clusterNameToFetchStatusFor == "" {
			return nodeStatusMsg{clusterShortName: clusterNameToFetchStatusFor, forMC: isMC, err: fmt.Errorf("cluster name for health check is empty")}
		}

		if isMC {
			contextClusterPart = clusterNameToFetchStatusFor
		} else { // For WC
			if mcNameIfWC != "" && !strings.HasPrefix(clusterNameToFetchStatusFor, mcNameIfWC+"-") {
				// If wc name (clusterNameToFetchStatusFor) doesn't already look like "mc-wc", construct it using mcNameIfWC.
				contextClusterPart = mcNameIfWC + "-" + clusterNameToFetchStatusFor
			} else {
				// clusterNameToFetchStatusFor already looks like "mc-wc", or mcNameIfWC is empty.
				// If mcNameIfWC is empty, we hope clusterNameToFetchStatusFor is the full "mc-wc" form.
				contextClusterPart = clusterNameToFetchStatusFor
			}
		}

		if contextClusterPart == "" { // Should be caught by earlier check, but as safeguard.
			return nodeStatusMsg{clusterShortName: clusterNameToFetchStatusFor, forMC: isMC, err: fmt.Errorf("derived cluster part for context is empty")}
		}

		fullContextName := contextClusterPart
		if !strings.HasPrefix(contextClusterPart, "teleport.giantswarm.io-") {
			fullContextName = "teleport.giantswarm.io-" + contextClusterPart
		}

		// Ensure fullContextName is not just the prefix if contextClusterPart was somehow empty and skipped previous checks.
		if fullContextName == "teleport.giantswarm.io-" {
			return nodeStatusMsg{clusterShortName: clusterNameToFetchStatusFor, forMC: isMC, err: fmt.Errorf("malformed full context name (prefix only)")}
		}

		// utils.GetNodeStatus would eventually use client-go
		ready, total, err := utils.GetNodeStatus(fullContextName)
		// clusterShortName in the msg is the original name passed, used by the model for logging against its m.managementCluster/m.workloadCluster fields.
		return nodeStatusMsg{clusterShortName: clusterNameToFetchStatusFor, forMC: isMC, readyNodes: ready, totalNodes: total, err: err}
	}
}

// getCurrentKubeContextCmd is a helper to create a tea.Cmd for fetching the current kube context.
func getCurrentKubeContextCmd() tea.Cmd {
	return func() tea.Msg {
		// utils.GetCurrentKubeContext would eventually use client-go
		currentCtx, err := utils.GetCurrentKubeContext()
		return kubeContextResultMsg{context: currentCtx, err: err}
	}
}

// performSwitchKubeContextCmd creates a tea.Cmd that attempts to switch the kubectl context.
func performSwitchKubeContextCmd(targetContextName string) tea.Cmd {
	return func() tea.Msg {
		// utils.SwitchKubeContext would eventually use client-go
		err := utils.SwitchKubeContext(targetContextName)
		return kubeContextSwitchedMsg{TargetContext: targetContextName, err: err}
	}
}

// performKubeLoginCmd attempts to log in to the specified Kubernetes cluster.
// It now returns a kubeLoginResultMsg.
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

// performPostLoginOperationsCmd handles context switching and diagnostics after successful logins.
// It returns a contextSwitchAndReinitializeResultMsg.
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

// fetchClusterListCmd creates a tea.Cmd to get the list of available clusters.
func fetchClusterListCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := utils.GetClusterInfo()
		return clusterListResultMsg{info: info, err: err}
	}
}

// startPortForwardCmd initiates a port-forwarding process using client-go.
// It communicates progress and status back to the TUI via the provided TUIChannel.
func startPortForwardCmd(label, context, namespace, service, port string, tuiChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		sendUpdateFunc := func(status, outputLog string, isError, isReady bool) {
			// The fmt.Printf debug logs previously here were for console debugging.
			// We are now focusing on TUI-based logging for handler behavior.
			if tuiChan == nil {
				// This case should ideally not be reached if TUI is initialized correctly.
				// If it does, a console log is still valuable for critical failure.
				fmt.Printf("[CRITICAL KERNEL PANIC AVERTED] tuiChan is nil in sendUpdateFunc for label: %s. This is a bug.\n", label)
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
