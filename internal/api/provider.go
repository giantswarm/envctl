package api

import (
	"envctl/internal/kube"
	"envctl/internal/reporting"
)

// Provider aggregates all API interfaces
type Provider struct {
	MCP         MCPServerAPI
	Kubernetes  KubernetesAPI
	PortForward PortForwardAPI
}

// NewProvider creates a new API provider with all implementations
func NewProvider(
	eventBus reporting.EventBus,
	stateStore reporting.StateStore,
	kubeMgr kube.Manager,
) *Provider {
	return &Provider{
		MCP:         NewMCPServerAPI(eventBus, stateStore),
		Kubernetes:  NewKubernetesAPI(eventBus, stateStore, kubeMgr),
		PortForward: NewPortForwardAPI(eventBus, stateStore),
	}
}
