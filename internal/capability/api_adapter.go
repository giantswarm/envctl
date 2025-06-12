package capability

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Adapter adapts the capability system to implement api.CapabilityHandler
type Adapter struct {
	loader           *CapabilityLoader
	workflowExecutor api.ToolCaller
}

// NewAdapter creates a new capability adapter
func NewAdapter(definitionsPath string, toolChecker ToolAvailabilityChecker, workflowExecutor api.ToolCaller) *Adapter {
	registry := GetRegistry()
	loader := NewCapabilityLoader(definitionsPath, toolChecker, registry)

	return &Adapter{
		loader:           loader,
		workflowExecutor: workflowExecutor,
	}
}

// Register registers this adapter with the API package
func (a *Adapter) Register() {
	api.RegisterCapability(a)
}

// ExecuteCapability executes a capability operation
func (a *Adapter) ExecuteCapability(ctx context.Context, capabilityType, operation string, params map[string]interface{}) (*api.CallToolResult, error) {
	// Build the tool name from capability type and operation (using api_ format)
	toolName := fmt.Sprintf("api_%s_%s", capabilityType, operation)

	// Check if the capability is available
	if !a.IsCapabilityAvailable(capabilityType, operation) {
		return nil, fmt.Errorf("capability %s.%s is not available", capabilityType, operation)
	}

	logging.Info("CapabilityAdapter", "Executing capability %s.%s", capabilityType, operation)

	// Get the operation definition
	op, _, err := a.loader.GetOperationForTool(toolName)
	if err != nil {
		return nil, fmt.Errorf("unknown capability operation: %s", toolName)
	}

	// Execute the workflow associated with this operation
	if op.Workflow == "" {
		return nil, fmt.Errorf("no workflow defined for operation")
	}

	// Extract workflow name from the operation
	var workflowName string
	if workflowMap, ok := op.Workflow.(map[string]interface{}); ok {
		if name, ok := workflowMap["name"].(string); ok {
			workflowName = name
		}
	} else if name, ok := op.Workflow.(string); ok {
		workflowName = name
	}

	if workflowName == "" {
		return nil, fmt.Errorf("workflow name not found in operation")
	}

	logging.Info("CapabilityAdapter", "Executing workflow %s for capability %s.%s", workflowName, capabilityType, operation)

	// Call the workflow through the workflow executor
	result, err := a.workflowExecutor.CallToolInternal(ctx, fmt.Sprintf("action_%s", workflowName), params)
	if err != nil {
		return nil, fmt.Errorf("workflow execution failed: %w", err)
	}

	// Convert MCP result to API result
	content := make([]interface{}, len(result.Content))
	for i, c := range result.Content {
		content[i] = c
	}

	return &api.CallToolResult{
		Content: content,
		IsError: result.IsError,
	}, nil
}

// IsCapabilityAvailable checks if a capability operation is available
func (a *Adapter) IsCapabilityAvailable(capabilityType, operation string) bool {
	toolName := fmt.Sprintf("api_%s_%s", capabilityType, operation)

	availableTools := a.loader.GetAvailableCapabilityTools()
	for _, tool := range availableTools {
		if tool == toolName {
			return true
		}
	}

	return false
}

