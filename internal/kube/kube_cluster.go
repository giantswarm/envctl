package kube

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// GetNodeStatus retrieves the number of ready and total nodes in a cluster using client-go.
var GetNodeStatus = func(clientset kubernetes.Interface) (readyNodes int, totalNodes int, err error) {
	// No longer needs to create clientset from kubeContext here
	// Assumes clientset is already configured for the correct context.

	// 3. List Nodes with an explicit context timeout to ensure the call cannot hang indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	nodeList, errList := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if errList != nil {
		return 0, 0, fmt.Errorf("failed to list nodes: %w", errList)
	}

	totalNodes = len(nodeList.Items)
	for _, node := range nodeList.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
				break
			}
		}
	}
	return readyNodes, totalNodes, nil
}

// determineProviderFromNode is an unexported helper that inspects a single node's
// ProviderID and labels to determine the cloud provider.
func determineProviderFromNode(node *corev1.Node) string {
	if node == nil {
		return "unknown"
	}

	providerID := node.Spec.ProviderID

	if providerID != "" {
		if strings.HasPrefix(providerID, "aws://") {
			return "aws"
		} else if strings.HasPrefix(providerID, "azure://") {
			return "azure"
		} else if strings.HasPrefix(providerID, "gce://") {
			return "gcp"
		} else if strings.Contains(providerID, "vsphere") {
			return "vsphere"
		} else if strings.Contains(providerID, "openstack") {
			return "openstack"
		}
		// If providerID is present but not matched, try labels next
	}

	labels := node.GetLabels()
	if len(labels) > 0 {
		for k := range labels {
			if strings.Contains(k, "eks.amazonaws.com") || strings.Contains(k, "amazonaws.com/compute") {
				return "aws"
			} else if strings.Contains(k, "kubernetes.azure.com") || strings.Contains(k, "cloud-provider-azure") {
				return "azure"
			} else if strings.Contains(k, "cloud.google.com/gke") || strings.Contains(k, "instancegroup.gke.io") {
				return "gcp"
			}
		}
	}
	return "unknown"
}

// DetermineClusterProvider attempts to identify the cloud provider of a Kubernetes cluster
// by inspecting the `providerID` of the first node, then falling back to labels.
// It uses the Kubernetes Go client.
// - ctx: The context to use for the Kubernetes API call.
// - contextName: The Kubernetes context to use. If empty, the current context is used.
// Returns the determined provider name (e.g., "aws") or "unknown", and an error if API calls fail.
var DetermineClusterProvider = func(ctx context.Context, contextName string) (string, error) {
	// Use Teleport prefix for context name if not already prefixed and contextName is provided.
	k8sContextName := contextName
	if contextName != "" && !strings.HasPrefix(contextName, "teleport.giantswarm.io-") {
		k8sContextName = "teleport.giantswarm.io-" + contextName
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
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

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes in context '%s': %w", k8sContextName, err)
	}

	if len(nodes.Items) == 0 {
		return "unknown", fmt.Errorf("no nodes found in cluster with context '%s'", k8sContextName)
	}

	return determineProviderFromNode(&nodes.Items[0]), nil
}

// GetClientsetForContext creates a Kubernetes clientset for a specific context
func GetClientsetForContext(ctx context.Context, kubeContextName string) (kubernetes.Interface, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContextName}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config for context %q: %w", kubeContextName, err)
	}
	restConfig.Timeout = 15 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset for context %q: %w", kubeContextName, err)
	}

	return clientset, nil
}
