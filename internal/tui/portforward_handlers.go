package tui

import (
	"envctl/internal/portforwarding"
	"fmt"
	"time"

	// "strings" // Likely not needed anymore with simplified handlers

	tea "github.com/charmbracelet/bubbletea"
)

// setupPortForwards initializes or re-initializes the port-forwarding configurations.
// It creates PortForwardConfig objects for each desired port-forward and stores them
// in the model, wrapped by the TUI's portForwardProcess struct.
func setupPortForwards(m *model, mcName, wcName string) {
	m.portForwards = make(map[string]*portForwardProcess) // Resetting
	m.portForwardOrder = make([]string, 0)

	// Add context pane keys first for navigation order
	m.portForwardOrder = append(m.portForwardOrder, mcPaneFocusKey)
	if wcName != "" {
		m.portForwardOrder = append(m.portForwardOrder, wcPaneFocusKey)
	}

	// kubeConfigPath can be set here if a specific path is needed.
	// For now, we rely on kubectl's default behavior or a path set elsewhere.
	kubeConfigPath := ""

	addPf := func(label, context, namespace, service, localPort, remotePort, bindAddress string, isWc bool) {
		cfg := portforwarding.PortForwardConfig{
			Label:          label,
			InstanceKey:    label, // Using label as the unique key
			KubeContext:    context,
			Namespace:      namespace,
			ServiceName:    service,
			LocalPort:      localPort,
			RemotePort:     remotePort,
			BindAddress:    bindAddress,
			KubeConfigPath: kubeConfigPath, // This can be populated if a specific kubeconfig is required
		}
		m.portForwardOrder = append(m.portForwardOrder, label)
		m.portForwards[label] = &portForwardProcess{
			label:     label,
			config:    cfg,
			active:    true, // Default to active, TUI can toggle
			statusMsg: "Awaiting Setup...",
		}
	}

	// Prometheus for MC
	if mcName != "" {
		addPf(
			"Prometheus (MC)",
			"teleport.giantswarm.io-"+mcName,
			"mimir",
			"service/mimir-query-frontend",
			"8080", "8080", "127.0.0.1",
			false,
		)
		// Grafana for MC
		addPf(
			"Grafana (MC)",
			"teleport.giantswarm.io-"+mcName,
			"monitoring",
			"service/grafana",
			"3000", "3000", "127.0.0.1",
			false,
		)
	}

	if wcName != "" {
		originalModelMcName := m.managementCluster
		m.managementCluster = mcName
		actualWcContextPart := m.getWorkloadClusterContextIdentifier()
		m.managementCluster = originalModelMcName
		addPf(
			"Alloy Metrics (WC)",
			"teleport.giantswarm.io-"+actualWcContextPart,
			"kube-system",
			"service/alloy-metrics-cluster",
			"12345", "12345", "127.0.0.1",
			true,
		)
	} else if mcName != "" {
		addPf(
			"Alloy Metrics (MC)",
			"teleport.giantswarm.io-"+mcName,
			"kube-system",
			"service/alloy-metrics-cluster",
			"12345", "12345", "127.0.0.1",
			false,
		)
	}
}

// handlePortForwardSetupResultMsg processes the message received after the initial call to
// portforwarding.StartAndManageIndividualPortForward.
func handlePortForwardSetupResultMsg(m model, msg portForwardSetupResultMsg) (model, tea.Cmd) {
	m.isLoading = false // Assume this setup result concludes the loading for this specific operation
	var clearStatusBarCmd tea.Cmd

	if pf, ok := m.portForwards[msg.InstanceKey]; ok {
		if msg.Err != nil {
			pf.err = msg.Err
			pf.statusMsg = fmt.Sprintf("Setup Failed: %v", msg.Err)
			pf.active = false // If setup fails, mark as inactive
			pf.stopChan = nil
			pf.cmd = nil
			pf.running = false
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s ERROR] Port-forward setup failed: %v", msg.InstanceKey, msg.Err))
			clearStatusBarCmd = m.setStatusMessage(fmt.Sprintf("[%s] PF Setup Failed", msg.InstanceKey), StatusBarError, 5*time.Second)
		} else {
			pf.stopChan = msg.StopChan
			pf.cmd = msg.Cmd
			pf.pid = msg.InitialPID
			pf.statusMsg = "Initializing..." // Initial status, will be updated by core updates
			pf.err = nil
			pf.active = true  // Successfully initiated
			pf.running = true // Assume running until a stop/error core update
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Port-forward async setup initiated.", msg.InstanceKey))
			clearStatusBarCmd = m.setStatusMessage(fmt.Sprintf("[%s] PF setup initiated.", msg.InstanceKey), StatusBarInfo, 3*time.Second)
		}
	} else {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[TUI WARNING] No Port-forward found for key['%s'] during SetupResult.", msg.InstanceKey))
		// Optionally set a status bar message for this warning too
	}

	// Trim combined output - typically done at end of model.Update
	if len(m.combinedOutput) > maxCombinedOutputLines+100 {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m, clearStatusBarCmd
}

