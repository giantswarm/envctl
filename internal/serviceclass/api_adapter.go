package serviceclass

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
	"time"
)

// Adapter implements the api.ServiceClassManagerHandler interface
// This adapter bridges the ServiceClassManager implementation with the central API layer
type Adapter struct {
	manager *ServiceClassManager
}

// NewAdapter creates a new API adapter for the ServiceClassManager
func NewAdapter(manager *ServiceClassManager) *Adapter {
	return &Adapter{
		manager: manager,
	}
}

// Register registers this adapter with the central API layer
// This method follows the project's API Service Locator Pattern
func (a *Adapter) Register() {
	api.RegisterServiceClassManager(a)
	logging.Info("ServiceClassAdapter", "Registered ServiceClass manager with API layer")
}

// ListServiceClasses returns information about all registered service classes
func (a *Adapter) ListServiceClasses() []api.ServiceClassInfo {
	if a.manager == nil {
		return []api.ServiceClassInfo{}
	}

	// Get service class info from the manager
	managerInfo := a.manager.ListServiceClasses()

	// Convert to API types
	result := make([]api.ServiceClassInfo, len(managerInfo))
	for i, info := range managerInfo {
		result[i] = api.ServiceClassInfo{
			Name:                     info.Name,
			Version:                  info.Version,
			Description:              info.Description,
			ServiceType:              info.ServiceType,
			Available:                info.Available,
			CreateToolAvailable:      info.CreateToolAvailable,
			DeleteToolAvailable:      info.DeleteToolAvailable,
			HealthCheckToolAvailable: info.HealthCheckToolAvailable,
			StatusToolAvailable:      info.StatusToolAvailable,
			RequiredTools:            info.RequiredTools,
			MissingTools:             info.MissingTools,
		}
	}

	return result
}

// GetServiceClass returns a specific service class definition by name
func (a *Adapter) GetServiceClass(name string) (*api.ServiceClassDefinition, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return nil, api.NewServiceClassNotFoundError(name)
	}

	// Convert to API type (lightweight version)
	apiDef := &api.ServiceClassDefinition{
		Name:        def.Name,
		Version:     def.Version,
		Description: def.Description,
	}

	return apiDef, nil
}

// GetStartTool returns start tool information for a service class
func (a *Adapter) GetStartTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, api.NewServiceClassNotFoundError(name)
	}

	startTool := def.ServiceConfig.LifecycleTools.Start
	if startTool.Tool == "" {
		return "", nil, nil, fmt.Errorf("no start tool defined for service class %s", name)
	}

	// Convert response mapping to simple map
	respMapping := map[string]string{
		"serviceId": startTool.ResponseMapping.ServiceID,
		"status":    startTool.ResponseMapping.Status,
		"health":    startTool.ResponseMapping.Health,
		"error":     startTool.ResponseMapping.Error,
	}

	return startTool.Tool, startTool.Arguments, respMapping, nil
}

// GetStopTool returns stop tool information for a service class
func (a *Adapter) GetStopTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, api.NewServiceClassNotFoundError(name)
	}

	stopTool := def.ServiceConfig.LifecycleTools.Stop
	if stopTool.Tool == "" {
		return "", nil, nil, fmt.Errorf("no stop tool defined for service class %s", name)
	}

	// Convert response mapping to simple map
	respMapping := map[string]string{
		"serviceId": stopTool.ResponseMapping.ServiceID,
		"status":    stopTool.ResponseMapping.Status,
		"health":    stopTool.ResponseMapping.Health,
		"error":     stopTool.ResponseMapping.Error,
	}

	return stopTool.Tool, stopTool.Arguments, respMapping, nil
}

