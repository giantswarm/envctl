package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// LoginToKubeCluster executes 'tsh kube login <clusterName>'.
func LoginToKubeCluster(clusterName string) error {
	cmd := exec.Command("tsh", "kube", "login", clusterName)
	cmd.Stdout = os.Stdout // Restore output
	cmd.Stderr = os.Stderr // Restore output
	cmd.Stdin = os.Stdin   // Keep stdin for potential prompts

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute 'tsh kube login %s': %w", clusterName, err)
	}
	return nil
}

// StartPrometheusPortForward starts 'kubectl port-forward' for Mimir,
// waits for it to complete, and streams its output.
// It uses the specified kubectl context if provided, otherwise the current context.
func StartPrometheusPortForward(contextName string) error {
	fmt.Println("Attempting to start Prometheus (Mimir) port-forward...")

	// Apply Teleport prefix to context name if it doesn't already have it
	kubectlContextName := contextName
	if contextName != "" && !strings.HasPrefix(contextName, "teleport.giantswarm.io-") {
		kubectlContextName = "teleport.giantswarm.io-" + contextName
	}

	args := []string{"port-forward", "-n", "mimir", "service/mimir-query-frontend", "8080:8080"}
	if kubectlContextName != "" {
		args = append([]string{"--context", kubectlContextName}, args...)
		fmt.Printf("Using Kubernetes context: %s\n", kubectlContextName)
	} else {
		fmt.Println("Using current Kubernetes context.")
	}

	cmd := exec.Command("kubectl", args...)

	// Stream output to the parent process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil

	fmt.Println("Starting port-forward process. Press Ctrl+C to stop.")
	fmt.Println("Ensure PROMETHEUS_URL in mcp.json is http://localhost:8080/prometheus")
	fmt.Println("You might need to restart MCP servers or your IDE if they were running before the port-forward started.")

	// Run the command and wait for it to finish
	err := cmd.Run()
	if err != nil {
		// Check if the error is due to the process being killed (e.g., Ctrl+C)
		// This might not be strictly necessary depending on desired exit message,
		// as cmd.Run() often returns a non-nil error in this case.
		// We can refine this error handling if needed.
		if exitErr, ok := err.(*exec.ExitError); ok {
			// The command exited with a non-zero status.
			// Often, killing the process via signal results in specific exit codes.
			// For now, just return a formatted error.
			fmt.Fprintf(os.Stderr, "Port-forward process exited.\n")
			return fmt.Errorf("port-forward command failed: %w", exitErr)
		}
		// Other errors (e.g., command not found)
		return fmt.Errorf("failed to run kubectl port-forward: %w", err)
	}

	fmt.Println("Port-forward process finished.")
	return nil // Return nil if cmd.Run() completes without error (e.g., port-forward exits cleanly)
}

// StopProcess sends a SIGTERM signal to the process.
// This function might become less relevant for port-forward if it's always run synchronously,
// but keeping it for potential other uses.
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

// DetermineClusterProvider determines the cloud provider (AWS, Azure, GCP, etc.)
// of a Kubernetes cluster by examining node information.
// It uses the specified kubectl context if provided, otherwise the current context.
func DetermineClusterProvider(contextName string) (string, error) {
	fmt.Println("Determining cluster provider...")

	// Apply Teleport prefix to context name if it doesn't already have it
	kubectlContextName := contextName
	if contextName != "" && !strings.HasPrefix(contextName, "teleport.giantswarm.io-") {
		kubectlContextName = "teleport.giantswarm.io-" + contextName
	}

	// Command to get node information in JSON format
	args := []string{"get", "nodes", "-o", "jsonpath={.items[0].spec.providerID}"}
	if kubectlContextName != "" {
		args = append([]string{"--context", kubectlContextName}, args...)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get node provider information: %w", err)
	}

	providerID := string(output)

	// The providerID is typically in the format: <provider>://<provider-specific-info>
	// Example: aws://us-west-2/i-0123456789abcdef0
	if strings.HasPrefix(providerID, "aws://") {
		return "aws", nil
	} else if strings.HasPrefix(providerID, "azure://") {
		return "azure", nil
	} else if strings.HasPrefix(providerID, "gce://") {
		return "gcp", nil
	} else if strings.Contains(providerID, "vsphere") {
		return "vsphere", nil
	} else if strings.Contains(providerID, "openstack") {
		return "openstack", nil
	} else if len(providerID) == 0 {
		// If no providerID is returned, attempt to check labels
		return determineProviderFromLabels(kubectlContextName)
	}

	// If we can't determine the provider from the ID, return unknown
	return "unknown", nil
}

// determineProviderFromLabels tries to determine the provider from node labels
// as a fallback when providerID is not available.
func determineProviderFromLabels(contextName string) (string, error) {
	args := []string{"get", "nodes", "-o", "jsonpath={.items[0].metadata.labels}"}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
	}

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get node labels: %w", err)
	}

	labels := string(output)

	// Look for known provider-specific labels
	if strings.Contains(labels, "eks.amazonaws.com") || strings.Contains(labels, "aws") {
		return "aws", nil
	} else if strings.Contains(labels, "azure") || strings.Contains(labels, "aks") {
		return "azure", nil
	} else if strings.Contains(labels, "gke") || strings.Contains(labels, "cloud.google.com") {
		return "gcp", nil
	}

	// If we can't determine the provider from labels either, return unknown
	return "unknown", nil
}
