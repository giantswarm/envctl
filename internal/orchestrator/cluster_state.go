package orchestrator

import (
	"envctl/internal/config"
	"fmt"
	"sync"
)

// ClusterState tracks runtime cluster configuration and active clusters
type ClusterState struct {
	mu sync.RWMutex

	// Available clusters by role
	availableClusters map[config.ClusterRole][]config.ClusterDefinition

	// Currently active cluster for each role
	activeClusters map[config.ClusterRole]string // role -> cluster name

	// All clusters indexed by name for quick lookup
	clustersByName map[string]config.ClusterDefinition
}

// NewClusterState creates a new cluster state from configuration
func NewClusterState(clusters []config.ClusterDefinition, activeClusters map[config.ClusterRole]string) *ClusterState {
	cs := &ClusterState{
		availableClusters: make(map[config.ClusterRole][]config.ClusterDefinition),
		activeClusters:    make(map[config.ClusterRole]string),
		clustersByName:    make(map[string]config.ClusterDefinition),
	}

	// Index clusters by role and name
	for _, cluster := range clusters {
		cs.availableClusters[cluster.Role] = append(cs.availableClusters[cluster.Role], cluster)
		cs.clustersByName[cluster.Name] = cluster
	}

	// Set initial active clusters
	for role, clusterName := range activeClusters {
		cs.activeClusters[role] = clusterName
	}

	return cs
}

// SetActiveCluster sets the active cluster for a role
func (cs *ClusterState) SetActiveCluster(role config.ClusterRole, clusterName string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Validate cluster exists for role
	clusters, exists := cs.availableClusters[role]
	if !exists {
		return fmt.Errorf("no clusters defined for role %s", role)
	}

	found := false
	for _, c := range clusters {
		if c.Name == clusterName {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("cluster %s not found for role %s", clusterName, role)
	}

	cs.activeClusters[role] = clusterName
	return nil
}

// GetActiveCluster returns the active cluster name for a role
func (cs *ClusterState) GetActiveCluster(role config.ClusterRole) (string, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	name, exists := cs.activeClusters[role]
	return name, exists
}

// GetActiveClusterContext returns the kubernetes context for the active cluster of a role
func (cs *ClusterState) GetActiveClusterContext(role config.ClusterRole) (string, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	clusterName, exists := cs.activeClusters[role]
	if !exists {
		return "", fmt.Errorf("no active cluster for role %s", role)
	}

	cluster, exists := cs.clustersByName[clusterName]
	if !exists {
		return "", fmt.Errorf("cluster %s not found", clusterName)
	}

	return cluster.Context, nil
}

// GetClusterByName returns a cluster definition by name
func (cs *ClusterState) GetClusterByName(name string) (config.ClusterDefinition, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cluster, exists := cs.clustersByName[name]
	return cluster, exists
}

// GetAvailableClusters returns all clusters for a role
func (cs *ClusterState) GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.availableClusters[role]
}

// GetAllClusters returns all configured clusters
func (cs *ClusterState) GetAllClusters() []config.ClusterDefinition {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var clusters []config.ClusterDefinition
	for _, cluster := range cs.clustersByName {
		clusters = append(clusters, cluster)
	}
	return clusters
}
