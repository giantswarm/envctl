package workflow

import (
	"context"
	"encoding/json"
	"fmt"

	"envctl/internal/config"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

// ManagementTools provides MCP tools for workflow CRUD operations
type ManagementTools struct {
	storage *WorkflowStorage
}

// NewManagementTools creates management tools for workflows
func NewManagementTools(storage *WorkflowStorage) *ManagementTools {
	return &ManagementTools{
		storage: storage,
	}
}

// GetManagementTools returns all workflow management tools
func (mt *ManagementTools) GetManagementTools() []mcp.Tool {
	return []mcp.Tool{
		mt.listWorkflowsTool(),
		mt.getWorkflowTool(),
		mt.createWorkflowTool(),
		mt.updateWorkflowTool(),
		mt.deleteWorkflowTool(),
		mt.validateWorkflowTool(),
		mt.getWorkflowSpecTool(),
	}
}

// listWorkflowsTool returns the tool definition for listing workflows
func (mt *ManagementTools) listWorkflowsTool() mcp.Tool {
	return mcp.NewTool("workflow_list",
		mcp.WithDescription("List all available workflows"),
		mcp.WithBoolean("include_system",
			mcp.Description("Include system (non-modifiable) workflows"),
			mcp.DefaultBool(true),
		),
	)
}

// getWorkflowTool returns the tool definition for getting a workflow
func (mt *ManagementTools) getWorkflowTool() mcp.Tool {
	return mcp.NewTool("workflow_get",
		mcp.WithDescription("Get details of a specific workflow"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the workflow to retrieve"),
		),
	)
}

// createWorkflowTool returns the tool definition for creating workflows
func (mt *ManagementTools) createWorkflowTool() mcp.Tool {
	return mcp.NewTool("workflow_create",
		mcp.WithDescription("Create a new workflow"),
		mcp.WithString("yaml_definition",
			mcp.Required(),
			mcp.Description("YAML definition of the workflow"),
		),
		mcp.WithBoolean("validate_only",
			mcp.Description("Only validate the workflow without creating it"),
			mcp.DefaultBool(false),
		),
	)
}

// updateWorkflowTool returns the tool definition for updating workflows
func (mt *ManagementTools) updateWorkflowTool() mcp.Tool {
	return mcp.NewTool("workflow_update",
		mcp.WithDescription("Update an existing workflow (only agent-modifiable workflows)"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the workflow to update"),
		),
		mcp.WithString("yaml_definition",
			mcp.Required(),
			mcp.Description("Updated YAML definition of the workflow"),
		),
		mcp.WithBoolean("validate_only",
			mcp.Description("Only validate the workflow without updating it"),
			mcp.DefaultBool(false),
		),
	)
}

// deleteWorkflowTool returns the tool definition for deleting workflows
func (mt *ManagementTools) deleteWorkflowTool() mcp.Tool {
	return mcp.NewTool("workflow_delete",
		mcp.WithDescription("Delete a workflow (only agent-created workflows)"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the workflow to delete"),
		),
	)
}

// validateWorkflowTool returns the tool definition for validating workflows
func (mt *ManagementTools) validateWorkflowTool() mcp.Tool {
	return mcp.NewTool("workflow_validate",
		mcp.WithDescription("Validate a workflow definition without creating it"),
		mcp.WithString("yaml_definition",
			mcp.Required(),
			mcp.Description("YAML definition of the workflow to validate"),
		),
	)
}

// getWorkflowSpecTool returns the tool definition for getting workflow specification
func (mt *ManagementTools) getWorkflowSpecTool() mcp.Tool {
	return mcp.NewTool("workflow_spec",
		mcp.WithDescription("Get the workflow YAML specification, schema, and examples"),
		mcp.WithString("format",
			mcp.Description("Format of response: 'schema', 'template', 'examples', or 'full' (default: full)"),
			mcp.Enum("schema", "template", "examples", "full"),
		),
	)
}

// HandleListWorkflows handles the list workflows tool call
func (mt *ManagementTools) HandleListWorkflows(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeSystem := req.GetBool("include_system", true)

	workflows := mt.storage.ListWorkflows()
	var result []map[string]interface{}

	for _, wf := range workflows {
		// Skip non-modifiable workflows if not requested
		if !includeSystem && !wf.AgentModifiable {
			continue
		}

		workflowInfo := map[string]interface{}{
			"name":            wf.Name,
			"description":     wf.Description,
			"agentModifiable": wf.AgentModifiable,
			"createdBy":       wf.CreatedBy,
			"version":         wf.Version,
			"inputSchema":     wf.InputSchema,
			"stepCount":       len(wf.Steps),
		}

		if !wf.CreatedAt.IsZero() {
			workflowInfo["createdAt"] = wf.CreatedAt.Format("2006-01-02T15:04:05Z")
		}
		if !wf.LastModified.IsZero() {
			workflowInfo["lastModified"] = wf.LastModified.Format("2006-01-02T15:04:05Z")
		}

		result = append(result, workflowInfo)
	}

	resultJSON, _ := json.MarshalIndent(map[string]interface{}{
		"workflows": result,
		"total":     len(result),
	}, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleGetWorkflow handles the get workflow tool call
func (mt *ManagementTools) HandleGetWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	workflow, err := mt.storage.GetWorkflow(name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get workflow: %v", err)), nil
	}

	// Convert to YAML for easier editing
	yamlData, err := yaml.Marshal(workflow)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal workflow: %v", err)), nil
	}

	result := map[string]interface{}{
		"workflow": workflow,
		"yaml":     string(yamlData),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleCreateWorkflow handles the create workflow tool call
func (mt *ManagementTools) HandleCreateWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	yamlDef, err := req.RequireString("yaml_definition")
	if err != nil {
		return mcp.NewToolResultError("yaml_definition is required"), nil
	}

	validateOnly := req.GetBool("validate_only", false)

	// Parse YAML
	var workflow config.WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlDef), &workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid YAML: %v", err)), nil
	}

	// Validate workflow
	if err := mt.validateWorkflowDefinition(&workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Validation failed: %v", err)), nil
	}

	if validateOnly {
		return mcp.NewToolResultText("Workflow definition is valid"), nil
	}

	// Create workflow
	if err := mt.storage.CreateWorkflow(workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create workflow: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created workflow '%s'", workflow.Name)), nil
}

// HandleUpdateWorkflow handles the update workflow tool call
func (mt *ManagementTools) HandleUpdateWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	yamlDef, err := req.RequireString("yaml_definition")
	if err != nil {
		return mcp.NewToolResultError("yaml_definition is required"), nil
	}

	validateOnly := req.GetBool("validate_only", false)

	// Parse YAML
	var workflow config.WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlDef), &workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid YAML: %v", err)), nil
	}

	// Validate workflow
	if err := mt.validateWorkflowDefinition(&workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Validation failed: %v", err)), nil
	}

	if validateOnly {
		return mcp.NewToolResultText("Workflow definition is valid"), nil
	}

	// Update workflow
	if err := mt.storage.UpdateWorkflow(name, workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update workflow: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully updated workflow '%s'", name)), nil
}

