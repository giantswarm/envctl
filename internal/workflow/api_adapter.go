package workflow

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

// Adapter adapts the workflow system to implement api.WorkflowHandler
type Adapter struct {
	manager    *WorkflowManager
	toolCaller api.ToolCaller
}

// NewAdapter creates a new workflow adapter
func NewAdapter(configDir string, toolCaller api.ToolCaller) (*Adapter, error) {
	// Create workflow manager
	manager, err := NewWorkflowManager(configDir, toolCaller)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow manager: %w", err)
	}

	return &Adapter{
		manager:    manager,
		toolCaller: toolCaller,
	}, nil
}

// Register registers this adapter with the API
func (a *Adapter) Register() {
	api.RegisterWorkflow(a)
	logging.Info("WorkflowAdapter", "Registered workflow adapter with API")
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
	storage := a.manager.GetStorage()
	workflows := storage.ListWorkflows()
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
	storage := a.manager.GetStorage()
	workflow, err := storage.GetWorkflow(name)
	if err != nil {
		return nil, err
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

// CreateWorkflow creates a new workflow from YAML
func (a *Adapter) CreateWorkflow(yamlStr string) error {
	// Parse YAML to workflow definition
	var wf WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &wf); err != nil {
		return fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	storage := a.manager.GetStorage()
	return storage.CreateWorkflow(wf)
}

// UpdateWorkflow updates an existing workflow
func (a *Adapter) UpdateWorkflow(name, yamlStr string) error {
	// Parse YAML to workflow definition
	var wf WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &wf); err != nil {
		return fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	storage := a.manager.GetStorage()
	return storage.UpdateWorkflow(name, wf)
}

// DeleteWorkflow deletes a workflow
func (a *Adapter) DeleteWorkflow(name string) error {
	storage := a.manager.GetStorage()
	return storage.DeleteWorkflow(name)
}

// ValidateWorkflow validates a workflow YAML
func (a *Adapter) ValidateWorkflow(yamlStr string) error {
	// Parse YAML to validate structure
	var wf WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &wf); err != nil {
		return fmt.Errorf("invalid workflow YAML: %w", err)
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

// CallToolInternal calls a tool internally
func (a *Adapter) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Delegate to the tool caller
	return a.toolCaller.CallToolInternal(ctx, toolName, args)
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
		// Get the storage and reload workflows
		storage := a.manager.GetStorage()
		return storage.LoadWorkflows()
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
				{
					Name:        "yaml_definition",
					Type:        "string",
					Required:    true,
					Description: "YAML definition of the workflow",
				},
			},
		},
		{
			Name:        "workflow_update",
			Description: "Update an existing workflow",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the workflow to update",
				},
				{
					Name:        "yaml_definition",
					Type:        "string",
					Required:    true,
					Description: "Updated YAML definition",
				},
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
			Description: "Validate a workflow YAML definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "yaml_definition",
					Type:        "string",
					Required:    true,
					Description: "YAML definition to validate",
				},
			},
		},
		{
			Name:        "workflow_spec",
			Description: "Get the workflow specification and examples",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "format",
					Type:        "string",
					Required:    false,
					Description: "Format of response: 'schema', 'template', 'examples', or 'full'",
					Default:     "full",
				},
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
	case toolName == "workflow_spec":
		return a.handleSpec(args)
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
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get workflow: %v", err)},
			IsError: true,
		}, nil
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
	yamlDef, ok := args["yaml_definition"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"yaml_definition is required"},
			IsError: true,
		}, nil
	}

	if err := a.CreateWorkflow(yamlDef); err != nil {
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

	yamlDef, ok := args["yaml_definition"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"yaml_definition is required"},
			IsError: true,
		}, nil
	}

	if err := a.UpdateWorkflow(name, yamlDef); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update workflow: %v", err)},
			IsError: true,
		}, nil
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
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete workflow: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Workflow '%s' deleted successfully", name)},
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

	if err := a.ValidateWorkflow(yamlDef); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Validation failed: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"Workflow definition is valid"},
		IsError: false,
	}, nil
}

func (a *Adapter) handleSpec(args map[string]interface{}) (*api.CallToolResult, error) {
	format := "full"
	if f, ok := args["format"].(string); ok {
		format = f
	}

	// Return workflow specification based on format
	spec := map[string]interface{}{
		"description": "Workflows are defined in YAML format",
	}

	// Add components based on format
	switch format {
	case "schema":
		spec["schema"] = getWorkflowSchema()
	case "template":
		spec["template"] = getWorkflowTemplate()
	case "examples":
		spec["examples"] = getWorkflowExamples()
	case "full":
		spec["schema"] = getWorkflowSchema()
		spec["template"] = getWorkflowTemplate()
		spec["examples"] = getWorkflowExamples()
	}

	return &api.CallToolResult{
		Content: []interface{}{spec},
		IsError: false,
	}, nil
}

// Workflow specification helpers
func getWorkflowSchema() map[string]interface{} {
	return map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"required":    true,
			"description": "Unique workflow identifier",
		},
		"description": map[string]interface{}{
			"type":        "string",
			"required":    true,
			"description": "Human-readable description",
		},
		"inputSchema": map[string]interface{}{
			"type":        "object",
			"required":    true,
			"description": "JSON Schema defining workflow inputs",
		},
		"steps": map[string]interface{}{
			"type":        "array",
			"required":    true,
			"description": "Sequential list of tool calls",
		},
	}
}

func getWorkflowTemplate() string {
	return `name: my_workflow
description: "Clear description of what this workflow does"
inputSchema:
  type: object
  properties:
    param1:
      type: string
      description: "Description of parameter 1"
  required:
    - param1
steps:
  - id: step1
    tool: some_tool
    args:
      arg1: "{{ .input.param1 }}"`
}

func getWorkflowExamples() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "simple_connection",
			"description": "Basic example of connecting to a cluster",
			"yaml": `name: connect_cluster
description: "Connect to a Teleport cluster"
inputSchema:
  type: object
  properties:
    cluster:
      type: string
      description: "Cluster name"
  required:
    - cluster
steps:
  - id: login
    tool: teleport_kube
    args:
      command: "login"
      cluster: "{{ .input.cluster }}"`,
		},
	}
}
