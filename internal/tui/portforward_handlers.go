package tui

import (
	"envctl/internal/utils"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func handlePortForwardStartedMsg(m model, msg portForwardStartedMsg) (model, tea.Cmd) {
	if pf, ok := m.portForwards[msg.label]; ok {
		if pf.statusMsg != "Restart failed" {
			pf.statusMsg = fmt.Sprintf("Running (PID: %d)", msg.pid)
		}
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Process started (PID: %d)", msg.label, msg.pid))
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
	}
	return m, nil
}

func handlePortForwardOutputMsg(m model, msg portForwardOutputMsg) (model, tea.Cmd) {
	var cmds []tea.Cmd
	if pf, ok := m.portForwards[msg.label]; ok {
		line := fmt.Sprintf("[%s %s] %s", msg.label, msg.streamType, msg.line)
		pf.output = append(pf.output, msg.line)
		m.combinedOutput = append(m.combinedOutput, line)

		if !pf.forwardingEstablished && strings.Contains(msg.line, "Forwarding from") {
			pf.forwardingEstablished = true
			// Try to extract the local port more robustly
			parts := strings.Fields(msg.line)
			localPort := "unknown"
			for i, p := range parts {
				if (p == "from" || p == "From") && i+1 < len(parts) {
					addressAndPort := parts[i+1]
					lastColon := strings.LastIndex(addressAndPort, ":")
					if lastColon != -1 && lastColon+1 < len(addressAndPort) {
						localPort = addressAndPort[lastColon+1:]
						break
					}
				}
			}
			pf.statusMsg = fmt.Sprintf("Forwarding Active (Local Port: %s)", localPort)
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Confirmed: %s", msg.label, pf.statusMsg))
		}

		// Trim outputs
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
		if len(pf.output) > maxCombinedOutputLines { // Assuming same trim size for individual PFs
			pf.output = pf.output[len(pf.output)-maxCombinedOutputLines:]
		}

		// Continue listening on streams if they are not closed
		if msg.streamType == "stdout" && !pf.stdoutClosed {
			cmds = append(cmds, waitForPortForwardActivity(msg.label, "stdout", pf.stdout))
		} else if msg.streamType == "stderr" && !pf.stderrClosed {
			cmds = append(cmds, waitForPortForwardActivity(msg.label, "stderr", pf.stderr))
		}
	}
	return m, tea.Batch(cmds...)
}

func handlePortForwardErrorMsg(m model, msg portForwardErrorMsg) (model, tea.Cmd) {
	if pf, ok := m.portForwards[msg.label]; ok {
		errMsgText := fmt.Sprintf("[%s %s ERROR] %s", msg.label, msg.streamType, msg.err.Error())
		pf.err = msg.err
		pf.output = append(pf.output, "ERROR: "+msg.err.Error())
		m.combinedOutput = append(m.combinedOutput, errMsgText)

		// Update status unless it's a more specific startup/restart failure message
		if pf.statusMsg != "Failed to start" && pf.statusMsg != "Restart failed" {
			pf.statusMsg = "Error"
		}

		// Mark streams as closed based on the error message context
		// This is a heuristic; specific errors might imply specific streams closed.
		// For a general error, or if streamType is "general", it might not close them.
		if msg.streamType == "stdout" {
			pf.stdoutClosed = true
		} else if msg.streamType == "stderr" {
			pf.stderrClosed = true
		} else if msg.streamType == "general" {
			// If it's a general error (e.g. process failed to start), both streams are effectively closed.
			pf.stdoutClosed = true
			pf.stderrClosed = true
			pf.active = false // Mark as inactive if the process couldn't start or failed critically
		}

		// Trim outputs
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
		if len(pf.output) > maxCombinedOutputLines {
			pf.output = pf.output[len(pf.output)-maxCombinedOutputLines:]
		}
	}
	return m, nil
}

// handlePortForwardStreamEndedMsg handles the scenario where a port forward stream (stdout/stderr) ends.
func handlePortForwardStreamEndedMsg(m model, msg portForwardStreamEndedMsg) (model, tea.Cmd) {
	if pf, ok := m.portForwards[msg.label]; ok {
		logMsg := fmt.Sprintf("[%s %s] Stream closed.", msg.label, msg.streamType)
		m.combinedOutput = append(m.combinedOutput, logMsg)

		if msg.streamType == "stdout" {
			pf.stdoutClosed = true
		} else if msg.streamType == "stderr" {
			pf.stderrClosed = true
		}

		// If both streams are closed and the process was active, update status to Exited.
		// This doesn't mean the process necessarily exited cleanly, just that we are no longer reading from it.
		// Actual process exit might be handled by a different OS-level signal or a dedicated "process exited" message if implemented.
		if pf.stdoutClosed && pf.stderrClosed && pf.active {
			// Avoid overwriting more specific error states like "Killed", "Error", "Failed to start", "Restart failed"
			isErrorState := pf.statusMsg == "Killed" || pf.statusMsg == "Error" || pf.statusMsg == "Failed to start" || pf.statusMsg == "Restart failed"
			if !isErrorState {
				pf.statusMsg = "Exited"
			}
			// pf.active = false // Consider if connection should be marked inactive once streams close.
			// For now, keep it active as user might want to restart it.
			pf.cmd = nil // Clear the command as we are no longer tracking its streams actively.
		}

		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
	}
	return m, nil
}

// getInitialPortForwardCmds iterates over the model's portForwards and starts them,
// returning a slice of tea.Cmd for their initial activity and status updates.
// This is called from model.Init().
func getInitialPortForwardCmds(m *model) []tea.Cmd { // Pass model as pointer to modify pf status directly
	var pfCmds []tea.Cmd
	for _, label := range m.portForwardOrder {
		// Check if the label corresponds to an actual port-forward process
		// and not a special focus key like mcPaneFocusKey or wcPaneFocusKey.
		pf, isActualPortForward := m.portForwards[label]

		if isActualPortForward && pf.active { // Only proceed if it's a defined and active port-forward
			pf_loop := pf // Capture loop variable for closure
			// Attempt to start the port forward using the utility function
			cmd, stdout, stderr, err := utils.StartPortForward(pf_loop.context, pf_loop.namespace, pf_loop.service, pf_loop.port, pf_loop.label)
			if err != nil {
				// If starting fails, update the process status and send an error message.
				m.portForwards[pf_loop.label].err = err
				m.portForwards[pf_loop.label].statusMsg = "Failed to start"
				m.portForwards[pf_loop.label].stdoutClosed = true // Mark streams as closed as they won't be used.
				m.portForwards[pf_loop.label].stderrClosed = true
				pfCmds = append(pfCmds, func() tea.Msg {
					return portForwardErrorMsg{label: pf_loop.label, streamType: "general", err: fmt.Errorf("failed to start %s: %w", pf_loop.label, err)}
				})
			} else {
				// If successful, store the command and stream readers, and set initial status.
				processID := cmd.Process.Pid // Evaluate and capture the PID now.
				m.portForwards[pf_loop.label].cmd = cmd
				m.portForwards[pf_loop.label].stdout = stdout
				m.portForwards[pf_loop.label].stderr = stderr
				m.portForwards[pf_loop.label].statusMsg = "Starting..."
				// Add commands to listen for activity on stdout/stderr and a message for when it's started.
				pfCmds = append(pfCmds, waitForPortForwardActivity(pf_loop.label, "stdout", stdout))
				pfCmds = append(pfCmds, waitForPortForwardActivity(pf_loop.label, "stderr", stderr))
				pfCmds = append(pfCmds, func() tea.Msg { return portForwardStartedMsg{label: pf_loop.label, pid: processID} })
			}
		}
	}
	return pfCmds
}