// HandleDeleteWorkflow handles the delete workflow tool call
func (mt *ManagementTools) HandleDeleteWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := mt.storage.DeleteWorkflow(name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete workflow: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted workflow '%s'", name)), nil
}

// HandleValidateWorkflow handles the validate workflow tool call
func (mt *ManagementTools) HandleValidateWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	yamlDef, err := req.RequireString("yaml_definition")
	if err != nil {
		return mcp.NewToolResultError("yaml_definition is required"), nil
	}

	// Parse YAML
	var workflow config.WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlDef), &workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid YAML: %v", err)), nil
	}

	// Validate workflow
	if err := mt.validateWorkflowDefinition(&workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Validation failed: %v", err)), nil
	}

	return mcp.NewToolResultText("Workflow definition is valid"), nil
}

// HandleGetWorkflowSpec handles the get workflow spec tool call
func (mt *ManagementTools) HandleGetWorkflowSpec(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	format := req.GetString("format", "full")

	spec := map[string]interface{}{}

	// Always include description
	spec["description"] = "Workflows are defined in YAML format with the following structure"

	// Add components based on format
	switch format {
	case "schema":
		spec["schema"] = mt.getWorkflowSchema()
	case "template":
		spec["template"] = mt.getWorkflowTemplate()
	case "examples":
		spec["examples"] = mt.getWorkflowExamples()
	case "full":
		spec["schema"] = mt.getWorkflowSchema()
		spec["template"] = mt.getWorkflowTemplate()
		spec["examples"] = mt.getWorkflowExamples()
		spec["template_syntax"] = mt.getTemplateSyntax()
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid format: %s", format)), nil
	}

	resultJSON, _ := json.MarshalIndent(spec, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// getWorkflowSchema returns the workflow YAML schema
func (mt *ManagementTools) getWorkflowSchema() map[string]interface{} {
	return map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"required":    true,
			"pattern":     "^[a-zA-Z][a-zA-Z0-9_]*$",
			"description": "Unique workflow identifier (alphanumeric with underscores, must start with letter)",
		},
		"description": map[string]interface{}{
			"type":        "string",
			"required":    true,
			"description": "Human-readable description of the workflow's purpose",
		},
		"icon": map[string]interface{}{
			"type":        "string",
			"required":    false,
			"description": "Optional emoji icon for UI display",
			"example":     "🚀",
		},
		"agentModifiable": map[string]interface{}{
			"type":        "boolean",
			"required":    false,
			"default":     true,
			"description": "Whether agents can modify this workflow. Agent-created workflows are always modifiable.",
		},
		"inputSchema": map[string]interface{}{
			"type":        "object",
			"required":    true,
			"description": "JSON Schema defining workflow inputs",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Always 'object' for workflow inputs",
					"value":       "object",
				},
				"properties": map[string]interface{}{
					"type":        "object",
					"description": "Map of input parameter names to their schemas",
					"example": map[string]interface{}{
						"cluster": map[string]interface{}{
							"type":        "string",
							"description": "Cluster name",
						},
					},
				},
				"required": map[string]interface{}{
					"type":        "array",
					"description": "Array of required parameter names",
					"example":     []string{"cluster"},
				},
			},
		},
		"steps": map[string]interface{}{
			"type":        "array",
			"required":    true,
			"description": "Sequential list of tool calls to execute",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"required":    true,
						"description": "Unique step identifier within the workflow",
						"example":     "login_step",
					},
					"tool": map[string]interface{}{
						"type":        "string",
						"required":    true,
						"description": "Name of MCP tool to call (must exist in aggregator)",
						"example":     "teleport_kube",
					},
					"args": map[string]interface{}{
						"type":        "object",
						"required":    true,
						"description": "Arguments for the tool call (supports template variables)",
						"example": map[string]interface{}{
							"command": "login",
							"cluster": "{{ .input.cluster }}",
						},
					},
					"store": map[string]interface{}{
						"type":        "string",
						"required":    false,
						"description": "Optional key to store the step result for use in later steps",
						"example":     "login_result",
					},
				},
			},
		},
	}
}

