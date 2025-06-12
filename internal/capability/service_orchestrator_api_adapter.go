package capability

import (
	"context"
	"fmt"

	"envctl/internal/api"
)

// ServiceOrchestratorAPIAdapter implements the api.ServiceOrchestratorHandler interface
// It bridges the ServiceOrchestrator with the API layer
type ServiceOrchestratorAPIAdapter struct {
	orchestrator *ServiceOrchestrator
	registry     *ServiceCapabilityRegistry
}

// NewServiceOrchestratorAPIAdapter creates a new API adapter for the service orchestrator
func NewServiceOrchestratorAPIAdapter(orchestrator *ServiceOrchestrator, registry *ServiceCapabilityRegistry) *ServiceOrchestratorAPIAdapter {
	return &ServiceOrchestratorAPIAdapter{
		orchestrator: orchestrator,
		registry:     registry,
	}
}

// ListServiceCapabilities returns all available service capabilities
func (a *ServiceOrchestratorAPIAdapter) ListServiceCapabilities() []api.ServiceCapabilityInfo {
	if a.registry == nil {
		return []api.ServiceCapabilityInfo{}
	}

	capabilities := a.registry.ListServiceCapabilities()
	result := make([]api.ServiceCapabilityInfo, 0, len(capabilities))

	for _, cap := range capabilities {
		// Convert service capability info to API format
		operations := []api.OperationInfo{
			{
				Name:        "create",
				Description: "Create a service instance",
				Available:   cap.CreateToolAvailable,
			},
			{
				Name:        "delete",
				Description: "Delete a service instance",
				Available:   cap.DeleteToolAvailable,
			},
			{
				Name:        "healthcheck",
				Description: "Check service health",
				Available:   cap.HealthCheckToolAvailable,
			},
			{
				Name:        "status",
				Description: "Get service status",
				Available:   cap.StatusToolAvailable,
			},
		}

		apiCap := api.ServiceCapabilityInfo{
			Name:        cap.Name,
			Type:        cap.Type,
			Version:     cap.Version,
			Description: cap.Description,
			ServiceType: cap.ServiceType,
			Available:   cap.Available,
			Operations:  operations,
			Metadata:    map[string]interface{}{}, // Service capabilities don't have metadata in the current structure
		}
		result = append(result, apiCap)
	}

	return result
}

// GetServiceCapability returns information about a specific service capability
func (a *ServiceOrchestratorAPIAdapter) GetServiceCapability(name string) (*api.ServiceCapabilityInfo, error) {
	if a.registry == nil {
		return nil, fmt.Errorf("registry not available")
	}

	capability, exists := a.registry.GetServiceCapabilityDefinition(name)
	if !exists {
		return nil, fmt.Errorf("service capability %s not found", name)
	}

	// Get the service capability info (which has the correct tool availability info)
	capInfo := a.registry.ListServiceCapabilities()
	var targetInfo *ServiceCapabilityInfo
	for _, info := range capInfo {
		if info.Name == name {
			targetInfo = &info
			break
		}
	}

	if targetInfo == nil {
		return nil, fmt.Errorf("service capability %s not found in info list", name)
	}

	operations := []api.OperationInfo{
		{
			Name:        "create",
			Description: "Create a service instance",
			Available:   targetInfo.CreateToolAvailable,
		},
		{
			Name:        "delete",
			Description: "Delete a service instance",
			Available:   targetInfo.DeleteToolAvailable,
		},
		{
			Name:        "healthcheck",
			Description: "Check service health",
			Available:   targetInfo.HealthCheckToolAvailable,
		},
		{
			Name:        "status",
			Description: "Get service status",
			Available:   targetInfo.StatusToolAvailable,
		},
	}

	apiCap := &api.ServiceCapabilityInfo{
		Name:        capability.Name,
		Type:        capability.Type,
		Version:     capability.Version,
		Description: capability.Description,
		ServiceType: targetInfo.ServiceType,
		Available:   targetInfo.Available,
		Operations:  operations,
		Metadata:    map[string]interface{}{}, // Service capabilities don't have metadata in the current structure
	}

	return apiCap, nil
}

// IsServiceCapabilityAvailable checks if a service capability is available
func (a *ServiceOrchestratorAPIAdapter) IsServiceCapabilityAvailable(name string) bool {
	if a.registry == nil {
		return false
	}
	return a.registry.IsServiceCapabilityAvailable(name)
}

