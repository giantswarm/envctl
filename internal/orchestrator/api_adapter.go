package orchestrator

import (
	"context"
	"fmt"
	"time"

	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/services"
)

// Adapter adapts the orchestrator to implement api.OrchestratorHandler
type Adapter struct {
	orchestrator *Orchestrator
}

// NewAPIAdapter creates a new orchestrator adapter
func NewAPIAdapter(orchestrator *Orchestrator) *Adapter {
	return &Adapter{
		orchestrator: orchestrator,
	}
}

// Register registers the adapter with the API
func (a *Adapter) Register() {
	api.RegisterOrchestrator(a)
}

// Service lifecycle management
func (a *Adapter) StartService(label string) error {
	return a.orchestrator.StartService(label)
}

func (a *Adapter) StopService(label string) error {
	return a.orchestrator.StopService(label)
}

func (a *Adapter) RestartService(label string) error {
	return a.orchestrator.RestartService(label)
}

func (a *Adapter) SubscribeToStateChanges() <-chan api.ServiceStateChangedEvent {
	// Convert internal events to API events
	internalChan := a.orchestrator.SubscribeToStateChanges()
	apiChan := make(chan api.ServiceStateChangedEvent, 100)

	go func() {
		for event := range internalChan {
			apiChan <- api.ServiceStateChangedEvent{
				Label:       event.Label,
				ServiceType: event.ServiceType,
				OldState:    event.OldState,
				NewState:    event.NewState,
				Health:      event.Health,
				Error:       event.Error,
				Timestamp:   time.Now(),
			}
		}
		close(apiChan)
	}()

	return apiChan
}

// Service status
func (a *Adapter) GetServiceStatus(label string) (*api.ServiceStatus, error) {
	service, exists := a.orchestrator.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("service %s not found", label)
	}

	status := &api.ServiceStatus{
		Label:       service.GetLabel(),
		ServiceType: string(service.GetType()),
		State:       api.ServiceState(service.GetState()),
		Health:      api.HealthStatus(service.GetHealth()),
	}

	// Add error if present
	if err := service.GetLastError(); err != nil {
		status.Error = err.Error()
	}

	// Add metadata if available
	if provider, ok := service.(services.ServiceDataProvider); ok {
		if data := provider.GetServiceData(); data != nil {
			status.Metadata = data
		}
	}

	return status, nil
}

