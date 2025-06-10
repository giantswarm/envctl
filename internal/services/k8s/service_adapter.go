package k8s

import (
	"context"
	"fmt"

	"envctl/internal/api"
)

// ServiceAdapter adapts the K8s service functionality to implement api.K8sServiceHandler
type ServiceAdapter struct{}

// NewServiceAdapter creates a new K8s service adapter
func NewServiceAdapter() *ServiceAdapter {
	return &ServiceAdapter{}
}

// Register registers the adapter with the API
func (a *ServiceAdapter) Register() {
	api.RegisterK8sServiceHandler(a)
}

// K8sServiceHandler implementation
func (a *ServiceAdapter) GetClusterLabel() string {
	// This is a global handler, individual services implement this
	return ""
}

func (a *ServiceAdapter) GetMetadata() map[string]interface{} {
	return nil
}

// ListConnections lists all K8s connections
func (a *ServiceAdapter) ListConnections(ctx context.Context) ([]*api.K8sConnectionInfo, error) {
	serviceAPI := api.GetK8sServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("K8s service API not available")
	}
	return serviceAPI.ListConnections(ctx)
}

// GetConnectionInfo gets info about a specific connection
func (a *ServiceAdapter) GetConnectionInfo(ctx context.Context, label string) (*api.K8sConnectionInfo, error) {
	serviceAPI := api.GetK8sServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("K8s service API not available")
	}
	return serviceAPI.GetConnectionInfo(ctx, label)
}

// GetConnectionByContext gets connection by context name
func (a *ServiceAdapter) GetConnectionByContext(ctx context.Context, contextName string) (*api.K8sConnectionInfo, error) {
	serviceAPI := api.GetK8sServiceAPI()
	if serviceAPI == nil {
		return nil, fmt.Errorf("K8s service API not available")
	}
	return serviceAPI.GetConnectionByContext(ctx, contextName)
}

// GetTools returns all tools this provider offers
func (a *ServiceAdapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		{
			Name:        "k8s_connection_list",
			Description: "List all Kubernetes connections",
		},
		{
			Name:        "k8s_connection_info",
			Description: "Get information about a specific Kubernetes connection",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "K8s connection label",
				},
			},
		},
		{
			Name:        "k8s_connection_by_context",
			Description: "Find Kubernetes connection by context name",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "context",
					Type:        "string",
					Required:    true,
					Description: "Kubernetes context name",
				},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *ServiceAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "k8s_connection_list":
		return a.handleK8sConnectionList(ctx)
	case "k8s_connection_info":
		return a.handleK8sConnectionInfo(ctx, args)
	case "k8s_connection_by_context":
		return a.handleK8sConnectionByContext(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (a *ServiceAdapter) handleK8sConnectionList(ctx context.Context) (*api.CallToolResult, error) {
	connections, err := a.ListConnections(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to list K8s connections: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"connections": connections,
		"total":       len(connections),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleK8sConnectionInfo(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	info, err := a.GetConnectionInfo(ctx, label)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get K8s connection info: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{info},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleK8sConnectionByContext(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	contextName, ok := args["context"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"context is required"},
			IsError: true,
		}, nil
	}

	info, err := a.GetConnectionByContext(ctx, contextName)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to find K8s connection: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{info},
		IsError: false,
	}, nil
} 