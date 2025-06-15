package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
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

// Execute executes a tool and formats the output
func (e *ToolExecutor) Execute(ctx context.Context, toolName string, arguments map[string]interface{}) error {
	result, err := e.client.CallTool(ctx, toolName, arguments)
	if err != nil {
		return fmt.Errorf("failed to execute tool %s: %w", toolName, err)
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

// formatOutput formats the tool output according to the specified format
func (e *ToolExecutor) formatOutput(result *mcp.CallToolResult) error {
	if len(result.Content) == 0 {
		if !e.options.Quiet {
			fmt.Println("No results")
		}
		return nil
	}

	content := result.Content[0]
	textContent, ok := mcp.AsTextContent(content)
	if !ok {
		return fmt.Errorf("content is not text")
	}
	
	switch e.options.Format {
	case OutputFormatJSON:
		fmt.Println(textContent.Text)
		return nil
	case OutputFormatYAML:
		return e.outputYAML(textContent.Text)
	case OutputFormatTable:
		return e.outputTable(textContent.Text)
	default:
		return fmt.Errorf("unsupported output format: %s", e.options.Format)
	}
}

// outputYAML converts JSON to YAML and prints it
func (e *ToolExecutor) outputYAML(jsonData string) error {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to convert to YAML: %w", err)
	}

	fmt.Print(string(yamlData))
	return nil
}

// outputTable formats data as a professional table
func (e *ToolExecutor) outputTable(jsonData string) error {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		fmt.Println(jsonData) // Fallback to raw text if not JSON
		return nil
	}

	switch d := data.(type) {
	case map[string]interface{}:
		return e.formatTableFromObject(d)
	case []interface{}:
		return e.formatTableFromArray(d)
	default:
		// Simple value, just print it
		fmt.Println(jsonData)
		return nil
	}
}

// formatTableFromObject handles object data that might contain arrays
func (e *ToolExecutor) formatTableFromObject(data map[string]interface{}) error {
	// Check for common wrapper patterns like {"services": [...], "total": N}
	arrayKey := e.findArrayKey(data)
	if arrayKey != "" {
		if arr, ok := data[arrayKey].([]interface{}); ok {
			if err := e.formatTableFromArray(arr); err != nil {
				return err
			}
			// Show total if available
			if total, ok := data["total"]; ok {
				fmt.Printf("\n%s %v %s\n", 
					text.FgHiBlue.Sprint("Total:"), 
					text.FgHiWhite.Sprint(total),
					e.pluralize(arrayKey))
			}
			return nil
		}
	}

	// No array found, format as key-value pairs
	return e.formatKeyValueTable(data)
}

// findArrayKey looks for common array keys in wrapped objects
func (e *ToolExecutor) findArrayKey(data map[string]interface{}) string {
	arrayKeys := []string{"services", "serviceClasses", "mcpServers", "workflows", "capabilities", "items", "results"}
	
	for _, key := range arrayKeys {
		if value, exists := data[key]; exists {
			if _, isArray := value.([]interface{}); isArray {
				return key
			}
		}
	}
	return ""
}

// formatTableFromArray creates a table from an array of objects
func (e *ToolExecutor) formatTableFromArray(data []interface{}) error {
	if len(data) == 0 {
		fmt.Println(text.FgYellow.Sprint("No items found"))
		return nil
	}

	// Get the first object to determine columns
	firstObj, ok := data[0].(map[string]interface{})
	if !ok {
		// Array of simple values
		return e.formatSimpleList(data)
	}

	// Determine table type and optimize columns
	columns := e.optimizeColumns(firstObj)
	
	// Create professional table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	
	// Set headers with colors
	headers := make([]interface{}, len(columns))
	for i, col := range columns {
		headers[i] = text.FgHiCyan.Sprint(strings.ToUpper(col))
	}
	t.AppendHeader(headers)

	// Add rows with enhanced formatting
	for _, item := range data {
		if itemMap, ok := item.(map[string]interface{}); ok {
			row := make([]interface{}, len(columns))
			for i, col := range columns {
				row[i] = e.formatCellValue(col, itemMap[col])
			}
			t.AppendRow(row)
		}
	}

	t.Render()
	return nil
}

