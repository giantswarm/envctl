package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// OutputFormat represents the output format for CLI commands
type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatYAML  OutputFormat = "yaml"
)

// ExecutorOptions contains options for tool execution
type ExecutorOptions struct {
	Format OutputFormat
	Quiet  bool
}

// ToolExecutor provides high-level tool execution functionality
type ToolExecutor struct {
	client  *CLIClient
	options ExecutorOptions
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(options ExecutorOptions) (*ToolExecutor, error) {
	// Check if server is running first
	if err := CheckServerRunning(); err != nil {
		return nil, err
	}

	client, err := NewCLIClient()
	if err != nil {
		return nil, err
	}

	return &ToolExecutor{
		client:  client,
		options: options,
	}, nil
}

// Connect establishes connection to the aggregator
func (e *ToolExecutor) Connect(ctx context.Context) error {
	return e.client.Connect(ctx)
}

// Close closes the connection
func (e *ToolExecutor) Close() error {
	return e.client.Close()
}

// Execute executes a tool and formats the output according to the specified format
func (e *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]interface{}) error {
	result, err := e.client.CallTool(ctx, toolName, args)
	if err != nil {
		return err
	}

	if result.IsError {
		return e.formatError(result)
	}

	return e.formatOutput(result)
}

// ExecuteSimple executes a tool and returns the result as a string
func (e *ToolExecutor) ExecuteSimple(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	return e.client.CallToolSimple(ctx, toolName, args)
}

// ExecuteJSON executes a tool and returns the result as parsed JSON
func (e *ToolExecutor) ExecuteJSON(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	return e.client.CallToolJSON(ctx, toolName, args)
}

// formatOutput formats the tool result according to the output format
func (e *ToolExecutor) formatOutput(result *mcp.CallToolResult) error {
	var content []string
	for _, c := range result.Content {
		if textContent, ok := mcp.AsTextContent(c); ok {
			content = append(content, textContent.Text)
		}
	}

	if len(content) == 0 {
		if !e.options.Quiet {
			fmt.Println("No output")
		}
		return nil
	}

	output := strings.Join(content, "\n")

	switch e.options.Format {
	case OutputFormatJSON:
		// Try to parse and pretty-print JSON
		var jsonData interface{}
		if err := json.Unmarshal([]byte(output), &jsonData); err == nil {
			prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
			if err == nil {
				fmt.Println(string(prettyJSON))
				return nil
			}
		}
		// If not JSON, print as-is
		fmt.Println(output)

	case OutputFormatYAML:
		// For now, just print as-is. Could add YAML formatting later
		fmt.Println(output)

	case OutputFormatTable:
		fallthrough
	default:
		// Try to format as table if it's JSON array
		if e.tryFormatAsTable(output) {
			return nil
		}
		// Otherwise print as-is
		fmt.Println(output)
	}

	return nil
}

// formatError formats error output
func (e *ToolExecutor) formatError(result *mcp.CallToolResult) error {
	var errorMsgs []string
	for _, content := range result.Content {
		if textContent, ok := mcp.AsTextContent(content); ok {
			errorMsgs = append(errorMsgs, textContent.Text)
		}
	}

	errorMsg := strings.Join(errorMsgs, "\n")
	fmt.Fprintf(os.Stderr, "Error: %s\n", errorMsg)
	return fmt.Errorf("%s", errorMsg)
}

// tryFormatAsTable attempts to format JSON output as a table
func (e *ToolExecutor) tryFormatAsTable(output string) bool {
	var jsonData interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
		return false
	}

	// Check if it's an array of objects (suitable for table format)
	if array, ok := jsonData.([]interface{}); ok && len(array) > 0 {
		if obj, ok := array[0].(map[string]interface{}); ok {
			e.formatJSONAsTable(array, obj)
			return true
		}
	}

	// Check if it's a single object
	if obj, ok := jsonData.(map[string]interface{}); ok {
		e.formatSingleObjectAsTable(obj)
		return true
	}

	return false
}

// formatJSONAsTable formats an array of JSON objects as a table
func (e *ToolExecutor) formatJSONAsTable(array []interface{}, firstObj map[string]interface{}) {
	// Get column headers from the first object
	var headers []string
	for key := range firstObj {
		headers = append(headers, key)
	}

	// Print headers
	fmt.Println(strings.Join(headers, "\t"))

	// Print separator
	var separators []string
	for range headers {
		separators = append(separators, "---")
	}
	fmt.Println(strings.Join(separators, "\t"))

	// Print rows
	for _, item := range array {
		if obj, ok := item.(map[string]interface{}); ok {
			var values []string
			for _, header := range headers {
				if val, exists := obj[header]; exists {
					values = append(values, fmt.Sprintf("%v", val))
				} else {
					values = append(values, "")
				}
			}
			fmt.Println(strings.Join(values, "\t"))
		}
	}
}

// formatSingleObjectAsTable formats a single JSON object as a key-value table
func (e *ToolExecutor) formatSingleObjectAsTable(obj map[string]interface{}) {
	fmt.Println("Key\tValue")
	fmt.Println("---\t---")
	for key, value := range obj {
		fmt.Printf("%s\t%v\n", key, value)
	}
} 