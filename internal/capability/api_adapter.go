package capability

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/workflow"
	"envctl/pkg/logging"
	"fmt"
	"strings"
)

// Adapter adapts the capability system to implement api.CapabilityHandler
type Adapter struct {
	loader           *CapabilityLoader
	workflowExecutor workflow.ToolCaller
}

// NewAdapter creates a new capability adapter
func NewAdapter(definitionsPath string, toolChecker ToolAvailabilityChecker, workflowExecutor workflow.ToolCaller) *Adapter {
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
	// Build the tool name from capability type and operation
	toolName := fmt.Sprintf("x_%s_%s", capabilityType, operation)

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
	toolName := fmt.Sprintf("x_%s_%s", capabilityType, operation)

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

		// Add operation to capability
		capInfo.Operations = append(capInfo.Operations, api.OperationInfo{
			Name:        opName,
			Description: op.Description,
			Available:   true,
		})
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
					Description: "Capability type (e.g., 'auth_provider')",
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
	}

	// Add tools for each capability operation
	capabilities := a.ListCapabilities()
	for _, cap := range capabilities {
		for _, op := range cap.Operations {
			if op.Available {
				// Tool name is just type_operation (e.g., "auth_login")
				toolName := fmt.Sprintf("%s_%s", cap.Type, op.Name)

				// Get the operation definition to extract parameters
				opDef, _, err := a.loader.GetOperationForTool(fmt.Sprintf("x_%s", toolName))
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
