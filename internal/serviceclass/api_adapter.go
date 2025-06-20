package serviceclass

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
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
			Type:                     info.Type,
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
		Type:        def.Type,
		Version:     def.Version,
		Description: def.Description,
		Metadata:    def.Metadata,
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

// LoadServiceDefinitions loads all service class definitions from the configured path
func (a *Adapter) LoadServiceDefinitions() error {
	if a.manager == nil {
		return fmt.Errorf("service class manager not available")
	}

	return a.manager.LoadServiceDefinitions()
}

// RefreshAvailability refreshes the availability status of all service classes
func (a *Adapter) RefreshAvailability() {
	if a.manager == nil {
		logging.Warn("ServiceClassAdapter", "Cannot refresh availability: manager not available")
		return
	}

	a.manager.RefreshAvailability()
}

// RegisterDefinition registers a service class definition programmatically
func (a *Adapter) RegisterDefinition(apiDef *api.ServiceClassDefinition) error {
	if a.manager == nil {
		return fmt.Errorf("service class manager not available")
	}

	// Convert from API type to internal type
	// Note: This is a basic conversion - full ServiceClassDefinition would need more fields
	internalDef := &ServiceClassDefinition{
		Name:        apiDef.Name,
		Type:        apiDef.Type,
		Version:     apiDef.Version,
		Description: apiDef.Description,
		Metadata:    apiDef.Metadata,
		// ServiceConfig and Operations would need to be provided separately
		// for a complete programmatic registration
	}

	return a.manager.RegisterDefinition(internalDef)
}

// UnregisterDefinition unregisters a service class definition
func (a *Adapter) UnregisterDefinition(name string) error {
	if a.manager == nil {
		return fmt.Errorf("service class manager not available")
	}

	return a.manager.UnregisterDefinition(name)
}

// GetDefinitionsPath returns the path where service class definitions are loaded from
func (a *Adapter) GetDefinitionsPath() string {
	if a.manager == nil {
		return ""
	}

	return a.manager.GetDefinitionsPath()
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
			Name:        "serviceclass_refresh",
			Description: "Refresh the availability status of all ServiceClass definitions",
		},
		{
			Name:        "serviceclass_load",
			Description: "Load ServiceClass definitions from the configured directory",
		},
		{
			Name:        "serviceclass_definitions_path",
			Description: "Get the path where ServiceClass definitions are loaded from",
		},
		{
			Name:        "serviceclass_register",
			Description: "Register a ServiceClass from YAML on the fly",
			Parameters: []api.ParameterMetadata{
				{Name: "yaml", Type: "string", Required: true, Description: "Full ServiceClass YAML definition"},
				{Name: "merge", Type: "boolean", Required: false, Description: "If true, replace existing ServiceClass of the same name"},
			},
		},
		{
			Name:        "serviceclass_unregister",
			Description: "Unregister a ServiceClass by name",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the ServiceClass to remove"},
			},
		},
		{
			Name:        "serviceclass_create",
			Description: "Create a new service class",
			Parameters: []api.ParameterMetadata{
				{Name: "yaml", Type: "string", Required: true, Description: "Full ServiceClass YAML definition"},
			},
		},
		{
			Name:        "serviceclass_update",
			Description: "Update an existing service class",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the ServiceClass to update"},
				{Name: "yaml", Type: "string", Required: true, Description: "Updated ServiceClass YAML definition"},
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
	case "serviceclass_refresh":
		return a.handleServiceClassRefresh()
	case "serviceclass_load":
		return a.handleServiceClassLoad()
	case "serviceclass_definitions_path":
		return a.handleServiceClassDefinitionsPath()
	case "serviceclass_register":
		return a.handleServiceClassRegister(args)
	case "serviceclass_unregister":
		return a.handleServiceClassUnregister(args)
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

func (a *Adapter) handleServiceClassRefresh() (*api.CallToolResult, error) {
	a.RefreshAvailability()

	return &api.CallToolResult{
		Content: []interface{}{"ServiceClass availability refreshed successfully"},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceClassLoad() (*api.CallToolResult, error) {
	err := a.LoadServiceDefinitions()
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to load ServiceClass definitions: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"ServiceClass definitions loaded successfully"},
		IsError: false,
	}, nil
}

func (a *Adapter) handleServiceClassDefinitionsPath() (*api.CallToolResult, error) {
	path := a.GetDefinitionsPath()

	result := map[string]interface{}{
		"definitionsPath": path,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
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

func (a *Adapter) handleServiceClassRegister(args map[string]interface{}) (*api.CallToolResult, error) {
	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return simpleError("yaml parameter is required")
	}
	merge, _ := args["merge"].(bool)

	var def ServiceClassDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return simpleError(fmt.Sprintf("YAML parse error: %v", err))
	}

	// Optionally overwrite existing definition
	if merge {
		_ = a.manager.UnregisterDefinition(def.Name)
	}
	if err := a.manager.RegisterDefinition(&def); err != nil {
		return simpleError(fmt.Sprintf("Register failed: %v", err))
	}
	// after registering, refresh availability so missing tools list is updated
	a.manager.RefreshAvailability()
	return simpleOK(fmt.Sprintf("registered ServiceClass %s", def.Name))
}

func (a *Adapter) handleServiceClassUnregister(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}
	if err := a.manager.UnregisterDefinition(name); err != nil {
		return api.HandleErrorWithPrefix(err, "Unregister failed"), nil
	}
	return simpleOK(fmt.Sprintf("unregistered ServiceClass %s", name))
}

func (a *Adapter) handleServiceClassCreate(args map[string]interface{}) (*api.CallToolResult, error) {
	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return simpleError("yaml parameter is required")
	}

	var def ServiceClassDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return simpleError(fmt.Sprintf("YAML parse error: %v", err))
	}

	if err := a.manager.CreateServiceClass(def); err != nil {
		return api.HandleErrorWithPrefix(err, "Create failed"), nil
	}

	return simpleOK(fmt.Sprintf("created service class %s", def.Name))
}

func (a *Adapter) handleServiceClassUpdate(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}
	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return simpleError("yaml parameter is required")
	}

	var def ServiceClassDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return simpleError(fmt.Sprintf("YAML parse error: %v", err))
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
