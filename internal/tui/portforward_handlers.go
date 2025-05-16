package tui

import (
	"fmt"
	"strings"

	// "strings" // Likely not needed anymore with simplified handlers

	tea "github.com/charmbracelet/bubbletea"
)

// setupPortForwards initializes or re-initializes the port-forwarding configurations.
// It clears any existing port forwards and sets up new ones based on the provided
// management cluster (mcName) and workload cluster (wcName).
// This function defines the services to be port-forwarded (e.g., Prometheus, Grafana, Alloy Metrics)
// and their respective configurations.
// It directly modifies the model's portForwards and portForwardOrder fields.
func setupPortForwards(m *model, mcName, wcName string) {
	// Clear existing port forwards before setting up new ones
	m.portForwards = make(map[string]*portForwardProcess)
	m.portForwardOrder = make([]string, 0)

	// Add context pane keys first for navigation order
	m.portForwardOrder = append(m.portForwardOrder, mcPaneFocusKey)
	if wcName != "" {
		m.portForwardOrder = append(m.portForwardOrder, wcPaneFocusKey)
	}

	// Prometheus for MC
	if mcName != "" {
		promLabel := "Prometheus (MC)"
		m.portForwardOrder = append(m.portForwardOrder, promLabel)
		m.portForwards[promLabel] = &portForwardProcess{
			label:     promLabel,
			port:      "8080:8080",
			isWC:      false,
			context:   "teleport.giantswarm.io-" + mcName, // mcName is sufficient, no need for m.getManagementClusterContextIdentifier()
			namespace: "mimir",
			service:   "service/mimir-query-frontend",
			active:    true,
			statusMsg: "Awaiting Setup...",
		}

		// Grafana for MC
		grafanaLabel := "Grafana (MC)"
		m.portForwardOrder = append(m.portForwardOrder, grafanaLabel)
		m.portForwards[grafanaLabel] = &portForwardProcess{
			label:     grafanaLabel,
			port:      "3000:3000",
			isWC:      false,
			context:   "teleport.giantswarm.io-" + mcName, // mcName is sufficient
			namespace: "monitoring",
			service:   "service/grafana",
			active:    true,
			statusMsg: "Awaiting Setup...",
		}
	}

	// Alloy Metrics for WC
	if wcName != "" {
		alloyLabel := "Alloy Metrics (WC)"
		m.portForwardOrder = append(m.portForwardOrder, alloyLabel)

		// Construct the correct context name part for WC using the model's helper.
		// This ensures consistent and correct context identifier generation.
		// We need to use mcName here as well, because getWorkloadClusterContextIdentifier
		// might rely on m.managementCluster which is set to mcName.
		// To avoid issues if m.managementCluster is not yet mcName when this is called
		// (e.g. during initial setup), we'll temporarily set it.
		// A cleaner way would be to pass mcName to getWorkloadClusterContextIdentifier
		// or make getWorkloadClusterContextIdentifier use the passed mcName/wcName.
		// For now, this direct usage of the model's method after ensuring mcName is correct is acceptable.
		originalModelMcName := m.managementCluster
		m.managementCluster = mcName                                   // Temporarily align model's MC name
		actualWcContextPart := m.getWorkloadClusterContextIdentifier() // Uses the model's method
		m.managementCluster = originalModelMcName                      // Restore original

		m.portForwards[alloyLabel] = &portForwardProcess{
			label:     alloyLabel,
			port:      "12345:12345",
			isWC:      true,
			context:   "teleport.giantswarm.io-" + actualWcContextPart,
			namespace: "kube-system",
			service:   "service/alloy-metrics-cluster",
			active:    true,
			statusMsg: "Awaiting Setup...",
		}
	} else if mcName != "" { // Alloy Metrics for MC if no WC is specified
		alloyLabel := "Alloy Metrics (MC)"
		m.portForwardOrder = append(m.portForwardOrder, alloyLabel)

		m.portForwards[alloyLabel] = &portForwardProcess{
			label:     alloyLabel,
			port:      "12345:12345",
			isWC:      false,                              // Targets MC
			context:   "teleport.giantswarm.io-" + mcName, // Context for MC
			namespace: "kube-system",                      // Assuming same namespace
			service:   "service/alloy-metrics-cluster",    // Assuming same service name
			active:    true,
			statusMsg: "Awaiting Setup...",
		}
	}
}

// handlePortForwardSetupCompletedMsg processes the message received after the synchronous part
// of a port-forward setup attempt (StartPortForwardClientGo) is finished.
// It updates the model based on whether the initial setup was successful or encountered an error.
//   - m: The current TUI model.
//   - msg: The portForwardSetupCompletedMsg containing the label of the port-forward,
//     its initial status, a stop channel (if successful), and any error encountered during setup.
//
// Returns the updated model and a nil command as no further async operations are directly initiated here.
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
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[TUI WARNING] No Port-forward found for label['%s'] during SetupCompleted.", msg.label))
	}

	// Trim combined output - typically done at end of model.Update
	if len(m.combinedOutput) > maxCombinedOutputLines+100 {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m, nil
}

// handlePortForwardStatusUpdateMsg processes asynchronous status updates received from an active port-forwarding process.
// These updates can include new log messages, changes in status (e.g., "Forwarding from...", "Error: ..."),
// or notifications about the port-forwarding becoming ready or encountering an error.
// It updates the specific port-forward's state in the model and appends relevant information to the combined activity log.
// - m: The current TUI model.
// - msg: The portForwardStatusUpdateMsg containing the label, status text, log output, and flags indicating readiness or error.
// Returns the updated model and a nil command.
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

		// Add log output to combinedOutput when provided
		if msg.outputLog != "" {
			// Don't modify the original message - preserve it exactly as sent
			// pf.output = append(pf.output, msg.outputLog) // REMOVED: This line sent logs to individual panel

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
			fmt.Sprintf("[TUI WARNING] No Port-forward found for label['%s']", msg.label))
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

// getInitialPortForwardCmds generates a slice of tea.Cmds to initiate all active port-forwarding processes
// when the TUI starts or when connections are re-initialized.
// It iterates through the configured port forwards in m.portForwardOrder and, for each active one,
// creates a startPortForwardCmd. This command, when executed, will begin the port-forwarding process
// asynchronously and send updates back to the TUI via the TUIChannel.
// - m: A pointer to the current TUI model, used to access port-forward configurations and the TUIChannel.
// Returns a slice of tea.Cmds for starting port forwards.
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
