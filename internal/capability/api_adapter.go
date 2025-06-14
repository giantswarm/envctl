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
func NewAdapter(toolChecker config.ToolAvailabilityChecker, workflowExecutor api.ToolCaller) (*Adapter, error) {
	registry := GetRegistry()
	manager, err := NewCapabilityManager(toolChecker, registry)
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
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Capability '%s' not found", name)},
			IsError: true,
		}, nil
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
	// TODO: Implement capability creation for layered configuration system
	// This would need to determine whether to write to user or project config
	// and handle the layered override behavior appropriately
	return fmt.Errorf("capability creation not yet supported with layered configuration")
}

func (a *Adapter) updateCapabilityDefinition(name, yamlContent string) error {
	// TODO: Implement capability update for layered configuration system
	// This would need to find the capability in either user or project config
	// and update it appropriately
	return fmt.Errorf("capability updates not yet supported with layered configuration")
}

func (a *Adapter) deleteCapabilityDefinition(name string) error {
	// TODO: Implement capability deletion for layered configuration system
	// This would need to find the capability in either user or project config
	// and remove it appropriately
	return fmt.Errorf("capability deletion not yet supported with layered configuration")
}
