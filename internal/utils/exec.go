package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LoginToKubeCluster executes `tsh kube login <clusterName>` to authenticate with a Teleport Kubernetes cluster.
// It captures and returns the standard output and standard error from the command.
// Note: This function currently passes os.Stdin to the command, which might cause issues
// if `tsh` prompts for interactive input (e.g., 2FA) in a non-interactive environment like the TUI.
// - clusterName: The name of the Teleport Kubernetes cluster to log into.
// Returns the stdout string, stderr string, and an error if the command execution fails.
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

// DetermineClusterProvider attempts to identify the cloud provider (e.g., AWS, Azure, GCP)
// of a Kubernetes cluster by inspecting the `providerID` of the first node.
// It uses `kubectl get nodes -o jsonpath={.items[0].spec.providerID}`.
// If `providerID` is not available, it falls back to `determineProviderFromLabels`.
// - contextName: The Kubernetes context to use. If empty, the current context is used.
// Returns the determined provider name (e.g., "aws") or "unknown", and an error if `kubectl` fails.
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

// determineProviderFromLabels is a fallback mechanism for `DetermineClusterProvider`.
// It inspects node labels for known provider-specific labels using
// `kubectl get nodes -o jsonpath={.items[0].metadata.labels}`
// - contextName: The Kubernetes context to use.
// Returns the provider name or "unknown", and an error if `kubectl` fails.
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

// GetCurrentKubeContext retrieves the name of the currently active Kubernetes context using
// `kubectl config current-context`.
// Returns the context name (trimmed of whitespace) and an error if the command fails.
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

// SwitchKubeContext changes the active Kubernetes context to the specified context name
// using `kubectl config use-context <contextName>`.
// - contextName: The name of the Kubernetes context to switch to.
// Returns an error if the command fails, including the command's output in the error message.
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
