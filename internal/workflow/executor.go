package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

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
func (we *WorkflowExecutor) ExecuteWorkflow(ctx context.Context, workflow *WorkflowDefinition, args map[string]interface{}) (*mcp.CallToolResult, error) {
	logging.Debug("WorkflowExecutor", "Executing workflow %s", workflow.Name)

	// Validate inputs against schema
	if err := we.validateInputs(workflow.InputSchema, args); err != nil {
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
			var resultData interface{}
			if len(result.Content) > 0 {
				// Check if it's a TextContent
				if textContent, ok := result.Content[0].(mcp.TextContent); ok {
					// Try to parse as JSON first
					if err := json.Unmarshal([]byte(textContent.Text), &resultData); err != nil {
						// If not JSON, store as string
						resultData = textContent.Text
					}
				}
			}
			execCtx.results[step.Store] = resultData
			logging.Debug("WorkflowExecutor", "Stored result from step %s as %s", step.ID, step.Store)
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
func (we *WorkflowExecutor) validateInputs(schema WorkflowInputSchema, args map[string]interface{}) error {
	// Check required fields
	for _, required := range schema.Required {
		if _, exists := args[required]; !exists {
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
		if _, exists := args[key]; !exists && prop.Default != nil {
			args[key] = prop.Default
		}
	}

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
			return nil, fmt.Errorf("failed to resolve argument '%s': %w", key, err)
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

	// Parse and execute template
	tmpl, err := template.New("arg").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateCtx); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	result := buf.String()

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