// GetRestartTool returns restart tool information for a service class
func (a *Adapter) GetRestartTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, api.NewServiceClassNotFoundError(name)
	}

	restartTool := def.ServiceConfig.LifecycleTools.Restart
	if restartTool == nil || restartTool.Tool == "" {
		// This is an optional tool, so we return no error, just empty info
		return "", nil, nil, nil
	}

	// Convert response mapping to simple map
	respMapping := map[string]string{
		"serviceId": restartTool.ResponseMapping.ServiceID,
		"status":    restartTool.ResponseMapping.Status,
		"health":    restartTool.ResponseMapping.Health,
		"error":     restartTool.ResponseMapping.Error,
	}

	return restartTool.Tool, restartTool.Arguments, respMapping, nil
}

// GetHealthCheckTool returns health check tool information for a service class
func (a *Adapter) GetHealthCheckTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, api.NewServiceClassNotFoundError(name)
	}

	if def.ServiceConfig.LifecycleTools.HealthCheck == nil {
		return "", nil, nil, fmt.Errorf("no health check tool defined for service class %s", name)
	}

	healthTool := *def.ServiceConfig.LifecycleTools.HealthCheck
	if healthTool.Tool == "" {
		return "", nil, nil, fmt.Errorf("no health check tool defined for service class %s", name)
	}

	// Convert response mapping to simple map
	respMapping := map[string]string{
		"serviceId": healthTool.ResponseMapping.ServiceID,
		"status":    healthTool.ResponseMapping.Status,
		"health":    healthTool.ResponseMapping.Health,
		"error":     healthTool.ResponseMapping.Error,
	}

	return healthTool.Tool, healthTool.Arguments, respMapping, nil
}

// GetHealthCheckConfig returns health check configuration for a service class
func (a *Adapter) GetHealthCheckConfig(name string) (enabled bool, interval time.Duration, failureThreshold, successThreshold int, err error) {
	if a.manager == nil {
		return false, 0, 0, 0, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return false, 0, 0, 0, api.NewServiceClassNotFoundError(name)
	}

	config := def.ServiceConfig.HealthCheck
	return config.Enabled, config.Interval, config.FailureThreshold, config.SuccessThreshold, nil
}

// GetServiceDependencies returns dependencies for a service class
func (a *Adapter) GetServiceDependencies(name string) ([]string, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return nil, api.NewServiceClassNotFoundError(name)
	}

	return def.ServiceConfig.Dependencies, nil
}

// IsServiceClassAvailable checks if a service class is available
func (a *Adapter) IsServiceClassAvailable(name string) bool {
	if a.manager == nil {
		return false
	}

	return a.manager.IsServiceClassAvailable(name)
}

// GetManager returns the underlying ServiceClassManager (for internal use)
// This should only be used by other internal packages that need direct access
func (a *Adapter) GetManager() *ServiceClassManager {
	return a.manager
}

// ToolProvider implementation

