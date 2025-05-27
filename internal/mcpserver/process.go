package mcpserver

import (
	"bufio"
	"envctl/internal/config" // Added for new config type
	"envctl/pkg/logging"     // Added for logging
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

var execCommand = exec.Command

// StartAndManageIndividualMcpServer prepares, starts, and manages a single MCP server process.
// It calls updateFn with McpDiscreteStatusUpdate for state changes and uses pkg/logging for verbose output.
func StartAndManageIndividualMcpServer(
	serverConfig config.MCPServerDefinition, // Updated type
	updateFn McpUpdateFunc, // Now func(update McpDiscreteStatusUpdate)
	wg *sync.WaitGroup,
) (pid int, stopChan chan struct{}, initialError error) {

	label := serverConfig.Name
	subsystem := "MCPServer-" + label

	// Logging needs to be adjusted as ProxyPort is available in serverConfig.
	logging.Info(subsystem, "Initializing MCP server %s (underlying: %s)", label, strings.Join(serverConfig.Command, " "))
	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessInitializing", PID: 0, ProxyPort: 0})
	}

	// For now, assuming mcp-proxy is available and takes the command to execute.
	// The MCPServerDefinition.Command is []string, first element is command, rest are args.
	if len(serverConfig.Command) == 0 {
		errMsg := fmt.Errorf("command not defined for MCP server %s", label)
		logging.Error(subsystem, errMsg, "Cannot start MCP server")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg, ProxyPort: 0})
		}
		return 0, nil, errMsg
	}

	// Construct arguments for mcp-proxy.
	proxyCmd := "mcp-proxy"
	mcpProxyArgs := []string{}

	// Add port argument if specified
	if serverConfig.ProxyPort > 0 {
		mcpProxyArgs = append(mcpProxyArgs, "--port", fmt.Sprintf("%d", serverConfig.ProxyPort))
		logging.Info(subsystem, "Using specified port %d for mcp-proxy", serverConfig.ProxyPort)
	} else {
		logging.Info(subsystem, "Using random port assignment for mcp-proxy")
	}

	// Add the pass-environment flag and separator
	mcpProxyArgs = append(mcpProxyArgs, "--pass-environment", "--")
	mcpProxyArgs = append(mcpProxyArgs, serverConfig.Command...)

	cmd := execCommand(proxyCmd, mcpProxyArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ() // Inherit current environment
	// Add environment variables from serverConfig.Env
	for k, v := range serverConfig.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutPipe, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		logging.Error(subsystem, pipeErr, "Failed to create stdout pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: pipeErr, ProxyPort: 0})
		}
		return 0, nil, fmt.Errorf("stdout pipe for %s: %w", label, pipeErr)
	}
	stderrPipe, pipeErr := cmd.StderrPipe()
	if pipeErr != nil {
		stdoutPipe.Close()
		logging.Error(subsystem, pipeErr, "Failed to create stderr pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: pipeErr, ProxyPort: 0})
		}
		return 0, nil, fmt.Errorf("stderr pipe for %s: %w", label, pipeErr)
	}

	currentStopChan := make(chan struct{})

	if err := cmd.Start(); err != nil {
		// Adjusted error message to reflect the command being run
		errMsg := fmt.Errorf("failed to start mcp-proxy for %s (executing: %s): %w", label, strings.Join(serverConfig.Command, " "), err)
		logging.Error(subsystem, err, "Failed to start mcp-proxy process")
		stdoutPipe.Close()
		stderrPipe.Close()
		close(currentStopChan)
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: err, PID: 0, ProxyPort: 0})
		}
		return 0, nil, errMsg
	}

	processPid := cmd.Process.Pid
	logging.Debug(subsystem, "Process started successfully with PID %d", processPid)

	// Initialize with 0 port, will be updated when we parse the output
	actualPort := 0

	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{
			Label:         label,
			PID:           processPid,
			ProcessStatus: "ProcessStarting",
			ProxyPort:     actualPort,
		})
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}
		defer stdoutPipe.Close()
		defer stderrPipe.Close()

		// Track whether the server has fully started
		serverReady := false

		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				logLine := scanner.Text()
				logging.Info(subsystem+"-stdout", "%s", logLine)
			}
			if err := scanner.Err(); err != nil {
				// Check if the error is due to the pipe being closed (expected during shutdown)
				if !strings.Contains(err.Error(), "file already closed") && !strings.Contains(err.Error(), "closed pipe") {
					logging.Error(subsystem+"-stdout", err, "Error reading stdout")
				}
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()

				// Check for mcp-proxy port announcement
				// Example: "Starting MCP SSE server on port 8080"
				if strings.Contains(line, "Starting MCP SSE server on port") ||
					strings.Contains(line, "MCP server running on") ||
					strings.Contains(line, "Server running on port") {
					// Try to extract port number
					parts := strings.Fields(line)
					for i, part := range parts {
						if part == "port" && i+1 < len(parts) {
							portStr := strings.TrimSuffix(parts[i+1], ",")
							portStr = strings.TrimSuffix(portStr, ".")
							if port, err := fmt.Sscanf(portStr, "%d", &actualPort); err == nil && port == 1 {
								logging.Info(subsystem, "Detected mcp-proxy listening on port %d", actualPort)
								// If server is already ready, update with the port
								if serverReady && updateFn != nil {
									updateFn(McpDiscreteStatusUpdate{
										Label:         label,
										PID:           processPid,
										ProcessStatus: "ProcessRunning",
										ProxyPort:     actualPort,
									})
								}
								break
							}
						}
					}
				}

				// Check if server is fully ready - expanded to handle more server types
				if !serverReady && (strings.Contains(line, "Uvicorn running on") ||
					strings.Contains(line, "Application startup complete.") ||
					strings.Contains(line, "StreamableHTTP session manager started") ||
					strings.Contains(line, "Application started with StreamableHTTP session manager!") ||
					strings.Contains(line, "Starting Grafana MCP server") || // Grafana MCP
					strings.Contains(line, "MCP server started") || // Generic
					strings.Contains(line, "Server started successfully") || // Generic
					strings.Contains(line, "Ready to accept connections")) { // Generic
					serverReady = true
					logging.Info(subsystem, "MCP server is now fully ready")
					if updateFn != nil {
						updateFn(McpDiscreteStatusUpdate{
							Label:         label,
							PID:           processPid,
							ProcessStatus: "ProcessRunning", // Now we're actually running
							ProxyPort:     actualPort,
						})
					}
				}

				if strings.Contains(line, "Uvicorn running on") || strings.Contains(line, "Application startup complete.") || strings.Contains(line, "StreamableHTTP session manager started") || strings.Contains(line, "Application started with StreamableHTTP session manager!") {
					logging.Info(subsystem+"-stderr", "%s", line)
				} else if strings.HasPrefix(line, "INFO:") || strings.HasPrefix(line, "[I ") {
					logging.Debug(subsystem+"-stderr", "%s", line)
				} else {
					logging.Error(subsystem+"-stderr", nil, "%s", line)
				}
			}
			if err := scanner.Err(); err != nil {
				// Check if the error is due to the pipe being closed (expected during shutdown)
				if !strings.Contains(err.Error(), "file already closed") && !strings.Contains(err.Error(), "closed pipe") {
					logging.Error(subsystem+"-stderr", err, "Error reading stderr")
				}
			}
		}()

		processDone := make(chan error, 1)
		go func() { processDone <- cmd.Wait() }()

		select {
		case err := <-processDone:
			status := "ProcessExitedGracefully"
			finalErr := err
			if err != nil {
				status = "ProcessExitedWithError"
				logging.Error(subsystem, err, "Process exited with error")
			} else {
				logging.Info(subsystem, "Process exited gracefully")
			}
			if updateFn != nil {
				updateFn(McpDiscreteStatusUpdate{Label: label, PID: processPid, ProcessStatus: status, ProcessErr: finalErr, ProxyPort: actualPort})
			}

		case <-currentStopChan:
			logging.Debug(subsystem, "Received stop signal for PID %d", processPid)
			finalProcessStatus := "ProcessStoppedByUser"
			var stopErr error
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				if err := syscall.Kill(-processPid, syscall.SIGKILL); err != nil {
					logging.Error(subsystem, err, "Failed to kill process group for PID %d, attempting to kill main process", processPid)
					if mainProcessKillErr := cmd.Process.Kill(); mainProcessKillErr != nil {
						logging.Error(subsystem, mainProcessKillErr, "Failed to kill main process PID %d after group kill attempt failed", processPid)
						finalProcessStatus = "ProcessKillFailed"
						stopErr = mainProcessKillErr
					} else {
						logging.Debug(subsystem, "Successfully sent SIGKILL to main process PID %d (fallback)", processPid)
					}
				} else {
					logging.Debug(subsystem, "Successfully sent SIGKILL to process group for PID %d", processPid)
				}
				<-processDone
			} else {
				logging.Info(subsystem, "Process PID %d already exited before stop signal processing.", processPid)
				finalProcessStatus = "ProcessAlreadyExited"
			}
			if updateFn != nil {
				updateFn(McpDiscreteStatusUpdate{Label: label, PID: processPid, ProcessStatus: finalProcessStatus, ProcessErr: stopErr, ProxyPort: actualPort})
			}
		}
	}()

	return processPid, currentStopChan, nil
}
