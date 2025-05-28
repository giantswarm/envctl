// Package kube provides Kubernetes cluster management functionality for envctl.
//
// This package handles all interactions with Kubernetes clusters through Teleport,
// including authentication, context management, and cluster information retrieval.
// It serves as the foundation layer for all Kubernetes-related operations in envctl.
//
// # Core Components
//
// Manager: The central component that manages Kubernetes contexts and provides
// cluster information. It wraps kubectl and tsh commands to interact with clusters.
//
// Context Management: Handles switching between different Kubernetes contexts,
// building context names for management and workload clusters, and tracking
// the current active context.
//
// Cluster Discovery: Provides functionality to list available clusters through
// Teleport, distinguishing between management clusters and their associated
// workload clusters.
//
// # Teleport Integration
//
// All cluster access goes through Teleport (tsh) for secure authentication:
//
//   - LoginToKubeCluster: Uses 'tsh kube login' to authenticate to a cluster
//   - Context names follow the pattern: teleport.giantswarm.io-{cluster-name}
//   - Supports both management clusters (MC) and workload clusters (WC)
//
// # Context Naming Convention
//
// The package follows Giant Swarm's naming conventions:
//
//   - Management Cluster: teleport.giantswarm.io-{mc-name}
//   - Workload Cluster: teleport.giantswarm.io-{mc-name}-{wc-name}
//
// # Usage Example
//
//	// Create a manager
//	mgr := kube.NewManager(nil)
//
//	// Login to a cluster
//	stdout, stderr, err := kube.LoginToKubeCluster("myinstallation")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get current context
//	ctx, err := mgr.GetCurrentContext()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List available clusters
//	info, err := mgr.ListClusters()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Switch context
//	if err := mgr.SwitchContext("teleport.giantswarm.io-myinstallation"); err != nil {
//	    log.Fatal(err)
//	}
//
// # Error Handling
//
// The package provides detailed error messages for common issues:
//
//   - Authentication failures with Teleport
//   - Missing kubectl or tsh binaries
//   - Invalid context names
//   - Network connectivity issues
//
// # Thread Safety
//
// The Manager type is thread-safe and can be used concurrently from multiple
// goroutines. It uses internal locking to ensure consistent state.
package kube