// CreateService creates a new service instance
func (a *ServiceOrchestratorAPIAdapter) CreateService(ctx context.Context, req api.CreateServiceRequest) (*api.ServiceInstanceInfo, error) {
	if a.orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not available")
	}

	// Convert API request to orchestrator request
	orchReq := CreateServiceRequest{
		CapabilityName: req.CapabilityName,
		Label:          req.Label,
		Parameters:     req.Parameters,
	}

	// Create the service
	instance, err := a.orchestrator.CreateService(ctx, orchReq)
	if err != nil {
		return nil, err
	}

	// Convert orchestrator response to API response
	return convertServiceInstanceToAPI(instance), nil
}

// DeleteService deletes a service instance
func (a *ServiceOrchestratorAPIAdapter) DeleteService(ctx context.Context, serviceID string) error {
	if a.orchestrator == nil {
		return fmt.Errorf("orchestrator not available")
	}
	return a.orchestrator.DeleteService(ctx, serviceID)
}

// GetService returns information about a service instance
func (a *ServiceOrchestratorAPIAdapter) GetService(serviceID string) (*api.ServiceInstanceInfo, error) {
	if a.orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not available")
	}

	instance, err := a.orchestrator.GetService(serviceID)
	if err != nil {
		return nil, err
	}

	return convertServiceInstanceToAPI(instance), nil
}

// GetServiceByLabel returns information about a service instance by label
func (a *ServiceOrchestratorAPIAdapter) GetServiceByLabel(label string) (*api.ServiceInstanceInfo, error) {
	if a.orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not available")
	}

	instance, err := a.orchestrator.GetServiceByLabel(label)
	if err != nil {
		return nil, err
	}

	return convertServiceInstanceToAPI(instance), nil
}

// ListServices returns all service instances
func (a *ServiceOrchestratorAPIAdapter) ListServices() []api.ServiceInstanceInfo {
	if a.orchestrator == nil {
		return []api.ServiceInstanceInfo{}
	}

	instances := a.orchestrator.ListServices()
	result := make([]api.ServiceInstanceInfo, 0, len(instances))

	for _, instance := range instances {
		result = append(result, *convertServiceInstanceToAPI(&instance))
	}

	return result
}

// SubscribeToServiceEvents returns a channel for receiving service instance events
func (a *ServiceOrchestratorAPIAdapter) SubscribeToServiceEvents() <-chan api.ServiceInstanceEvent {
	if a.orchestrator == nil {
		// Return a closed channel if orchestrator is not available
		ch := make(chan api.ServiceInstanceEvent)
		close(ch)
		return ch
	}

	// Get the orchestrator event channel
	orchEventChan := a.orchestrator.SubscribeToEvents()

	// Create a new channel for API events
	apiEventChan := make(chan api.ServiceInstanceEvent, 100)

	// Start a goroutine to convert events
	go func() {
		defer close(apiEventChan)
		for orchEvent := range orchEventChan {
			apiEvent := api.ServiceInstanceEvent{
				ServiceID:   orchEvent.ServiceID,
				Label:       orchEvent.Label,
				ServiceType: orchEvent.ServiceType,
				OldState:    string(orchEvent.OldState),
				NewState:    string(orchEvent.NewState),
				OldHealth:   string(orchEvent.OldHealth),
				NewHealth:   string(orchEvent.NewHealth),
				Error:       orchEvent.Error,
				Timestamp:   orchEvent.Timestamp,
				Metadata:    orchEvent.Metadata,
			}
			apiEventChan <- apiEvent
		}
	}()

	return apiEventChan
}

// GetTools returns all tools this provider offers
func (a *ServiceOrchestratorAPIAdapter) GetTools() []api.ToolMetadata {
	tools := []api.ToolMetadata{
		{
			Name:        "service_capability_list",
			Description: "List all available service capabilities",
			Parameters:  []api.ParameterMetadata{},
		},
		{
			Name:        "service_capability_info",
			Description: "Get information about a specific service capability",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the service capability",
				},
			},
		},
		{
			Name:        "service_capability_check",
			Description: "Check if a service capability is available",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the service capability",
				},
			},
		},
		{
			Name:        "service_create",
			Description: "Create a new service instance",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "capability_name",
					Type:        "string",
					Required:    true,
					Description: "Name of the service capability to use",
				},
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Unique label for the service instance",
				},
				{
					Name:        "parameters",
					Type:        "object",
					Required:    false,
					Description: "Parameters for service creation",
					Default:     map[string]interface{}{},
				},
			},
		},
		{
			Name:        "service_delete",
			Description: "Delete a service instance",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "service_id",
					Type:        "string",
					Required:    true,
					Description: "ID of the service instance to delete",
				},
			},
		},
		{
			Name:        "service_get",
			Description: "Get information about a service instance",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "service_id",
					Type:        "string",
					Required:    true,
					Description: "ID of the service instance",
				},
			},
		},
		{
			Name:        "service_get_by_label",
			Description: "Get information about a service instance by label",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Label of the service instance",
				},
			},
		},
		{
			Name:        "service_list",
			Description: "List all service instances",
			Parameters:  []api.ParameterMetadata{},
		},
	}

	return tools
}

