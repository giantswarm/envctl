package portforwarding

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
)

// StartAndManageIndividualPortForward sets up and manages a single port forwarding connection.
// It takes a configuration, an update callback function, and an optional existing command (cmd) for re-attachment (not fully implemented here).
func StartAndManageIndividualPortForward(
	cfg PortForwardConfig,
	updateFn PortForwardUpdateFunc,
	initialCmd *exec.Cmd, // Used if we ever want to re-attach to an existing kubectl process
) (*exec.Cmd, chan struct{}, error) {
	stopChan := make(chan struct{})
	var cmd *exec.Cmd

	// Send initial update that we're starting
	updateFn(PortForwardProcessUpdate{
		InstanceKey: cfg.InstanceKey,
		ServiceName: cfg.ServiceName,
		Namespace:   cfg.Namespace,
		LocalPort:   cfg.LocalPort,
		RemotePort:  cfg.RemotePort,
		StatusMsg:   "Initializing",
		Running:     false,
	})

	args := []string{
		"port-forward",
	}
	if cfg.KubeConfigPath != "" {
		args = append(args, "--kubeconfig", cfg.KubeConfigPath)
	}
	if cfg.KubeContext != "" {
		args = append(args, "--context", cfg.KubeContext)
	}
	if cfg.Namespace != "" {
		args = append(args, "--namespace", cfg.Namespace)
	}
	if cfg.BindAddress != "" && cfg.BindAddress != "0.0.0.0" { // kubectl default is 0.0.0.0 for service, 127.0.0.1 for pod
		args = append(args, "--address", cfg.BindAddress)
	}

	target := cfg.ServiceName
	if strings.HasPrefix(cfg.ServiceName, "pod/") || strings.HasPrefix(cfg.ServiceName, "pods/") {
		// Already a pod, or correctly prefixed.
	} else if strings.HasPrefix(cfg.ServiceName, "service/") {
		// Already a service, or correctly prefixed.
	} else {
		target = "service/" + cfg.ServiceName // Default to service if no prefix
	}
	args = append(args, target, fmt.Sprintf("%s:%s", cfg.LocalPort, cfg.RemotePort))

	cmd = exec.CommandContext(context.Background(), "kubectl", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return cmd, stopChan, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return cmd, stopChan, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Goroutine to stream stdout
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   "Running", // Or parse output for more detail
				OutputLog:   strings.TrimSpace(line),
				Running:     true, // Assuming if we get logs, it's running
				Cmd:         cmd,
			})
		}
		if err := scanner.Err(); err != nil {
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   "Error (stdout)",
				Error:       err,
				Running:     false,
				Cmd:         cmd,
			})
		}
	}()

	// Goroutine to stream stderr
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   "Error", // Or parse output for more detail
				OutputLog:   strings.TrimSpace(line),
				Error:       fmt.Errorf(line), // Treat stderr line as an error message
				Running:     cmd.ProcessState == nil || !cmd.ProcessState.Exited(),
				Cmd:         cmd,
			})
		}
		if err := scanner.Err(); err != nil {
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   "Error (stderr)",
				Error:       err,
				Running:     false,
				Cmd:         cmd,
			})
		}
	}()

	go func() {
		defer func() {
			// Ensure pipes are closed after command finishes or is stopped
			closer, ok := stdoutPipe.(io.Closer)
			if ok {
				closer.Close()
			}
			closer, ok = stderrPipe.(io.Closer)
			if ok {
				closer.Close()
			}
		}()

		err := cmd.Start()
		if err != nil {
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   "Failed to start",
				Error:       fmt.Errorf("failed to start port-forward: %w", err),
				Running:     false,
				Cmd:         cmd,
			})
			return
		}

		// Send an update that it's now potentially running
		// A more robust way would be to parse kubectl's output for "Forwarding from ..."
		updateFn(PortForwardProcessUpdate{
			InstanceKey: cfg.InstanceKey,
			ServiceName: cfg.ServiceName,
			Namespace:   cfg.Namespace,
			LocalPort:   cfg.LocalPort,
			RemotePort:  cfg.RemotePort,
			StatusMsg:   "Starting", // Kubectl usually prints to stdout when ready.
			PID:         cmd.Process.Pid,
			Running:     true, // Optimistic, actual running state confirmed by logs or successful port bind
			Cmd:         cmd,
		})

		waitErrChan := make(chan error, 1)
		go func() {
			waitErrChan <- cmd.Wait()
		}()

		select {
		case err := <-waitErrChan:
			statusMsg := "Stopped"
			if err != nil {
				statusMsg = "Stopped with error"
				if exitErr, ok := err.(*exec.ExitError); ok {
					if !strings.Contains(string(exitErr.Stderr), "signal interrupt") && // SIGINT is normal stop
						!strings.Contains(string(exitErr.Stderr), "signal terminated") { // SIGTERM is normal stop
						// It's an actual error
					} else {
						// It's a signal, so not an application error for the PF itself
						err = nil
						statusMsg = "Stopped"
					}
				} else if strings.Contains(err.Error(), "signal: killed") {
					// This can happen if stopChan is closed and we send SIGKILL
					err = nil
					statusMsg = "Stopped"
				}
			}
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   statusMsg,
				Error:       err,
				Running:     false,
				Cmd:         cmd,
			})
		case <-stopChan:
			if cmd.Process != nil {
				// Try to terminate gracefully first
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					// If SIGTERM fails, force kill
					cmd.Process.Kill()
				}
			}
			// Wait for the process to exit after signaling stop
			<-waitErrChan // Consume the error from cmd.Wait()
			updateFn(PortForwardProcessUpdate{
				InstanceKey: cfg.InstanceKey,
				ServiceName: cfg.ServiceName,
				Namespace:   cfg.Namespace,
				LocalPort:   cfg.LocalPort,
				RemotePort:  cfg.RemotePort,
				StatusMsg:   "Stopped (requested)",
				Running:     false,
				Cmd:         cmd,
			})
		}
	}()

	return cmd, stopChan, nil
}