// optimizeColumns determines the best columns to show based on the data type
func (e *ToolExecutor) optimizeColumns(sample map[string]interface{}) []string {
	// Extract all available keys
	var allKeys []string
	for key := range sample {
		allKeys = append(allKeys, key)
	}
	sort.Strings(allKeys)

	// Define priority columns for different resource types
	priorityColumns := map[string][]string{
		"services":       {"label", "health", "state", "service_type", "metadata"},
		"serviceClasses": {"name", "available", "type", "description", "requiredTools"},
		"mcpServers":     {"name", "available", "type", "description", "requiredTools"},
		"workflows":      {"name", "status", "description", "steps"},
		"capabilities":   {"name", "available", "type", "description"},
	}

	// Detect resource type and use optimized columns
	resourceType := e.detectResourceType(sample)
	if priorities, exists := priorityColumns[resourceType]; exists {
		var columns []string
		// Add priority columns that exist
		for _, col := range priorities {
			if e.keyExists(sample, col) {
				columns = append(columns, col)
			}
		}
		// For serviceClasses, be more conservative to avoid wrapping
		if resourceType == "serviceClasses" {
			return columns // Don't add extra columns for serviceClasses
		}
		
		// Add remaining columns (up to reasonable limit) for other types
		remaining := e.getRemainingKeys(allKeys, columns)
		if len(columns) < 6 { // Reasonable column limit
			for _, key := range remaining[:min(3, len(remaining))] {
				columns = append(columns, key)
			}
		}
		return columns
	}

	// Default: use first 5 keys to avoid wrapping
	if len(allKeys) > 5 {
		return allKeys[:5]
	}
	return allKeys
}

// detectResourceType attempts to determine what type of resource this is
func (e *ToolExecutor) detectResourceType(sample map[string]interface{}) string {
	// Look for distinctive fields
	if e.keyExists(sample, "health") && e.keyExists(sample, "service_type") {
		return "services"
	}
	if e.keyExists(sample, "serviceType") && e.keyExists(sample, "requiredTools") {
		return "serviceClasses"
	}
	if e.keyExists(sample, "serverType") || e.keyExists(sample, "serverCommand") {
		return "mcpServers"
	}
	if e.keyExists(sample, "steps") || e.keyExists(sample, "workflow") {
		return "workflows"
	}
	if e.keyExists(sample, "capabilityType") {
		return "capabilities"
	}
	return "generic"
}

// formatCellValue formats individual cell values with appropriate styling
func (e *ToolExecutor) formatCellValue(column string, value interface{}) interface{} {
	if value == nil {
		return text.FgHiBlack.Sprint("-")
	}

	strValue := fmt.Sprintf("%v", value)
	
	// Handle different column types
	switch strings.ToLower(column) {
	case "health", "status":
		return e.formatHealthStatus(strValue)
	case "available":
		return e.formatAvailableStatus(value)
	case "state":
		return e.formatState(strValue)
	case "metadata":
		return e.formatMetadata(value)
	case "requiredtools", "tools":
		return e.formatToolsList(value)
	case "description":
		return e.formatDescription(strValue)
	case "type", "service_type", "servicetype":
		return e.formatType(strValue)
	default:
		// Default formatting for strings
		if len(strValue) > 30 {
			return strValue[:27] + "..."
		}
		return strValue
	}
}

// formatHealthStatus adds color coding to health status
func (e *ToolExecutor) formatHealthStatus(status string) interface{} {
	switch strings.ToLower(status) {
	case "healthy":
		return text.FgGreen.Sprint("✅ " + status)
	case "unhealthy":
		return text.FgRed.Sprint("❌ " + status)
	case "warning":
		return text.FgYellow.Sprint("⚠️  " + status)
	case "running":
		return text.FgGreen.Sprint("🟢 " + status)
	case "stopped":
		return text.FgRed.Sprint("🔴 " + status)
	case "starting":
		return text.FgYellow.Sprint("🟡 " + status)
	default:
		return status
	}
}

// formatAvailableStatus formats boolean availability
func (e *ToolExecutor) formatAvailableStatus(value interface{}) interface{} {
	switch v := value.(type) {
	case bool:
		if v {
			return text.FgGreen.Sprint("✅ Available")
		}
		return text.FgRed.Sprint("❌ Unavailable")
	case string:
		if v == "true" {
			return text.FgGreen.Sprint("✅ Available")
		}
		return text.FgRed.Sprint("❌ Unavailable")
	default:
		return fmt.Sprintf("%v", value)
	}
}

