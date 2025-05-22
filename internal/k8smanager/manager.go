package k8smanager

import (
	"context"
	"envctl/internal/kube" // For GetCurrentKubeContext, SwitchKubeContext
	"envctl/internal/reporting"
	"envctl/internal/utils"
	"envctl/pkg/logging" // Added for logging
	"fmt"                // For GetAvailableContexts error handling
	"time"               // For restConfig.Timeout

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
	subsystem := "KubeLogin-" + clusterName
	logging.Info(subsystem, "Attempting to login to cluster: %s", clusterName)
	// Reporter update for TUI (state change)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "Login-" + clusterName,
		State:       reporting.StateStarting, // Or a custom "LoginAttempting" state
		ServiceLevel: reporting.LogLevelInfo,
	})

	stdout, stderr, err := utils.LoginToKubeCluster(clusterName)

	if err != nil {
		logging.Error(subsystem, err, "Login failed. Stdout: %s, Stderr: %s", stdout, stderr)
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "Login-" + clusterName,
			State:       reporting.StateFailed,
			ServiceLevel: reporting.LogLevelError,
			ErrorDetail: err,
		})
		// Log stdout/stderr from tsh as separate log entries if they contain useful info
		if stdout != "" {
			logging.Info(subsystem+"-stdout", "tsh stdout: %s", stdout)
		}
		if stderr != "" {
			// Stderr from tsh might not be a Go 'error' per se but still error-indicative output
			logging.Warn(subsystem+"-stderr", "tsh stderr: %s", stderr) 
		}
		return stdout, stderr, err
	}

	logging.Info(subsystem, "Login successful. Stdout: %s, Stderr: %s", stdout, stderr)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "Login-" + clusterName,
		State:       reporting.StateRunning, // Or a custom "LoginSuccessful" state
		ServiceLevel: reporting.LogLevelInfo,
		IsReady:     true,
	})
	// Log stdout/stderr from tsh. These are not errors but command output.
	// ServiceManager used to send these via reporter with Details, now we log directly.
	if stdout != "" {
		// Log as Info or Debug. SourceTypeExternalCmd might be too generic if these logs are always from tsh login.
		// For now, using subsystem specific to tsh output.
		logging.Info(subsystem+"-stdout", "tsh stdout: %s", stdout)
	}
	if stderr != "" {
		logging.Info(subsystem+"-stderr", "tsh stderr: %s", stderr)
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
	subsystem := "KubeSwitchContext-" + targetContextName
	logging.Info(subsystem, "Attempting to switch Kubernetes context to: %s", targetContextName)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "SwitchContext-" + targetContextName,
		State:       reporting.StateStarting, // Or "ContextSwitching"
		ServiceLevel: reporting.LogLevelInfo,
	})

	err := kube.SwitchKubeContext(targetContextName)
	if err != nil {
		logging.Error(subsystem, err, "Failed to switch context")
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "SwitchContext-" + targetContextName,
			State:       reporting.StateFailed,
			ServiceLevel: reporting.LogLevelError,
			ErrorDetail: err,
		})
		return err
	}

	logging.Info(subsystem, "Successfully switched Kubernetes context")
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: "SwitchContext-" + targetContextName,
		State:       reporting.StateRunning, // Or "ContextSwitched"
		ServiceLevel: reporting.LogLevelInfo,
		IsReady:     true, // Assuming context switch implies readiness for operations in that context
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
	debugOperation := "GetClusterNodeHealth-" + kubeContextName
	logging.Debug(debugOperation, "Fetching node health for context: %s", kubeContextName)
	// Reporter update for TUI (state change)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: debugOperation,
		State:       reporting.StateStarting, // Or "FetchingHealth"
		ServiceLevel: reporting.LogLevelDebug, // Since it's a debug-level operation for logging
	})

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContextName}
	kubeConfig := K8sNewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		wrappedErr := fmt.Errorf("failed to get REST config for context %s: %w", kubeContextName, err)
		logging.Error(debugOperation, wrappedErr, "Failed to get REST config")
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: debugOperation,
			State:       reporting.StateFailed,
			ServiceLevel: reporting.LogLevelError,
			ErrorDetail: wrappedErr,
		})
		return NodeHealth{Error: wrappedErr}, wrappedErr
	}
	restConfig.Timeout = 15 * time.Second

	clientset, err := NewK8sClientsetFromConfig(restConfig)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to create Kubernetes clientset for context %s: %w", kubeContextName, err)
		logging.Error(debugOperation, wrappedErr, "Failed to create Kubernetes clientset")
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: debugOperation,
			State:       reporting.StateFailed,
			ServiceLevel: reporting.LogLevelError,
			ErrorDetail: wrappedErr,
		})
		return NodeHealth{Error: wrappedErr}, wrappedErr
	}

	ready, total, statusErr := kube.GetNodeStatusClientGo(clientset)
	if statusErr != nil {
		logging.Error(debugOperation, statusErr, "Failed to get node status")
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: debugOperation,
			State:       reporting.StateFailed,
			ServiceLevel: reporting.LogLevelError,
			ErrorDetail: statusErr,
		})
		return NodeHealth{ReadyNodes: ready, TotalNodes: total, Error: statusErr}, statusErr
	}

	logging.Debug(debugOperation, "Node health for %s: %d/%d ready", kubeContextName, ready, total)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: debugOperation,
		State:       reporting.StateRunning, // Or "HealthRetrieved"
		ServiceLevel: reporting.LogLevelInfo, // Changed from Debug for a successful outcome
		IsReady:     ready == total && total > 0, 
	})
	return NodeHealth{ReadyNodes: ready, TotalNodes: total, Error: nil}, nil
}

func (km *kubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	subsystem := "DetermineClusterProvider-" + kubeContextName
	logging.Debug(subsystem, "Determining cluster provider for context: %s", kubeContextName)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: subsystem,
		State:       reporting.StateStarting, // Or "DeterminingProvider"
		ServiceLevel: reporting.LogLevelDebug,
	})

	provider, err := kube.DetermineClusterProvider(ctx, kubeContextName)
	if err != nil {
		logging.Error(subsystem, err, "Failed to determine cluster provider")
		km.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: subsystem,
			State:       reporting.StateFailed,
			ServiceLevel: reporting.LogLevelError,
			ErrorDetail: err,
		})
		return provider, err
	}
	logging.Info(subsystem, "Determined cluster provider for %s: %s", kubeContextName, provider)
	km.reporter.Report(reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeKube,
		SourceLabel: subsystem,
		State:       reporting.StateRunning, // Or "ProviderDetermined"
		ServiceLevel: reporting.LogLevelInfo,
		// IsReady not directly applicable here, but the operation succeeded.
	})
	return provider, nil
}
