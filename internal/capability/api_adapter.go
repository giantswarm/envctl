package capability

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
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
			Name:        "capability_validate",
			Description: "Validate a capability definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Capability name"},
				{Name: "type", Type: "string", Required: true, Description: "Capability type"},
				{Name: "version", Type: "string", Required: false, Description: "Capability version"},
				{Name: "description", Type: "string", Required: false, Description: "Capability description"},
				{Name: "operations", Type: "object", Required: true, Description: "Map of operation name to operation definition"},
			},
		},
		{
			Name:        "capability_create",
			Description: "Create a new capability definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Capability name"},
				{Name: "type", Type: "string", Required: true, Description: "Capability type"},
				{Name: "version", Type: "string", Required: false, Description: "Capability version"},
				{Name: "description", Type: "string", Required: false, Description: "Capability description"},
				{Name: "operations", Type: "object", Required: true, Description: "Map of operation name to operation definition"},
				{Name: "metadata", Type: "object", Required: false, Description: "Key-value metadata pairs"},
			},
		},
		{
			Name:        "capability_update",
			Description: "Update an existing capability definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the capability to update"},
				{Name: "type", Type: "string", Required: false, Description: "Capability type"},
				{Name: "version", Type: "string", Required: false, Description: "Capability version"},
				{Name: "description", Type: "string", Required: false, Description: "Capability description"},
				{Name: "operations", Type: "object", Required: false, Description: "Map of operation name to operation definition"},
				{Name: "metadata", Type: "object", Required: false, Description: "Key-value metadata pairs"},
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
	case "capability_validate":
		return a.handleCapabilityValidate(ctx, args)
	case "capability_create":
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("name parameter is required")
		}
		capType, ok := args["type"].(string)
		if !ok {
			return nil, fmt.Errorf("type parameter is required")
		}
		version, _ := args["version"].(string)
		description, _ := args["description"].(string)
		operations, ok := args["operations"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("operations parameter is required")
		}
		metadata, _ := args["metadata"].(map[string]interface{})
		return a.handleCapabilityCreate(ctx, name, capType, version, description, operations, metadata)
	case "capability_update":
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("name parameter is required")
		}
		capType, _ := args["type"].(string)
		version, _ := args["version"].(string)
		description, _ := args["description"].(string)
		operations, _ := args["operations"].(map[string]interface{})
		metadata, _ := args["metadata"].(map[string]interface{})
		return a.handleCapabilityUpdate(ctx, name, capType, version, description, operations, metadata)
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

