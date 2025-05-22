package mcpserver

import (
	"bufio"
	"envctl/pkg/logging" // Added for logging
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

var execCommand = exec.Command

// StartAndManageIndividualMcpServer prepares, starts, and manages a single MCP server process.
// It calls updateFn with McpDiscreteStatusUpdate for state changes and uses pkg/logging for verbose output.
func StartAndManageIndividualMcpServer(
	serverConfig MCPServerConfig,
	updateFn McpUpdateFunc, // Now func(update McpDiscreteStatusUpdate)
	wg *sync.WaitGroup,
) (pid int, stopChan chan struct{}, initialError error) {

	label := serverConfig.Name
	proxyPort := serverConfig.ProxyPort
	subsystem := "MCPServer-" + label

	logging.Info(subsystem, "Initializing MCP server %s (underlying: %s %v) on port %d", label, serverConfig.Command, serverConfig.Args, proxyPort)
	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "NpxInitializing", PID: 0})
	}

	// Prepare exec.Cmd for mcp-proxy
	proxyArgs := []string{
		"--port", fmt.Sprintf("%d", proxyPort),
		"--pass-environment",
		"--",
		serverConfig.Command,
	}
	proxyArgs = append(proxyArgs, serverConfig.Args...)

	cmd := execCommand("mcp-proxy", proxyArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ() // Inherit current environment
	for k, v := range serverConfig.Env {
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
		errMsg := fmt.Errorf("failed to start mcp-proxy for %s (underlying: %s %v): %w", label, serverConfig.Command, serverConfig.Args, err)
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
	logging.Info(subsystem, "Process started successfully with PID %d", processPid)

	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{
			Label:         label,
			PID:           processPid,
			ProcessStatus: "NpxRunning", // Changed from "Running" Status to ProcessStatus
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
				logLine := scanner.Text()
				logging.Error(subsystem+"-stderr", nil, "%s", logLine)
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
			logging.Info(subsystem, "Received stop signal for PID %d", processPid)
			finalProcessStatus := "NpxStoppedByUser"
			var stopErr error
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
				if err := syscall.Kill(-processPid, syscall.SIGKILL); err != nil {
					logging.Error(subsystem, err, "Failed to kill process group for PID %d", processPid)
					finalProcessStatus = "NpxKillFailed"
					stopErr = err
				} else {
					logging.Info(subsystem, "Successfully sent SIGKILL to process group for PID %d", processPid)
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
