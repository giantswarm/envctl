package k8smanager

import (
	"context"
	"envctl/internal/kube" // For GetCurrentKubeContext, SwitchKubeContext
	"envctl/internal/reporting"
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
	reporter reporting.ServiceReporter
}

// NewKubeManager creates a new instance of KubeManager.
func NewKubeManager(reporter reporting.ServiceReporter) KubeManagerAPI {
	if reporter == nil {
		// Default to a ConsoleReporter if nil is provided, to prevent panics.
		// This ensures that KubeManager can always report, even if not in TUI mode.
		reporter = reporting.NewConsoleReporter()
	}
	return &kubeManager{reporter: reporter}
}

// SetReporter allows changing the reporter after initialization.
func (km *kubeManager) SetReporter(reporter reporting.ServiceReporter) {
	if reporter == nil {
		km.reporter = reporting.NewConsoleReporter() // Fallback
	} else {
		km.reporter = reporter
	}
}

// --- KubeManagerAPI method implementations (stubs for now) ---

func (km *kubeManager) Login(clusterName string) (string, string, error) {
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "Login",
		Level:       reporting.LogLevelInfo,
		Message:     fmt.Sprintf("Attempting to login to cluster: %s", clusterName),
	})

	stdout, stderr, err := utils.LoginToKubeCluster(clusterName)

	if err != nil {
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "Login",
			Level:       reporting.LogLevelError,
			Message:     fmt.Sprintf("Login failed for cluster %s", clusterName),
			Details:     fmt.Sprintf("Stdout: %s\\nStderr: %s", stdout, stderr),
			IsError:     true,
			ErrorDetail: err,
		})
		return stdout, stderr, err
	}

	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "Login",
		Level:       reporting.LogLevelInfo,
		Message:     fmt.Sprintf("Login successful for cluster: %s", clusterName),
		Details:     fmt.Sprintf("Stdout: %s\\nStderr: %s", stdout, stderr),
		IsReady:     true,
	})
	if stdout != "" {
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeExternalCmd,
			SourceLabel: "tsh-login-stdout",
			Level:       reporting.LogLevelStdout,
			Details:     stdout,
		})
	}
	if stderr != "" {
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeExternalCmd,
			SourceLabel: "tsh-login-stderr",
			Level:       reporting.LogLevelStderr,
			Details:     stderr,
		})
	}
	return stdout, stderr, nil
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
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "SwitchContext",
		Level:       reporting.LogLevelInfo,
		Message:     fmt.Sprintf("Attempting to switch Kubernetes context to: %s", targetContextName),
	})

	err := kube.SwitchKubeContext(targetContextName)
	if err != nil {
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "SwitchContext",
			Level:       reporting.LogLevelError,
			Message:     fmt.Sprintf("Failed to switch context to %s", targetContextName),
			IsError:     true,
			ErrorDetail: err,
		})
		return err
	}

	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "SwitchContext",
		Level:       reporting.LogLevelInfo,
		Message:     fmt.Sprintf("Successfully switched Kubernetes context to: %s", targetContextName),
		IsReady:     true,
	})
	return nil
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
	debugOperation := "GetClusterNodeHealth"
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: debugOperation,
		Level:       reporting.LogLevelDebug,
		Message:     fmt.Sprintf("Fetching node health for context: %s", kubeContextName),
	})

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContextName}
	kubeConfig := K8sNewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		wrappedErr := fmt.Errorf("failed to get REST config for context %s: %w", kubeContextName, err)
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: debugOperation,
			Level:       reporting.LogLevelError,
			Message:     "Failed to get REST config",
			Details:     wrappedErr.Error(),
			IsError:     true,
			ErrorDetail: wrappedErr,
		})
		return NodeHealth{Error: wrappedErr}, wrappedErr
	}
	restConfig.Timeout = 15 * time.Second

	clientset, err := NewK8sClientsetFromConfig(restConfig)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to create Kubernetes clientset for context %s: %w", kubeContextName, err)
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: debugOperation,
			Level:       reporting.LogLevelError,
			Message:     "Failed to create Kubernetes clientset",
			Details:     wrappedErr.Error(),
			IsError:     true,
			ErrorDetail: wrappedErr,
		})
		return NodeHealth{Error: wrappedErr}, wrappedErr
	}

	ready, total, statusErr := kube.GetNodeStatusClientGo(clientset)
	if statusErr != nil {
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: debugOperation,
			Level:       reporting.LogLevelError,
			Message:     fmt.Sprintf("Failed to get node status for %s", kubeContextName),
			Details:     statusErr.Error(),
			IsError:     true,
			ErrorDetail: statusErr,
		})
		return NodeHealth{ReadyNodes: ready, TotalNodes: total, Error: statusErr}, statusErr
	}

	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: debugOperation,
		Level:       reporting.LogLevelInfo,
		Message:     fmt.Sprintf("Node health for %s: %d/%d ready", kubeContextName, ready, total),
		IsReady:     ready == total && total > 0, // Consider ready if all nodes are ready and there's at least one node
	})
	return NodeHealth{ReadyNodes: ready, TotalNodes: total, Error: nil}, nil
}

func (km *kubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	// To be implemented if needed
	// panic("DetermineClusterProvider not implemented")
	// For now, let's call the actual kube function, passing through the context.
	// If a more specific context is needed here (e.g., with timeout), it can be created.
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "DetermineClusterProvider",
		Level:       reporting.LogLevelDebug,
		Message:     fmt.Sprintf("Determining cluster provider for context: %s", kubeContextName),
	})

	provider, err := kube.DetermineClusterProvider(ctx, kubeContextName)
	if err != nil {
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "DetermineClusterProvider",
			Level:       reporting.LogLevelError,
			Message:     fmt.Sprintf("Failed to determine cluster provider for %s", kubeContextName),
			Details:     err.Error(),
			IsError:     true,
			ErrorDetail: err,
		})
		return provider, err
	}
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "DetermineClusterProvider",
		Level:       reporting.LogLevelInfo,
		Message:     fmt.Sprintf("Determined cluster provider for %s: %s", kubeContextName, provider),
	})
	return provider, nil
}
