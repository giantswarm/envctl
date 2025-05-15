package tui

import (
	"fmt"
	"strings"
	// "strings" // Likely not needed anymore with simplified handlers

	tea "github.com/charmbracelet/bubbletea"
)

// handlePortForwardSetupCompletedMsg handles the result of the initial port-forward setup command.
func handlePortForwardSetupCompletedMsg(m model, msg portForwardSetupCompletedMsg) (model, tea.Cmd) {
	if pf, ok := m.portForwards[msg.label]; ok {
		if msg.err != nil { // Error during synchronous setup in StartPortForwardClientGo
			pf.err = msg.err
			// msg.status might be empty if error was very early, or could be a partial status.
			// It's safer to construct a clear error status.
			pf.statusMsg = fmt.Sprintf("Setup Failed: %v", msg.err)
			pf.active = false
			pf.stopChan = nil
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s ERROR] Port-forward direct setup failed: %v. Async process not started.", msg.label, msg.err))
		} else {
			// Synchronous setup in StartPortForwardClientGo was successful.
			// msg.status contains the initial status log (e.g., "Initializing...").
			pf.stopChan = msg.stopChan
			pf.statusMsg = msg.status // Set initial status for TUI display
			pf.err = nil
			pf.active = true
			// The sendUpdate call within StartPortForwardClientGo also sent this initialStatus for logging.
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Port-forward async setup initiated. Initial TUI status: %s", msg.label, msg.status))
		}

		// Trim combined output
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
	} else {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[TUI WARNING] No PF found for Lbl['%s'] during SetupCompleted.", msg.label))
	}

	// Trim combined output - typically done at end of model.Update
	if len(m.combinedOutput) > maxCombinedOutputLines+100 {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m, nil
}

// handlePortForwardStatusUpdateMsg handles ongoing status updates from a client-go port-forward.
func handlePortForwardStatusUpdateMsg(m model, msg portForwardStatusUpdateMsg) (model, tea.Cmd) {
	if pf, ok := m.portForwards[msg.label]; ok {
		// If status is provided, update the port-forward's status message
		if msg.status != "" {
			pf.statusMsg = msg.status
			// Only log status changes to the activity log if they're meaningful
			if !strings.HasPrefix(msg.status, "Initializing") && 
			   !strings.Contains(msg.status, "Forwarding from") {
				m.combinedOutput = append(m.combinedOutput, 
					fmt.Sprintf("[%s] Status changed: %s", msg.label, msg.status))
			}
		}
		
		// Always add log output to both pf.output and combinedOutput when provided
		if msg.outputLog != "" {
			// Don't modify the original message - preserve it exactly as sent
			pf.output = append(pf.output, msg.outputLog)
			
			// Format for the combined log with a prefix
			logEntry := fmt.Sprintf("[%s] %s", msg.label, msg.outputLog)
			m.combinedOutput = append(m.combinedOutput, logEntry)
		}

		// Update port-forward state based on message flags
		if msg.isError {
			pf.active = false
			pf.forwardingEstablished = false
			
			// Add an error notification if there was no outputLog
			if msg.outputLog == "" && msg.status == "" {
				m.combinedOutput = append(m.combinedOutput, 
					fmt.Sprintf("[%s] Error occurred (no details provided)", msg.label))
			}
		} else if msg.isReady {
			pf.forwardingEstablished = true
			pf.active = true
			
			// Add a ready notification if there was no status message
			if msg.status == "" {
				m.combinedOutput = append(m.combinedOutput, 
					fmt.Sprintf("[%s] Port-forwarding established", msg.label))
			}
		}
	} else {
		// Only add this warning if the port-forward doesn't exist
		m.combinedOutput = append(m.combinedOutput, 
			fmt.Sprintf("[TUI WARNING] No port-forward found for label '%s'", msg.label))
	}

	// Trim combined output to prevent excessive growth
	if len(m.combinedOutput) > maxCombinedOutputLines+100 {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	
	// Trim port-forward's output if it exists
	if pf, ok := m.portForwards[msg.label]; ok {
		if len(pf.output) > maxCombinedOutputLines {
			pf.output = pf.output[len(pf.output)-maxCombinedOutputLines:]
		}
	}

	return m, nil
}

// getInitialPortForwardCmds now uses startPortForwardCmd.
func getInitialPortForwardCmds(m *model) []tea.Cmd {
	var pfCmds []tea.Cmd
	for _, label := range m.portForwardOrder {
		pf, isActualPortForward := m.portForwards[label]
		// Check if pf.active is true to decide to start.
		// The statusMsg is initialized to "Awaiting Setup..." in setupPortForwards.
		// It will be updated by portForwardStatusUpdateMsg once StartPortForwardClientGo sends an update.
		if isActualPortForward && pf.active { 
			if m.TUIChannel == nil {
				// This is a critical error, should ideally not happen.
				// We can update the status directly here as no command will be issued.
				if p, exists := m.portForwards[label]; exists {
					p.statusMsg = "Error: TUIChannel nil"
					p.active = false
				}
				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[CRITICAL ERROR] TUIChannel is nil for %s. PF not started.", label))
				continue
			}
			pfCmds = append(pfCmds, startPortForwardCmd(pf.label, pf.context, pf.namespace, pf.service, pf.port, m.TUIChannel))
		}
	}
	return pfCmds
}

