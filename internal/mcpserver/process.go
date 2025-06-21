package mcpserver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"envctl/internal/api"
	"envctl/pkg/logging"
)

// ProcessRunner handles local command-based MCP servers
type ProcessRunner struct {
	definition *api.MCPServer
	cmd        *exec.Cmd
	stopChan   chan struct{}
	port       int
}

// NewProcessRunner creates a new process runner
func NewProcessRunner(definition *api.MCPServer) *ProcessRunner {
	return &ProcessRunner{
		definition: definition,
		stopChan:   make(chan struct{}),
	}
}

// Start starts the local command process
func (pr *ProcessRunner) Start(ctx context.Context) error {
	logging.Info("ProcessRunner", "Starting process for MCP server: %s", pr.definition.Name)

	// Validate command configuration
	if len(pr.definition.Command) == 0 {
		return fmt.Errorf("command is required for localCommand-type MCP server")
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, pr.definition.Command[0], pr.definition.Command[1:]...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range pr.definition.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Setup stdout/stderr capture
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start command: %w", err)
	}

	pr.cmd = cmd
	logging.Info("ProcessRunner", "Process started with PID: %d", cmd.Process.Pid)

	// Start log monitoring
	go pr.monitorLogs(stdout, stderr)

	// Start health monitoring if configured
	if len(pr.definition.HealthCheckCmd) > 0 && pr.definition.HealthCheckInterval > 0 {
		go pr.startHealthMonitoring(ctx)
	}

	return nil
}

// Stop stops the process
func (pr *ProcessRunner) Stop(ctx context.Context) error {
	if pr.cmd == nil || pr.cmd.Process == nil {
		return nil
	}

	logging.Info("ProcessRunner", "Stopping process: %d", pr.cmd.Process.Pid)

	// Signal monitoring to stop
	select {
	case <-pr.stopChan:
		// Already closed
	default:
		close(pr.stopChan)
	}

	// Try graceful shutdown first
	if err := pr.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		logging.Warn("ProcessRunner", "Failed to send SIGTERM: %v", err)
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- pr.cmd.Wait()
	}()

	select {
	case <-time.After(10 * time.Second):
		// Force kill if graceful shutdown takes too long
		logging.Warn("ProcessRunner", "Force killing process: %d", pr.cmd.Process.Pid)
		if err := pr.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
		<-done // Wait for cmd.Wait() to finish
	case err := <-done:
		if err != nil {
			logging.Debug("ProcessRunner", "Process ended with error: %v", err)
		}
	}

	pr.cmd = nil
	return nil
}

// IsRunning checks if the process is running
func (pr *ProcessRunner) IsRunning(ctx context.Context) bool {
	if pr.cmd == nil || pr.cmd.Process == nil {
		return false
	}

	// Check if process is still running
	if err := pr.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return true
}

// GetPort returns the detected port
func (pr *ProcessRunner) GetPort() int {
	return pr.port
}

// monitorLogs monitors stdout and stderr from the process
func (pr *ProcessRunner) monitorLogs(stdout, stderr io.ReadCloser) {
	defer stdout.Close()
	defer stderr.Close()

	// Monitor stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			pr.processLogLine(line, "stdout")
		}
	}()

	// Monitor stderr
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		pr.processLogLine(line, "stderr")
	}
}

// processLogLine processes a single log line
func (pr *ProcessRunner) processLogLine(line, source string) {
	subsystem := fmt.Sprintf("MCP-%s-%s", pr.definition.Name, source)

	// Log the line based on content
	if strings.Contains(line, "ERROR") || strings.Contains(line, "FATAL") {
		logging.Error(subsystem, nil, "%s", line)
	} else if strings.Contains(line, "WARN") {
		logging.Warn(subsystem, "%s", line)
	} else {
		logging.Info(subsystem, "%s", line)
	}

	// Try to detect port information
	if pr.port == 0 {
		if detectedPort := pr.detectPortFromLog(line); detectedPort > 0 {
			pr.port = detectedPort
			logging.Info("ProcessRunner", "Detected process listening on port %d", pr.port)
		}
	}
}

// detectPortFromLog tries to detect port information from log lines
func (pr *ProcessRunner) detectPortFromLog(line string) int {
	// Common patterns for port announcements
	patterns := []string{
		"Starting MCP SSE server on port",
		"MCP server running on",
		"Server running on port",
		"Listening on port",
		"listening on :",
		"Started server on :",
	}

	for _, pattern := range patterns {
		if strings.Contains(line, pattern) {
			// Try to extract port number
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "port" && i+1 < len(parts) {
					portStr := strings.TrimSuffix(parts[i+1], ",")
					portStr = strings.TrimSuffix(portStr, ".")
					if port, err := strconv.Atoi(portStr); err == nil {
						return port
					}
				}
				// Also check for :PORT pattern
				if strings.Contains(part, ":") {
					subparts := strings.Split(part, ":")
					if len(subparts) >= 2 {
						portStr := strings.TrimSuffix(subparts[len(subparts)-1], ",")
						portStr = strings.TrimSuffix(portStr, ".")
						if port, err := strconv.Atoi(portStr); err == nil {
							return port
						}
					}
				}
			}
		}
	}

	return 0
}

// startHealthMonitoring starts health monitoring for the process
func (pr *ProcessRunner) startHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(pr.definition.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pr.stopChan:
			return
		case <-ticker.C:
			if !pr.IsRunning(ctx) {
				logging.Warn("ProcessRunner", "Process %s is not running", pr.definition.Name)
			} else {
				// Optionally run health check command if configured
				if len(pr.definition.HealthCheckCmd) > 0 {
					pr.runHealthCheck(ctx)
				}
			}
		}
	}
}

// runHealthCheck runs the configured health check command
func (pr *ProcessRunner) runHealthCheck(ctx context.Context) {
	if len(pr.definition.HealthCheckCmd) == 0 {
		return
	}

	// Create a timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, pr.definition.HealthCheckCmd[0], pr.definition.HealthCheckCmd[1:]...)
	if err := cmd.Run(); err != nil {
		logging.Warn("ProcessRunner", "Health check failed for %s: %v", pr.definition.Name, err)
	} else {
		logging.Debug("ProcessRunner", "Health check passed for %s", pr.definition.Name)
	}
}
