package kube

// ClusterInfo holds structured information about Kubernetes clusters, as parsed from `tsh kube ls` output.
// It differentiates between management clusters and their associated workload clusters.
type ClusterInfo struct {
	ManagementClusters []string            // A list of standalone management cluster names.
	WorkloadClusters   map[string][]string // A map where the key is a management cluster name,
	// and the value is a list of short workload cluster names belonging to that MC.
}

// NodeHealth represents the health status of nodes in a cluster
type NodeHealth struct {
	ReadyNodes int
	TotalNodes int
	Error      error
}
