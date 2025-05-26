package kube

import (
	"context"
	"envctl/internal/reporting"
	"envctl/internal/state"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// Manager provides a unified interface for all Kubernetes-related operations
type Manager interface {
	// Authentication & Setup
	Login(clusterName string) (stdout string, stderr string, err error)
	ListClusters() (*ClusterInfo, error)

	// Context Management
	GetCurrentContext() (string, error)
	SwitchContext(targetContextName string) error
	GetAvailableContexts() ([]string, error)

	// Context Name Construction
	BuildMcContextName(mcShortName string) string
	BuildWcContextName(mcShortName, wcShortName string) string
	StripTeleportPrefix(contextName string) string
	HasTeleportPrefix(contextName string) bool

	// Cluster Operations
	GetClusterNodeHealth(ctx context.Context, kubeContextName string) (NodeHealth, error)
	DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error)

	// State Management
	GetK8sStateManager() state.K8sStateManager
	SetReporter(reporter reporting.ServiceReporter)
}

// manager is the concrete implementation of Manager
type manager struct {
	reporter reporting.ServiceReporter
	stateMgr state.K8sStateManager
	mu       sync.RWMutex
}

// NewManager creates a new K8s manager instance
func NewManager(reporter reporting.ServiceReporter) Manager {
	if reporter == nil {
		reporter = reporting.NewConsoleReporter()
	}
	return &manager{
		reporter: reporter,
		stateMgr: state.NewK8sStateManager(),
	}
}

// SetReporter allows changing the reporter after initialization
func (m *manager) SetReporter(reporter reporting.ServiceReporter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reporter == nil {
		m.reporter = reporting.NewConsoleReporter()
	} else {
		m.reporter = reporter
	}
}

// GetK8sStateManager returns the K8s state manager
func (m *manager) GetK8sStateManager() state.K8sStateManager {
	return m.stateMgr
}

// Login performs a Kubernetes cluster login
func (m *manager) Login(clusterName string) (string, string, error) {
	subsystem := fmt.Sprintf("KubeLogin-%s", clusterName)
	logging.Debug(subsystem, "Attempting to login to cluster: %s", clusterName)

	// Report login starting
	if m.reporter != nil {
		m.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "Login-" + clusterName,
			State:       reporting.StateStarting,
			CausedBy:    "user_login",
		})
	}

	// Perform the actual login
	stdout, stderr, err := LoginToKubeCluster(clusterName)

	// Update state and report result
	contextName := BuildMcContext(clusterName)
	if IsWorkloadClusterName(clusterName) {
		// Extract MC and WC names for proper context building
		mcName, wcName := ParseWorkloadClusterName(clusterName)
		if mcName != "" && wcName != "" {
			contextName = BuildWcContext(mcName, wcName)
		}
	}

	if err != nil {
		logging.Error(subsystem, err, "Login failed")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "Login-" + clusterName,
				State:       reporting.StateFailed,
				ErrorDetail: err,
				CausedBy:    "user_login",
			})
		}
		// Update state manager
		m.stateMgr.UpdateConnectionState(contextName, state.K8sConnectionState{
			ContextName:     contextName,
			IsHealthy:       false,
			Error:           err,
			LastHealthCheck: time.Now(),
		})
	} else {
		logging.Info(subsystem, "Login successful")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "Login-" + clusterName,
				State:       reporting.StateRunning,
				IsReady:     true,
				CausedBy:    "user_login",
			})
		}
		// Update state manager - mark as authenticated
		m.stateMgr.UpdateConnectionState(contextName, state.K8sConnectionState{
			ContextName:     contextName,
			IsHealthy:       true,
			LastHealthCheck: time.Now(),
		})
	}

	return stdout, stderr, err
}

// ListClusters returns structured information about available clusters
func (m *manager) ListClusters() (*ClusterInfo, error) {
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

	// Report switch starting
	if m.reporter != nil {
		m.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "SwitchContext",
			State:       reporting.StateStarting,
			CausedBy:    "context_switch",
		})
	}

	err := SwitchKubeContext(targetContextName)

	if err != nil {
		logging.Error(subsystem, err, "Context switch failed")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "SwitchContext",
				State:       reporting.StateFailed,
				ErrorDetail: err,
				CausedBy:    "context_switch",
			})
		}
	} else {
		logging.Info(subsystem, "Context switch successful")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "SwitchContext",
				State:       reporting.StateRunning,
				IsReady:     true,
				CausedBy:    "context_switch",
			})
		}
	}

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
func (m *manager) BuildMcContextName(mcShortName string) string {
	return BuildMcContext(mcShortName)
}

// BuildWcContextName builds a workload cluster context name
func (m *manager) BuildWcContextName(mcShortName, wcShortName string) string {
	return BuildWcContext(mcShortName, wcShortName)
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

	// Report health check starting
	if m.reporter != nil {
		m.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: "HealthCheck-" + kubeContextName,
			State:       reporting.StateStarting,
			CausedBy:    "health_check",
		})
	}

	// Get clientset for the context
	clientset, err := GetClientsetForContext(ctx, kubeContextName)
	if err != nil {
		logging.Error(debugOperation, err, "Failed to create clientset")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "HealthCheck-" + kubeContextName,
				State:       reporting.StateFailed,
				ErrorDetail: err,
				CausedBy:    "health_check",
			})
		}
		return NodeHealth{Error: err}, err
	}

	// Get node status
	readyNodes, totalNodes, err := GetNodeStatus(clientset)
	health := NodeHealth{
		ReadyNodes: readyNodes,
		TotalNodes: totalNodes,
		Error:      err,
	}

	// Update state manager
	m.stateMgr.UpdateConnectionState(kubeContextName, state.K8sConnectionState{
		ContextName:     kubeContextName,
		IsHealthy:       err == nil,
		Error:           err,
		LastHealthCheck: time.Now(),
		ReadyNodes:      readyNodes,
		TotalNodes:      totalNodes,
	})

	// Report result
	if err != nil {
		logging.Error(debugOperation, err, "Health check failed")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "HealthCheck-" + kubeContextName,
				State:       reporting.StateFailed,
				ErrorDetail: err,
				CausedBy:    "health_check",
			})
		}
	} else {
		logging.Debug(debugOperation, "Health check successful: %d/%d nodes ready", readyNodes, totalNodes)
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: "HealthCheck-" + kubeContextName,
				State:       reporting.StateRunning,
				IsReady:     true,
				CausedBy:    "health_check",
			})
		}
	}

	return health, err
}

// DetermineClusterProvider determines the cloud provider for a cluster
func (m *manager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	subsystem := fmt.Sprintf("DetermineClusterProvider-%s", kubeContextName)
	logging.Debug(subsystem, "Determining cluster provider for context: %s", kubeContextName)

	// Report operation starting
	if m.reporter != nil {
		m.reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeKube,
			SourceLabel: subsystem,
			State:       reporting.StateStarting,
			CausedBy:    "provider_check",
		})
	}

	provider, err := DetermineClusterProvider(ctx, kubeContextName)

	if err != nil {
		logging.Error(subsystem, err, "Failed to determine provider")
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: subsystem,
				State:       reporting.StateFailed,
				ErrorDetail: err,
				CausedBy:    "provider_check",
			})
		}
	} else {
		logging.Info(subsystem, "Determined provider: %s", provider)
		if m.reporter != nil {
			m.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeKube,
				SourceLabel: subsystem,
				State:       reporting.StateRunning,
				IsReady:     true,
				CausedBy:    "provider_check",
			})
		}
	}

	return provider, err
}
