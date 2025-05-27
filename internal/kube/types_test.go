package kube

import (
	"errors"
	"testing"
)

func TestClusterInfo(t *testing.T) {
	// Test ClusterInfo structure
	clusterInfo := ClusterInfo{
		ManagementClusters: []string{"mc1", "mc2"},
		WorkloadClusters: map[string][]string{
			"mc1": {"wc1", "wc2"},
			"mc2": {"wc3"},
		},
	}

	// Test ManagementClusters
	if len(clusterInfo.ManagementClusters) != 2 {
		t.Errorf("Expected 2 management clusters, got %d", len(clusterInfo.ManagementClusters))
	}

	if clusterInfo.ManagementClusters[0] != "mc1" {
		t.Errorf("Expected first MC to be 'mc1', got %s", clusterInfo.ManagementClusters[0])
	}

	if clusterInfo.ManagementClusters[1] != "mc2" {
		t.Errorf("Expected second MC to be 'mc2', got %s", clusterInfo.ManagementClusters[1])
	}

	// Test WorkloadClusters
	if len(clusterInfo.WorkloadClusters) != 2 {
		t.Errorf("Expected 2 MC entries in WorkloadClusters, got %d", len(clusterInfo.WorkloadClusters))
	}

	mc1Workloads, exists := clusterInfo.WorkloadClusters["mc1"]
	if !exists {
		t.Error("Expected mc1 to exist in WorkloadClusters")
	}

	if len(mc1Workloads) != 2 {
		t.Errorf("Expected 2 workload clusters for mc1, got %d", len(mc1Workloads))
	}

	if mc1Workloads[0] != "wc1" || mc1Workloads[1] != "wc2" {
		t.Errorf("Expected workload clusters [wc1, wc2] for mc1, got %v", mc1Workloads)
	}

	mc2Workloads, exists := clusterInfo.WorkloadClusters["mc2"]
	if !exists {
		t.Error("Expected mc2 to exist in WorkloadClusters")
	}

	if len(mc2Workloads) != 1 {
		t.Errorf("Expected 1 workload cluster for mc2, got %d", len(mc2Workloads))
	}

	if mc2Workloads[0] != "wc3" {
		t.Errorf("Expected workload cluster 'wc3' for mc2, got %s", mc2Workloads[0])
	}
}

func TestNodeHealth(t *testing.T) {
	// Test NodeHealth with all nodes ready
	health := NodeHealth{
		ReadyNodes: 3,
		TotalNodes: 3,
		Error:      nil,
	}

	if health.ReadyNodes != 3 {
		t.Errorf("Expected 3 ready nodes, got %d", health.ReadyNodes)
	}

	if health.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", health.TotalNodes)
	}

	if health.Error != nil {
		t.Errorf("Expected no error, got %v", health.Error)
	}

	// Test NodeHealth with some nodes not ready
	health2 := NodeHealth{
		ReadyNodes: 2,
		TotalNodes: 3,
		Error:      nil,
	}

	if health2.ReadyNodes != 2 {
		t.Errorf("Expected 2 ready nodes, got %d", health2.ReadyNodes)
	}

	if health2.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", health2.TotalNodes)
	}
}

func TestEmptyClusterInfo(t *testing.T) {
	// Test empty ClusterInfo
	clusterInfo := ClusterInfo{}

	if clusterInfo.ManagementClusters != nil {
		t.Errorf("Expected nil ManagementClusters, got %v", clusterInfo.ManagementClusters)
	}

	if clusterInfo.WorkloadClusters != nil {
		t.Errorf("Expected nil WorkloadClusters, got %v", clusterInfo.WorkloadClusters)
	}
}

func TestNodeHealthWithError(t *testing.T) {
	// Test NodeHealth with error
	testErr := errors.New("failed to get node health")
	health := NodeHealth{
		ReadyNodes: 0,
		TotalNodes: 0,
		Error:      testErr,
	}

	if health.Error == nil {
		t.Error("Expected error to be set")
	}

	if health.Error.Error() != testErr.Error() {
		t.Errorf("Expected error %v, got %v", testErr, health.Error)
	}
}

func TestNodeHealthZeroValues(t *testing.T) {
	// Test NodeHealth with zero values
	nodeHealth := NodeHealth{}

	if nodeHealth.ReadyNodes != 0 {
		t.Errorf("Expected ReadyNodes to be 0, got %d", nodeHealth.ReadyNodes)
	}

	if nodeHealth.TotalNodes != 0 {
		t.Errorf("Expected TotalNodes to be 0, got %d", nodeHealth.TotalNodes)
	}

	if nodeHealth.Error != nil {
		t.Errorf("Expected no error, got %v", nodeHealth.Error)
	}
}