// handleCapabilityValidate validates a capability definition
func (a *Adapter) handleCapabilityValidate(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	// Parse parameters same as create method (without metadata)
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	capType, ok := args["type"].(string)
	if !ok || capType == "" {
		return &api.CallToolResult{
			Content: []interface{}{"type parameter is required"},
			IsError: true,
		}, nil
	}

	operations, ok := args["operations"].(map[string]interface{})
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"operations parameter is required"},
			IsError: true,
		}, nil
	}

	version, _ := args["version"].(string)
	description, _ := args["description"].(string)

	// Build CapabilityDefinition from structured parameters (without metadata)
	def := &CapabilityDefinition{
		Name:        name,
		Type:        capType,
		Version:     version,
		Description: description,
		Operations:  make(map[string]OperationDefinition),
		Metadata:    make(map[string]string), // Empty for validation
	}

	// Convert operations map to OperationDefinition structs
	for opName, opData := range operations {
		opMap, ok := opData.(map[string]interface{})
		if !ok {
			return &api.CallToolResult{
				Content: []interface{}{fmt.Sprintf("Invalid operation definition for '%s': expected object", opName)},
				IsError: true,
			}, nil
		}

		opDef := OperationDefinition{
			Parameters: make(map[string]Parameter),
			Requires:   []string{},
		}

		// Extract operation description
		if desc, ok := opMap["description"].(string); ok {
			opDef.Description = desc
		}

		// Extract operation parameters
		if params, ok := opMap["parameters"].(map[string]interface{}); ok {
			for paramName, paramData := range params {
				paramMap, ok := paramData.(map[string]interface{})
				if !ok {
					return &api.CallToolResult{
						Content: []interface{}{fmt.Sprintf("Invalid parameter definition for '%s.%s': expected object", opName, paramName)},
						IsError: true,
					}, nil
				}

				param := Parameter{}
				if paramType, ok := paramMap["type"].(string); ok {
					param.Type = paramType
				}
				if required, ok := paramMap["required"].(bool); ok {
					param.Required = required
				}
				if desc, ok := paramMap["description"].(string); ok {
					param.Description = desc
				}
				if defaultVal := paramMap["default"]; defaultVal != nil {
					param.Default = defaultVal
				}

				opDef.Parameters[paramName] = param
			}
		}

		// Extract required tools
		if requires, ok := opMap["requires"].([]interface{}); ok {
			for _, req := range requires {
				if reqStr, ok := req.(string); ok {
					opDef.Requires = append(opDef.Requires, reqStr)
				}
			}
		}

		def.Operations[opName] = opDef
	}

	// Validate without persisting
	if err := a.manager.ValidateDefinition(def); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Validation failed: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Validation successful for capability %s", def.Name)},
		IsError: false,
	}, nil
}

// GetCapability returns a specific capability definition (implements CapabilityHandler interface)
func (a *Adapter) GetCapability(name string) (interface{}, error) {
	def, exists := a.manager.GetDefinition(name)
	if !exists {
		return nil, api.NewCapabilityNotFoundError(name)
	}
	return &def, nil
}

// LoadDefinitions loads capability definitions from YAML files
func (a *Adapter) LoadDefinitions() error {
	return a.manager.LoadDefinitions()
}

// SetConfigPath sets the custom configuration path for the capability manager
func (a *Adapter) SetConfigPath(configPath string) {
	a.manager.SetConfigPath(configPath)
}

// Handler methods for new CRUD tools

