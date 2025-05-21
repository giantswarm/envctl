package k8smanager

import (
	"context"
	"envctl/internal/kube" // For GetCurrentKubeContext, SwitchKubeContext
	"envctl/internal/utils"
	"fmt"  // For GetAvailableContexts error handling
	"time" // For restConfig.Timeout

	"k8s.io/client-go/kubernetes"      // For clientset
	"k8s.io/client-go/rest"            // Added for rest.Config type in function variable
	"k8s.io/client-go/tools/clientcmd" // For clientcmd
	// kube and other imports will be added as needed
)

// NewK8sClientsetFromConfig is a package-level variable for creating a clientset from rest.Config.
// Exported to allow overriding in tests.
var NewK8sClientsetFromConfig = func(c *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(c)
}

// K8sNewNonInteractiveDeferredLoadingClientConfig is a package-level variable to allow mocking of clientcmd.NewNonInteractiveDeferredLoadingClientConfig.
var K8sNewNonInteractiveDeferredLoadingClientConfig = func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides)
}

// kubeManager is the concrete implementation of KubeManagerAPI.
type kubeManager struct {
	// No internal state needed for many of these operations as they rely on external config/commands.
}

// NewKubeManager creates a new instance of KubeManager.
func NewKubeManager() KubeManagerAPI {
	return &kubeManager{}
}

// --- KubeManagerAPI method implementations (stubs for now) ---

func (km *kubeManager) Login(clusterName string) (string, string, error) {
	return utils.LoginToKubeCluster(clusterName)
}

func (km *kubeManager) ListClusters() (*ClusterList, error) {
	utilsInfo, err := utils.GetClusterInfo()
	if err != nil {
		return nil, err
	}

	// Adapt utils.ClusterInfo to k8smanager.ClusterList
	kmlist := &ClusterList{
		ManagementClusters: make([]Cluster, 0),
		WorkloadClusters:   make(map[string][]Cluster),
		AllClusters:        make(map[string]Cluster),
		ContextNames:       make([]string, 0),
	}

	for _, mcName := range utilsInfo.ManagementClusters {
		kcContextName := utils.BuildMcContext(mcName) // Use existing util for now
		cluster := Cluster{
			Name:                  mcName,
			KubeconfigContextName: kcContextName,
			IsManagement:          true,
		}
		kmlist.ManagementClusters = append(kmlist.ManagementClusters, cluster)
		kmlist.AllClusters[kcContextName] = cluster
		if kcContextName != "" {
			kmlist.ContextNames = append(kmlist.ContextNames, kcContextName)
		}
	}

	for mcName, wcShortNames := range utilsInfo.WorkloadClusters {
		kmlist.WorkloadClusters[mcName] = make([]Cluster, 0)
		for _, wcShortName := range wcShortNames {
			kcContextName := utils.BuildWcContext(mcName, wcShortName) // Use existing util
			cluster := Cluster{
				Name:                  wcShortName, // Store short name as Name for WC
				KubeconfigContextName: kcContextName,
				IsManagement:          false,
				MCName:                mcName,
				WCShortName:           wcShortName,
			}
			kmlist.WorkloadClusters[mcName] = append(kmlist.WorkloadClusters[mcName], cluster)
			kmlist.AllClusters[kcContextName] = cluster
			if kcContextName != "" {
				kmlist.ContextNames = append(kmlist.ContextNames, kcContextName)
			}
		}
	}
	return kmlist, nil
}

func (km *kubeManager) GetCurrentContext() (string, error) {
	return kube.GetCurrentKubeContext()
}

func (km *kubeManager) SwitchContext(targetContextName string) error {
	return kube.SwitchKubeContext(targetContextName)
}

func (km *kubeManager) GetAvailableContexts() ([]string, error) {
	pathOptions := clientcmd.NewDefaultPathOptions()
	if pathOptions == nil {
		return nil, fmt.Errorf("failed to get default kubeconfig path options")
	}
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get starting kubeconfig: %w", err)
	}

	contexts := make([]string, 0, len(config.Contexts))
	for contextName := range config.Contexts {
		contexts = append(contexts, contextName)
	}
	return contexts, nil
}

func (km *kubeManager) BuildMcContextName(mcShortName string) string {
	return utils.BuildMcContext(mcShortName)
}

func (km *kubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	return utils.BuildWcContext(mcShortName, wcShortName)
}

func (km *kubeManager) StripTeleportPrefix(contextName string) string {
	return utils.StripTeleportPrefix(contextName)
}

func (km *kubeManager) HasTeleportPrefix(contextName string) bool {
	return utils.HasTeleportPrefix(contextName)
}

func (km *kubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (NodeHealth, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContextName}
	kubeConfig := K8sNewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		wrappedErr := fmt.Errorf("failed to get REST config for context %s: %w", kubeContextName, err)
		return NodeHealth{Error: wrappedErr}, wrappedErr
	}
	restConfig.Timeout = 15 * time.Second

	clientset, err := NewK8sClientsetFromConfig(restConfig)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to create Kubernetes clientset for context %s: %w", kubeContextName, err)
		return NodeHealth{Error: wrappedErr}, wrappedErr
	}

	ready, total, statusErr := kube.GetNodeStatusClientGo(clientset)
	if statusErr != nil {
		return NodeHealth{ReadyNodes: ready, TotalNodes: total, Error: statusErr}, statusErr
	}

	return NodeHealth{ReadyNodes: ready, TotalNodes: total, Error: nil}, nil
}

// func (km *kubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
// 	 // To be implemented if needed
// 	 panic("DetermineClusterProvider not implemented")
// }
