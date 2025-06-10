package portforward

import (
	"context"
	"fmt"

	"envctl/internal/api"
)

// ServiceAdapter adapts the port forward service functionality to implement api.PortForwardServiceHandler
type ServiceAdapter struct{}

// NewServiceAdapter creates a new port forward service adapter
func NewServiceAdapter() *ServiceAdapter {
	return &ServiceAdapter{}
}

// Register registers the adapter with the API
func (a *ServiceAdapter) Register() {
	api.RegisterPortForwardServiceHandler(a)
}

// PortForwardServiceHandler implementation
func (a *ServiceAdapter) GetClusterLabel() string {
	// This is a global handler, individual services implement this
	return ""
}

func (a *ServiceAdapter) GetNamespace() string {
	return ""
}

func (a *ServiceAdapter) GetServiceName() string {
	return ""
}

func (a *ServiceAdapter) GetLocalPort() int {
	return 0
}

func (a *ServiceAdapter) GetRemotePort() int {
	return 0
}

// ListForwards lists all port forwards
func (a *ServiceAdapter) ListForwards(ctx context.Context) ([]*api.PortForwardInfo, error) {
	serviceAPI := api.GetPortForwardServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("port forward service API not available")
	}
	
	serviceInfos, err := serviceAPI.ListForwards(ctx)
	if err != nil {
		return nil, err
	}
	
	// Convert PortForwardServiceInfo to PortForwardInfo
	infos := make([]*api.PortForwardInfo, 0, len(serviceInfos))
	for _, si := range serviceInfos {
		info := &api.PortForwardInfo{
			Label:        si.Label,
			ClusterLabel: si.Context, // Map context to cluster label
			Namespace:    si.Namespace,
			ServiceName:  si.TargetName,
			LocalPort:    si.LocalPort,
			RemotePort:   si.RemotePort,
			State:        si.State,
			Health:       si.Health,
			Error:        si.Error,
		}
		infos = append(infos, info)
	}
	
	return infos, nil
}

// GetForwardInfo gets info about a specific port forward
func (a *ServiceAdapter) GetForwardInfo(ctx context.Context, label string) (*api.PortForwardInfo, error) {
	serviceAPI := api.GetPortForwardServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("port forward service API not available")
	}
	
	si, err := serviceAPI.GetForwardInfo(ctx, label)
	if err != nil {
		return nil, err
	}
	
	// Convert PortForwardServiceInfo to PortForwardInfo
	info := &api.PortForwardInfo{
		Label:        si.Label,
		ClusterLabel: si.Context, // Map context to cluster label
		Namespace:    si.Namespace,
		ServiceName:  si.TargetName,
		LocalPort:    si.LocalPort,
		RemotePort:   si.RemotePort,
		State:        si.State,
		Health:       si.Health,
		Error:        si.Error,
	}
	
	return info, nil
}

// GetTools returns all tools this provider offers
func (a *ServiceAdapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		{
			Name:        "portforward_list",
			Description: "List all port forwards",
		},
		{
			Name:        "portforward_info",
			Description: "Get information about a specific port forward",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Port forward label",
				},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *ServiceAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "portforward_list":
		return a.handlePortForwardList(ctx)
	case "portforward_info":
		return a.handlePortForwardInfo(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (a *ServiceAdapter) handlePortForwardList(ctx context.Context) (*api.CallToolResult, error) {
	forwards, err := a.ListForwards(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to list port forwards: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"port_forwards": forwards,
		"total":         len(forwards),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handlePortForwardInfo(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	info, err := a.GetForwardInfo(ctx, label)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get port forward info: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{info},
		IsError: false,
	}, nil
} 