// ListCapabilities returns information about all available capabilities
func (a *Adapter) ListCapabilities() []api.CapabilityInfo {
	// Map to group operations by capability
	capMap := make(map[string]*api.CapabilityInfo)

	// Get all capability definitions
	for _, toolName := range a.loader.GetAvailableCapabilityTools() {
		op, def, err := a.loader.GetOperationForTool(toolName)
		if err != nil {
			continue
		}

		// Get or create capability info
		capKey := def.Name
		capInfo, exists := capMap[capKey]
		if !exists {
			capInfo = &api.CapabilityInfo{
				Type:        def.Type,
				Name:        def.Name,
				Description: def.Description,
				Version:     def.Version,
				Operations:  []api.OperationInfo{},
			}
			capMap[capKey] = capInfo
		}

		// Extract operation name from the definition
		var opName string
		// Get the workflow name from the operation
		var opWorkflowName string
		if workflowMap, ok := op.Workflow.(map[string]interface{}); ok {
			if name, ok := workflowMap["name"].(string); ok {
				opWorkflowName = name
			}
		} else if workflowName, ok := op.Workflow.(string); ok {
			opWorkflowName = workflowName
		}

		// Find the matching operation by comparing workflow names
		for name, operation := range def.Operations {
			var defWorkflowName string
			if workflowDef, ok := operation.Workflow.(map[string]interface{}); ok {
				if wfName, ok := workflowDef["name"].(string); ok {
					defWorkflowName = wfName
				}
			} else if workflowName, ok := operation.Workflow.(string); ok {
				defWorkflowName = workflowName
			}

			if defWorkflowName == opWorkflowName {
				opName = name
				break
			}
		}

		// Check if this operation is already added (deduplication)
		operationExists := false
		for _, existingOp := range capInfo.Operations {
			if existingOp.Name == opName {
				operationExists = true
				break
			}
		}

		// Only add operation if it doesn't already exist
		if !operationExists {
			capInfo.Operations = append(capInfo.Operations, api.OperationInfo{
				Name:        opName,
				Description: op.Description,
				Available:   true,
			})
		}
	}

	// Convert map to slice
	capInfos := make([]api.CapabilityInfo, 0, len(capMap))
	for _, capInfo := range capMap {
		capInfos = append(capInfos, *capInfo)
	}

	return capInfos
}

// LoadDefinitions loads capability definitions from the configured path
func (a *Adapter) LoadDefinitions() error {
	return a.loader.LoadDefinitions()
}

// ReloadDefinitions reloads capability definitions from disk
func (a *Adapter) ReloadDefinitions() error {
	// Get the singleton registry and clear it
	registry := GetRegistry()

	// Lock the registry while clearing
	registry.mu.Lock()
	registry.capabilities = make(map[string]*Capability)
	registry.byType = make(map[CapabilityType][]*Capability)
	registry.byProvider = make(map[string][]*Capability)
	registry.mu.Unlock()

	// Reload from disk
	return a.loader.LoadDefinitions()
}

// GetTools returns all tools this provider offers
func (a *Adapter) GetTools() []api.ToolMetadata {
	tools := []api.ToolMetadata{
		// Capability management tools
		{
			Name:        "capability_list",
			Description: "List all capabilities",
		},
		{
			Name:        "capability_info",
			Description: "Get capability information",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "type",
					Type:        "string",
					Required:    true,
					Description: "Capability type (e.g., 'auth')",
				},
			},
		},
		{
			Name:        "capability_check",
			Description: "Check if a capability operation is available",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "type",
					Type:        "string",
					Required:    true,
					Description: "Capability type",
				},
				{
					Name:        "operation",
					Type:        "string",
					Required:    true,
					Description: "Operation name",
				},
			},
		},
		{
			Name:        "capability_create",
			Description: "Create a new capability definition from YAML",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "yaml_definition",
					Type:        "string",
					Required:    true,
					Description: "YAML capability definition",
				},
				{
					Name:        "filename",
					Type:        "string",
					Required:    false,
					Description: "Optional filename for the capability definition",
				},
			},
		},
		{
			Name:        "capability_update",
			Description: "Update an existing capability definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the capability to update",
				},
				{
					Name:        "yaml_definition",
					Type:        "string",
					Required:    true,
					Description: "Updated YAML capability definition",
				},
			},
		},
		{
			Name:        "capability_delete",
			Description: "Delete a capability definition and cleanup resources",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the capability to delete",
				},
				{
					Name:        "force",
					Type:        "boolean",
					Required:    false,
					Description: "Force deletion without confirmation",
				},
			},
		},
		{
			Name:        "capability_validate",
			Description: "Validate a capability definition syntax",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "yaml_definition",
					Type:        "string",
					Required:    true,
					Description: "YAML capability definition to validate",
				},
			},
		},
	}

	// Add tools for each capability operation
	capabilities := a.ListCapabilities()
	for _, cap := range capabilities {
		for _, op := range cap.Operations {
			if op.Available {
				// Tool name is just type_operation (e.g., "auth_login")
				toolName := fmt.Sprintf("%s_%s", cap.Type, op.Name)

				// Get the operation definition to extract parameters
				opDef, _, err := a.loader.GetOperationForTool(fmt.Sprintf("api_%s", toolName))
				if err != nil {
					continue
				}

				// Convert operation parameters to tool parameters
				var params []api.ParameterMetadata
				for paramName, param := range opDef.Parameters {
					params = append(params, api.ParameterMetadata{
						Name:        paramName,
						Type:        param.Type,
						Required:    param.Required,
						Description: param.Description,
					})
				}

				tools = append(tools, api.ToolMetadata{
					Name:        toolName,
					Description: op.Description,
					Parameters:  params,
				})
			}
		}
	}

	return tools
}

