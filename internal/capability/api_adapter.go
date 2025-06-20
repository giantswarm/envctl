package capability

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Adapter adapts the capability system to implement api.CapabilityHandler
type Adapter struct {
	manager          *CapabilityManager
	workflowExecutor api.ToolCaller
}

// NewAdapter creates a new capability adapter
func NewAdapter(toolChecker config.ToolAvailabilityChecker, workflowExecutor api.ToolCaller, storage *config.Storage) (*Adapter, error) {
	registry := GetRegistry()
	manager, err := NewCapabilityManager(toolChecker, registry, storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create capability manager: %w", err)
	}

	return &Adapter{
		manager:          manager,
		workflowExecutor: workflowExecutor,
	}, nil
}

// Register registers this adapter with the API
func (a *Adapter) Register() {
	api.RegisterCapability(a)
}

// GetTools returns MCP tools for capability management
func (a *Adapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		{
			Name:        "capability_list",
			Description: "List all capability definitions",
		},
		{
			Name:        "capability_get",
			Description: "Get a specific capability definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Capability name"},
			},
		},
		{
			Name:        "capability_available",
			Description: "Check if a capability is available",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Capability name"},
			},
		},
		{
			Name:        "capability_definitions_path",
			Description: "Get the paths where capability definitions are loaded from",
		},
		{
			Name:        "capability_load",
			Description: "Reload capability definitions from disk",
		},
		{
			Name:        "capability_create",
			Description: "Create a new capability definition in storage",
			Parameters: []api.ParameterMetadata{
				{Name: "yamlContent", Type: "string", Required: true, Description: "YAML content of the capability definition"},
			},
		},
		{
			Name:        "capability_update",
			Description: "Update an existing capability definition in storage",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the capability to update"},
				{Name: "yamlContent", Type: "string", Required: true, Description: "Updated YAML content of the capability definition"},
			},
		},
		{
			Name:        "capability_delete",
			Description: "Delete a capability definition from YAML files",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the capability to delete"},
			},
		},
	}
}

// ExecuteTool executes a capability management tool
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "capability_list":
		return a.listCapabilities(ctx)
	case "capability_get":
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("name parameter is required")
		}
		return a.getCapability(ctx, name)
	case "capability_available":
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("name parameter is required")
		}
		return a.checkCapabilityAvailable(ctx, name)
	case "capability_definitions_path":
		return a.getDefinitionsPath(ctx)
	case "capability_load":
		return a.loadCapabilities(ctx)
	case "capability_create":
		yamlContent, ok := args["yamlContent"].(string)
		if !ok {
			return nil, fmt.Errorf("yamlContent parameter is required")
		}
		return a.handleCapabilityCreate(ctx, yamlContent)
	case "capability_update":
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("name parameter is required")
		}
		yamlContent, ok := args["yamlContent"].(string)
		if !ok {
			return nil, fmt.Errorf("yamlContent parameter is required")
		}
		return a.handleCapabilityUpdate(ctx, name, yamlContent)
	case "capability_delete":
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("name parameter is required")
		}
		return a.handleCapabilityDelete(ctx, name)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// ExecuteCapability executes a capability operation (implements CapabilityHandler interface)
func (a *Adapter) ExecuteCapability(ctx context.Context, capabilityType, operation string, params map[string]interface{}) (*api.CallToolResult, error) {
	// Find the operation
	toolName := fmt.Sprintf("api_%s_%s", capabilityType, operation)
	opDef, capDef, err := a.manager.GetOperationForTool(toolName)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Operation not found: %v", err)},
			IsError: true,
		}, nil
	}

	// Check if operation is available
	if !a.manager.IsAvailable(capDef.Name) {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Capability %s is not available (missing required tools)", capabilityType)},
			IsError: true,
		}, nil
	}

	// Execute the capability operation
	logging.Info("CapabilityAdapter", "Executing capability operation: %s.%s (description: %s)", capabilityType, operation, opDef.Description)

	// For now, return a placeholder result
	// TODO: Implement actual capability execution logic
	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Executed %s.%s successfully", capabilityType, operation)},
		IsError: false,
	}, nil
}