// GetTools returns all tools this provider offers
func (a *Adapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		{
			Name:        "serviceclass_list",
			Description: "List all ServiceClass definitions with their availability status",
		},
		{
			Name:        "serviceclass_get",
			Description: "Get detailed information about a specific ServiceClass definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the ServiceClass to retrieve",
				},
			},
		},
		{
			Name:        "serviceclass_available",
			Description: "Check if a ServiceClass is available (all required tools present)",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the ServiceClass to check",
				},
			},
		},
		{
			Name:        "serviceclass_validate",
			Description: "Validate a serviceclass definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "ServiceClass name"},
				{Name: "serviceConfig", Type: "object", Required: true, Description: "Service configuration with lifecycle tools"},
				{Name: "description", Type: "string", Required: false, Description: "ServiceClass description"},
				{Name: "version", Type: "string", Required: false, Description: "ServiceClass version"},
			},
		},
		{
			Name:        "serviceclass_create",
			Description: "Create a new service class",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "ServiceClass name"},
				{Name: "serviceConfig", Type: "object", Required: true, Description: "Service configuration with lifecycle tools"},
				{Name: "description", Type: "string", Required: false, Description: "ServiceClass description"},
				{Name: "version", Type: "string", Required: false, Description: "ServiceClass version"},
			},
		},
		{
			Name:        "serviceclass_update",
			Description: "Update an existing service class",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the ServiceClass to update"},
				{Name: "serviceConfig", Type: "object", Required: false, Description: "Service configuration with lifecycle tools"},
				{Name: "description", Type: "string", Required: false, Description: "ServiceClass description"},
				{Name: "version", Type: "string", Required: false, Description: "ServiceClass version"},
			},
		},
		{
			Name:        "serviceclass_delete",
			Description: "Delete a service class",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the ServiceClass to delete"},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "serviceclass_list":
		return a.handleServiceClassList()
	case "serviceclass_get":
		return a.handleServiceClassGet(args)
	case "serviceclass_available":
		return a.handleServiceClassAvailable(args)
	case "serviceclass_validate":
		return a.handleServiceClassValidate(args)
	case "serviceclass_create":
		return a.handleServiceClassCreate(args)
	case "serviceclass_update":
		return a.handleServiceClassUpdate(args)
	case "serviceclass_delete":
		return a.handleServiceClassDelete(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Tool handlers

func (a *Adapter) handleServiceClassList() (*api.CallToolResult, error) {
	serviceClasses := a.ListServiceClasses()

	result := map[string]interface{}{
		"serviceClasses": serviceClasses,
		"total":          len(serviceClasses),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceClassGet(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	serviceClass, err := a.GetServiceClass(name)
	if err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to get ServiceClass"), nil
	}

	return &api.CallToolResult{
		Content: []interface{}{serviceClass},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceClassAvailable(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	available := a.IsServiceClassAvailable(name)

	result := map[string]interface{}{
		"name":      name,
		"available": available,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

// handleServiceClassValidate validates a serviceclass definition
func (a *Adapter) handleServiceClassValidate(args map[string]interface{}) (*api.CallToolResult, error) {
	// Parse parameters (without type and metadata)
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	serviceConfigParam, ok := args["serviceConfig"].(map[string]interface{})
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"serviceConfig parameter is required"},
			IsError: true,
		}, nil
	}

	version, _ := args["version"].(string)
	description, _ := args["description"].(string)

	// Convert serviceConfig from map[string]interface{} to ServiceConfig
	serviceConfig, err := convertServiceConfig(serviceConfigParam)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("invalid serviceConfig: %v", err)},
			IsError: true,
		}, nil
	}

	// Build ServiceClassDefinition from structured parameters (without type and metadata)
	def := ServiceClassDefinition{
		Name:          name,
		Version:       version,
		Description:   description,
		ServiceConfig: serviceConfig,
		Operations:    make(map[string]OperationDefinition), // Empty for validation
	}

	// Validate without persisting
	if err := a.manager.ValidateDefinition(&def); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Validation failed: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Validation successful for serviceclass %s", def.Name)},
		IsError: false,
	}, nil
}

// helper to create simple error CallToolResult
func simpleError(msg string) (*api.CallToolResult, error) {
	return &api.CallToolResult{Content: []interface{}{msg}, IsError: true}, nil
}

func simpleOK(msg string) (*api.CallToolResult, error) {
	return &api.CallToolResult{Content: []interface{}{msg}, IsError: false}, nil
}

func (a *Adapter) handleServiceClassCreate(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}
	version, _ := args["version"].(string)
	description, _ := args["description"].(string)
	serviceConfigParam, ok := args["serviceConfig"].(map[string]interface{})
	if !ok {
		return simpleError("serviceConfig parameter is required")
	}

	// Convert serviceConfig from map[string]interface{} to ServiceConfig
	serviceConfig, err := convertServiceConfig(serviceConfigParam)
	if err != nil {
		return simpleError(fmt.Sprintf("invalid serviceConfig: %v", err))
	}

	// Build ServiceClassDefinition from structured parameters (type/metadata will be set internally)
	def := ServiceClassDefinition{
		Name:          name,
		Version:       version,
		Description:   description,
		ServiceConfig: serviceConfig,
		Operations:    make(map[string]OperationDefinition), // Not provided via API anymore
	}

	if err := a.manager.CreateServiceClass(def); err != nil {
		return api.HandleErrorWithPrefix(err, "Create failed"), nil
	}

	return simpleOK(fmt.Sprintf("created service class %s", name))
}