// ExecuteTool executes a tool by name
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "capability_list":
		return a.handleList()
	case "capability_info":
		return a.handleInfo(args)
	case "capability_check":
		return a.handleCheck(args)
	case "capability_create":
		return a.handleCreate(args)
	case "capability_update":
		return a.handleUpdate(args)
	case "capability_delete":
		return a.handleDelete(args)
	case "capability_validate":
		return a.handleValidate(args)
	default:
		// Try to parse as capability operation (e.g., "auth_login")
		parts := strings.SplitN(toolName, "_", 2)
		if len(parts) == 2 {
			return a.ExecuteCapability(ctx, parts[0], parts[1], args)
		}
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Helper methods for handling management operations
func (a *Adapter) handleList() (*api.CallToolResult, error) {
	capabilities := a.ListCapabilities()

	result := map[string]interface{}{
		"capabilities": capabilities,
		"total":        len(capabilities),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleInfo(args map[string]interface{}) (*api.CallToolResult, error) {
	capType, ok := args["type"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"type is required"},
			IsError: true,
		}, nil
	}

	// Find the capability in the list
	capabilities := a.ListCapabilities()
	for _, cap := range capabilities {
		if cap.Type == capType {
			return &api.CallToolResult{
				Content: []interface{}{cap},
				IsError: false,
			}, nil
		}
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Capability type '%s' not found", capType)},
		IsError: true,
	}, nil
}

func (a *Adapter) handleCheck(args map[string]interface{}) (*api.CallToolResult, error) {
	capType, ok := args["type"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"type is required"},
			IsError: true,
		}, nil
	}

	operation, ok := args["operation"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"operation is required"},
			IsError: true,
		}, nil
	}

	available := a.IsCapabilityAvailable(capType, operation)

	result := map[string]interface{}{
		"type":      capType,
		"operation": operation,
		"available": available,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

// GetLoader returns the capability loader (for aggregator to use as tool checker)
func (a *Adapter) GetLoader() *CapabilityLoader {
	return a.loader
}

// CRUD operation handlers

func (a *Adapter) handleCreate(args map[string]interface{}) (*api.CallToolResult, error) {
	yamlDef, ok := args["yaml_definition"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"yaml_definition is required"},
			IsError: true,
		}, nil
	}

	filename, _ := args["filename"].(string)

	// Validate the capability definition first
	if err := a.validateCapabilityYAML(yamlDef); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Invalid capability definition: %s", err.Error())},
			IsError: true,
		}, nil
	}

	// Create the capability definition
	if err := a.createCapabilityDefinition(yamlDef, filename); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to create capability: %s", err.Error())},
			IsError: true,
		}, nil
	}

	// Reload definitions to pick up the new capability
	if err := a.loader.LoadDefinitions(); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to reload definitions: %s", err.Error())},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"Capability created successfully"},
		IsError: false,
	}, nil
}

