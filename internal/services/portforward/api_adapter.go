package portforward

import (
	"context"
	"envctl/internal/api"
)

// APIAdapter adapts the PortForwardService to implement api.PortForwardHandler
type APIAdapter struct {
	service *PortForwardService
}

// NewAPIAdapter creates a new port forward API adapter
func NewAPIAdapter(s *PortForwardService) *APIAdapter {
	return &APIAdapter{service: s}
}

// GetClusterLabel returns the cluster label for the port forward
func (a *APIAdapter) GetClusterLabel() string {
	// Cluster label is based on the Kubernetes context
	return a.service.config.KubeContextTarget
}

// GetNamespace returns the namespace for the port forward
func (a *APIAdapter) GetNamespace() string {
	return a.service.config.Namespace
}

// GetServiceName returns the service name for the port forward
func (a *APIAdapter) GetServiceName() string {
	return a.service.config.TargetName
}

// GetLocalPort returns the local port for the port forward
func (a *APIAdapter) GetLocalPort() int {
	return a.service.localPort
}

// GetRemotePort returns the remote port for the port forward
func (a *APIAdapter) GetRemotePort() int {
	return a.service.remotePort
}

// Register registers this adapter with the API package
func (a *APIAdapter) Register() {
	api.RegisterPortForward(a.service.GetLabel(), a)
}

// Unregister removes this adapter from the API package
func (a *APIAdapter) Unregister() {
	api.UnregisterPortForward(a.service.GetLabel())
}

// ToolProvider interface implementation
// These are no-ops since the global adapter handles tools

// GetTools returns no tools - the global adapter handles port forward tools
func (a *APIAdapter) GetTools() []api.ToolMetadata {
	return nil
}

// ExecuteTool is not implemented at the instance level
func (a *APIAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	return nil, nil
}

// PortForwardServiceHandler interface implementation (delegated to global adapter)

// ListForwards is handled by the global adapter
func (a *APIAdapter) ListForwards(ctx context.Context) ([]*api.PortForwardInfo, error) {
	// This should be handled by the global port forward service adapter
	return nil, nil
}

// GetForwardInfo is handled by the global adapter
func (a *APIAdapter) GetForwardInfo(ctx context.Context, label string) (*api.PortForwardInfo, error) {
	// This should be handled by the global port forward service adapter
	return nil, nil
}
