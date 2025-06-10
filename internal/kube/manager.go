package kube

import (
	"context"
	"envctl/pkg/logging"
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Manager provides Kubernetes cluster management functionality
type Manager interface {
	// Authentication & Setup
	Login(clusterName string) (stdout string, stderr string, err error)
	ListClusters() (*ClusterInfo, error)

	// Authentication Provider Management
	SetAuthProvider(provider AuthProvider)
	GetAuthProvider() AuthProvider

	// Context Management
	GetCurrentContext() (string, error)
	SwitchContext(targetContextName string) error
	GetAvailableContexts() ([]string, error)

	// Context Name Construction
	BuildMcContextName(mcName string) string
	BuildWcContextName(mcName, wcName string) string
	StripTeleportPrefix(contextName string) string
	HasTeleportPrefix(contextName string) bool

	// Cluster Operations
	GetClusterNodeHealth(ctx context.Context, kubeContextName string) (NodeHealth, error)
	DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error)
	CheckAPIHealth(ctx context.Context, kubeContextName string) (string, error)
}

// manager implements the Manager interface
type manager struct {
	clientCache  map[string]*kubernetes.Clientset
	configCache  map[string]*rest.Config
	authProvider AuthProvider
	mu           sync.RWMutex
}

// NewManager creates a new Kubernetes manager
func NewManager(reporter interface{}) Manager {
	// Reporter is no longer used
	m := &manager{
		clientCache: make(map[string]*kubernetes.Clientset),
		configCache: make(map[string]*rest.Config),
	}
	// Initialize with legacy Teleport provider for backward compatibility
	m.authProvider = NewLegacyTeleportAuthProvider()
	return m
}

// SetAuthProvider sets the authentication provider
func (m *manager) SetAuthProvider(provider AuthProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authProvider = provider
}

// GetAuthProvider returns the current authentication provider
func (m *manager) GetAuthProvider() AuthProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authProvider
}

// Login performs a Kubernetes cluster login
func (m *manager) Login(clusterName string) (string, string, error) {
	subsystem := fmt.Sprintf("KubeLogin-%s", clusterName)
	logging.Debug(subsystem, "Attempting to login to cluster: %s", clusterName)

	// Use authentication provider if available
	if m.authProvider != nil {
		logging.Debug(subsystem, "Using auth provider: %s", m.authProvider.GetProviderName())
		return m.authProvider.Login(context.Background(), clusterName)
	}

	// Fallback to direct call (should not happen with default initialization)
	stdout, stderr, err := LoginToKubeCluster(clusterName)
	return stdout, stderr, err
}

// ListClusters returns structured information about available clusters
func (m *manager) ListClusters() (*ClusterInfo, error) {
	// Use authentication provider if available
	if m.authProvider != nil {
		return m.authProvider.ListClusters(context.Background())
	}

	// Fallback to direct call (should not happen with default initialization)
	return GetClusterInfo()
}

// GetCurrentContext returns the current Kubernetes context
func (m *manager) GetCurrentContext() (string, error) {
	return GetCurrentKubeContext()
}

// SwitchContext switches to a different Kubernetes context
func (m *manager) SwitchContext(targetContextName string) error {
	subsystem := fmt.Sprintf("KubeSwitchContext-%s", targetContextName)
	logging.Info(subsystem, "Attempting to switch Kubernetes context to: %s", targetContextName)

	err := SwitchKubeContext(targetContextName)

	return err
}

// GetAvailableContexts returns all available Kubernetes contexts
func (m *manager) GetAvailableContexts() ([]string, error) {
	config, err := GetStartingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	contexts := make([]string, 0, len(config.Contexts))
	for contextName := range config.Contexts {
		contexts = append(contexts, contextName)
	}
	return contexts, nil
}

// BuildMcContextName builds a management cluster context name
func (m *manager) BuildMcContextName(mcName string) string {
	return BuildMcContext(mcName)
}

// BuildWcContextName builds a workload cluster context name
func (m *manager) BuildWcContextName(mcName, wcName string) string {
	return BuildWcContext(mcName, wcName)
}

// StripTeleportPrefix removes the teleport prefix from a context name
func (m *manager) StripTeleportPrefix(contextName string) string {
	return StripTeleportPrefix(contextName)
}

// HasTeleportPrefix checks if a context name has the teleport prefix
func (m *manager) HasTeleportPrefix(contextName string) bool {
	return HasTeleportPrefix(contextName)
}

// GetClusterNodeHealth gets the health status of a cluster
func (m *manager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (NodeHealth, error) {
	debugOperation := fmt.Sprintf("GetClusterNodeHealth-%s", kubeContextName)
	logging.Debug(debugOperation, "Fetching node health for context: %s", kubeContextName)

	// Get clientset for the context
	clientset, err := GetClientsetForContext(ctx, kubeContextName)
	if err != nil {
		logging.Error(debugOperation, err, "Failed to create clientset")
		return NodeHealth{Error: err}, err
	}

	// Get node status
	readyNodes, totalNodes, err := GetNodeStatus(clientset)
	health := NodeHealth{
		ReadyNodes: readyNodes,
		TotalNodes: totalNodes,
		Error:      err,
	}

	return health, err
}

// DetermineClusterProvider determines the cloud provider for a cluster
func (m *manager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	subsystem := fmt.Sprintf("DetermineClusterProvider-%s", kubeContextName)
	logging.Debug(subsystem, "Determining cluster provider for context: %s", kubeContextName)

	provider, err := DetermineClusterProvider(ctx, kubeContextName)

	return provider, err
}

// CheckAPIHealth checks the API health of a cluster
func (m *manager) CheckAPIHealth(ctx context.Context, kubeContextName string) (string, error) {
	subsystem := fmt.Sprintf("CheckAPIHealth-%s", kubeContextName)
	logging.Debug(subsystem, "Checking API health for context: %s", kubeContextName)

	// Get clientset for the context
	clientset, err := GetClientsetForContext(ctx, kubeContextName)
	if err != nil {
		logging.Error(subsystem, err, "Failed to create clientset")
		return "", err
	}

	// Check API health
	version, err := CheckAPIHealth(clientset)

	return version, err
}
