package tui

import (
	"bufio"
	"envctl/internal/mcpserver"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// restartMcpServerMsg is a message type for restarting MCP servers
type restartMcpServerMsg struct {
	Label string // Which MCP server to restart
}

// handleMcpServerSetupCompletedMsg processes the message received after the synchronous part
// of the MCP server setup attempt (startMcpServerCmd) is finished.
// It updates the model based on whether the initial setup was successful or encountered an error.
func handleMcpServerSetupCompletedMsg(m model, msg mcpServerSetupCompletedMsg) (model, tea.Cmd) {
	m.isLoading = false // Assuming this setup result (even if error) concludes loading for this specific proxy setup/restart
	var clearStatusBarCmd tea.Cmd

	if msg.Label == "kubernetes" && m.debugMode { // Specific debug only when debug mode is enabled
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[DEBUG KubeProxy] Handler: SetupCompletedMsg received for %s. Error: %v. Status: %s", msg.Label, msg.err, msg.status))
	}
	if m.mcpServers == nil {
		m.mcpServers = make(map[string]*mcpServerProcess)
	}
	if _, ok := m.mcpServers[msg.Label]; !ok {
		m.mcpServers[msg.Label] = &mcpServerProcess{label: msg.Label}
	}

	serverProc := m.mcpServers[msg.Label]

	if msg.err != nil {
		serverProc.statusMsg = fmt.Sprintf("Setup Error: %v", msg.err)
		serverProc.err = msg.err
		serverProc.active = false
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy] Setup failed: %v", msg.Label, msg.err))
		clearStatusBarCmd = m.setStatusMessage(fmt.Sprintf("[%s] MCP Setup Failed", msg.Label), StatusBarError, 5*time.Second)
	} else {
		serverProc.stopChan = msg.stopChan
		serverProc.statusMsg = msg.status // e.g., "Initializing proxy..."
		serverProc.pid = msg.pid          // PID from setup, though it might be 0 if sent later
		serverProc.active = true
		serverProc.err = nil
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy] %s", msg.Label, msg.status))
		clearStatusBarCmd = m.setStatusMessage(fmt.Sprintf("[%s] MCP %s", msg.Label, msg.status), StatusBarInfo, 3*time.Second)
		if msg.pid > 0 { // PID might still be sent via setup if available early
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy] Early PID: %d", msg.Label, msg.pid))
		}
		if msg.Label == "kubernetes" && m.debugMode { // Specific debug only when debug mode is enabled
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[DEBUG KubeProxy] Handler: Set m.mcpServers[%s].active = %t, statusMsg = %s, pid = %d", msg.Label, serverProc.active, serverProc.statusMsg, serverProc.pid))
		}
	}
	return m, clearStatusBarCmd
}

