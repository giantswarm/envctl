package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// CallCommand handles calling MCP tools
type CallCommand struct {
	*BaseCommand
}

// NewCallCommand creates a new call command
func NewCallCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *CallCommand {
	return &CallCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the call command
func (c *CallCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", c.Usage())
	}

	toolName := args[0]
	argsStr := strings.Join(args[1:], " ")

	// Find and validate tool exists
	tools := c.client.GetToolCache()
	tool := c.getFormatters().FindTool(tools, toolName)
	if tool == nil {
		return fmt.Errorf("tool not found: %s", toolName)
	}

	// Parse arguments
	arguments, err := c.parseJSONArgs(argsStr, "call", toolName)
	if err != nil {
		return err
	}

	// Execute the tool
	fmt.Printf("Executing tool: %s...\n", toolName)
	result, err := c.client.CallTool(ctx, toolName, arguments)
	if err != nil {
		return fmt.Errorf("tool execution failed: %w", err)
	}

	// Display results
	if result.IsError {
		fmt.Println("Tool returned an error:")
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				fmt.Printf("  %s\n", textContent.Text)
			}
		}
	} else {
		fmt.Println("Result:")
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				// Try to pretty-print if it's JSON
				var jsonData interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &jsonData); err == nil {
					b, _ := json.MarshalIndent(jsonData, "", "  ")
					fmt.Println(string(b))
				} else {
					fmt.Println(textContent.Text)
				}
			} else if imageContent, ok := mcp.AsImageContent(content); ok {
				fmt.Printf("[Image: MIME type %s, %d bytes]\n", imageContent.MIMEType, len(imageContent.Data))
			} else if audioContent, ok := mcp.AsAudioContent(content); ok {
				fmt.Printf("[Audio: MIME type %s, %d bytes]\n", audioContent.MIMEType, len(audioContent.Data))
			}
		}
	}

	return nil
}

// Usage returns the usage string
func (c *CallCommand) Usage() string {
	return "call <tool-name> [args...]"
}

// Description returns the command description
func (c *CallCommand) Description() string {
	return "Execute a tool with JSON arguments"
}

// Completions returns possible completions
func (c *CallCommand) Completions(input string) []string {
	return c.getToolCompletions()
}

// Aliases returns command aliases
func (c *CallCommand) Aliases() []string {
	return []string{}
}

// parseJSONArgs parses JSON arguments from string, providing helpful error messages
func (c *CallCommand) parseJSONArgs(argsStr, itemType, itemName string) (map[string]interface{}, error) {
	if argsStr == "" {
		return nil, nil
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		fmt.Printf("Error: Arguments must be valid JSON\n")
		fmt.Printf("Example: %s %s {\"param1\": \"value1\", \"param2\": 123}\n", itemType, itemName)
		return nil, fmt.Errorf("invalid JSON arguments: %w", err)
	}

	return args, nil
}

 