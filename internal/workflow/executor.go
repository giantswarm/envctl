package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"envctl/internal/api"
	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

// ToolCaller interface - what we need from the aggregator
type ToolCaller interface {
	CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error)
}

// WorkflowExecutor executes workflow steps
type WorkflowExecutor struct {
	toolCaller ToolCaller
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(toolCaller ToolCaller) *WorkflowExecutor {
	return &WorkflowExecutor{
		toolCaller: toolCaller,
	}
}

// ExecuteWorkflow executes a workflow with the given arguments
func (we *WorkflowExecutor) ExecuteWorkflow(ctx context.Context, workflow *api.Workflow, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logging.Error("WorkflowExecutor", fmt.Errorf("workflow execution started"), "ExecuteWorkflow called with workflow=%s, args=%+v, required=%+v", workflow.Name, args, workflow.InputSchema.Required)
	logging.Debug("WorkflowExecutor", "Executing workflow %s", workflow.Name)

	// Validate inputs against schema
	if err := we.validateInputs(workflow.InputSchema, args); err != nil {
		logging.Error("WorkflowExecutor", err, "Input validation failed for workflow %s", workflow.Name)
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Create execution context with initial variables
	execCtx := &executionContext{
		input:     args,
		variables: make(map[string]interface{}),
		results:   make(map[string]interface{}),
	}

	// Execute each step
	for _, step := range workflow.Steps {
		logging.Debug("WorkflowExecutor", "Executing step %s, tool: %s", step.ID, step.Tool)

		// Resolve template variables in arguments
		resolvedArgs, err := we.resolveArguments(step.Args, execCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve arguments for step %s: %w", step.ID, err)
		}

		// Execute the tool
		result, err := we.toolCaller.CallToolInternal(ctx, step.Tool, resolvedArgs)
		if err != nil {
			return nil, fmt.Errorf("step %s failed: %w", step.ID, err)
		}

		// Store result if requested
		if step.Store != "" {
			logging.Debug("WorkflowExecutor", "Processing result for step %s: %+v", step.ID, result)
			var resultData interface{}
			if len(result.Content) > 0 {
				logging.Debug("WorkflowExecutor", "Result content[0]: %+v (type: %T)", result.Content[0], result.Content[0])
				// Check if it's a TextContent
				if textContent, ok := result.Content[0].(mcp.TextContent); ok {
					logging.Debug("WorkflowExecutor", "TextContent.Text: %s", textContent.Text)
					// Try to parse as JSON first
					if err := json.Unmarshal([]byte(textContent.Text), &resultData); err != nil {
						logging.Debug("WorkflowExecutor", "Failed to parse as JSON: %v, storing as string", err)
						// If not JSON, store as string
						resultData = textContent.Text
					} else {
						logging.Debug("WorkflowExecutor", "Successfully parsed JSON: %+v", resultData)
					}
				}
			}
			execCtx.results[step.Store] = resultData
			logging.Debug("WorkflowExecutor", "Stored result from step %s as %s: %+v", step.ID, step.Store, resultData)
		}

		// Check if result indicates an error
		if result.IsError {
			// Return the error result immediately
			return result, nil
		}
	}

	// Return the final result
	finalResult := map[string]interface{}{
		"workflow": workflow.Name,
		"results":  execCtx.results,
		"status":   "completed",
	}

	resultJSON, _ := json.Marshal(finalResult)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// executionContext holds the state during workflow execution
type executionContext struct {
	input     map[string]interface{} // Original input parameters
	variables map[string]interface{} // User-defined variables
	results   map[string]interface{} // Results from previous steps
}

// validateInputs validates the input arguments against the schema
func (we *WorkflowExecutor) validateInputs(schema api.WorkflowInputSchema, args map[string]interface{}) error {
	logging.Debug("WorkflowExecutor", "validateInputs called with args: %+v", args)
	logging.Debug("WorkflowExecutor", "validateInputs schema properties: %+v", schema.Properties)
	
	// Check required fields
	logging.Debug("WorkflowExecutor", "Checking required fields: %+v", schema.Required)
	for _, required := range schema.Required {
		if _, exists := args[required]; !exists {
			logging.Error("WorkflowExecutor", fmt.Errorf("missing required field"), "Required field '%s' is missing from args %+v", required, args)
			return fmt.Errorf("required field '%s' is missing", required)
		}
	}

	// Validate each provided argument
	for key, value := range args {
		prop, exists := schema.Properties[key]
		if !exists {
			// Allow extra properties for flexibility
			continue
		}

		// Basic type validation
		if !we.validateType(value, prop.Type) {
			return fmt.Errorf("field '%s' has wrong type, expected %s", key, prop.Type)
		}
	}

	// Apply defaults for missing optional fields
	for key, prop := range schema.Properties {
		logging.Debug("WorkflowExecutor", "Checking property %s: exists=%v, default=%+v", key, args[key] != nil, prop.Default)
		if _, exists := args[key]; !exists && prop.Default != nil {
			logging.Debug("WorkflowExecutor", "Applying default value for %s: %v", key, prop.Default)
			args[key] = prop.Default
		}
	}
	
	logging.Debug("WorkflowExecutor", "validateInputs final args: %+v", args)
	return nil
}

// validateType performs basic type validation
func (we *WorkflowExecutor) validateType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		switch value.(type) {
		case []interface{}, []string, []int, []float64:
			return true
		default:
			return false
		}
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return true // Unknown types pass validation
	}
}

// resolveArguments resolves template variables in step arguments
func (we *WorkflowExecutor) resolveArguments(args map[string]interface{}, ctx *executionContext) (map[string]interface{}, error) {
	resolved := make(map[string]interface{})

	for key, value := range args {
		resolvedValue, err := we.resolveValue(value, ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to render arguments for argument '%s': %w", key, err)
		}
		resolved[key] = resolvedValue
	}

	return resolved, nil
}

// resolveValue recursively resolves template variables in a value
func (we *WorkflowExecutor) resolveValue(value interface{}, ctx *executionContext) (interface{}, error) {
	switch v := value.(type) {
	case string:
		// Check if it's a template
		if len(v) >= 4 && v[:2] == "{{" && v[len(v)-2:] == "}}" {
			return we.resolveTemplate(v, ctx)
		}
		return v, nil

	case map[string]interface{}:
		// Recursively resolve map values
		resolved := make(map[string]interface{})
		for k, val := range v {
			resolvedVal, err := we.resolveValue(val, ctx)
			if err != nil {
				return nil, err
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil

	case []interface{}:
		// Recursively resolve array elements
		resolved := make([]interface{}, len(v))
		for i, val := range v {
			resolvedVal, err := we.resolveValue(val, ctx)
			if err != nil {
				return nil, err
			}
			resolved[i] = resolvedVal
		}
		return resolved, nil

	default:
		// Return other types as-is
		return value, nil
	}
}

// resolveTemplate resolves a template string
func (we *WorkflowExecutor) resolveTemplate(templateStr string, ctx *executionContext) (interface{}, error) {
	// Create template context
	templateCtx := map[string]interface{}{
		"input":   ctx.input,
		"vars":    ctx.variables,
		"results": ctx.results,
	}

	// Parse and execute template with strict mode options
	tmpl, err := template.New("arg").Option("missingkey=error").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateCtx); err != nil {
		// Check for missing key errors and provide more context
		if strings.Contains(err.Error(), "executing") && strings.Contains(err.Error(), "no such key") {
			return nil, fmt.Errorf("failed to render arguments: template references non-existent variable in %s: %w", templateStr, err)
		}
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	result := buf.String()
	
	// Additional check for "<no value>" which shouldn't happen with missingkey=error but just in case
	if result == "<no value>" {
		return nil, fmt.Errorf("failed to render arguments: template %s produced no value", templateStr)
	}

	// Try to parse as JSON to preserve types
	var jsonValue interface{}
	if err := json.Unmarshal([]byte(result), &jsonValue); err == nil {
		return jsonValue, nil
	}

	// Try YAML as fallback
	var yamlValue interface{}
	if err := yaml.Unmarshal([]byte(result), &yamlValue); err == nil {
		return yamlValue, nil
	}

	// Return as string
	return result, nil
}
