package portforward

import (
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