// handlePortForwardCoreUpdateMsg processes asynchronous status updates from the core portforwarding package.
func handlePortForwardCoreUpdateMsg(m model, msg portForwardCoreUpdateMsg) (model, tea.Cmd) {
	update := msg.update
	if pf, ok := m.portForwards[update.InstanceKey]; ok {
		pf.statusMsg = update.StatusMsg
		pf.err = update.Error
		pf.pid = update.PID
		pf.running = update.Running

		if update.OutputLog != "" {
			logEntry := fmt.Sprintf("[%s] %s", update.InstanceKey, update.OutputLog)
			m.combinedOutput = append(m.combinedOutput, logEntry)
			pf.output = append(pf.output, update.OutputLog) // Also add to individual panel log
			if len(pf.output) > maxPanelLogLines {          // Assuming maxPanelLogLines is defined elsewhere (e.g. model.go)
				pf.output = pf.output[len(pf.output)-maxPanelLogLines:]
			}
		}

		if update.Error != nil {
			pf.active = false // Typically an error means it's no longer active/running as intended
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s ERROR] %s: %v", update.InstanceKey, update.StatusMsg, update.Error))
		}

		if !update.Running {
			pf.active = false // If core reports not running, TUI should reflect that.
		}

	} else {
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[TUI WARNING] No Port-forward found for key['%s'] during CoreUpdate.", update.InstanceKey))
	}

	if len(m.combinedOutput) > maxCombinedOutputLines+100 {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}
	return m, nil
}

// getInitialPortForwardCmds generates tea.Cmds to start all configured port-forwards.
func getInitialPortForwardCmds(m *model) []tea.Cmd {
	var pfCmds []tea.Cmd
	for _, label := range m.portForwardOrder {
		pfProcess, isActualPortForward := m.portForwards[label]
		if isActualPortForward && pfProcess.active { // Only start if marked active
			if m.TUIChannel == nil {
				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[CRITICAL ERROR] TUIChannel is nil for %s. PF not started.", label))
				pfProcess.statusMsg = "Error: TUIChannel nil"
				pfProcess.active = false
				continue
			}

			// Create a new variable for the loop iteration to avoid closure issues
			currentPfConfig := pfProcess.config

			cmdFunc := func() tea.Msg {
				// Define the update callback function for the core package
				tuiUpdateFn := func(update portforwarding.PortForwardProcessUpdate) {
					m.TUIChannel <- portForwardCoreUpdateMsg{update: update}
				}

				// Call the core function to start and manage the port forward
				cmd, stopChan, err := portforwarding.StartAndManageIndividualPortForward(currentPfConfig, tuiUpdateFn)

				initialPID := 0
				if cmd != nil && cmd.Process != nil {
					initialPID = cmd.Process.Pid
				}

				return portForwardSetupResultMsg{
					InstanceKey: currentPfConfig.InstanceKey,
					Cmd:         cmd,
					StopChan:    stopChan,
					InitialPID:  initialPID,
					Err:         err,
				}
			}
			pfCmds = append(pfCmds, cmdFunc)
		}
	}
	return pfCmds
}

// createRestartPortForwardCmd generates a tea.Cmd that restarts a single existing
// port-forward process.  It is used by the MCP restart handler to ensure that
// dependent port-forwards are up before the MCP proxy comes back online.
func createRestartPortForwardCmd(m *model, pfProcess *portForwardProcess) tea.Cmd {
	if pfProcess == nil {
		return nil
	}

	// Close existing stopChan if running
	if pfProcess.stopChan != nil {
		safeCloseChan(pfProcess.stopChan)
		pfProcess.stopChan = nil
	}

	pfProcess.statusMsg = "Restarting..."
	pfProcess.output = nil
	pfProcess.err = nil
	pfProcess.running = false
	pfProcess.pid = 0

	currentPfConfig := pfProcess.config

	return func() tea.Msg {
		// Define the update callback function for the core package
		tuiUpdateFn := func(update portforwarding.PortForwardProcessUpdate) {
			m.TUIChannel <- portForwardCoreUpdateMsg{update: update}
		}

		cmd, stopChan, err := portforwarding.StartAndManageIndividualPortForward(currentPfConfig, tuiUpdateFn)

		initialPID := 0
		if cmd != nil && cmd.Process != nil {
			initialPID = cmd.Process.Pid
		}

		return portForwardSetupResultMsg{
			InstanceKey: currentPfConfig.InstanceKey,
			Cmd:         cmd,
			StopChan:    stopChan,
			InitialPID:  initialPID,
			Err:         err,
		}
	}
}
