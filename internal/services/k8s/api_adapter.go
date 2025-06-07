package k8s

import (
	"envctl/internal/api"
)

// APIAdapter adapts the K8sConnectionService to implement api.K8sServiceHandler
type APIAdapter struct {
	service *K8sConnectionService
}

// NewAPIAdapter creates a new K8s service API adapter
func NewAPIAdapter(s *K8sConnectionService) *APIAdapter {
	return &APIAdapter{service: s}
}

// GetClusterLabel returns the cluster label for the K8s connection
func (a *APIAdapter) GetClusterLabel() string {
	return a.service.GetLabel()
}

// GetMetadata returns metadata about the K8s connection
func (a *APIAdapter) GetMetadata() map[string]interface{} {
	// Get service data which includes metadata
	data := a.service.GetServiceData()

	// Extract metadata fields
	metadata := make(map[string]interface{})
	if context, ok := data["context"].(string); ok {
		metadata["context"] = context
	}
	if cluster, ok := data["cluster"].(string); ok {
		metadata["cluster"] = cluster
	}
	if role, ok := data["role"].(string); ok {
		metadata["role"] = role
	}
	if displayName, ok := data["displayName"].(string); ok {
		metadata["displayName"] = displayName
	}
	if icon, ok := data["icon"].(string); ok {
		metadata["icon"] = icon
	}

	return metadata
}

// Register registers this adapter with the API package
func (a *APIAdapter) Register() {
	api.RegisterK8sService(a.service.GetLabel(), a)
}

// Unregister removes this adapter from the API package
func (a *APIAdapter) Unregister() {
	api.UnregisterK8sService(a.service.GetLabel())
}
