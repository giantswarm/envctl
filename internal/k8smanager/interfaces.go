package k8smanager

import "context"

// Cluster represents basic info about a discovered cluster
type Cluster struct {
	Name                  string
	KubeconfigContextName string // The full context name from kubeconfig (e.g., teleport.giantswarm.io-mc-wc)
	IsManagement          bool   // True if it's an MC
	MCName                string // If WC, this is its parent MC's short name. Empty for MCs.
	WCShortName           string // If WC, its short name. Empty for MCs.
}

// ClusterList holds MCs and WCs
type ClusterList struct {
	ManagementClusters []Cluster            // List of Management Clusters
	WorkloadClusters   map[string][]Cluster // Key: MC short name, Value: List of its WCs
	AllClusters        map[string]Cluster   // Key: KubeconfigContextName, Value: Cluster struct for quick lookup
	ContextNames       []string             // All KubeconfigContextNames found by tsh kube ls
}

// NodeHealth represents basic node status
type NodeHealth struct {
	ReadyNodes int
	TotalNodes int
	Error      error
}

// KubeManagerAPI defines the interface for all Kubernetes related operations,
// including those previously in ClusterService and utils/kube functions.
type KubeManagerAPI interface {
	// Authentication & Setup - these often involve external commands like 'tsh'
	Login(clusterName string) (stdout string, stderr string, err error)
	ListClusters() (*ClusterList, error) // Parses 'tsh kube ls'

	// Context Management - direct kubeconfig interactions
	GetCurrentContext() (string, error)
	SwitchContext(targetContextName string) error
	GetAvailableContexts() ([]string, error) // Lists all contexts in current kubeconfig

	// Context Name Construction (Utilities, could be helpers outside interface too)
	BuildMcContextName(mcShortName string) string
	BuildWcContextName(mcShortName, wcShortName string) string
	StripTeleportPrefix(contextName string) string
	HasTeleportPrefix(contextName string) bool

	// Cluster Operations / Info - direct Kubernetes API interactions
	GetClusterNodeHealth(ctx context.Context, kubeContextName string) (NodeHealth, error)
}
