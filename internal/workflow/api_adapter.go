package workflow

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

// Adapter adapts the workflow system to implement api.WorkflowHandler
type Adapter struct {
	manager *WorkflowManager
}

// NewAdapter creates a new API adapter for the workflow manager
func NewAdapter(manager *WorkflowManager, toolCaller api.ToolCaller) *Adapter {
	return &Adapter{
		manager: manager,
	}
}

// Register registers this adapter with the central API layer
func (a *Adapter) Register() {
	api.RegisterWorkflow(a)
	logging.Info("WorkflowAdapter", "Registered workflow adapter with API")
}

// SetToolCaller sets the ToolCaller on the underlying workflow manager
func (a *Adapter) SetToolCaller(toolCaller interface{}) {
	if a.manager != nil {
		// Type assert to workflow.ToolCaller
		if wfToolCaller, ok := toolCaller.(ToolCaller); ok {
			a.manager.SetToolCaller(wfToolCaller)
			logging.Debug("WorkflowAdapter", "Set ToolCaller on workflow manager")
		} else {
			logging.Warn("WorkflowAdapter", "Provided toolCaller does not implement workflow.ToolCaller interface")
		}
	}
}

// ExecuteWorkflow executes a workflow
func (a *Adapter) ExecuteWorkflow(ctx context.Context, workflowName string, args map[string]interface{}) (*api.CallToolResult, error) {
	logging.Info("WorkflowAdapter", "Executing workflow through API: %s", workflowName)

	// Execute workflow through manager
	result, err := a.manager.ExecuteWorkflow(ctx, workflowName, args)
	if err != nil {
		return nil, err
	}

	// Convert mcp.CallToolResult to api.CallToolResult
	content := make([]interface{}, len(result.Content))
	for i, c := range result.Content {
		if textContent, ok := c.(mcp.TextContent); ok {
			content[i] = textContent.Text
		} else {
			content[i] = c
		}
	}

	return &api.CallToolResult{
		Content: content,
		IsError: result.IsError,
	}, nil
}

// GetWorkflows returns information about all workflows
func (a *Adapter) GetWorkflows() []api.WorkflowInfo {
	workflows := a.manager.ListDefinitions()
	infos := make([]api.WorkflowInfo, 0, len(workflows))

	for _, wf := range workflows {
		infos = append(infos, api.WorkflowInfo{
			Name:        wf.Name,
			Description: wf.Description,
			Version:     strconv.Itoa(wf.Version),
		})
	}

	return infos
}

// GetWorkflow returns a specific workflow definition
func (a *Adapter) GetWorkflow(name string) (*api.WorkflowDefinition, error) {
	workflow, exists := a.manager.GetDefinition(name)
	if !exists {
		return nil, api.NewWorkflowNotFoundError(name)
	}

	// Convert workflow.WorkflowConfig to api.WorkflowDefinition
	steps := make([]api.WorkflowStep, len(workflow.Steps))
	for i, step := range workflow.Steps {
		steps[i] = api.WorkflowStep{
			ID:    step.ID,
			Tool:  step.Tool,
			Args:  step.Args,
			Store: step.Store,
			// Note: WorkflowStep doesn't have Condition or Description fields
		}
	}

	// Convert InputSchema
	inputSchema := make(map[string]interface{})
	if workflow.InputSchema.Type != "" {
		inputSchema["type"] = workflow.InputSchema.Type
	}
	if len(workflow.InputSchema.Properties) > 0 {
		props := make(map[string]interface{})
		for name, prop := range workflow.InputSchema.Properties {
			propMap := map[string]interface{}{
				"type":        prop.Type,
				"description": prop.Description,
			}
			if prop.Default != nil {
				propMap["default"] = prop.Default
			}
			props[name] = propMap
		}
		inputSchema["properties"] = props
	}
	if len(workflow.InputSchema.Required) > 0 {
		inputSchema["required"] = workflow.InputSchema.Required
	}

	return &api.WorkflowDefinition{
		Name:        workflow.Name,
		Description: workflow.Description,
		Version:     strconv.Itoa(workflow.Version),
		InputSchema: inputSchema,
		Steps:       steps,
		// Note: WorkflowDefinition doesn't have OutputSchema
	}, nil
}

