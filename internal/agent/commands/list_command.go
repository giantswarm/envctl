package commands

import (
	"context"
	"fmt"
	"strings"
)

// ListCommand handles listing various resources
type ListCommand struct {
	*BaseCommand
}

// NewListCommand creates a new list command
func NewListCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *ListCommand {
	return &ListCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the list command
func (l *ListCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", l.Usage())
	}

	target := strings.ToLower(args[0])
	
	switch target {
	case "tools", "tool":
		return l.listTools()
	case "resources", "resource":
		return l.listResources()
	case "prompts", "prompt":
		return l.listPrompts()
	case "core-tools", "coretools":
		return l.listCoreTools()
	default:
		return fmt.Errorf("unknown list target: %s. Use 'tools', 'resources', 'prompts', or 'core-tools'", args[0])
	}
}

// Usage returns the usage string
func (l *ListCommand) Usage() string {
	return "list <tools|resources|prompts|core-tools>"
}

// Description returns the command description
func (l *ListCommand) Description() string {
	return "List available tools, resources, prompts, or core envctl tools"
}

// Completions returns possible completions
func (l *ListCommand) Completions(input string) []string {
	return []string{"tools", "resources", "prompts", "core-tools"}
}

// Aliases returns command aliases
func (l *ListCommand) Aliases() []string {
	return []string{"ls"}
}

// listTools displays available tools
func (l *ListCommand) listTools() error {
	tools := l.client.GetToolCache()
	fmt.Println(l.client.GetFormatters().FormatToolsList(tools))
	return nil
}

// listResources displays available resources
func (l *ListCommand) listResources() error {
	resources := l.client.GetResourceCache()
	fmt.Println(l.client.GetFormatters().FormatResourcesList(resources))
	return nil
}

// listPrompts displays available prompts
func (l *ListCommand) listPrompts() error {
	prompts := l.client.GetPromptCache()
	fmt.Println(l.client.GetFormatters().FormatPromptsList(prompts))
	return nil
}

// listCoreTools displays core envctl tools
func (l *ListCommand) listCoreTools() error {
	// Define core envctl tools that are built-in functionality
	// These are tools that envctl provides natively, separate from external MCP servers
	// Names are shown with the "x_" prefix as they appear in the aggregator
	coreTools := []map[string]interface{}{
		{
			"name":        "x_capability_create",
			"description": "Create a new capability definition",
			"category":    "capability",
		},
		{
			"name":        "x_capability_list",
			"description": "List all available capabilities",
			"category":    "capability",
		},
		{
			"name":        "x_capability_get",
			"description": "Get detailed information about a specific capability",
			"category":    "capability",
		},
		{
			"name":        "x_capability_update",
			"description": "Update an existing capability definition",
			"category":    "capability",
		},
		{
			"name":        "x_capability_delete",
			"description": "Delete a capability definition",
			"category":    "capability",
		},
		{
			"name":        "x_serviceclass_create",
			"description": "Create a new service class definition",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_list",
			"description": "List all available service classes",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_get",
			"description": "Get detailed information about a specific service class",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_update",
			"description": "Update an existing service class definition",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_delete",
			"description": "Delete a service class definition",
			"category":    "serviceclass",
		},
		{
			"name":        "x_workflow_create",
			"description": "Create a new workflow definition",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_list",
			"description": "List all available workflows",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_get",
			"description": "Get detailed information about a specific workflow",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_update",
			"description": "Update an existing workflow definition",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_delete",
			"description": "Delete a workflow definition",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_run",
			"description": "Execute a workflow with given inputs",
			"category":    "workflow",
		},
		{
			"name":        "x_mcpserver_create",
			"description": "Create a new MCP server definition",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_list",
			"description": "List all available MCP servers",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_get",
			"description": "Get detailed information about a specific MCP server",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_update",
			"description": "Update an existing MCP server definition",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_delete",
			"description": "Delete an MCP server definition",
			"category":    "mcpserver",
		},
		{
			"name":        "x_service_create",
			"description": "Create a new service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_list",
			"description": "List all service instances",
			"category":    "service",
		},
		{
			"name":        "x_service_get",
			"description": "Get detailed information about a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_start",
			"description": "Start a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_stop",
			"description": "Stop a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_restart",
			"description": "Restart a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_delete",
			"description": "Delete a service instance",
			"category":    "service",
		},
	}

	fmt.Printf("Core envctl tools (%d):\n", len(coreTools))
	
	// Group by category
	categories := make(map[string][]map[string]interface{})
	for _, tool := range coreTools {
		category := tool["category"].(string)
		categories[category] = append(categories[category], tool)
	}

	// Display by category
	for _, category := range []string{"capability", "serviceclass", "workflow", "mcpserver", "service"} {
		if tools, exists := categories[category]; exists {
			// Capitalize the first letter manually
			displayName := strings.ToUpper(category[:1]) + category[1:]
			fmt.Printf("\n%s tools:\n", displayName)
			for i, tool := range tools {
				fmt.Printf("  %d. %-27s - %s\n", i+1, tool["name"], tool["description"])
			}
		}
	}

	return nil
} 