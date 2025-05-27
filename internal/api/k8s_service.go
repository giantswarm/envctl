package api

import (
	"context"
	"envctl/internal/services"
	"fmt"
	"time"
)

// K8sConnectionInfo contains information about a K8s connection service
type K8sConnectionInfo struct {
	Label      string    `json:"label"`
	Context    string    `json:"context"`
	IsMC       bool      `json:"isMC"`
	State      string    `json:"state"`
	Health     string    `json:"health"`
	ReadyNodes int       `json:"readyNodes"`
	TotalNodes int       `json:"totalNodes"`
	LastCheck  time.Time `json:"lastCheck"`
	Error      string    `json:"error,omitempty"`
}

// K8sServiceAPI provides access to K8s connection service information
type K8sServiceAPI interface {
	// GetConnectionInfo returns information about a specific K8s connection
	GetConnectionInfo(ctx context.Context, label string) (*K8sConnectionInfo, error)

	// ListConnections returns information about all K8s connections
	ListConnections(ctx context.Context) ([]*K8sConnectionInfo, error)

	// GetConnectionByContext returns connection info by context name
	GetConnectionByContext(ctx context.Context, contextName string) (*K8sConnectionInfo, error)
}

// k8sServiceAPI implements K8sServiceAPI
type k8sServiceAPI struct {
	registry services.ServiceRegistry
}

// NewK8sServiceAPI creates a new K8s service API
func NewK8sServiceAPI(registry services.ServiceRegistry) K8sServiceAPI {
	return &k8sServiceAPI{
		registry: registry,
	}
}

// GetConnectionInfo returns information about a specific K8s connection
func (api *k8sServiceAPI) GetConnectionInfo(ctx context.Context, label string) (*K8sConnectionInfo, error) {
	service, exists := api.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("K8s connection %s not found", label)
	}

	if service.GetType() != services.TypeKubeConnection {
		return nil, fmt.Errorf("service %s is not a K8s connection", label)
	}

	info := &K8sConnectionInfo{
		Label:  service.GetLabel(),
		State:  string(service.GetState()),
		Health: string(service.GetHealth()),
	}

	// Get error if any
	if err := service.GetLastError(); err != nil {
		info.Error = err.Error()
	}

	// Get service-specific data if available
	if provider, ok := service.(services.ServiceDataProvider); ok {
		data := provider.GetServiceData()

		if context, ok := data["context"].(string); ok {
			info.Context = context
		}
		if isMC, ok := data["isMC"].(bool); ok {
			info.IsMC = isMC
		}
		if readyNodes, ok := data["readyNodes"].(int); ok {
			info.ReadyNodes = readyNodes
		}
		if totalNodes, ok := data["totalNodes"].(int); ok {
			info.TotalNodes = totalNodes
		}
		if lastCheck, ok := data["lastCheck"].(time.Time); ok {
			info.LastCheck = lastCheck
		}
	}

	return info, nil
}

// ListConnections returns information about all K8s connections
func (api *k8sServiceAPI) ListConnections(ctx context.Context) ([]*K8sConnectionInfo, error) {
	allServices := api.registry.GetByType(services.TypeKubeConnection)

	connections := make([]*K8sConnectionInfo, 0, len(allServices))
	for _, service := range allServices {
		info, err := api.GetConnectionInfo(ctx, service.GetLabel())
		if err != nil {
			// Log error but continue with other connections
			continue
		}
		connections = append(connections, info)
	}

	return connections, nil
}

// GetConnectionByContext returns connection info by context name
func (api *k8sServiceAPI) GetConnectionByContext(ctx context.Context, contextName string) (*K8sConnectionInfo, error) {
	allServices := api.registry.GetByType(services.TypeKubeConnection)

	for _, service := range allServices {
		if provider, ok := service.(services.ServiceDataProvider); ok {
			data := provider.GetServiceData()
			if context, ok := data["context"].(string); ok && context == contextName {
				return api.GetConnectionInfo(ctx, service.GetLabel())
			}
		}
	}

	return nil, fmt.Errorf("no K8s connection found for context %s", contextName)
}