// CreateWorkflowFromStructured creates a new workflow from structured parameters
func (a *Adapter) CreateWorkflowFromStructured(args map[string]interface{}) error {
	// Convert structured parameters to WorkflowDefinition
	wf, err := convertToWorkflowDefinition(args)
	if err != nil {
		return err
	}

	return a.manager.CreateWorkflow(wf)
}

// UpdateWorkflowFromStructured updates an existing workflow from structured parameters
func (a *Adapter) UpdateWorkflowFromStructured(name string, args map[string]interface{}) error {
	// Convert structured parameters to WorkflowDefinition
	wf, err := convertToWorkflowDefinition(args)
	if err != nil {
		return err
	}

	return a.manager.UpdateWorkflow(name, wf)
}

// ValidateWorkflowFromStructured validates a workflow definition from structured parameters
func (a *Adapter) ValidateWorkflowFromStructured(args map[string]interface{}) error {
	// Convert structured parameters to validate structure
	wf, err := convertToWorkflowDefinition(args)
	if err != nil {
		return err
	}

	// Basic validation
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(wf.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	return nil
}

// DeleteWorkflow deletes a workflow
func (a *Adapter) DeleteWorkflow(name string) error {
	return a.manager.DeleteWorkflow(name)
}

// CallToolInternal calls a tool internally - required by ToolCaller interface
func (a *Adapter) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("workflow manager not available")
	}

	// Delegate to the manager's tool caller (which should be the API-based one)
	return a.manager.executor.toolCaller.CallToolInternal(ctx, toolName, args)
}

// Stop stops the workflow adapter
func (a *Adapter) Stop() {
	if a.manager != nil {
		a.manager.Stop()
	}
}

// ReloadWorkflows reloads workflow definitions from disk
func (a *Adapter) ReloadWorkflows() error {
	if a.manager != nil {
		return a.manager.LoadDefinitions()
	}
	return nil
}

// GetTools returns all tools this provider offers
func (a *Adapter) GetTools() []api.ToolMetadata {
	tools := []api.ToolMetadata{
		// Workflow management tools
		{
			Name:        "workflow_list",
			Description: "List all workflows",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "include_system",
					Type:        "boolean",
					Required:    false,
					Description: "Include system-defined workflows",
					Default:     true,
				},
			},
		},
		{
			Name:        "workflow_get",
			Description: "Get workflow details",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the workflow",
				},
			},
		},
		{
			Name:        "workflow_create",
			Description: "Create a new workflow",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Workflow name"},
				{Name: "description", Type: "string", Required: false, Description: "Workflow description"},
				{Name: "icon", Type: "string", Required: false, Description: "Icon/emoji for display"},
				{Name: "agentModifiable", Type: "boolean", Required: false, Description: "Whether workflow can be modified by agents"},
				{Name: "createdBy", Type: "string", Required: false, Description: "Creator of the workflow"},
				{Name: "version", Type: "number", Required: false, Description: "Workflow version"},
				{Name: "inputSchema", Type: "object", Required: true, Description: "Input schema for the workflow"},
				{Name: "steps", Type: "array", Required: true, Description: "Array of workflow steps"},
			},
		},
		{
			Name:        "workflow_update",
			Description: "Update an existing workflow",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the workflow to update"},
				{Name: "description", Type: "string", Required: false, Description: "Workflow description"},
				{Name: "icon", Type: "string", Required: false, Description: "Icon/emoji for display"},
				{Name: "agentModifiable", Type: "boolean", Required: false, Description: "Whether workflow can be modified by agents"},
				{Name: "createdBy", Type: "string", Required: false, Description: "Creator of the workflow"},
				{Name: "version", Type: "number", Required: false, Description: "Workflow version"},
				{Name: "inputSchema", Type: "object", Required: true, Description: "Input schema for the workflow"},
				{Name: "steps", Type: "array", Required: true, Description: "Array of workflow steps"},
			},
		},
		{
			Name:        "workflow_delete",
			Description: "Delete a workflow",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the workflow to delete",
				},
			},
		},
		{
			Name:        "workflow_validate",
			Description: "Validate a workflow definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Workflow name"},
				{Name: "description", Type: "string", Required: false, Description: "Workflow description"},
				{Name: "version", Type: "string", Required: false, Description: "Workflow version"},
				{Name: "inputSchema", Type: "object", Required: true, Description: "Input schema for the workflow"},
				{Name: "steps", Type: "array", Required: true, Description: "Array of workflow steps"},
			},
		},

	}

	// Add a tool for each workflow
	workflows := a.GetWorkflows()
	for _, wf := range workflows {
		tools = append(tools, api.ToolMetadata{
			Name:        fmt.Sprintf("action_%s", wf.Name),
			Description: wf.Description,
			Parameters:  a.convertWorkflowParameters(wf.Name),
		})
	}

	return tools
}