// IsCapabilityAvailable checks if a capability operation is available (implements CapabilityHandler interface)
func (a *Adapter) IsCapabilityAvailable(capabilityType, operation string) bool {
	toolName := fmt.Sprintf("api_%s_%s", capabilityType, operation)
	_, _, err := a.manager.GetOperationForTool(toolName)
	if err != nil {
		return false
	}

	// Check if the capability itself is available
	def, exists := a.manager.GetDefinition(capabilityType)
	if !exists {
		return false
	}

	return a.manager.IsAvailable(def.Name)
}

// ListCapabilities returns information about all available capabilities (implements CapabilityHandler interface)
func (a *Adapter) ListCapabilities() []api.CapabilityInfo {
	definitions := a.manager.ListDefinitions()
	result := make([]api.CapabilityInfo, len(definitions))

	for i, def := range definitions {
		operations := make([]api.OperationInfo, 0, len(def.Operations))
		for opName, opDef := range def.Operations {
			operations = append(operations, api.OperationInfo{
				Name:        opName,
				Description: opDef.Description,
				Available:   a.IsCapabilityAvailable(def.Type, opName),
			})
		}

		result[i] = api.CapabilityInfo{
			Type:        def.Type,
			Name:        def.Name,
			Description: def.Description,
			Version:     def.Version,
			Operations:  operations,
		}
	}

	return result
}

// listCapabilities lists all capability definitions
func (a *Adapter) listCapabilities(ctx context.Context) (*api.CallToolResult, error) {
	definitions := a.manager.ListDefinitions()

	result := make([]map[string]interface{}, len(definitions))
	for i, def := range definitions {
		available := a.manager.IsAvailable(def.Name)
		result[i] = map[string]interface{}{
			"name":        def.Name,
			"type":        def.Type,
			"version":     def.Version,
			"description": def.Description,
			"available":   available,
			"operations":  len(def.Operations),
		}
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Found %d capability definitions", len(result)), result},
		IsError: false,
	}, nil
}

// getCapability gets a specific capability definition
func (a *Adapter) getCapability(ctx context.Context, name string) (*api.CallToolResult, error) {
	def, exists := a.manager.GetDefinition(name)
	if !exists {
		return api.HandleErrorWithPrefix(api.NewCapabilityNotFoundError(name), "Failed to get capability"), nil
	}

	available := a.manager.IsAvailable(name)

	result := map[string]interface{}{
		"name":        def.Name,
		"type":        def.Type,
		"version":     def.Version,
		"description": def.Description,
		"available":   available,
		"operations":  def.Operations,
		"metadata":    def.Metadata,
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Capability: %s (Type: %s, Available: %v)", def.Name, def.Type, available), result},
		IsError: false,
	}, nil
}

// checkCapabilityAvailable checks if a capability is available
func (a *Adapter) checkCapabilityAvailable(ctx context.Context, name string) (*api.CallToolResult, error) {
	available := a.manager.IsAvailable(name)

	result := map[string]interface{}{
		"name":      name,
		"available": available,
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Capability '%s' available: %v", name, available), result},
		IsError: false,
	}, nil
}

// getDefinitionsPath returns the paths where capability definitions are loaded from
func (a *Adapter) getDefinitionsPath(ctx context.Context) (*api.CallToolResult, error) {
	path := a.manager.GetDefinitionsPath()

	result := map[string]interface{}{
		"path": path,
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Capability definitions path: %s", path), result},
		IsError: false,
	}, nil
}

// loadCapabilities reloads capability definitions from disk
func (a *Adapter) loadCapabilities(ctx context.Context) (*api.CallToolResult, error) {
	err := a.manager.LoadDefinitions()
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to load capabilities: %v", err)},
			IsError: true,
		}, nil
	}

	definitions := a.manager.ListDefinitions()
	available := a.manager.ListAvailableDefinitions()

	result := map[string]interface{}{
		"loaded":    len(definitions),
		"available": len(available),
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Loaded %d capabilities, %d available", len(definitions), len(available)), result},
		IsError: false,
	}, nil
}