func (a *Adapter) handleUpdate(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	yamlDef, ok := args["yaml_definition"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"yaml_definition is required"},
			IsError: true,
		}, nil
	}

	// Check if capability exists
	if _, exists := a.loader.GetCapabilityDefinition(name); !exists {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Capability '%s' not found", name)},
			IsError: true,
		}, nil
	}

	// Validate the updated capability definition
	if err := a.validateCapabilityYAML(yamlDef); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Invalid capability definition: %s", err.Error())},
			IsError: true,
		}, nil
	}

	// Update the capability definition
	if err := a.updateCapabilityDefinition(name, yamlDef); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update capability: %s", err.Error())},
			IsError: true,
		}, nil
	}

	// Reload definitions to pick up the changes
	if err := a.loader.LoadDefinitions(); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to reload definitions: %s", err.Error())},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Capability '%s' updated successfully", name)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleDelete(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	force, _ := args["force"].(bool)

	// Check if capability exists
	if _, exists := a.loader.GetCapabilityDefinition(name); !exists {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Capability '%s' not found", name)},
			IsError: true,
		}, nil
	}

	// Delete the capability definition
	if err := a.deleteCapabilityDefinition(name, force); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete capability: %s", err.Error())},
			IsError: true,
		}, nil
	}

	// Reload definitions to remove the capability from memory
	if err := a.loader.LoadDefinitions(); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to reload definitions: %s", err.Error())},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Capability '%s' deleted successfully", name)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleValidate(args map[string]interface{}) (*api.CallToolResult, error) {
	yamlDef, ok := args["yaml_definition"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"yaml_definition is required"},
			IsError: true,
		}, nil
	}

	// Validate the capability definition
	if err := a.validateCapabilityYAML(yamlDef); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Validation failed: %s", err.Error())},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"Capability definition is valid"},
		IsError: false,
	}, nil
}

// Helper methods for CRUD operations

func (a *Adapter) validateCapabilityYAML(yamlContent string) error {
	var def CapabilityDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if def.Name == "" {
		return fmt.Errorf("capability name is required")
	}
	if def.Type == "" {
		return fmt.Errorf("capability type is required")
	}
	if len(def.Operations) == 0 {
		return fmt.Errorf("at least one operation is required")
	}

	// Validate capability type
	if !IsValidCapabilityType(def.Type) {
		return fmt.Errorf("invalid capability type: %s", def.Type)
	}

	return nil
}

func (a *Adapter) createCapabilityDefinition(yamlContent, filename string) error {
	// Parse the YAML to get the capability name
	var def CapabilityDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Generate filename if not provided
	if filename == "" {
		filename = fmt.Sprintf("%s.yaml", def.Name)
	}

	// Write to the definitions directory
	filePath := filepath.Join(a.loader.definitionsPath, filename)
	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write capability file: %w", err)
	}

	return nil
}

func (a *Adapter) updateCapabilityDefinition(name, yamlContent string) error {
	// Find the existing file for this capability
	files, err := filepath.Glob(filepath.Join(a.loader.definitionsPath, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to list capability files: %w", err)
	}

	var targetFile string
	for _, file := range files {
		def, err := a.loader.loadDefinitionFile(file)
		if err != nil {
			continue
		}
		if def.Name == name {
			targetFile = file
			break
		}
	}

	if targetFile == "" {
		return fmt.Errorf("capability file for '%s' not found", name)
	}

	// Update the file
	if err := os.WriteFile(targetFile, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to update capability file: %w", err)
	}

	return nil
}

func (a *Adapter) deleteCapabilityDefinition(name string, force bool) error {
	// Find the existing file for this capability
	files, err := filepath.Glob(filepath.Join(a.loader.definitionsPath, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to list capability files: %w", err)
	}

	var targetFile string
	for _, file := range files {
		def, err := a.loader.loadDefinitionFile(file)
		if err != nil {
			continue
		}
		if def.Name == name {
			targetFile = file
			break
		}
	}

	if targetFile == "" {
		return fmt.Errorf("capability file for '%s' not found", name)
	}

	// Delete the file
	if err := os.Remove(targetFile); err != nil {
		return fmt.Errorf("failed to delete capability file: %w", err)
	}

	return nil
}
