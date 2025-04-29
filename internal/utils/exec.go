package utils

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// LoginToKubeCluster executes 'tsh kube login <clusterName>'.
func LoginToKubeCluster(clusterName string) error {
	fmt.Printf("Attempting to log into Kubernetes cluster '%s' via tsh...\n", clusterName)
	cmd := exec.Command("tsh", "kube", "login", clusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // Allow potential interactive prompts

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute 'tsh kube login %s': %w", clusterName, err)
	}
	fmt.Printf("Successfully logged into %s via tsh.\n", clusterName)
	return nil
}

// StartPrometheusPortForward starts 'kubectl port-forward' for Mimir in the background.
// It uses the specified kubectl context if provided, otherwise the current context.
// Returns the command process and any error encountered during startup.
func StartPrometheusPortForward(contextName string) (*exec.Cmd, error) {
	fmt.Println("Attempting to start Prometheus (Mimir) port-forward...")

	args := []string{"port-forward", "-n", "mimir", "service/mimir-query-frontend", "8080:8080"}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
		fmt.Printf("Using Kubernetes context: %s\n", contextName)
	} else {
		fmt.Println("Using current Kubernetes context.")
	}

	cmd := exec.Command("kubectl", args...)

	// Redirect stdout/stderr to /dev/null or a log file if needed,
	// otherwise the background process might hang if buffers fill.
	// For now, let's discard them. In the future, logging could be added.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil // Prevent it from consuming stdin

	// Start the command in the background
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start kubectl port-forward: %w", err)
	}

	fmt.Printf("Started Prometheus port-forward (PID: %d) in the background.\n", cmd.Process.Pid)
	fmt.Println("Ensure PROMETHEUS_URL in mcp.json is http://localhost:8080/prometheus")
	fmt.Println("You might need to restart MCP servers or your IDE.")
	fmt.Println("To stop port-forwarding later, you may need to manually kill the process (e.g., kill %d).", cmd.Process.Pid)

	// It's generally good practice to release the process immediately in Go
	// if we don't plan to wait for it or manage it further within this specific function.
	// The process will continue running in the background.
	// _ = cmd.Process.Release() // Consider uncommenting if sure no more interaction is needed.

	return cmd, nil
}

// StopProcess sends a SIGTERM signal to the process.
func StopProcess(process *os.Process) error {
	if process == nil {
		return fmt.Errorf("process is nil")
	}
	fmt.Printf("Attempting to stop process with PID: %d\n", process.Pid)
	err := process.Signal(syscall.SIGTERM)
	if err != nil {
		// Fallback to SIGKILL if SIGTERM fails or isn't appropriate
		fmt.Printf("SIGTERM failed (%v), trying SIGKILL for PID: %d\n", err, process.Pid)
		err = process.Signal(syscall.SIGKILL)
	}
	if err != nil {
		return fmt.Errorf("failed to stop process %d: %w", process.Pid, err)
	}
	fmt.Printf("Successfully sent termination signal to process %d\n", process.Pid)
	// Note: Sending the signal doesn't guarantee immediate termination.
	// We could add a Wait() here if synchronous stop is needed.
	return nil
}
