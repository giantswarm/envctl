package mcpserver

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// StartAndManageIndividualMcpServer prepares, starts, and manages a single MCP server process.
// It takes a serverConfig, an updateFn callback, and an optional WaitGroup.
// It returns the PID, a stop channel for terminating the process, and any initial error during startup.
func StartAndManageIndividualMcpServer(
	serverConfig PredefinedMcpServer,
	updateFn McpUpdateFunc,
	wg *sync.WaitGroup,
) (pid int, stopChan chan struct{}, initialError error) {

	label := serverConfig.Name
	proxyPort := serverConfig.ProxyPort

	// Prepare exec.Cmd for mcp-proxy
	proxyArgs := []string{
		"--port", fmt.Sprintf("%d", proxyPort),
		"--pass-environment",
		"--",
		serverConfig.Command,
	}
	proxyArgs = append(proxyArgs, serverConfig.Args...)

	cmd := exec.Command("mcp-proxy", proxyArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ() // Inherit current environment
	for k, v := range serverConfig.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutPipe, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return 0, nil, fmt.Errorf("stdout pipe for %s: %w", label, pipeErr)
	}
	stderrPipe, pipeErr := cmd.StderrPipe()
	if pipeErr != nil {
		stdoutPipe.Close()
		return 0, nil, fmt.Errorf("stderr pipe for %s: %w", label, pipeErr)
	}

	// Create the stop channel before starting, so it can be returned.
	currentStopChan := make(chan struct{})

	if err := cmd.Start(); err != nil {
		errMsg := fmt.Errorf("failed to start mcp-proxy for %s (underlying: %s %v): %w", label, serverConfig.Command, serverConfig.Args, err)
		stdoutPipe.Close()
		stderrPipe.Close()
		close(currentStopChan) // Close the created stop chan as it won't be used
		return 0, nil, errMsg
	}

	processPid := cmd.Process.Pid

	// If an update function is provided, send an initial "Running" status update with the PID.
	if updateFn != nil {
		updateFn(McpProcessUpdate{
			Label:     label,
			PID:       processPid,
			Status:    "Running",
			OutputLog: fmt.Sprintf("Proxy for %s (PID: %d) with underlying %s %v, listening on http://localhost:%d/sse", label, processPid, serverConfig.Command, serverConfig.Args, proxyPort),
		})
	}

	// Launch the goroutine to manage the running process
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		defer stdoutPipe.Close()
		defer stderrPipe.Close()

		// Goroutine for stdout
		go func() {
			scanner := bufio.NewScanner(stdoutPipe)
			for scanner.Scan() {
				logLine := scanner.Text()
				if updateFn != nil {
					updateFn(McpProcessUpdate{Label: label, PID: processPid, OutputLog: fmt.Sprintf("[%s STDOUT] %s", label, logLine)})
				}
			}
		}()

		// Goroutine for stderr
		go func() {
			scanner := bufio.NewScanner(stderrPipe)
			for scanner.Scan() {
				logLine := scanner.Text()
				if updateFn != nil {
					// Send plain stderr log. The updateFn consumer can decide if it's an error "status".
					updateFn(McpProcessUpdate{Label: label, PID: processPid, OutputLog: fmt.Sprintf("[%s STDERR] %s", label, logLine)})
				}
			}
		}()

		processDone := make(chan error, 1)
		go func() { processDone <- cmd.Wait() }()

		select {
		case err := <-processDone:
			status := "Stopped"
			logMsg := fmt.Sprintf("Proxy for %s (PID: %d, underlying: %s) exited.", label, processPid, serverConfig.Command)
			finalErr := err
			isErrFlag := false
			if err != nil {
				status = "Error"
				logMsg = fmt.Sprintf("Proxy for %s (PID: %d, underlying: %s) exited with error: %v", label, processPid, serverConfig.Command, err)
				isErrFlag = true
			}
			if updateFn != nil {
				updateFn(McpProcessUpdate{Label: label, PID: processPid, Status: status, OutputLog: logMsg, IsError: isErrFlag, Err: finalErr})
			}

		case <-currentStopChan: // Use the stopChan created and returned by this function
			if cmd.ProcessState == nil || !cmd.ProcessState.Exited() { // Check if process is still running
				if err := syscall.Kill(-processPid, syscall.SIGKILL); err != nil {
					logMsg := fmt.Sprintf("Failed to kill proxy for %s (PID: %d): %v", label, processPid, err)
					if updateFn != nil {
						updateFn(McpProcessUpdate{Label: label, PID: processPid, Status: "Error", OutputLog: logMsg, IsError: true, Err: err})
					}
				} else {
					logMsg := fmt.Sprintf("Proxy for %s (PID: %d) stopped via signal.", label, processPid)
					if updateFn != nil {
						updateFn(McpProcessUpdate{Label: label, PID: processPid, Status: "Stopped", OutputLog: logMsg})
					}
				}
				<-processDone // Wait for the process to actually exit after kill
			} else {
				logMsg := fmt.Sprintf("Proxy for %s (PID: %d) already exited before stop signal processing.", label, processPid)
				if updateFn != nil {
					updateFn(McpProcessUpdate{Label: label, PID: processPid, Status: "Stopped", OutputLog: logMsg})
				}
			}
		}
	}() // End of main managing goroutine

	return processPid, currentStopChan, nil // Return successfully started PID and its stopChan
}