// convertServiceConfig converts a map[string]interface{} to ServiceConfig
func convertServiceConfig(configMap map[string]interface{}) (ServiceConfig, error) {
	var config ServiceConfig

	// Convert lifecycleTools (required)
	lifecycleToolsMap, ok := configMap["lifecycleTools"].(map[string]interface{})
	if !ok {
		return config, fmt.Errorf("lifecycleTools is required")
	}

	lifecycleTools, err := convertLifecycleTools(lifecycleToolsMap)
	if err != nil {
		return config, fmt.Errorf("invalid lifecycleTools: %v", err)
	}
	config.LifecycleTools = lifecycleTools

	// Convert optional fields
	if serviceType, ok := configMap["serviceType"].(string); ok {
		config.ServiceType = serviceType
	}
	if defaultLabel, ok := configMap["defaultLabel"].(string); ok {
		config.DefaultLabel = defaultLabel
	}
	if deps, ok := configMap["dependencies"].([]interface{}); ok {
		config.Dependencies = make([]string, len(deps))
		for i, dep := range deps {
			if depStr, ok := dep.(string); ok {
				config.Dependencies[i] = depStr
			} else {
				return config, fmt.Errorf("dependency at index %d must be a string", i)
			}
		}
	}

	return config, nil
}

// convertLifecycleTools converts a map[string]interface{} to LifecycleTools
func convertLifecycleTools(toolsMap map[string]interface{}) (LifecycleTools, error) {
	var tools LifecycleTools

	// Convert start tool (required)
	if startMap, ok := toolsMap["start"].(map[string]interface{}); ok {
		start, err := convertToolCall(startMap)
		if err != nil {
			return tools, fmt.Errorf("invalid start tool: %v", err)
		}
		tools.Start = start
	} else {
		return tools, fmt.Errorf("start tool is required")
	}

	// Convert stop tool (required)
	if stopMap, ok := toolsMap["stop"].(map[string]interface{}); ok {
		stop, err := convertToolCall(stopMap)
		if err != nil {
			return tools, fmt.Errorf("invalid stop tool: %v", err)
		}
		tools.Stop = stop
	} else {
		return tools, fmt.Errorf("stop tool is required")
	}

	// Convert optional tools
	if restartMap, ok := toolsMap["restart"].(map[string]interface{}); ok {
		restart, err := convertToolCall(restartMap)
		if err != nil {
			return tools, fmt.Errorf("invalid restart tool: %v", err)
		}
		tools.Restart = &restart
	}

	if healthCheckMap, ok := toolsMap["healthCheck"].(map[string]interface{}); ok {
		healthCheck, err := convertToolCall(healthCheckMap)
		if err != nil {
			return tools, fmt.Errorf("invalid healthCheck tool: %v", err)
		}
		tools.HealthCheck = &healthCheck
	}

	if statusMap, ok := toolsMap["status"].(map[string]interface{}); ok {
		status, err := convertToolCall(statusMap)
		if err != nil {
			return tools, fmt.Errorf("invalid status tool: %v", err)
		}
		tools.Status = &status
	}

	return tools, nil
}