// formatState formats service state with icons
func (e *ToolExecutor) formatState(state string) interface{} {
	switch strings.ToLower(state) {
	case "running":
		return text.FgGreen.Sprint("▶️  Running")
	case "stopped":
		return text.FgRed.Sprint("⏹️  Stopped")
	case "starting":
		return text.FgYellow.Sprint("⏳ Starting")
	case "stopping":
		return text.FgYellow.Sprint("⏸️  Stopping")
	default:
		return state
	}
}

// formatMetadata extracts useful information from metadata objects
func (e *ToolExecutor) formatMetadata(value interface{}) interface{} {
	if value == nil {
		return text.FgHiBlack.Sprint("-")
	}

	// Handle metadata object
	if metaMap, ok := value.(map[string]interface{}); ok {
		var parts []string
		
		// Extract icon if available
		if icon, exists := metaMap["icon"]; exists && icon != nil {
			parts = append(parts, fmt.Sprintf("%v", icon))
		}
		
		// Extract type
		if typ, exists := metaMap["type"]; exists && typ != nil {
			parts = append(parts, fmt.Sprintf("%v", typ))
		}
		
		// Extract enabled status
		if enabled, exists := metaMap["enabled"]; exists {
			if enabledBool, ok := enabled.(bool); ok {
				if enabledBool {
					parts = append(parts, text.FgGreen.Sprint("enabled"))
				} else {
					parts = append(parts, text.FgRed.Sprint("disabled"))
				}
			}
		}
		
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}
	
	return text.FgHiBlack.Sprint("[metadata]")
}

// formatToolsList shows actual tool names instead of "[N items]"
func (e *ToolExecutor) formatToolsList(value interface{}) interface{} {
	if value == nil {
		return text.FgHiBlack.Sprint("-")
	}

	if toolsArray, ok := value.([]interface{}); ok {
		if len(toolsArray) == 0 {
			return text.FgHiBlack.Sprint("none")
		}
		
		var toolNames []string
		for _, tool := range toolsArray {
			if toolStr, ok := tool.(string); ok {
				// Simplify tool names for display
				simplified := e.simplifyToolName(toolStr)
				toolNames = append(toolNames, simplified)
			}
		}
		
		if len(toolNames) <= 2 {
			return strings.Join(toolNames, ", ")
		} else {
			// Show first 2 and count
			return fmt.Sprintf("%s, %s (+%d more)", 
				toolNames[0], toolNames[1], len(toolNames)-2)
		}
	}
	
	return fmt.Sprintf("%v", value)
}

// simplifyToolName removes common prefixes to make tool names more readable
func (e *ToolExecutor) simplifyToolName(toolName string) string {
	// Remove common prefixes
	prefixes := []string{"x_kubernetes_", "x_", "core_", "mcp_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(toolName, prefix) {
			return strings.TrimPrefix(toolName, prefix)
		}
	}
	return toolName
}

// formatDescription truncates long descriptions appropriately
func (e *ToolExecutor) formatDescription(desc string) interface{} {
	if len(desc) <= 50 {
		return desc
	}
	return desc[:45] + text.FgHiBlack.Sprint("...")
}

// formatType adds subtle styling to types
func (e *ToolExecutor) formatType(typ string) interface{} {
	return text.FgCyan.Sprint(typ)
}

// formatKeyValueTable formats an object as key-value pairs
func (e *ToolExecutor) formatKeyValueTable(data map[string]interface{}) error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{
		text.FgHiCyan.Sprint("PROPERTY"), 
		text.FgHiCyan.Sprint("VALUE"),
	})

	// Sort keys for consistent output
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := e.formatCellValue(key, data[key])
		t.AppendRow(table.Row{
			text.FgYellow.Sprint(key),
			value,
		})
	}

	t.Render()
	return nil
}

// formatSimpleList formats an array of simple values
func (e *ToolExecutor) formatSimpleList(data []interface{}) error {
	for _, item := range data {
		fmt.Println(item)
	}
	return nil
}

// Helper functions
func (e *ToolExecutor) keyExists(data map[string]interface{}, key string) bool {
	_, exists := data[key]
	return exists
}

func (e *ToolExecutor) getRemainingKeys(allKeys, usedKeys []string) []string {
	usedSet := make(map[string]bool)
	for _, key := range usedKeys {
		usedSet[key] = true
	}
	
	var remaining []string
	for _, key := range allKeys {
		if !usedSet[key] {
			remaining = append(remaining, key)
		}
	}
	return remaining
}

func (e *ToolExecutor) pluralize(word string) string {
	if strings.HasSuffix(word, "s") {
		return word
	}
	return word + "s"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
