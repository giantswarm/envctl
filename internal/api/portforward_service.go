package api

import (
	"context"
	"envctl/internal/services"
	"fmt"
)

// PortForwardServiceInfo contains detailed information about a port forward service
type PortForwardServiceInfo struct {
	Label       string `json:"label"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	TargetType  string `json:"targetType"`
	TargetName  string `json:"targetName"`
	LocalPort   int    `json:"localPort"`
	RemotePort  int    `json:"remotePort"`
	BindAddress string `json:"bindAddress"`
	Context     string `json:"context"`
	State       string `json:"state"`
	Health      string `json:"health"`
	Enabled     bool   `json:"enabled"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category,omitempty"`
	TargetPod   string `json:"targetPod,omitempty"`
	Error       string `json:"error,omitempty"`
}

// PortForwardServiceAPI provides access to port forward service information
type PortForwardServiceAPI interface {
	// GetForwardInfo returns information about a specific port forward
	GetForwardInfo(ctx context.Context, label string) (*PortForwardServiceInfo, error)

	// ListForwards returns information about all port forwards
	ListForwards(ctx context.Context) ([]*PortForwardServiceInfo, error)
}

// portForwardServiceAPI implements PortForwardServiceAPI
type portForwardServiceAPI struct {
	registry services.ServiceRegistry
}

// NewPortForwardServiceAPI creates a new port forward service API
func NewPortForwardServiceAPI(registry services.ServiceRegistry) PortForwardServiceAPI {
	return &portForwardServiceAPI{
		registry: registry,
	}
}

// GetForwardInfo returns information about a specific port forward
func (api *portForwardServiceAPI) GetForwardInfo(ctx context.Context, label string) (*PortForwardServiceInfo, error) {
	service, exists := api.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("port forward %s not found", label)
	}

	if service.GetType() != services.TypePortForward {
		return nil, fmt.Errorf("service %s is not a port forward", label)
	}

	info := &PortForwardServiceInfo{
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

		if name, ok := data["name"].(string); ok {
			info.Name = name
		}
		if namespace, ok := data["namespace"].(string); ok {
			info.Namespace = namespace
		}
		if targetType, ok := data["targetType"].(string); ok {
			info.TargetType = targetType
		}
		if targetName, ok := data["targetName"].(string); ok {
			info.TargetName = targetName
		}
		if localPort, ok := data["localPort"].(int); ok {
			info.LocalPort = localPort
		}
		if remotePort, ok := data["remotePort"].(int); ok {
			info.RemotePort = remotePort
		}
		if bindAddress, ok := data["bindAddress"].(string); ok {
			info.BindAddress = bindAddress
		}
		if context, ok := data["context"].(string); ok {
			info.Context = context
		}
		if enabled, ok := data["enabled"].(bool); ok {
			info.Enabled = enabled
		}
		if icon, ok := data["icon"].(string); ok {
			info.Icon = icon
		}
		if category, ok := data["category"].(string); ok {
			info.Category = category
		}
		if targetPod, ok := data["targetPod"].(string); ok {
			info.TargetPod = targetPod
		}
	}

	return info, nil
}

// ListForwards returns information about all port forwards
func (api *portForwardServiceAPI) ListForwards(ctx context.Context) ([]*PortForwardServiceInfo, error) {
	allServices := api.registry.GetByType(services.TypePortForward)

	forwards := make([]*PortForwardServiceInfo, 0, len(allServices))
	for _, service := range allServices {
		info, err := api.GetForwardInfo(ctx, service.GetLabel())
		if err != nil {
			// Log error but continue with other forwards
			continue
		}
		forwards = append(forwards, info)
	}

	return forwards, nil
}