// handleMcpServerStatusUpdateMsg processes asynchronous updates from the MCP server process.
// It logs output, updates status, and handles errors or process termination.
func handleMcpServerStatusUpdateMsg(m model, msg mcpServerStatusUpdateMsg) (model, tea.Cmd) {
	if msg.Label == "kubernetes" && m.debugMode { // Specific debug only when debug mode is enabled
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[DEBUG KubeProxy] Handler: StatusUpdateMsg received for %s. Status: %s, PID: %d, Err: %v, Log: %s", msg.Label, msg.status, msg.pid, msg.err, msg.outputLog))
	}
	if m.mcpServers == nil { // Should not happen if initialized
		m.mcpServers = make(map[string]*mcpServerProcess) // Defensive initialization
	}

	serverProc, ok := m.mcpServers[msg.Label]
	if !ok {
		// Entry doesn't exist, create it. This can happen if a status update (e.g. "Running")
		// arrives before the mcpServerSetupCompletedMsg for this label.
		m.mcpServers[msg.Label] = &mcpServerProcess{label: msg.Label}
		serverProc = m.mcpServers[msg.Label]
		if msg.Label == "kubernetes" && m.debugMode { // Specific debug only when debug mode is enabled
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[DEBUG KubeProxy] Handler: Created m.mcpServers[%s] in StatusUpdateMsg.", msg.Label))
		}
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[MCP SYSTEM INFO] Created new state for MCP proxy label: %s on first status update.", msg.Label))
	}

	if msg.outputLog != "" {
		m.combinedOutput = append(m.combinedOutput, msg.outputLog) // outputLog already has prefix like "[<label> STDOUT/STDERR]"
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}
		serverProc.output = append(serverProc.output, msg.outputLog)
		const maxMcpServerOutputLines = 50
		if len(serverProc.output) > maxMcpServerOutputLines {
			serverProc.output = serverProc.output[len(serverProc.output)-maxMcpServerOutputLines:]
		}
	}

	if msg.pid > 0 && serverProc.pid == 0 { // Store PID if received and not already set
		serverProc.pid = msg.pid
		// Log is already sent by the command: e.g. "Proxy for <label> (PID: %d) listening on..."
	}

	if msg.status != "" {
		serverProc.statusMsg = msg.status
	}

	var cmds []tea.Cmd // Slice to hold commands, including status bar clear command

	if msg.err != nil {
		serverProc.err = msg.err
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy ERROR] %v", msg.Label, msg.err))
		cmds = append(cmds, m.setStatusMessage(fmt.Sprintf("[%s] MCP Error", msg.Label), StatusBarError, 5*time.Second))

		// Check if this is a critical error that needs recovery
		if shouldAttemptRecovery(msg.status, msg.err) {
			serverProc.statusMsg = "Recovery attempt..."

			// Create command for recovery attempt
			recoveryCmd := func() tea.Msg {
				// Wait a bit before retry
				time.Sleep(3 * time.Second)
				return restartMcpServerMsg{Label: msg.Label}
			}
			cmds = append(cmds, recoveryCmd)
		}
	}

	if strings.Contains(strings.ToLower(msg.status), "stopped") {
		serverProc.active = false
		// Set status bar message for stopped state, unless an error message was already set
		if serverProc.err == nil { // Only show 'Stopped' if not already showing an error for this server
			cmds = append(cmds, m.setStatusMessage(fmt.Sprintf("[%s] MCP Stopped", msg.Label), StatusBarInfo, 3*time.Second))
		}
	} else if strings.Contains(strings.ToLower(msg.status), "running") { // Added for running status
		if serverProc.err == nil { // Only show 'Running' if no error
			cmds = append(cmds, m.setStatusMessage(fmt.Sprintf("[%s] MCP Running", msg.Label), StatusBarSuccess, 3*time.Second))
		}
	}

	if msg.Label == "kubernetes" && m.debugMode { // Specific debug only when debug mode is enabled
		updatedPid := serverProc.pid          // pid might have been updated from msg.pid
		updatedStatus := serverProc.statusMsg // statusMsg might have been updated from msg.status
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[DEBUG KubeProxy] Handler: Updated m.mcpServers[%s].statusMsg = %s, pid = %d", msg.Label, updatedStatus, updatedPid))
	}
	return m, tea.Batch(cmds...) // Batch any accumulated commands
}

// shouldAttemptRecovery determines if the MCP server process should be restarted based on the error
func shouldAttemptRecovery(status string, err error) bool {
	if err == nil {
		return false
	}

	// Don't attempt recovery if process was stopped intentionally
	if strings.Contains(strings.ToLower(status), "stopping") ||
		strings.Contains(strings.ToLower(status), "stopped") {
		return false
	}

	// Check for common errors that might be recoverable
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such file") ||
		strings.Contains(errStr, "process exited") {
		return true
	}

	return false
}