// convertToolCall converts a map[string]interface{} to ToolCall
func convertToolCall(toolMap map[string]interface{}) (ToolCall, error) {
	var tool ToolCall

	// Tool name is required
	if toolName, ok := toolMap["tool"].(string); ok {
		tool.Tool = toolName
	} else {
		return tool, fmt.Errorf("tool name is required")
	}

	// Arguments are optional
	if args, ok := toolMap["arguments"].(map[string]interface{}); ok {
		tool.Arguments = args
	}

	// ResponseMapping is optional
	if respMap, ok := toolMap["responseMapping"].(map[string]interface{}); ok {
		var responseMapping ResponseMapping
		if serviceID, ok := respMap["serviceId"].(string); ok {
			responseMapping.ServiceID = serviceID
		}
		if status, ok := respMap["status"].(string); ok {
			responseMapping.Status = status
		}
		if health, ok := respMap["health"].(string); ok {
			responseMapping.Health = health
		}
		if errorField, ok := respMap["error"].(string); ok {
			responseMapping.Error = errorField
		}
		if metadata, ok := respMap["metadata"].(map[string]interface{}); ok {
			responseMapping.Metadata = make(map[string]string)
			for key, value := range metadata {
				if strValue, ok := value.(string); ok {
					responseMapping.Metadata[key] = strValue
				}
			}
		}
		tool.ResponseMapping = responseMapping
	}

	return tool, nil
}

// convertOperationDefinition converts a map[string]interface{} to OperationDefinition
func convertOperationDefinition(opMap map[string]interface{}) (OperationDefinition, error) {
	var operation OperationDefinition

	if desc, ok := opMap["description"].(string); ok {
		operation.Description = desc
	}

	if requires, ok := opMap["requires"].([]interface{}); ok {
		operation.Requires = make([]string, len(requires))
		for i, req := range requires {
			if reqStr, ok := req.(string); ok {
				operation.Requires[i] = reqStr
			} else {
				return operation, fmt.Errorf("requires at index %d must be a string", i)
			}
		}
	}

	if params, ok := opMap["parameters"].(map[string]interface{}); ok {
		operation.Parameters = make(map[string]Parameter)
		for paramName, paramData := range params {
			paramMap, ok := paramData.(map[string]interface{})
			if !ok {
				return operation, fmt.Errorf("parameter '%s' must be an object", paramName)
			}

			var param Parameter
			if paramType, ok := paramMap["type"].(string); ok {
				param.Type = paramType
			}
			if required, ok := paramMap["required"].(bool); ok {
				param.Required = required
			}
			if description, ok := paramMap["description"].(string); ok {
				param.Description = description
			}
			if defaultValue, ok := paramMap["default"]; ok {
				param.Default = defaultValue
			}

			operation.Parameters[paramName] = param
		}
	}

	return operation, nil
}

func (a *Adapter) handleServiceClassUpdate(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}
	version, _ := args["version"].(string)
	description, _ := args["description"].(string)
	serviceConfigParam, _ := args["serviceConfig"].(map[string]interface{})

	// Get existing serviceclass to preserve current type/metadata/operations
	existingDef, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return api.HandleErrorWithPrefix(api.NewServiceClassNotFoundError(name), "Failed to update ServiceClass"), nil
	}

	// Start with existing definition and update provided fields
	def := ServiceClassDefinition{
		Name:          name,
		Version:       existingDef.Version,     // Will be updated if provided
		Description:   existingDef.Description, // Will be updated if provided
		ServiceConfig: existingDef.ServiceConfig,
		Operations:    existingDef.Operations, // Preserve existing operations
	}

	// Update fields if provided
	if version != "" {
		def.Version = version
	}
	if description != "" {
		def.Description = description
	}
	if serviceConfigParam != nil {
		serviceConfig, err := convertServiceConfig(serviceConfigParam)
		if err != nil {
			return simpleError(fmt.Sprintf("invalid serviceConfig: %v", err))
		}
		def.ServiceConfig = serviceConfig
	}

	if err := a.manager.UpdateServiceClass(name, def); err != nil {
		return api.HandleErrorWithPrefix(err, "Update failed"), nil
	}

	return simpleOK(fmt.Sprintf("updated service class %s", name))
}

func (a *Adapter) handleServiceClassDelete(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}

	if err := a.manager.DeleteServiceClass(name); err != nil {
		return api.HandleErrorWithPrefix(err, "Delete failed"), nil
	}

	return simpleOK(fmt.Sprintf("deleted service class %s", name))
}
