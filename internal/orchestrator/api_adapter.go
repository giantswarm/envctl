package orchestrator

import (
	"context"
	"fmt"
	"time"

	"envctl/internal/api"
	"envctl/internal/services"
)

// Adapter adapts the orchestrator to implement api.ServiceManagerHandler
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
	api.RegisterServiceManager(a)
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

// ServiceClass-based dynamic service instance management

// CreateServiceClassInstance creates a new ServiceClass-based service instance
func (a *Adapter) CreateServiceClassInstance(ctx context.Context, req api.CreateServiceClassRequest) (*api.ServiceClassInstanceInfo, error) {
	// Convert API request to internal request
	internalReq := CreateServiceRequest{
		ServiceClassName: req.ServiceClassName,
		Label:            req.Label,
		Parameters:       req.Parameters,
		CreateTimeout:    req.CreateTimeout,
		DeleteTimeout:    req.DeleteTimeout,
	}

	// Create the instance using the orchestrator
	internalInfo, err := a.orchestrator.CreateServiceClassInstance(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	// Convert internal response to API response
	return a.convertToAPIServiceClassInstanceInfo(internalInfo), nil
}

// DeleteServiceClassInstance deletes a ServiceClass-based service instance
func (a *Adapter) DeleteServiceClassInstance(ctx context.Context, serviceID string) error {
	return a.orchestrator.DeleteServiceClassInstance(ctx, serviceID)
}

// GetServiceClassInstance returns information about a ServiceClass-based service instance
func (a *Adapter) GetServiceClassInstance(serviceID string) (*api.ServiceClassInstanceInfo, error) {
	internalInfo, err := a.orchestrator.GetServiceClassInstance(serviceID)
	if err != nil {
		return nil, err
	}
	return a.convertToAPIServiceClassInstanceInfo(internalInfo), nil
}

// GetServiceClassInstanceByLabel returns information about a ServiceClass-based service instance by label
func (a *Adapter) GetServiceClassInstanceByLabel(label string) (*api.ServiceClassInstanceInfo, error) {
	internalInfo, err := a.orchestrator.GetServiceClassInstanceByLabel(label)
	if err != nil {
		return nil, err
	}
	return a.convertToAPIServiceClassInstanceInfo(internalInfo), nil
}

// ListServiceClassInstances returns information about all ServiceClass-based service instances
func (a *Adapter) ListServiceClassInstances() []api.ServiceClassInstanceInfo {
	internalInfos := a.orchestrator.ListServiceClassInstances()
	apiInfos := make([]api.ServiceClassInstanceInfo, len(internalInfos))

	for i, internalInfo := range internalInfos {
		apiInfos[i] = *a.convertToAPIServiceClassInstanceInfo(&internalInfo)
	}

	return apiInfos
}

// SubscribeToServiceInstanceEvents returns a channel for receiving ServiceClass-based service instance events
func (a *Adapter) SubscribeToServiceInstanceEvents() <-chan api.ServiceClassInstanceEvent {
	// Convert internal events to API events
	internalChan := a.orchestrator.SubscribeToServiceInstanceEvents()
	apiChan := make(chan api.ServiceClassInstanceEvent, 100)

	go func() {
		for event := range internalChan {
			apiChan <- api.ServiceClassInstanceEvent{
				ServiceID:   event.ServiceID,
				Label:       event.Label,
				ServiceType: event.ServiceType,
				OldState:    event.OldState,
				NewState:    event.NewState,
				OldHealth:   event.OldHealth,
				NewHealth:   event.NewHealth,
				Error:       event.Error,
				Timestamp:   event.Timestamp,
				Metadata:    event.Metadata,
			}
		}
		close(apiChan)
	}()

	return apiChan
}

// Conversion helpers

// convertToAPIServiceClassInstanceInfo converts internal ServiceInstanceInfo to API ServiceClassInstanceInfo
func (a *Adapter) convertToAPIServiceClassInstanceInfo(internalInfo *ServiceInstanceInfo) *api.ServiceClassInstanceInfo {
	return &api.ServiceClassInstanceInfo{
		ServiceID:          internalInfo.ServiceID,
		Label:              internalInfo.Label,
		ServiceClassName:   internalInfo.ServiceClassName,
		ServiceClassType:   internalInfo.ServiceClassType,
		State:              internalInfo.State,
		Health:             internalInfo.Health,
		LastError:          internalInfo.LastError,
		CreatedAt:          internalInfo.CreatedAt,
		LastChecked:        internalInfo.LastChecked,
		ServiceData:        internalInfo.ServiceData,
		CreationParameters: internalInfo.CreationParameters,
	}
}

// GetTools returns all tools this provider offers
func (a *Adapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		// Unified service management tools
		{
			Name:        "service_list",
			Description: "List all services (both static and ServiceClass-based) with their current status",
		},
		{
			Name:        "service_start",
			Description: "Start a specific service (works for both static and ServiceClass-based services)",
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
			Description: "Stop a specific service (works for both static and ServiceClass-based services)",
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
			Description: "Restart a specific service (works for both static and ServiceClass-based services)",
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
			Description: "Get status of a specific service (works for both static and ServiceClass-based services)",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "label",
					Type:        "string",
					Required:    true,
					Description: "Service label to get status for",
				},
			},
		},
		{
			Name:        "service_create",
			Description: "Create a new ServiceClass-based service instance",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "serviceClassName",
					Type:        "string",
					Required:    true,
					Description: "Name of the ServiceClass to instantiate",
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
				},
			},
		},
		{
			Name:        "service_delete",
			Description: "Delete a ServiceClass-based service instance (static services cannot be deleted)",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "labelOrServiceId",
					Type:        "string",
					Required:    true,
					Description: "Label or ID of the ServiceClass instance to delete",
				},
			},
		},
		{
			Name:        "service_get",
			Description: "Get detailed information about a ServiceClass-based service instance",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "labelOrServiceId",
					Type:        "string",
					Required:    true,
					Description: "Label or ID of the ServiceClass instance to get",
				},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	// Static service management
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
	// ServiceClass instance management
	case "service_create":
		return a.handleServiceClassInstanceCreate(ctx, args)
	case "service_delete":
		return a.handleServiceClassInstanceDelete(ctx, args)
	case "service_get":
		return a.handleServiceClassInstanceGet(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Static service management handlers
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

// ServiceClass instance management handlers

func (a *Adapter) handleServiceClassInstanceCreate(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	serviceClassName, ok := args["serviceClassName"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"serviceClassName is required"},
			IsError: true,
		}, nil
	}

	label, ok := args["label"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"label is required"},
			IsError: true,
		}, nil
	}

	parameters, _ := args["parameters"].(map[string]interface{})
	if parameters == nil {
		parameters = make(map[string]interface{})
	}

	req := api.CreateServiceClassRequest{
		ServiceClassName: serviceClassName,
		Label:            label,
		Parameters:       parameters,
	}

	instance, err := a.CreateService(ctx, req)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to create ServiceClass instance: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{instance},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceClassInstanceDelete(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	labelOrServiceId, ok := args["labelOrServiceId"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"labelOrServiceId is required"},
			IsError: true,
		}, nil
	}

	if err := a.DeleteService(ctx, labelOrServiceId); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete ServiceClass instance: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully deleted ServiceClass instance '%s'", labelOrServiceId)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceClassInstanceGet(args map[string]interface{}) (*api.CallToolResult, error) {
	labelOrServiceId, ok := args["labelOrServiceId"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"labelOrServiceId is required"},
			IsError: true,
		}, nil
	}

	instance, err := a.GetService(labelOrServiceId)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get ServiceClass instance: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{instance},
		IsError: false,
	}, nil
}