// ExecuteTool executes a tool by name
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch {
	case toolName == "workflow_list":
		return a.handleList(args)
	case toolName == "workflow_get":
		return a.handleGet(args)
	case toolName == "workflow_create":
		return a.handleCreate(args)
	case toolName == "workflow_update":
		return a.handleUpdate(args)
	case toolName == "workflow_delete":
		return a.handleDelete(args)
	case toolName == "workflow_validate":
		return a.handleValidate(args)

	case strings.HasPrefix(toolName, "action_"):
		// Execute workflow
		workflowName := strings.TrimPrefix(toolName, "action_")
		return a.ExecuteWorkflow(ctx, workflowName, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// convertWorkflowParameters converts workflow input schema to parameter metadata
func (a *Adapter) convertWorkflowParameters(workflowName string) []api.ParameterMetadata {
	workflow, err := a.GetWorkflow(workflowName)
	if err != nil {
		return nil
	}

	var params []api.ParameterMetadata

	// Extract properties from input schema
	if workflow.InputSchema != nil {
		if props, ok := workflow.InputSchema["properties"].(map[string]interface{}); ok {
			for name, prop := range props {
				param := api.ParameterMetadata{
					Name: name,
				}

				if propMap, ok := prop.(map[string]interface{}); ok {
					if t, ok := propMap["type"].(string); ok {
						param.Type = t
					}
					if desc, ok := propMap["description"].(string); ok {
						param.Description = desc
					}
					if def, ok := propMap["default"]; ok {
						param.Default = def
					}
				}

				// Check if required
				if required, ok := workflow.InputSchema["required"].([]interface{}); ok {
					for _, req := range required {
						if reqStr, ok := req.(string); ok && reqStr == name {
							param.Required = true
							break
						}
					}
				}

				params = append(params, param)
			}
		}
	}

	return params
}

// Helper methods for handling management operations
func (a *Adapter) handleList(args map[string]interface{}) (*api.CallToolResult, error) {
	workflows := a.GetWorkflows()

	// Filter by include_system if provided
	// includeSystem := true
	// if val, ok := args["include_system"].(bool); ok {
	// 	includeSystem = val
	// }
	// TODO: implement system workflow filtering when WorkflowInfo has system flag

	var result []map[string]interface{}
	for _, wf := range workflows {
		// Skip system workflows if not requested
		// (would need to add system flag to WorkflowInfo)
		workflowInfo := map[string]interface{}{
			"name":        wf.Name,
			"description": wf.Description,
			"version":     wf.Version,
		}
		result = append(result, workflowInfo)
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleGet(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	workflow, err := a.GetWorkflow(name)
	if err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to get workflow"), nil
	}

	// Convert to YAML for easier viewing
	yamlData, err := yaml.Marshal(workflow)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to marshal workflow: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"workflow": workflow,
		"yaml":     string(yamlData),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleCreate(args map[string]interface{}) (*api.CallToolResult, error) {
	if err := a.CreateWorkflowFromStructured(args); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to create workflow: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"Workflow created successfully"},
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

	if err := a.UpdateWorkflowFromStructured(name, args); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to update workflow"), nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Workflow '%s' updated successfully", name)},
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

	if err := a.DeleteWorkflow(name); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to delete workflow"), nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Workflow '%s' deleted successfully", name)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleValidate(args map[string]interface{}) (*api.CallToolResult, error) {
	if err := a.ValidateWorkflowFromStructured(args); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Validation failed: %v", err)},
			IsError: true,
		}, nil
	}

	// Get workflow name for consistent response format
	name, _ := args["name"].(string)
	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Validation successful for workflow %s", name)},
		IsError: false,
	}, nil
}



