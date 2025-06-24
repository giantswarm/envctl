package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"envctl/internal/agent"

	"github.com/briandowns/spinner"
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
	client  *agent.Client
	options ExecutorOptions
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(options ExecutorOptions) (*ToolExecutor, error) {
	// Check if server is running first
	if err := CheckServerRunning(); err != nil {
		return nil, err
	}

	client, err := agent.NewCLIClient()
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
	if e.options.Quiet {
		return e.client.Connect(ctx)
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " Connecting to envctl server..."
	s.Start()
	defer s.Stop()

	err := e.client.Connect(ctx)
	if err != nil {
		s.FinalMSG = text.FgRed.Sprint("❌ Failed to connect to envctl server") + "\n"
		return err
	}

	// Remove the success message - connection success is implied by command working
	return nil
}

// Close closes the connection
func (e *ToolExecutor) Close() error {
	return e.client.Close()
}

// Execute executes a tool and formats the output
func (e *ToolExecutor) Execute(ctx context.Context, toolName string, arguments map[string]interface{}) error {
	var s *spinner.Spinner
	if !e.options.Quiet {
		s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Suffix = " Executing command..."
		s.Start()
	}

	result, err := e.client.CallTool(ctx, toolName, arguments)

	if s != nil {
		s.Stop()
	}

	if err != nil {
		if s != nil {
			fmt.Fprintf(os.Stderr, "%s\n", text.FgRed.Sprint("❌ Command failed"))
		}
		return fmt.Errorf("failed to execute tool %s: %w", toolName, err)
	}

	if result.IsError {
		if s != nil {
			fmt.Fprintf(os.Stderr, "%s\n", text.FgRed.Sprint("❌ Command returned error"))
		}
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
			// Just format the array - the summary will be handled by formatTableFromArray
			return e.formatTableFromArray(arr)
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
		fmt.Printf("%s %s\n",
			text.FgYellow.Sprint("📋"),
			text.FgYellow.Sprint("No items found"))
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
	resourceType := e.detectResourceType(firstObj)

	// Create professional table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)

	// Set headers with colors and icons
	headers := make([]interface{}, len(columns))
	for i, col := range columns {
		headers[i] = text.FgHiCyan.Sprint(strings.ToUpper(col))
	}
	t.AppendHeader(headers)

	// Add rows with enhanced formatting - sort by name field if present
	sortedData := e.sortDataByName(data, columns)
	for _, item := range sortedData {
		if itemMap, ok := item.(map[string]interface{}); ok {
			row := make([]interface{}, len(columns))
			for i, col := range columns {
				row[i] = e.formatCellValue(col, itemMap[col])
			}
			t.AppendRow(row)
		}
	}

	t.Render()

	// Add summary line with icon based on resource type
	icon := e.getResourceIcon(resourceType)
	resourceName := e.pluralize(resourceType)
	fmt.Printf("\n%s %s %s %s\n",
		icon,
		text.FgHiBlue.Sprint("Total:"),
		text.FgHiWhite.Sprint(len(data)),
		text.FgHiBlue.Sprint(resourceName))

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

	// Always prioritize name/ID fields first
	nameFields := []string{"name", "label", "id", "workflow", "capability"}
	var columns []string

	// Add the primary identifier first (Name/ID/Label)
	for _, nameField := range nameFields {
		if e.keyExists(sample, nameField) {
			columns = append(columns, nameField)
			break // Only add one primary identifier
		}
	}

	// Define priority columns for different resource types (excluding name fields already added)
	priorityColumns := map[string][]string{
		"services":       {"health", "state", "service_type", "metadata"},
		"serviceClasses": {"available", "serviceType", "description", "requiredTools"},
		"mcpServers":     {"available", "serverType", "description", "requiredTools"},
		"workflows":      {"status", "description", "steps"},
		"capabilities":   {"available", "capabilityType", "description"},
		"generic":        {"status", "type", "description", "available"},
	}

	// Detect resource type and use optimized columns
	resourceType := e.detectResourceType(sample)
	if priorities, exists := priorityColumns[resourceType]; exists {
		// Add priority columns that exist (and haven't been added yet)
		for _, col := range priorities {
			if e.keyExists(sample, col) && !e.containsString(columns, col) {
				columns = append(columns, col)
			}
		}
	}

	// For complex resource types, limit columns to prevent wrapping
	maxColumns := 6
	if resourceType == "serviceClasses" || resourceType == "mcpServers" {
		maxColumns = 5 // More conservative for wider data
	}

	// Add remaining columns alphabetically if we have space
	if len(columns) < maxColumns {
		remaining := e.getRemainingKeys(allKeys, columns)
		spaceLeft := maxColumns - len(columns)
		if spaceLeft > 0 && len(remaining) > 0 {
			addCount := min(spaceLeft, len(remaining))
			columns = append(columns, remaining[:addCount]...)
		}
	}

	return columns
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
	// Check for server-related fields for mcpServers
	if e.keyExists(sample, "serverType") || e.keyExists(sample, "serverCommand") ||
		(e.keyExists(sample, "type") && e.keyExists(sample, "command")) ||
		(e.keyExists(sample, "available") && e.keyExists(sample, "category")) {
		return "mcpServers"
	}
	// Check for workflow-related fields
	if e.keyExists(sample, "steps") || e.keyExists(sample, "workflow") ||
		(e.keyExists(sample, "name") && e.keyExists(sample, "version") && e.keyExists(sample, "description")) {
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
	colLower := strings.ToLower(column)

	// Handle different column types with enhanced formatting
	switch colLower {
	case "name", "label", "id", "workflow", "capability":
		// Primary identifiers - make them prominent
		return text.FgHiCyan.Sprint(strValue)
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
	case "type", "service_type", "servicetype", "servertype", "capabilitytype":
		return e.formatType(strValue)
	case "steps":
		return e.formatSteps(value)
	default:
		// Default formatting - handle arrays and objects better
		if arr, ok := value.([]interface{}); ok {
			return e.formatArray(arr)
		}
		if obj, ok := value.(map[string]interface{}); ok {
			return e.formatObject(obj)
		}
		// Default string truncation
		if len(strValue) > 30 {
			return strValue[:27] + text.FgHiBlack.Sprint("...")
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

// formatSteps formats workflow steps with a count
func (e *ToolExecutor) formatSteps(value interface{}) interface{} {
	if value == nil {
		return text.FgHiBlack.Sprint("-")
	}

	if stepsArray, ok := value.([]interface{}); ok {
		count := len(stepsArray)
		if count == 0 {
			return text.FgHiBlack.Sprint("No steps")
		}
		return text.FgBlue.Sprintf("%d steps", count)
	}

	return fmt.Sprintf("%v", value)
}

// formatArray provides clean display of arrays
func (e *ToolExecutor) formatArray(arr []interface{}) interface{} {
	if len(arr) == 0 {
		return text.FgHiBlack.Sprint("[]")
	}

	// For small arrays, show the items
	if len(arr) <= 2 {
		var items []string
		for _, item := range arr {
			items = append(items, fmt.Sprintf("%v", item))
		}
		return strings.Join(items, ", ")
	}

	// For larger arrays, show count
	return text.FgBlue.Sprintf("[%d items]", len(arr))
}

// formatObject provides clean display of objects
func (e *ToolExecutor) formatObject(obj map[string]interface{}) interface{} {
	if len(obj) == 0 {
		return text.FgHiBlack.Sprint("{}")
	}

	// Look for common display fields
	displayFields := []string{"name", "type", "status", "id"}
	for _, field := range displayFields {
		if value, exists := obj[field]; exists && value != nil {
			return fmt.Sprintf("%v", value)
		}
	}

	// Fallback to indicating it's an object
	return text.FgBlue.Sprintf("{%d fields}", len(obj))
}

// formatKeyValueTable formats an object as key-value pairs
func (e *ToolExecutor) formatKeyValueTable(data map[string]interface{}) error {
	// Check if this is workflow data and handle it specially
	if e.isWorkflowData(data) {
		return e.formatWorkflowDetails(data)
	}

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

// isWorkflowData checks if the data represents a workflow
func (e *ToolExecutor) isWorkflowData(data map[string]interface{}) bool {
	// Check for workflow-specific fields
	if _, hasWorkflow := data["workflow"]; hasWorkflow {
		return true
	}

	// Check if it has workflow-like structure (name, steps, etc.)
	hasName := e.keyExists(data, "name")
	hasSteps := e.keyExists(data, "steps")
	hasInputSchema := e.keyExists(data, "inputschema") || e.keyExists(data, "InputSchema")

	return hasName && (hasSteps || hasInputSchema)
}

// formatWorkflowDetails provides a clean, readable format for workflow data
func (e *ToolExecutor) formatWorkflowDetails(data map[string]interface{}) error {
	// Extract workflow data from the "workflow" field if it exists
	var workflowData map[string]interface{}
	if workflow, exists := data["workflow"]; exists {
		if workflowMap, ok := workflow.(map[string]interface{}); ok {
			workflowData = workflowMap
		}
	} else {
		workflowData = data
	}

	// Create main info table
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{
		text.FgHiCyan.Sprint("PROPERTY"),
		text.FgHiCyan.Sprint("VALUE"),
	})

	// Display basic workflow information
	basicFields := []string{"Name", "name", "Description", "description", "Version", "version"}
	for _, field := range basicFields {
		if value, exists := workflowData[field]; exists && value != nil {
			t.AppendRow(table.Row{
				text.FgYellow.Sprint(strings.ToLower(field)),
				text.FgHiWhite.Sprint(fmt.Sprintf("%v", value)),
			})
		}
	}

	t.Render()

	// Display Input Parameters if they exist
	e.displayWorkflowInputs(workflowData)

	// Display Steps if they exist
	e.displayWorkflowSteps(workflowData)

	return nil
}

// displayWorkflowInputs shows the input parameters in a readable format
func (e *ToolExecutor) displayWorkflowInputs(workflowData map[string]interface{}) {
	inputSchemaFields := []string{"InputSchema", "inputSchema", "inputs", "parameters"}

	var inputSchema map[string]interface{}
	for _, field := range inputSchemaFields {
		if schema, exists := workflowData[field]; exists && schema != nil {
			if schemaMap, ok := schema.(map[string]interface{}); ok {
				inputSchema = schemaMap
				break
			}
		}
	}

	if inputSchema == nil {
		return
	}

	fmt.Printf("\n%s\n", text.FgHiCyan.Sprint("📝 Input Parameters:"))

	// Look for properties in the schema
	if properties, exists := inputSchema["properties"]; exists {
		if propsMap, ok := properties.(map[string]interface{}); ok {
			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.SetStyle(table.StyleRounded)
			t.AppendHeader(table.Row{
				text.FgHiCyan.Sprint("PARAMETER"),
				text.FgHiCyan.Sprint("TYPE"),
				text.FgHiCyan.Sprint("DESCRIPTION"),
				text.FgHiCyan.Sprint("REQUIRED"),
			})

			// Get required fields
			var required []string
			if reqField, exists := inputSchema["required"]; exists {
				if reqArray, ok := reqField.([]interface{}); ok {
					for _, req := range reqArray {
						if reqStr, ok := req.(string); ok {
							required = append(required, reqStr)
						}
					}
				}
			}

			// Sort parameter names
			var paramNames []string
			for paramName := range propsMap {
				paramNames = append(paramNames, paramName)
			}
			sort.Strings(paramNames)

			for _, paramName := range paramNames {
				if paramDef, ok := propsMap[paramName].(map[string]interface{}); ok {
					paramType := "string"
					if typ, exists := paramDef["type"]; exists {
						paramType = fmt.Sprintf("%v", typ)
					}

					description := "-"
					if desc, exists := paramDef["description"]; exists {
						description = fmt.Sprintf("%v", desc)
						if len(description) > 40 {
							description = description[:37] + "..."
						}
					}

					isRequired := "No"
					for _, req := range required {
						if req == paramName {
							isRequired = text.FgYellow.Sprint("Yes")
							break
						}
					}

					t.AppendRow(table.Row{
						text.FgHiWhite.Sprint(paramName),
						text.FgCyan.Sprint(paramType),
						description,
						isRequired,
					})
				}
			}

			t.Render()
		}
	}
}

// displayWorkflowSteps shows the workflow steps in a readable format
func (e *ToolExecutor) displayWorkflowSteps(workflowData map[string]interface{}) {
	stepsFields := []string{"Steps", "steps", "actions"}

	var steps []interface{}
	for _, field := range stepsFields {
		if stepsData, exists := workflowData[field]; exists && stepsData != nil {
			if stepsArray, ok := stepsData.([]interface{}); ok {
				steps = stepsArray
				break
			}
		}
	}

	if len(steps) == 0 {
		return
	}

	fmt.Printf("\n%s\n", text.FgHiCyan.Sprint("🔄 Workflow Steps:"))

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{
		text.FgHiCyan.Sprint("STEP"),
		text.FgHiCyan.Sprint("TOOL"),
		text.FgHiCyan.Sprint("DESCRIPTION"),
	})

	for i, step := range steps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			stepNum := fmt.Sprintf("%d", i+1)

			tool := "-"
			if toolName, exists := stepMap["Tool"]; exists {
				tool = fmt.Sprintf("%v", toolName)
				// Simplify tool name for display
				tool = e.simplifyToolName(tool)
			}

			description := "-"
			if desc, exists := stepMap["Description"]; exists && desc != nil {
				description = fmt.Sprintf("%v", desc)
			} else if id, exists := stepMap["ID"]; exists && id != nil {
				description = fmt.Sprintf("Execute %v", id)
			}

			if len(description) > 50 {
				description = description[:47] + "..."
			}

			t.AppendRow(table.Row{
				text.FgYellow.Sprint(stepNum),
				text.FgCyan.Sprint(tool),
				description,
			})
		}
	}

	t.Render()
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

func (e *ToolExecutor) containsString(slice []string, item string) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func (e *ToolExecutor) sortDataByName(data []interface{}, columns []string) []interface{} {
	sort.SliceStable(data, func(i, j int) bool {
		iMap, iOk := data[i].(map[string]interface{})
		jMap, jOk := data[j].(map[string]interface{})
		if iOk && jOk {
			// Use the first column (usually name/id) for sorting
			if len(columns) > 0 {
				iVal := fmt.Sprintf("%v", iMap[columns[0]])
				jVal := fmt.Sprintf("%v", jMap[columns[0]])
				return strings.ToLower(iVal) < strings.ToLower(jVal)
			}
		}
		return false
	})
	return data
}

func (e *ToolExecutor) getResourceIcon(resourceType string) string {
	switch resourceType {
	case "services":
		return text.FgGreen.Sprint("🟢")
	case "serviceClasses":
		return text.FgYellow.Sprint("🟡")
	case "mcpServers":
		return text.FgRed.Sprint("🔴")
	case "workflows":
		return text.FgBlue.Sprint("🔵")
	case "capabilities":
		return text.FgMagenta.Sprint("🟣")
	default:
		return text.FgHiBlack.Sprint("⚫")
	}
}
