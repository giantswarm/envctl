package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// LoginToKubeCluster executes 'tsh kube login <clusterName>' and returns its output.
func LoginToKubeCluster(clusterName string) (stdout string, stderr string, err error) {
	cmd := exec.Command("tsh", "kube", "login", clusterName)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Stdin might still be needed if tsh prompts for anything (e.g., 2FA),
	// but for non-interactive TUI, this might be an issue if it hangs.
	// For now, keep os.Stdin, but this could be a point of failure if tsh blocks.
	// Consider if tsh login can be made fully non-interactive or if a timeout is needed.
	cmd.Stdin = os.Stdin

	runErr := cmd.Run()

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if runErr != nil {
		// Include tsh's stderr in the error message for better diagnostics
		return stdoutStr, stderrStr, fmt.Errorf("failed to execute 'tsh kube login %s': %w. Stderr: %s", clusterName, runErr, stderrStr)
	}
	return stdoutStr, stderrStr, nil
}

// PortForwardCommand represents a running port-forward command.
// type PortForwardCommand struct {
// 	Cmd    *exec.Cmd
// 	Stdout io.ReadCloser
// 	Stderr io.ReadCloser
// 	Label  string // For TUI display
// }

// StartPortForward prepares and starts a kubectl port-forward command but does not wait for it to complete.
// It returns the command object and pipes for its stdout and stderr.
func StartPortForward(contextName, namespace, service, ports, label string) (*exec.Cmd, io.ReadCloser, io.ReadCloser, error) {
	kubectlContextName := contextName
	if contextName != "" && !strings.HasPrefix(contextName, "teleport.giantswarm.io-") {
		kubectlContextName = "teleport.giantswarm.io-" + contextName
	}

	args := []string{"port-forward"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, service, ports)

	if kubectlContextName != "" {
		args = append([]string{"--context", kubectlContextName}, args...)
	}

	cmd := exec.Command("kubectl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get stdout pipe for %s: %w", label, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get stderr pipe for %s: %w", label, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to start port-forward for %s: %w", label, err)
	}

	return cmd, stdout, stderr, nil
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

// GetCurrentKubeContext returns the current kubectl context name.
func GetCurrentKubeContext() (string, error) {
	cmd := exec.Command("kubectl", "config", "current-context")
	output, err := cmd.Output()
	if err != nil {
		// If there's an error (e.g., kubectl not configured, no current context), return it.
		// The error from cmd.Output() for exec.Command includes stderr, which is useful.
		return "", fmt.Errorf("failed to get current kubectl context: %w", err)
	}
	// The output includes a newline character, so trim it.
	return strings.TrimSpace(string(output)), nil
}

// GetNodeStatus retrieves the number of ready and total nodes in a cluster.
func GetNodeStatus(contextName string) (readyNodes int, totalNodes int, err error) {
	kubectlContextArg := []string{}
	if contextName != "" {
		kubectlContextArg = []string{"--context", contextName}
	}

	// Get all nodes and their ready status
	// We'll count total nodes and then count how many are Ready: True
	getNodesArgs := append(kubectlContextArg, "get", "nodes", "-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}")
	cmdNodes := exec.Command("kubectl", getNodesArgs...)
	outputNodes, err := cmdNodes.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get node statuses for context '%s': %w", contextName, err)
	}

	statuses := strings.Fields(string(outputNodes))
	totalNodes = len(statuses)
	for _, status := range statuses {
		if strings.ToLower(status) == "true" {
			readyNodes++
		}
	}

	// As an alternative for totalNodes, to be absolutely sure, we could count metadata.name
	// but len(statuses) should be reliable if the jsonpath is correct for all nodes.

	return readyNodes, totalNodes, nil
}

// SwitchKubeContext changes the current kubectl context.
func SwitchKubeContext(contextName string) error {
	cmd := exec.Command("kubectl", "config", "use-context", contextName)
	// We don't want to inherit os.Stdout/Stderr directly for this one,
	// as successful output is minimal and errors will be captured.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch kubectl context to '%s': %w\\nOutput: %s", contextName, err, string(output))
	}
	return nil
}