// handleRestartMcpServerMsg handles the restart of an MCP server
func handleRestartMcpServerMsg(m model, msg restartMcpServerMsg) (model, tea.Cmd) {
	m.isLoading = true // Set loading before dispatching restart command
	clearStatusCmd := m.setStatusMessage(fmt.Sprintf("[%s] MCP Restarting...", msg.Label), StatusBarInfo, 3*time.Second)
	m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy] Attempting auto-recovery...", msg.Label))

	// Find the configuration for this server
	var serverConfig mcpserver.PredefinedMcpServer
	for _, cfg := range mcpserver.PredefinedMcpServers {
		if cfg.Name == msg.Label {
			serverConfig = cfg
			break
		}
	}

	// Clear error state
	if proc, ok := m.mcpServers[msg.Label]; ok {
		proc.err = nil
		proc.statusMsg = "Restarting..."
	}

	// Start a new instance of the MCP server
	restartProxyCmd := func() tea.Msg {
		return startMcpProxyCmdForServer(serverConfig, m.TUIChannel)()
	}
	return m, tea.Batch(clearStatusCmd, restartProxyCmd)
}

// startMcpProxyCmdForServer creates a tea.Cmd function for a specific MCP server
func startMcpProxyCmdForServer(serverCfg mcpserver.PredefinedMcpServer, tuiChan chan tea.Msg) func() tea.Msg {
	return func() tea.Msg {
		label := serverCfg.Name
		proxyPort := serverCfg.ProxyPort

		// Build arguments for mcp-proxy
		proxyArgs := []string{
			"-p", fmt.Sprintf("%d", proxyPort),
			"-c", serverCfg.Command,
		}

		// Add arguments for the underlying command
		for _, arg := range serverCfg.Args {
			proxyArgs = append(proxyArgs, "-a", arg)
		}

		// Add environment variables
		for key, value := range serverCfg.Env {
			proxyArgs = append(proxyArgs, "-e", fmt.Sprintf("%s=%s", key, value))
		}

		// Create command to start mcp-proxy
		cmd := exec.Command("mcp-proxy", proxyArgs...)

		// Set process group for easier termination
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		// Create pipes for stdout and stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return mcpServerSetupCompletedMsg{
				Label:    label,
				pid:      0,
				status:   "Setup Error",
				err:      fmt.Errorf("failed to create stdout pipe: %w", err),
				stopChan: nil,
			}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return mcpServerSetupCompletedMsg{
				Label:    label,
				pid:      0,
				status:   "Setup Error",
				err:      fmt.Errorf("failed to create stderr pipe: %w", err),
				stopChan: nil,
			}
		}

		// Start the process
		if err := cmd.Start(); err != nil {
			return mcpServerSetupCompletedMsg{
				Label:    label,
				pid:      0,
				status:   "Start Error",
				err:      fmt.Errorf("failed to start process: %w", err),
				stopChan: nil,
			}
		}

		// Create stop channel for signaling termination
		stopChan := make(chan struct{})

		// Start goroutines for handling output
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				tuiChan <- mcpServerStatusUpdateMsg{
					Label:     label,
					status:    "",
					outputLog: fmt.Sprintf("[%s STDOUT] %s", label, line),
				}
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				tuiChan <- mcpServerStatusUpdateMsg{
					Label:     label,
					status:    "",
					outputLog: fmt.Sprintf("[%s STDERR] %s", label, line),
				}
			}
		}()

		// Start goroutine for monitoring process completion
		go func() {
			waitErr := cmd.Wait()
			if waitErr != nil {
				tuiChan <- mcpServerStatusUpdateMsg{
					Label:  label,
					status: "Error",
					err:    fmt.Errorf("process exited with error: %w", waitErr),
				}
			} else {
				tuiChan <- mcpServerStatusUpdateMsg{
					Label:  label,
					status: "Stopped",
				}
			}
		}()

		// Return setup completed message
		return mcpServerSetupCompletedMsg{
			Label:    label,
			pid:      cmd.Process.Pid,
			status:   fmt.Sprintf("Running on port %d", proxyPort),
			stopChan: stopChan,
		}
	}
}
