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
		return nil, fmt.Errorf("service class %s not found", name)
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

// GetCreateTool returns create tool information for a service class
func (a *Adapter) GetCreateTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, fmt.Errorf("service class %s not found", name)
	}

	createTool := def.ServiceConfig.LifecycleTools.Create
	if createTool.Tool == "" {
		return "", nil, nil, fmt.Errorf("no create tool defined for service class %s", name)
	}

	// Convert response mapping to simple map
	respMapping := map[string]string{
		"serviceId": createTool.ResponseMapping.ServiceID,
		"status":    createTool.ResponseMapping.Status,
		"health":    createTool.ResponseMapping.Health,
		"error":     createTool.ResponseMapping.Error,
	}

	return createTool.Tool, createTool.Arguments, respMapping, nil
}

// GetDeleteTool returns delete tool information for a service class
func (a *Adapter) GetDeleteTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, fmt.Errorf("service class %s not found", name)
	}

	deleteTool := def.ServiceConfig.LifecycleTools.Delete
	if deleteTool.Tool == "" {
		return "", nil, nil, fmt.Errorf("no delete tool defined for service class %s", name)
	}

	// Convert response mapping to simple map
	respMapping := map[string]string{
		"serviceId": deleteTool.ResponseMapping.ServiceID,
		"status":    deleteTool.ResponseMapping.Status,
		"health":    deleteTool.ResponseMapping.Health,
		"error":     deleteTool.ResponseMapping.Error,
	}

	return deleteTool.Tool, deleteTool.Arguments, respMapping, nil
}

// GetHealthCheckTool returns health check tool information for a service class
func (a *Adapter) GetHealthCheckTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error) {
	if a.manager == nil {
		return "", nil, nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return "", nil, nil, fmt.Errorf("service class %s not found", name)
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
		return false, 0, 0, 0, fmt.Errorf("service class %s not found", name)
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
		return nil, fmt.Errorf("service class %s not found", name)
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
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get ServiceClass: %v", err)},
			IsError: true,
		}, nil
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