// ServiceClass instance creation and deletion (unified methods)

// CreateService creates a new ServiceClass-based service instance (unified method)
func (a *Adapter) CreateService(ctx context.Context, req api.CreateServiceClassRequest) (*api.ServiceClassInstanceInfo, error) {
	// Convert API request to internal request
	internalReq := CreateServiceRequest{
		ServiceClassName: req.ServiceClassName,
		Label:            req.Label,
		Parameters:       req.Parameters,
		CreateTimeout:    req.CreateTimeout,
		DeleteTimeout:    req.DeleteTimeout,
	}

	// Create the instance using the orchestrator
	internalInfo, err := a.orchestrator.CreateServiceClassInstance(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	// Convert internal response to API response
	return a.convertToAPIServiceClassInstanceInfo(internalInfo), nil
}

// DeleteService deletes a service (works for ServiceClass instances by label or serviceID)
func (a *Adapter) DeleteService(ctx context.Context, labelOrServiceID string) error {
	// Try to find by label first (check if it's a ServiceClass instance)
	if instance, err := a.orchestrator.GetServiceClassInstanceByLabel(labelOrServiceID); err == nil {
		// Found by label, delete using serviceID
		return a.orchestrator.DeleteServiceClassInstance(ctx, instance.ServiceID)
	}

	// Try to find by serviceID directly
	if _, err := a.orchestrator.GetServiceClassInstance(labelOrServiceID); err == nil {
		// Found by serviceID, delete directly
		return a.orchestrator.DeleteServiceClassInstance(ctx, labelOrServiceID)
	}

	// Not found as ServiceClass instance, cannot delete static services
	return fmt.Errorf("service '%s' not found or cannot be deleted (static services cannot be deleted)", labelOrServiceID)
}

// GetService returns detailed service information by label or serviceID
func (a *Adapter) GetService(labelOrServiceID string) (*api.ServiceClassInstanceInfo, error) {
	// Try to find by label first
	if internalInfo, err := a.orchestrator.GetServiceClassInstanceByLabel(labelOrServiceID); err == nil {
		return a.convertToAPIServiceClassInstanceInfo(internalInfo), nil
	}

	// Try to find by serviceID
	if internalInfo, err := a.orchestrator.GetServiceClassInstance(labelOrServiceID); err == nil {
		return a.convertToAPIServiceClassInstanceInfo(internalInfo), nil
	}

	// If it's a static service (in registry but not in ServiceClass instances), we need a different approach
	// For now, return error since GetService method expects ServiceClassInstanceInfo
	return nil, fmt.Errorf("service '%s' not found or is not a ServiceClass instance", labelOrServiceID)
}