// getWorkflowTemplate returns a minimal workflow template
func (mt *ManagementTools) getWorkflowTemplate() string {
	return `name: my_workflow
description: "Clear description of what this workflow does"
agentModifiable: true
inputSchema:
  type: object
  properties:
    param1:
      type: string
      description: "Description of parameter 1"
    param2:
      type: number
      description: "Description of parameter 2"
      default: 42
  required:
    - param1
steps:
  - id: step1
    tool: some_tool
    args:
      arg1: "{{ .input.param1 }}"
      arg2: "static value"
    store: step1_result
  - id: step2
    tool: another_tool
    args:
      data: "{{ .results.step1_result }}"
      param: "{{ .input.param2 }}"`
}

// getWorkflowExamples returns example workflows
func (mt *ManagementTools) getWorkflowExamples() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "simple_connection",
			"description": "Basic example of connecting to a cluster",
			"yaml": `name: connect_cluster
description: "Connect to a Teleport cluster and get context"
agentModifiable: false
inputSchema:
  type: object
  properties:
    cluster:
      type: string
      description: "Full cluster name to connect to"
  required:
    - cluster
steps:
  - id: login
    tool: teleport_kube
    args:
      command: "login"
      cluster: "{{ .input.cluster }}"
  - id: get_context
    tool: kubectl_context
    args:
      operation: "get"
    store: "current_context"`,
		},
		{
			"name":        "monitoring_setup",
			"description": "Example with multiple port forwards",
			"yaml": `name: setup_monitoring
description: "Set up port forwarding for Prometheus and Grafana"
agentModifiable: true
inputSchema:
  type: object
  properties:
    prometheus_port:
      type: number
      description: "Local port for Prometheus"
      default: 9090
    grafana_port:
      type: number
      description: "Local port for Grafana"
      default: 3000
steps:
  - id: forward_prometheus
    tool: port_forward
    args:
      resourceType: "service"
      resourceName: "prometheus-server"
      namespace: "monitoring"
      localPort: "{{ .input.prometheus_port }}"
      targetPort: 9090
    store: prometheus_info
  - id: forward_grafana
    tool: port_forward
    args:
      resourceType: "service"
      resourceName: "grafana"
      namespace: "monitoring"
      localPort: "{{ .input.grafana_port }}"
      targetPort: 3000
    store: grafana_info`,
		},
	}
}

