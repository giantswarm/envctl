package kube

import (
	"fmt"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// GetCurrentKubeContext retrieves the name of the currently active Kubernetes context
var GetCurrentKubeContext = func() (string, error) {
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
var SwitchKubeContext = func(contextName string) error {
	pathOptions := clientcmd.NewDefaultPathOptions()
	if pathOptions == nil {
		return fmt.Errorf("failed to get default kubeconfig path options for switching context")
	}
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	if _, exists := config.Contexts[contextName]; !exists {
		return fmt.Errorf("context '%s' does not exist in kubeconfig", contextName)
	}
	config.CurrentContext = contextName
	kubeconfigFilePath := pathOptions.GetDefaultFilename()
	if pathOptions.IsExplicitFile() {
		kubeconfigFilePath = pathOptions.GetExplicitFile()
	}
	if err := clientcmd.WriteToFile(*config, kubeconfigFilePath); err != nil {
		return fmt.Errorf("failed to write updated kubeconfig to '%s': %w", kubeconfigFilePath, err)
	}
	return nil
}

// TeleportPrefix is the canonical prefix that all Giant Swarm Teleport kubeconfig
// contexts start with.
const TeleportPrefix = "teleport.giantswarm.io-"

// HasTeleportPrefix checks whether a context already begins with the canonical
// Teleport prefix.
func HasTeleportPrefix(ctx string) bool {
	return strings.HasPrefix(ctx, TeleportPrefix)
}

// StripTeleportPrefix removes the Teleport prefix from a context name if it is
// present. If not, it returns the original string.
func StripTeleportPrefix(ctx string) string {
	if HasTeleportPrefix(ctx) {
		return strings.TrimPrefix(ctx, TeleportPrefix)
	}
	return ctx
}

// BuildMcContext returns the full kubeconfig context name for a Management
// Cluster given its short name (e.g. "ghost") ->
// "teleport.giantswarm.io-ghost".
func BuildMcContext(mc string) string {
	if mc == "" {
		return ""
	}
	return TeleportPrefix + mc
}

// BuildWcContext returns the full kubeconfig context name for a Workload
// Cluster given the MC short name and WC short name. Example:
// "teleport.giantswarm.io-ghost-acme".
func BuildWcContext(mc, wc string) string {
	if mc == "" || wc == "" {
		return ""
	}
	return TeleportPrefix + mc + "-" + wc
}

// IsWorkloadClusterName checks if a cluster name appears to be a workload cluster
// based on the naming convention (contains a hyphen)
func IsWorkloadClusterName(clusterName string) bool {
	return strings.Contains(clusterName, "-")
}

// ParseWorkloadClusterName extracts the MC and WC names from a full WC name
// e.g., "mcname-wcname" returns ("mcname", "wcname")
func ParseWorkloadClusterName(fullWCName string) (mcName, wcName string) {
	parts := strings.SplitN(fullWCName, "-", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// GetStartingConfig returns the starting kubeconfig
func GetStartingConfig() (*api.Config, error) {
	pathOptions := clientcmd.NewDefaultPathOptions()
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get starting kubeconfig: %w", err)
	}
	return config, nil
}