func (a *Adapter) GetAllServices() []api.ServiceStatus {
	allServices := a.orchestrator.registry.GetAll()
	statuses := make([]api.ServiceStatus, 0, len(allServices))

	for _, service := range allServices {
		status := api.ServiceStatus{
			Label:       service.GetLabel(),
			ServiceType: string(service.GetType()),
			State:       api.ServiceState(service.GetState()),
			Health:      api.HealthStatus(service.GetHealth()),
		}

		// Add error if present
		if err := service.GetLastError(); err != nil {
			status.Error = err.Error()
		}

		// Add metadata if available
		if provider, ok := service.(services.ServiceDataProvider); ok {
			if data := provider.GetServiceData(); data != nil {
				status.Metadata = data
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// Cluster management
func (a *Adapter) GetAvailableClusters(role api.ClusterRole) []api.ClusterDefinition {
	// Convert API role to config role
	configRole := config.ClusterRole(role)
	clusters := a.orchestrator.GetAvailableClusters(configRole)

	// Convert config clusters to API clusters
	apiClusters := make([]api.ClusterDefinition, 0, len(clusters))
	for _, cluster := range clusters {
		apiClusters = append(apiClusters, api.ClusterDefinition{
			Name:        cluster.Name,
			Context:     cluster.Context,
			Role:        api.ClusterRole(cluster.Role),
			DisplayName: cluster.DisplayName,
			Icon:        cluster.Icon,
		})
	}

	return apiClusters
}

func (a *Adapter) GetActiveCluster(role api.ClusterRole) (string, bool) {
	configRole := config.ClusterRole(role)
	return a.orchestrator.GetActiveCluster(configRole)
}

func (a *Adapter) SwitchCluster(role api.ClusterRole, clusterName string) error {
	configRole := config.ClusterRole(role)
	return a.orchestrator.SwitchCluster(configRole, clusterName)
}

// GetTools returns all tools this provider offers
func (a *Adapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		// Service management tools
		{
			Name:        "service_list",
			Description: "List all services with their current status",
		},
		{
			Name:        "service_start",
			Description: "Start a specific service",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Service label to start",
				},
			},
		},
		{
			Name:        "service_stop",
			Description: "Stop a specific service",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Service label to stop",
				},
			},
		},
		{
			Name:        "service_restart",
			Description: "Restart a specific service",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Service label to restart",
				},
			},
		},
		{
			Name:        "service_status",
			Description: "Get detailed status of a specific service",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Service label to get status for",
				},
			},
		},
		// Cluster management tools
		{
			Name:        "cluster_list",
			Description: "List available clusters by role",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "role",
					Type:        "string",
					Required:    true,
					Description: "Cluster role: talos, management, workload, or observability",
				},
			},
		},
		{
			Name:        "cluster_switch",
			Description: "Switch active cluster for a role",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "role",
					Type:        "string",
					Required:    true,
					Description: "Cluster role: talos, management, workload, or observability",
				},
				{
					Name:        "cluster_name",
					Type:        "string",
					Required:    true,
					Description: "Name of the cluster to switch to",
				},
			},
		},
		{
			Name:        "cluster_active",
			Description: "Get currently active cluster for a role",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "role",
					Type:        "string",
					Required:    true,
					Description: "Cluster role: talos, management, workload, or observability",
				},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "service_list":
		return a.handleServiceList()
	case "service_start":
		return a.handleServiceStart(args)
	case "service_stop":
		return a.handleServiceStop(args)
	case "service_restart":
		return a.handleServiceRestart(args)
	case "service_status":
		return a.handleServiceStatus(args)
	case "cluster_list":
		return a.handleClusterList(args)
	case "cluster_switch":
		return a.handleClusterSwitch(args)
	case "cluster_active":
		return a.handleClusterActive(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Service management handlers
func (a *Adapter) handleServiceList() (*api.CallToolResult, error) {
	services := a.GetAllServices()

	result := map[string]interface{}{
		"services": services,
		"total":    len(services),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceStart(args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	if err := a.StartService(label); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to start service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully started service '%s'", label)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceStop(args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	if err := a.StopService(label); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to stop service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully stopped service '%s'", label)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceRestart(args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	if err := a.RestartService(label); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to restart service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully restarted service '%s'", label)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceStatus(args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	status, err := a.GetServiceStatus(label)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get service status: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{status},
		IsError: false,
	}, nil
}

// Cluster management handlers
func (a *Adapter) handleClusterList(args map[string]interface{}) (*api.CallToolResult, error) {
	roleStr, ok := args["role"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"role is required"},
			IsError: true,
		}, nil
	}

	role := api.ClusterRole(roleStr)
	clusters := a.GetAvailableClusters(role)

	// Get active cluster
	activeName, _ := a.GetActiveCluster(role)

	result := map[string]interface{}{
		"clusters": clusters,
		"active":   activeName,
		"total":    len(clusters),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleClusterSwitch(args map[string]interface{}) (*api.CallToolResult, error) {
	roleStr, ok := args["role"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"role is required"},
			IsError: true,
		}, nil
	}

	clusterName, ok := args["cluster_name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"cluster_name is required"},
			IsError: true,
		}, nil
	}

	role := api.ClusterRole(roleStr)
	if err := a.SwitchCluster(role, clusterName); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to switch cluster: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully switched %s cluster to '%s'", roleStr, clusterName)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleClusterActive(args map[string]interface{}) (*api.CallToolResult, error) {
	roleStr, ok := args["role"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"role is required"},
			IsError: true,
		}, nil
	}

	role := api.ClusterRole(roleStr)
	activeName, exists := a.GetActiveCluster(role)

	result := map[string]interface{}{
		"role":   roleStr,
		"active": activeName,
		"exists": exists,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}