// ExecuteTool executes a tool by name
func (a *ServiceOrchestratorAPIAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "service_capability_list":
		return a.handleCapabilityList()
	case "service_capability_info":
		return a.handleCapabilityInfo(args)
	case "service_capability_check":
		return a.handleCapabilityCheck(args)
	case "service_create":
		return a.handleServiceCreate(ctx, args)
	case "service_delete":
		return a.handleServiceDelete(ctx, args)
	case "service_get":
		return a.handleServiceGet(args)
	case "service_get_by_label":
		return a.handleServiceGetByLabel(args)
	case "service_list":
		return a.handleServiceList()
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Helper methods for tool execution

func (a *ServiceOrchestratorAPIAdapter) handleCapabilityList() (*api.CallToolResult, error) {
	capabilities := a.ListServiceCapabilities()
	result := map[string]interface{}{
		"capabilities": capabilities,
		"total":        len(capabilities),
	}
	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleCapabilityInfo(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	capability, err := a.GetServiceCapability(name)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get service capability: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{capability},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleCapabilityCheck(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	available := a.IsServiceCapabilityAvailable(name)
	result := map[string]interface{}{
		"name":      name,
		"available": available,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleServiceCreate(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	capabilityName, ok := args["capability_name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"capability_name parameter is required"},
			IsError: true,
		}, nil
	}

	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label parameter is required"},
			IsError: true,
		}, nil
	}

	parameters, ok := args["parameters"].(map[string]interface{})
	if !ok {
		parameters = make(map[string]interface{})
	}

	req := api.CreateServiceRequest{
		CapabilityName: capabilityName,
		Label:          label,
		Parameters:     parameters,
	}

	instance, err := a.CreateService(ctx, req)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to create service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{instance},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleServiceDelete(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	serviceID, ok := args["service_id"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"service_id parameter is required"},
			IsError: true,
		}, nil
	}

	err := a.DeleteService(ctx, serviceID)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Service %s deleted successfully", serviceID)},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleServiceGet(args map[string]interface{}) (*api.CallToolResult, error) {
	serviceID, ok := args["service_id"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"service_id parameter is required"},
			IsError: true,
		}, nil
	}

	instance, err := a.GetService(serviceID)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{instance},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleServiceGetByLabel(args map[string]interface{}) (*api.CallToolResult, error) {
	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label parameter is required"},
			IsError: true,
		}, nil
	}

	instance, err := a.GetServiceByLabel(label)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get service: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{instance},
		IsError: false,
	}, nil
}

func (a *ServiceOrchestratorAPIAdapter) handleServiceList() (*api.CallToolResult, error) {
	services := a.ListServices()
	result := map[string]interface{}{
		"services": services,
		"total":    len(services),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

// Register registers this adapter with the API handler registry
func (a *ServiceOrchestratorAPIAdapter) Register() {
	api.RegisterServiceOrchestrator(a)
}

// Helper functions

func convertServiceInstanceToAPI(instance *ServiceInstanceInfo) *api.ServiceInstanceInfo {
	return &api.ServiceInstanceInfo{
		ServiceID:          instance.ServiceID,
		Label:              instance.Label,
		CapabilityName:     instance.CapabilityName,
		CapabilityType:     instance.CapabilityType,
		State:              string(instance.State),
		Health:             string(instance.Health),
		LastError:          instance.LastError,
		CreatedAt:          instance.CreatedAt,
		LastChecked:        instance.LastChecked,
		ServiceData:        instance.ServiceData,
		CreationParameters: instance.CreationParameters,
	}
}

func convertStringMapToInterface(stringMap map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range stringMap {
		result[k] = v
	}
	return result
}