func (a *Adapter) handleCapabilityCreate(ctx context.Context, name, capType, version, description string, operations map[string]interface{}, metadata map[string]interface{}) (*api.CallToolResult, error) {
	// Build CapabilityDefinition from structured parameters
	def := &CapabilityDefinition{
		Name:        name,
		Type:        capType,
		Version:     version,
		Description: description,
		Operations:  make(map[string]OperationDefinition),
		Metadata:    make(map[string]string),
	}

	// Convert operations map to OperationDefinition structs
	for opName, opData := range operations {
		opMap, ok := opData.(map[string]interface{})
		if !ok {
			return &api.CallToolResult{
				Content: []interface{}{fmt.Sprintf("Invalid operation definition for '%s': expected object", opName)},
				IsError: true,
			}, nil
		}

		opDef := OperationDefinition{
			Parameters: make(map[string]Parameter),
			Requires:   []string{},
		}

		// Extract operation description
		if desc, ok := opMap["description"].(string); ok {
			opDef.Description = desc
		}

		// Extract operation parameters
		if params, ok := opMap["parameters"].(map[string]interface{}); ok {
			for paramName, paramData := range params {
				paramMap, ok := paramData.(map[string]interface{})
				if !ok {
					return &api.CallToolResult{
						Content: []interface{}{fmt.Sprintf("Invalid parameter definition for '%s.%s': expected object", opName, paramName)},
						IsError: true,
					}, nil
				}

				param := Parameter{}
				if paramType, ok := paramMap["type"].(string); ok {
					param.Type = paramType
				}
				if required, ok := paramMap["required"].(bool); ok {
					param.Required = required
				}
				if desc, ok := paramMap["description"].(string); ok {
					param.Description = desc
				}
				if defaultVal := paramMap["default"]; defaultVal != nil {
					param.Default = defaultVal
				}

				opDef.Parameters[paramName] = param
			}
		}

		// Extract required tools
		if requires, ok := opMap["requires"].([]interface{}); ok {
			for _, req := range requires {
				if reqStr, ok := req.(string); ok {
					opDef.Requires = append(opDef.Requires, reqStr)
				}
			}
		}

		def.Operations[opName] = opDef
	}

	// Convert metadata map to string map
	for k, v := range metadata {
		if strVal, ok := v.(string); ok {
			def.Metadata[k] = strVal
		}
	}

	// Create the capability using the manager
	if err := a.manager.CreateCapability(def); err != nil {
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

func (a *Adapter) handleCapabilityUpdate(ctx context.Context, name, capType, version, description string, operations map[string]interface{}, metadata map[string]interface{}) (*api.CallToolResult, error) {
	// Get existing capability definition
	existingDef, exists := a.manager.GetDefinition(name)
	if !exists {
		return api.HandleErrorWithPrefix(api.NewCapabilityNotFoundError(name), "Failed to update capability"), nil
	}

	// Start with existing definition and update provided fields
	def := &CapabilityDefinition{
		Name:        name,
		Type:        existingDef.Type,
		Version:     existingDef.Version,
		Description: existingDef.Description,
		Operations:  make(map[string]OperationDefinition),
		Metadata:    make(map[string]string),
	}

	// Copy existing operations and metadata
	for opName, opDef := range existingDef.Operations {
		def.Operations[opName] = opDef
	}
	for k, v := range existingDef.Metadata {
		def.Metadata[k] = v
	}

	// Update fields if provided
	if capType != "" {
		def.Type = capType
	}
	if version != "" {
		def.Version = version
	}
	if description != "" {
		def.Description = description
	}

	// Update operations if provided
	if operations != nil {
		def.Operations = make(map[string]OperationDefinition)
		for opName, opData := range operations {
			opMap, ok := opData.(map[string]interface{})
			if !ok {
				return &api.CallToolResult{
					Content: []interface{}{fmt.Sprintf("Invalid operation definition for '%s': expected object", opName)},
					IsError: true,
				}, nil
			}

			opDef := OperationDefinition{
				Parameters: make(map[string]Parameter),
				Requires:   []string{},
			}

			// Extract operation description
			if desc, ok := opMap["description"].(string); ok {
				opDef.Description = desc
			}

			// Extract operation parameters
			if params, ok := opMap["parameters"].(map[string]interface{}); ok {
				for paramName, paramData := range params {
					paramMap, ok := paramData.(map[string]interface{})
					if !ok {
						return &api.CallToolResult{
							Content: []interface{}{fmt.Sprintf("Invalid parameter definition for '%s.%s': expected object", opName, paramName)},
							IsError: true,
						}, nil
					}

					param := Parameter{}
					if paramType, ok := paramMap["type"].(string); ok {
						param.Type = paramType
					}
					if required, ok := paramMap["required"].(bool); ok {
						param.Required = required
					}
					if desc, ok := paramMap["description"].(string); ok {
						param.Description = desc
					}
					if defaultVal := paramMap["default"]; defaultVal != nil {
						param.Default = defaultVal
					}

					opDef.Parameters[paramName] = param
				}
			}

			// Extract required tools
			if requires, ok := opMap["requires"].([]interface{}); ok {
				for _, req := range requires {
					if reqStr, ok := req.(string); ok {
						opDef.Requires = append(opDef.Requires, reqStr)
					}
				}
			}

			def.Operations[opName] = opDef
		}
	}

	// Update metadata if provided
	if metadata != nil {
		def.Metadata = make(map[string]string)
		for k, v := range metadata {
			if strVal, ok := v.(string); ok {
				def.Metadata[k] = strVal
			}
		}
	}

	// Update the capability using the manager
	if err := a.manager.UpdateCapability(def); err != nil {
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
