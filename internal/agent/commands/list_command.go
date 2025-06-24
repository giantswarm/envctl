package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ListCommand lists available tools, resources, or prompts
type ListCommand struct {
	*BaseCommand
}

// NewListCommand creates a new list command
func NewListCommand(client ClientInterface, output OutputLogger, transport TransportInterface) *ListCommand {
	return &ListCommand{
		BaseCommand: NewBaseCommand(client, output, transport),
	}
}

// Execute lists tools, resources, or prompts
func (l *ListCommand) Execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s", l.Usage())
	}

	target := strings.ToLower(args[0])
	switch target {
	case "tools":
		return l.listTools()
	case "resources":
		return l.listResources()
	case "prompts":
		return l.listPrompts()
	case "core-tools":
		return l.listCoreTools(ctx)
	default:
		return l.validateTarget(target, []string{"tools", "resources", "prompts", "core-tools"})
	}
}

// listTools lists all available tools
func (l *ListCommand) listTools() error {
	tools := l.client.GetToolCache()
	l.output.OutputLine(l.getFormatters().FormatToolsList(tools))
	return nil
}

// listResources lists all available resources
func (l *ListCommand) listResources() error {
	resources := l.client.GetResourceCache()
	l.output.OutputLine(l.getFormatters().FormatResourcesList(resources))
	return nil
}

// listPrompts lists all available prompts
func (l *ListCommand) listPrompts() error {
	prompts := l.client.GetPromptCache()
	l.output.OutputLine(l.getFormatters().FormatPromptsList(prompts))
	return nil
}

// listCoreTools lists envctl core tools by making an MCP call
func (l *ListCommand) listCoreTools(ctx context.Context) error {
	l.output.Info("Fetching core envctl tools...")

	// Call the list_core_tools MCP tool
	result, err := l.client.CallTool(ctx, "list_core_tools", map[string]interface{}{
		"random_string": "dummy", // Required parameter
	})
	if err != nil {
		l.output.Error("Failed to fetch core tools: %v", err)
		return fmt.Errorf("failed to list core tools: %w", err)
	}

	// Check if there are any text contents in the result
	if result.Content == nil || len(result.Content) == 0 {
		l.output.OutputLine("No core tools available")
		return nil
	}

	// Process the first text content
	var coreToolsData map[string]interface{}
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			// Try to parse as JSON
			if err := json.Unmarshal([]byte(textContent.Text), &coreToolsData); err != nil {
				// If not JSON, display as raw text
				l.output.OutputLine("Core tools information:")
				l.output.OutputLine(textContent.Text)
				return nil
			}
			break
		}
	}

	// Extract tools from the data
	if coreToolsData == nil {
		l.output.OutputLine("No core tools data available")
		return nil
	}

	// Look for tools in various possible structures
	var coreTools []map[string]interface{}

	// Try direct "tools" field
	if tools, ok := coreToolsData["tools"].([]interface{}); ok {
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				coreTools = append(coreTools, toolMap)
			}
		}
	} else {
		// Maybe the data itself is the tools array
		if toolsArray, ok := coreToolsData["core_tools"].([]interface{}); ok {
			for _, tool := range toolsArray {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					coreTools = append(coreTools, toolMap)
				}
			}
		}
	}

	if len(coreTools) == 0 {
		l.output.OutputLine("No core tools found")
		return nil
	}

	// Group tools by server type for better organization
	toolsByServer := make(map[string][]map[string]interface{})
	for _, tool := range coreTools {
		toolName, _ := tool["name"].(string)

		// Determine server type from tool name
		var serverType string
		if strings.HasPrefix(toolName, "mcp_") {
			parts := strings.Split(toolName, "_")
			if len(parts) >= 2 {
				serverType = parts[1] // e.g., "github" from "mcp_github_create_issue"
			} else {
				serverType = "mcp"
			}
		} else {
			serverType = "core"
		}

		toolsByServer[serverType] = append(toolsByServer[serverType], tool)
	}

	// Display tools grouped by server
	l.output.OutputLine("Core envctl tools (%d):", len(coreTools))

	// Show core tools first
	if coreTools, exists := toolsByServer["core"]; exists {
		l.output.OutputLine("\nCore tools:")
		for i, tool := range coreTools {
			l.output.OutputLine("  %d. %-27s - %s", i+1, tool["name"], tool["description"])
		}
		delete(toolsByServer, "core")
	}

	// Show other server tools
	for serverType, tools := range toolsByServer {
		displayName := strings.Title(serverType)
		l.output.OutputLine("\n%s tools:", displayName)
		for i, tool := range tools {
			l.output.OutputLine("  %d. %-27s - %s", i+1, tool["name"], tool["description"])
		}
	}

	return nil
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
	return l.getCompletionsForTargets([]string{"tools", "resources", "prompts", "core-tools"})
}

// Aliases returns command aliases
func (l *ListCommand) Aliases() []string {
	return []string{"ls"}
}
