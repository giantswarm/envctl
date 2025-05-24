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
	// proxyPort := serverConfig.ProxyPort // ProxyPort is not part of MCPServerDefinition, mcp-proxy manages its own port or it's passed via env
	subsystem := "MCPServer-" + label

	// Logging needs to be adjusted as ProxyPort is not directly available.
	// The command from serverConfig.Command will be executed directly by mcp-proxy.
	logging.Info(subsystem, "Initializing MCP server %s (underlying: %s)", label, strings.Join(serverConfig.Command, " "))
	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "NpxInitializing", PID: 0})
	}

	// mcp-proxy will now take the command and its arguments directly.
	// The old logic constructed proxyArgs including a --port for mcp-proxy itself.
	// Now, serverConfig.Command contains the actual command and its args.
	// mcp-proxy needs to be invoked in a way that it executes serverConfig.Command.
	// Assuming mcp-proxy is a generic process runner that takes the target command after "--".
	// If mcp-proxy has a fixed port, it needs to be configured when mcp-proxy itself is run, or via its own config.
	// If the MCP server (e.g. npx my-server) needs a port, it should be in its serverConfig.Env or serverConfig.Command args.

	// For now, assuming mcp-proxy is available and takes the command to execute.
	// The MCPServerDefinition.Command is []string, first element is command, rest are args.
	if len(serverConfig.Command) == 0 {
		errMsg := fmt.Errorf("command not defined for MCP server %s", label)
		logging.Error(subsystem, errMsg, "Cannot start MCP server")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "NpxStartFailed", ProcessErr: errMsg})
		}
		return 0, nil, errMsg
	}

	// Construct arguments for mcp-proxy. It needs to execute the command defined in serverConfig.Command.
	// This assumes mcp-proxy still uses a specific port for its own API if any, or that part is handled elsewhere.
	// The main change is that serverConfig.Command IS the command to run, not something mcp-proxy figures out.

	// Let's assume mcp-proxy still needs a port for itself, for example, if it offers a control plane.
	// This port is NOT the port of the service being proxied (that's implicit in the service's own config).
	// For this refactor, let's assume mcp-proxy doesn't need a specific port passed this way for the *target* service.
	// If it needs a port for *itself*, that is a separate concern from MCPServerDefinition.
	// The original `ProxyPort` was for `mcp-proxy` itself.
	// We will remove direct use of `ProxyPort` from `MCPServerDefinition` here.
	// The `mcp-proxy` command itself will be responsible for its own port management.

	proxyCmd := "mcp-proxy"                              // This could be configurable globally if needed
	mcpProxyArgs := []string{"--pass-environment", "--"} // mcp-proxy specific args before the actual command
	mcpProxyArgs = append(mcpProxyArgs, serverConfig.Command...)

	cmd := execCommand(proxyCmd, mcpProxyArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ() // Inherit current environment
	// Add environment variables from serverConfig.Env (for localCommand)
	// or serverConfig.ContainerEnv (for container type, though this function is for local commands)
	// The MCPServerDefinition has Env for localCommand and ContainerEnv for container.
	// This function seems to be generic for local commands run via mcp-proxy.
	for k, v := range serverConfig.Env { // Assuming this is for localCommand type
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutPipe, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		logging.Error(subsystem, pipeErr, "Failed to create stdout pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "NpxStartFailed", ProcessErr: pipeErr})
		}
		return 0, nil, fmt.Errorf("stdout pipe for %s: %w", label, pipeErr)
	}
	stderrPipe, pipeErr := cmd.StderrPipe()
	if pipeErr != nil {
		stdoutPipe.Close()
		logging.Error(subsystem, pipeErr, "Failed to create stderr pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "NpxStartFailed", ProcessErr: pipeErr})
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
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "NpxStartFailed", ProcessErr: err, PID: 0})
		}
		return 0, nil, errMsg
	}

	processPid := cmd.Process.Pid
	logging.Debug(subsystem, "Process started successfully with PID %d", processPid)

	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{
			Label:         label,
			PID:           processPid,
			ProcessStatus: "NpxRunning",
		})
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}
		defer stdoutPipe.Close()
		defer stderrPipe.Close()

		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				logLine := scanner.Text()
				logging.Info(subsystem+"-stdout", "%s", logLine)
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "Uvicorn running on") || strings.Contains(line, "Application startup complete.") || strings.Contains(line, "StreamableHTTP session manager started") || strings.Contains(line, "Application started with StreamableHTTP session manager!") {
					logging.Info(subsystem+"-stderr", "%s", line)
				} else if strings.HasPrefix(line, "INFO:") || strings.HasPrefix(line, "[I ") {
					logging.Debug(subsystem+"-stderr", "%s", line)
				} else {
					logging.Error(subsystem+"-stderr", nil, "%s", line)
				}
			}
			if err := scanner.Err(); err != nil {
				logging.Error(subsystem+"-stderr", err, "Error reading stderr")
			}
		}()

		processDone := make(chan error, 1)
		go func() { processDone <- cmd.Wait() }()

		select {
		case err := <-processDone:
			status := "NpxExitedGracefully"
			finalErr := err
			if err != nil {
				status = "NpxExitedWithError"
				logging.Error(subsystem, err, "Process exited with error")
			} else {
				logging.Info(subsystem, "Process exited gracefully")
			}
			if updateFn != nil {
				updateFn(McpDiscreteStatusUpdate{Label: label, PID: processPid, ProcessStatus: status, ProcessErr: finalErr})
			}

		case <-currentStopChan:
			logging.Debug(subsystem, "Received stop signal for PID %d", processPid)
			finalProcessStatus := "NpxStoppedByUser"
			var stopErr error
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				if err := syscall.Kill(-processPid, syscall.SIGKILL); err != nil {
					logging.Error(subsystem, err, "Failed to kill process group for PID %d, attempting to kill main process", processPid)
					if mainProcessKillErr := cmd.Process.Kill(); mainProcessKillErr != nil {
						logging.Error(subsystem, mainProcessKillErr, "Failed to kill main process PID %d after group kill attempt failed", processPid)
						finalProcessStatus = "NpxKillFailed"
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
				finalProcessStatus = "NpxAlreadyExited"
			}
			if updateFn != nil {
				updateFn(McpDiscreteStatusUpdate{Label: label, PID: processPid, ProcessStatus: finalProcessStatus, ProcessErr: stopErr})
			}
		}
	}()

	return processPid, currentStopChan, nil
}
