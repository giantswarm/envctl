package api

import (
	"context"
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
	Version    string    `json:"version,omitempty"`
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
	// No fields - uses handlers from registry
}

// NewK8sServiceAPI creates a new K8s service API
func NewK8sServiceAPI() K8sServiceAPI {
	return &k8sServiceAPI{}
}

// GetConnectionInfo returns information about a specific K8s connection
func (api *k8sServiceAPI) GetConnectionInfo(ctx context.Context, label string) (*K8sConnectionInfo, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	service, exists := registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("K8s connection %s not found", label)
	}

	if service.GetType() != TypeKubeConnection {
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
	if data := service.GetServiceData(); data != nil {
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
		if version, ok := data["version"].(string); ok {
			info.Version = version
		}
	}

	return info, nil
}

// ListConnections returns information about all K8s connections
func (api *k8sServiceAPI) ListConnections(ctx context.Context) ([]*K8sConnectionInfo, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	allServices := registry.GetByType(TypeKubeConnection)

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
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	allServices := registry.GetByType(TypeKubeConnection)

	for _, service := range allServices {
		if data := service.GetServiceData(); data != nil {
			if context, ok := data["context"].(string); ok && context == contextName {
				return api.GetConnectionInfo(ctx, service.GetLabel())
			}
		}
	}

	return nil, fmt.Errorf("no K8s connection found for context %s", contextName)
}
