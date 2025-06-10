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
		// Port forwarding tools
		{
			Name:        "k8s_port_forward",
			Description: "Create a port forward to a Kubernetes resource",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "namespace",
					Type:        "string",
					Required:    true,
					Description: "Kubernetes namespace",
				},
				{
					Name:        "resource_type",
					Type:        "string",
					Required:    true,
					Description: "Type of resource (pod, service, deployment)",
				},
				{
					Name:        "resource_name",
					Type:        "string",
					Required:    true,
					Description: "Name of the resource",
				},
				{
					Name:        "local_port",
					Type:        "string",
					Required:    true,
					Description: "Local port to forward to",
				},
				{
					Name:        "remote_port",
					Type:        "string",
					Required:    true,
					Description: "Remote port on the resource",
				},
				{
					Name:        "bind_address",
					Type:        "string",
					Required:    false,
					Description: "Local bind address (default: 127.0.0.1)",
				},
			},
		},
		{
			Name:        "k8s_port_forward_stop",
			Description: "Stop an existing port forward",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "forward_id",
					Type:        "string",
					Required:    true,
					Description: "ID of the port forward to stop",
				},
			},
		},
		{
			Name:        "k8s_port_forward_list",
			Description: "List all active port forwards",
		},
		{
			Name:        "k8s_port_forward_info",
			Description: "Get information about a specific port forward",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "forward_id",
					Type:        "string",
					Required:    true,
					Description: "ID of the port forward",
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
	case "k8s_port_forward":
		return a.handleK8sPortForward(ctx, args)
	case "k8s_port_forward_stop":
		return a.handleK8sPortForwardStop(ctx, args)
	case "k8s_port_forward_list":
		return a.handleK8sPortForwardList(ctx)
	case "k8s_port_forward_info":
		return a.handleK8sPortForwardInfo(ctx, args)
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

// Port forwarding handlers
func (a *ServiceAdapter) handleK8sPortForward(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	// For now, we'll return an informative message about how port forwards should be created
	// In the future, this will integrate with the orchestrator to create dynamic port forwards

	// Validate required parameters
	namespace, ok := args["namespace"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"namespace is required"},
			IsError: true,
		}, nil
	}

	resourceType, ok := args["resource_type"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"resource_type is required"},
			IsError: true,
		}, nil
	}

	resourceName, ok := args["resource_name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"resource_name is required"},
			IsError: true,
		}, nil
	}

	localPort, ok := args["local_port"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"local_port is required"},
			IsError: true,
		}, nil
	}

	remotePort, ok := args["remote_port"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"remote_port is required"},
			IsError: true,
		}, nil
	}

	bindAddress := "127.0.0.1"
	if addr, ok := args["bind_address"].(string); ok && addr != "" {
		bindAddress = addr
	}

	// Generate a unique forward ID
	forwardID := fmt.Sprintf("pf-%s-%s-%s-%s", namespace, resourceType, resourceName, localPort)

	// TODO: In a real implementation, this would:
	// 1. Create a new port forward configuration
	// 2. Register it with the orchestrator
	// 3. Start the port forward service
	// For now, we return a placeholder response

	result := map[string]interface{}{
		"forward_id":    forwardID,
		"namespace":     namespace,
		"resource_type": resourceType,
		"resource_name": resourceName,
		"local_port":    localPort,
		"remote_port":   remotePort,
		"bind_address":  bindAddress,
		"status":        "created",
		"message":       "Port forward created (placeholder implementation)",
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleK8sPortForwardStop(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	forwardID, ok := args["forward_id"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"forward_id is required"},
			IsError: true,
		}, nil
	}

	// Get orchestrator to stop the service
	orchestrator := api.GetOrchestrator()
	if orchestrator == nil {
		return &api.CallToolResult{
			Content: []interface{}{"Orchestrator not available"},
			IsError: true,
		}, nil
	}

	// Stop the port forward service by label
	if err := orchestrator.StopService(forwardID); err != nil {
		// If error contains "not found", it might already be stopped
		if fmt.Sprintf("%v", err) == fmt.Sprintf("service %s not found", forwardID) {
			result := map[string]interface{}{
				"forward_id": forwardID,
				"status":     "not_found",
				"message":    "Port forward not found or already stopped",
			}
			return &api.CallToolResult{
				Content: []interface{}{result},
				IsError: false,
			}, nil
		}

		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to stop port forward: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"forward_id": forwardID,
		"status":     "stopped",
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleK8sPortForwardList(ctx context.Context) (*api.CallToolResult, error) {
	// Get port forward API
	pfAPI := api.GetPortForwardServiceAPI()
	if pfAPI == nil {
		return &api.CallToolResult{
			Content: []interface{}{"Port forward service API not available"},
			IsError: true,
		}, nil
	}

	// List all port forwards using the existing API
	forwards, err := pfAPI.ListForwards(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to list port forwards: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"forwards": forwards,
		"total":    len(forwards),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceAdapter) handleK8sPortForwardInfo(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	forwardID, ok := args["forward_id"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"forward_id is required"},
			IsError: true,
		}, nil
	}

	// Get port forward API
	pfAPI := api.GetPortForwardServiceAPI()
	if pfAPI == nil {
		return &api.CallToolResult{
			Content: []interface{}{"Port forward service API not available"},
			IsError: true,
		}, nil
	}

	// Get port forward info using the existing API method
	info, err := pfAPI.GetForwardInfo(ctx, forwardID)
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
