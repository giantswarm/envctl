package view

import (
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// ClusterListItem represents a cluster in the list
type ClusterListItem struct {
	BaseListItem
	ClusterType string // "MC" or "WC"
	ReadyNodes  int
	TotalNodes  int
}

// Title returns the display title for the cluster
func (i ClusterListItem) Title() string {
	prefix := i.ClusterType + ": "
	return prefix + i.Name
}

// Description returns the display description for the cluster
func (i ClusterListItem) Description() string {
	nodeInfo := fmt.Sprintf("Nodes: %d/%d", i.ReadyNodes, i.TotalNodes)
	return nodeInfo + " " + i.BaseListItem.Description
}

// ConvertK8sConnectionToListItem converts API K8s connection to list item
func ConvertK8sConnectionToListItem(conn *api.K8sConnectionInfo) ClusterListItem {
	// Determine cluster type from label
	clusterType := "WC"
	if strings.Contains(strings.ToLower(conn.Label), "mc") ||
		strings.Contains(strings.ToLower(conn.Label), "management") {
		clusterType = "MC"
	}

	// Map API status to our common status
	var status ServiceStatus
	switch strings.ToLower(conn.State) {
	case "connected", "running":
		status = StatusRunning
	case "failed":
		status = StatusFailed
	case "connecting", "starting":
		status = StatusStarting
	case "disconnected", "stopped":
		status = StatusStopped
	default:
		status = StatusUnknown
	}

	// Map API health to our common health
	var health ServiceHealth
	if strings.ToLower(conn.Health) == "healthy" {
		health = HealthHealthy
	} else if strings.ToLower(conn.Health) == "unhealthy" {
		health = HealthUnhealthy
	} else if conn.ReadyNodes < conn.TotalNodes {
		health = HealthDegraded
	} else {
		health = HealthUnknown
	}

	return ClusterListItem{
		BaseListItem: BaseListItem{
			ID:          conn.Label,
			Name:        conn.Label,
			Status:      status,
			Health:      health,
			Icon:        SafeIcon(IconKubernetes),
			Description: conn.Context,
			Details:     fmt.Sprintf("Context: %s", conn.Context),
		},
		ClusterType: clusterType,
		ReadyNodes:  conn.ReadyNodes,
		TotalNodes:  conn.TotalNodes,
	}
}

// BuildClustersList creates a list model for clusters
func BuildClustersList(m *model.Model, width, height int, focused bool) *ServiceListModel {
	items := []list.Item{}

	// Add clusters in order
	for _, label := range m.K8sConnectionOrder {
		if conn, exists := m.K8sConnections[label]; exists {
			items = append(items, ConvertK8sConnectionToListItem(conn))
		}
	}

	l := CreateStyledList("Clusters (K8s Contexts)", items, width, height, focused)

	return &ServiceListModel{
		List:     l,
		ListType: "clusters",
	}
}