// convertToWorkflowDefinition converts structured parameters to WorkflowDefinition
func convertToWorkflowDefinition(args map[string]interface{}) (WorkflowDefinition, error) {
	var wf WorkflowDefinition

	// Required fields
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return wf, fmt.Errorf("name parameter is required")
	}
	wf.Name = name

	// Optional fields
	if desc, ok := args["description"].(string); ok {
		wf.Description = desc
	}
	if version, ok := args["version"].(string); ok {
		// Convert string version to int for WorkflowDefinition
		if version != "" {
			// For now, just store as 1 if version is provided, 0 otherwise
			// This maintains compatibility while using string input
			wf.Version = 1
		}
	}

	// Convert inputSchema
	if inputSchemaParam, ok := args["inputSchema"].(map[string]interface{}); ok {
		inputSchema, err := convertInputSchema(inputSchemaParam)
		if err != nil {
			return wf, fmt.Errorf("invalid inputSchema: %v", err)
		}
		wf.InputSchema = inputSchema
	} else {
		return wf, fmt.Errorf("inputSchema parameter is required")
	}

	// Convert steps
	if stepsParam, ok := args["steps"].([]interface{}); ok {
		steps, err := convertWorkflowSteps(stepsParam)
		if err != nil {
			return wf, fmt.Errorf("invalid steps: %v", err)
		}
		wf.Steps = steps
	} else {
		return wf, fmt.Errorf("steps parameter is required")
	}

	// Set timestamps
	wf.CreatedAt = time.Now()
	wf.LastModified = time.Now()

	return wf, nil
}

// convertInputSchema converts a map[string]interface{} to WorkflowInputSchema
func convertInputSchema(schemaParam map[string]interface{}) (WorkflowInputSchema, error) {
	var schema WorkflowInputSchema

	// Type field
	if schemaType, ok := schemaParam["type"].(string); ok {
		schema.Type = schemaType
	}

	// Properties field
	if props, ok := schemaParam["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]SchemaProperty)
		for propName, propData := range props {
			propMap, ok := propData.(map[string]interface{})
			if !ok {
				return schema, fmt.Errorf("property '%s' must be an object", propName)
			}

			var prop SchemaProperty
			if propType, ok := propMap["type"].(string); ok {
				prop.Type = propType
			}
			if description, ok := propMap["description"].(string); ok {
				prop.Description = description
			}
			if defaultValue, ok := propMap["default"]; ok {
				prop.Default = defaultValue
			}

			schema.Properties[propName] = prop
		}
	}

	// Required field
	if required, ok := schemaParam["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required[i] = reqStr
			} else {
				return schema, fmt.Errorf("required field at index %d must be a string", i)
			}
		}
	}

	return schema, nil
}

// convertWorkflowSteps converts a []interface{} to []WorkflowStep
func convertWorkflowSteps(stepsParam []interface{}) ([]WorkflowStep, error) {
	steps := make([]WorkflowStep, len(stepsParam))

	for i, stepData := range stepsParam {
		stepMap, ok := stepData.(map[string]interface{})
		if !ok {
			return steps, fmt.Errorf("step at index %d must be an object", i)
		}

		var step WorkflowStep

		// Required fields
		if id, ok := stepMap["id"].(string); ok {
			step.ID = id
		} else {
			return steps, fmt.Errorf("step at index %d missing required 'id' field", i)
		}

		if tool, ok := stepMap["tool"].(string); ok {
			step.Tool = tool
		} else {
			return steps, fmt.Errorf("step at index %d missing required 'tool' field", i)
		}

		// Optional fields
		if args, ok := stepMap["args"].(map[string]interface{}); ok {
			step.Args = args
		}
		if store, ok := stepMap["store"].(string); ok {
			step.Store = store
		}

		steps[i] = step
	}

	return steps, nil
}
