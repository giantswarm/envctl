package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

// DetermineClusterProvider attempts to identify the cloud provider of a Kubernetes cluster
// by inspecting the `providerID` of the first node, then falling back to labels.
// It uses the Kubernetes Go client.
// - contextName: The Kubernetes context to use. If empty, the current context is used.
// Returns the determined provider name (e.g., "aws") or "unknown", and an error if API calls fail.
func DetermineClusterProvider(contextName string) (string, error) {
	fmt.Println("Determining cluster provider using Go client...")

	// Use Teleport prefix for context name if not already prefixed and contextName is provided.
	k8sContextName := contextName
	if contextName != "" && !HasTeleportPrefix(contextName) {
		k8sContextName = TeleportPrefix + contextName
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	// If a specific context name is provided, use it.
	// Otherwise (k8sContextName is empty), it will use the current context from kubeconfig.
	if k8sContextName != "" {
		configOverrides.CurrentContext = k8sContextName
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes client config for context '%s': %w", k8sContextName, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create Kubernetes clientset for context '%s': %w", k8sContextName, err)
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{Limit: 1})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes in context '%s': %w", k8sContextName, err)
	}

	if len(nodes.Items) == 0 {
		return "unknown", fmt.Errorf("no nodes found in cluster with context '%s'", k8sContextName)
	}

	node := nodes.Items[0]
	providerID := node.Spec.ProviderID

	// The providerID is typically in the format: <provider>://<provider-specific-info>
	if providerID != "" {
		if strings.HasPrefix(providerID, "aws://") {
			return "aws", nil
		} else if strings.HasPrefix(providerID, "azure://") {
			return "azure", nil
		} else if strings.HasPrefix(providerID, "gce://") {
			return "gcp", nil
		} else if strings.Contains(providerID, "vsphere") { // vsphere might not have a URI prefix
			return "vsphere", nil
		} else if strings.Contains(providerID, "openstack") { // openstack might not have a URI prefix
			return "openstack", nil
		}
		// If providerID is present but not matched, try labels next
	}

	// Fallback to checking labels if providerID is empty or not recognized
	labels := node.GetLabels()
	if len(labels) > 0 {
		// Look for known provider-specific labels
		for k := range labels {
			if strings.Contains(k, "eks.amazonaws.com") || strings.Contains(k, "amazonaws.com/compute") {
				return "aws", nil
			} else if strings.Contains(k, "kubernetes.azure.com") || strings.Contains(k, "cloud-provider-azure") {
				return "azure", nil
			} else if strings.Contains(k, "cloud.google.com/gke") || strings.Contains(k, "instancegroup.gke.io") {
				return "gcp", nil
			}
		}
	}

	// If we can't determine the provider from providerID or labels, return unknown.
	// It could also be a bare metal cluster or a less common provider.
	return "unknown", nil
}

// GetCurrentKubeContext retrieves the name of the currently active Kubernetes context
// using the Kubernetes Go client.
// Returns the context name and an error if it fails.
func GetCurrentKubeContext() (string, error) {
	pathOptions := clientcmd.NewDefaultPathOptions()
	if pathOptions == nil {
		return "", fmt.Errorf("failed to get default kubeconfig path options")
	}

	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get starting kubeconfig: %w", err)
	}

	if config.CurrentContext == "" {
		return "", fmt.Errorf("current kubeconfig context is not set")
	}
	return config.CurrentContext, nil
}

// SwitchKubeContext changes the active Kubernetes context to the specified context name
// using the Kubernetes Go client, ensuring the full config is preserved.
// - contextName: The name of the Kubernetes context to switch to.
// Returns an error if the command fails.
func SwitchKubeContext(contextName string) error {
	pathOptions := clientcmd.NewDefaultPathOptions()
	if pathOptions == nil {
		return fmt.Errorf("failed to get default kubeconfig path options for switching context")
	}

	// Load the raw config, which preserves all existing data.
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Check if the target context exists.
	if _, exists := config.Contexts[contextName]; !exists {
		return fmt.Errorf("context '%s' does not exist in kubeconfig", contextName)
	}

	// Modify only the CurrentContext field.
	config.CurrentContext = contextName

	// Get the primary kubeconfig file path.
	// If ExplicitPath is set in pathOptions, it will be used, otherwise the default.
	kubeconfigFilePath := pathOptions.GetDefaultFilename()
	if pathOptions.IsExplicitFile() {
		kubeconfigFilePath = pathOptions.GetExplicitFile()
	}

	// Write the entire modified config back to the file.
	if err := clientcmd.WriteToFile(*config, kubeconfigFilePath); err != nil {
		return fmt.Errorf("failed to write updated kubeconfig to '%s': %w", kubeconfigFilePath, err)
	}

	return nil
}