// Legacy methods updated to work with CapabilityManager

func (a *Adapter) createCapabilityDefinition(yamlContent, filename string) error {
	// Parse the YAML content
	var def CapabilityDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Create the capability using the manager
	return a.manager.CreateCapability(&def)
}

func (a *Adapter) updateCapabilityDefinition(name, yamlContent string) error {
	// Parse the YAML content
	var def CapabilityDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Ensure the name matches
	if def.Name != name {
		return fmt.Errorf("capability name in YAML (%s) does not match provided name (%s)", def.Name, name)
	}

	// Update the capability using the manager
	return a.manager.UpdateCapability(&def)
}

func (a *Adapter) deleteCapabilityDefinition(name string) error {
	// Delete the capability using the manager
	return a.manager.DeleteCapability(name)
}

// GetCapability returns a specific capability definition (implements CapabilityHandler interface)
func (a *Adapter) GetCapability(name string) (interface{}, error) {
	def, exists := a.manager.GetDefinition(name)
	if !exists {
		return nil, api.NewCapabilityNotFoundError(name)
	}
	return &def, nil
}

// LoadDefinitions reloads capability definitions from disk (implements CapabilityHandler interface)
func (a *Adapter) LoadDefinitions() error {
	return a.manager.LoadDefinitions()
}

// RefreshAvailability refreshes the availability status of all capabilities (implements CapabilityHandler interface)
func (a *Adapter) RefreshAvailability() {
	// The manager automatically checks availability when needed
	// This method provides a way to force a refresh if needed in the future
	logging.Debug("CapabilityAdapter", "Refreshing capability availability")
}

// GetDefinitionsPath returns the path where capability definitions are loaded from (implements CapabilityHandler interface)
func (a *Adapter) GetDefinitionsPath() string {
	return a.manager.GetDefinitionsPath()
}

// Handler methods for new CRUD tools

func (a *Adapter) handleCapabilityCreate(ctx context.Context, yamlContent string) (*api.CallToolResult, error) {
	// Parse and validate the YAML content
	var def CapabilityDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse YAML: %v", err)},
			IsError: true,
		}, nil
	}

	// Create the capability
	if err := a.manager.CreateCapability(&def); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to create capability"), nil
	}

	result := map[string]interface{}{
		"action":  "created",
		"name":    def.Name,
		"type":    def.Type,
		"version": def.Version,
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully created capability: %s", def.Name), result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleCapabilityUpdate(ctx context.Context, name, yamlContent string) (*api.CallToolResult, error) {
	// Parse and validate the YAML content
	var def CapabilityDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse YAML: %v", err)},
			IsError: true,
		}, nil
	}

	// Ensure the name matches
	if def.Name != name {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Capability name in YAML (%s) does not match provided name (%s)", def.Name, name)},
			IsError: true,
		}, nil
	}

	// Update the capability
	if err := a.manager.UpdateCapability(&def); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to update capability"), nil
	}

	result := map[string]interface{}{
		"action":  "updated",
		"name":    def.Name,
		"type":    def.Type,
		"version": def.Version,
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully updated capability: %s", def.Name), result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleCapabilityDelete(ctx context.Context, name string) (*api.CallToolResult, error) {
	// Check if the capability exists
	_, exists := a.manager.GetDefinition(name)
	if !exists {
		return api.HandleErrorWithPrefix(api.NewCapabilityNotFoundError(name), "Failed to delete capability"), nil
	}

	// Delete the capability
	if err := a.manager.DeleteCapability(name); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to delete capability"), nil
	}

	result := map[string]interface{}{
		"action": "deleted",
		"name":   name,
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully deleted capability: %s", name), result},
		IsError: false,
	}, nil
}