// getTemplateSyntax returns template syntax documentation
func (mt *ManagementTools) getTemplateSyntax() map[string]interface{} {
	return map[string]interface{}{
		"description": "Workflow arguments support Go template syntax for variable substitution",
		"variables": map[string]interface{}{
			".input": map[string]interface{}{
				"description": "Access input parameters passed to the workflow",
				"example":     "{{ .input.cluster }}",
			},
			".results": map[string]interface{}{
				"description": "Access stored results from previous steps",
				"example":     "{{ .results.step1_result }}",
			},
		},
		"rules": []string{
			"Template expressions must be enclosed in {{ }}",
			"Only string values can contain templates",
			"Templates are resolved before each step execution",
			"If a template resolves to JSON, it will be parsed to preserve types",
			"Missing variables will cause the workflow to fail",
		},
		"examples": []map[string]string{
			{"template": "{{ .input.param }}", "description": "Access input parameter 'param'"},
			{"template": "{{ .results.step1 }}", "description": "Access result stored by step with store='step1'"},
			{"template": "prefix-{{ .input.name }}-suffix", "description": "Combine static text with variables"},
		},
	}
}

// validateWorkflowDefinition validates a workflow definition
func (mt *ManagementTools) validateWorkflowDefinition(workflow *config.WorkflowDefinition) error {
	// Check required fields
	if workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if workflow.Description == "" {
		return fmt.Errorf("workflow description is required")
	}
	if len(workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	// Validate input schema
	if workflow.InputSchema.Type == "" {
		workflow.InputSchema.Type = "object"
	}
	if workflow.InputSchema.Properties == nil {
		workflow.InputSchema.Properties = make(map[string]config.SchemaProperty)
	}

	// Validate each step
	stepIDs := make(map[string]bool)
	for i, step := range workflow.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: ID is required", i+1)
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("step %d: duplicate ID '%s'", i+1, step.ID)
		}
		stepIDs[step.ID] = true

		if step.Tool == "" {
			return fmt.Errorf("step %d (%s): tool is required", i+1, step.ID)
		}

		// Validate template variables in args reference valid sources
		if err := mt.validateStepArgs(step, workflow.InputSchema.Properties, stepIDs); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.ID, err)
		}
	}

	return nil
}

// validateStepArgs validates that step arguments only reference valid variables
func (mt *ManagementTools) validateStepArgs(step config.WorkflowStep, inputProps map[string]config.SchemaProperty, previousSteps map[string]bool) error {
	// This is a simplified validation - in production you'd want to parse
	// the template strings and validate variable references
	for key, value := range step.Args {
		if str, ok := value.(string); ok {
			// Check for template syntax
			if len(str) >= 4 && str[:2] == "{{" && str[len(str)-2:] == "}}" {
				// Basic validation - just check it's not empty
				content := str[2 : len(str)-2]
				if len(content) == 0 {
					return fmt.Errorf("empty template in argument '%s'", key)
				}
			}
		}
	}
	return nil
}
