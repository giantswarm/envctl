package k8s

import (
	"context"
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

// ToolProvider interface implementation
// These are no-ops since the global adapter handles tools

// GetTools returns no tools - the global adapter handles K8s tools
func (a *APIAdapter) GetTools() []api.ToolMetadata {
	return nil
}

// ExecuteTool is not implemented at the instance level
func (a *APIAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	return nil, nil
}

// K8sServiceHandler interface implementation (delegated to global adapter)

// ListConnections is handled by the global adapter
func (a *APIAdapter) ListConnections(ctx context.Context) ([]*api.K8sConnectionInfo, error) {
	// This should be handled by the global K8s service adapter
	return nil, nil
}

// GetConnectionInfo is handled by the global adapter  
func (a *APIAdapter) GetConnectionInfo(ctx context.Context, label string) (*api.K8sConnectionInfo, error) {
	// This should be handled by the global K8s service adapter
	return nil, nil
}

// GetConnectionByContext is handled by the global adapter
func (a *APIAdapter) GetConnectionByContext(ctx context.Context, contextName string) (*api.K8sConnectionInfo, error) {
	// This should be handled by the global K8s service adapter
	return nil, nil
}
