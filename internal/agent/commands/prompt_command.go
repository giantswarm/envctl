package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// PromptCommand handles retrieving MCP prompts
type PromptCommand struct {
	*BaseCommand
}

// NewPromptCommand creates a new prompt command
func NewPromptCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *PromptCommand {
	return &PromptCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the prompt command
func (p *PromptCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", p.Usage())
	}

	promptName := args[0]
	argsStr := strings.Join(args[1:], " ")

	// Find and validate prompt exists
	prompts := p.client.GetPromptCache()
	prompt := p.getFormatters().FindPrompt(prompts, promptName)
	if prompt == nil {
		return fmt.Errorf("prompt not found: %s", promptName)
	}

	// Parse arguments
	arguments, err := p.parseJSONStringArgs(argsStr, "prompt", promptName, prompt.Arguments)
	if err != nil {
		return err
	}

	// Validate required arguments
	if err := p.validateRequiredArgs(arguments, prompt.Arguments); err != nil {
		return err
	}

	// Get the prompt
	fmt.Printf("Getting prompt: %s...\n", promptName)
	result, err := p.client.GetPrompt(ctx, promptName, arguments)
	if err != nil {
		return fmt.Errorf("prompt retrieval failed: %w", err)
	}

	// Display messages
	fmt.Println("Messages:")
	for i, msg := range result.Messages {
		fmt.Printf("\n[%d] Role: %s\n", i+1, msg.Role)
		if textContent, ok := mcp.AsTextContent(msg.Content); ok {
			fmt.Printf("Content: %s\n", textContent.Text)
		} else if imageContent, ok := mcp.AsImageContent(msg.Content); ok {
			fmt.Printf("Content: [Image: MIME type %s, %d bytes]\n", imageContent.MIMEType, len(imageContent.Data))
		} else if audioContent, ok := mcp.AsAudioContent(msg.Content); ok {
			fmt.Printf("Content: [Audio: MIME type %s, %d bytes]\n", audioContent.MIMEType, len(audioContent.Data))
		} else if resource, ok := mcp.AsEmbeddedResource(msg.Content); ok {
			fmt.Printf("Content: [Embedded Resource: %v]\n", resource.Resource)
		} else {
			// Fallback for unknown content types
			fmt.Printf("Content: %+v\n", msg.Content)
		}
	}

	return nil
}

// Usage returns the usage string
func (p *PromptCommand) Usage() string {
	return "prompt <prompt-name> [args...]"
}

// Description returns the command description
func (p *PromptCommand) Description() string {
	return "Get a prompt with JSON arguments"
}

// Completions returns possible completions
func (p *PromptCommand) Completions(input string) []string {
	return p.getPromptCompletions()
}

// Aliases returns command aliases
func (p *PromptCommand) Aliases() []string {
	return []string{}
}

// parseJSONStringArgs parses JSON arguments and converts all values to strings
func (p *PromptCommand) parseJSONStringArgs(argsStr, itemType, itemName string, requiredArgs []mcp.PromptArgument) (map[string]string, error) {
	if argsStr == "" {
		return make(map[string]string), nil
	}

	var jsonArgs map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &jsonArgs); err != nil {
		fmt.Printf("Error: Arguments must be valid JSON\n")
		fmt.Printf("Example: %s %s {\"arg1\": \"value1\", \"arg2\": \"value2\"}\n", itemType, itemName)

		// Show required arguments
		if len(requiredArgs) > 0 {
			fmt.Println("Required arguments:")
			for _, arg := range requiredArgs {
				if arg.Required {
					fmt.Printf("  - %s: %s\n", arg.Name, arg.Description)
				}
			}
		}
		return nil, fmt.Errorf("invalid JSON arguments: %w", err)
	}

	// Convert to string map
	args := make(map[string]string)
	for k, v := range jsonArgs {
		args[k] = fmt.Sprintf("%v", v)
	}

	return args, nil
}

// validateRequiredArgs checks that all required arguments are provided
func (p *PromptCommand) validateRequiredArgs(args map[string]string, requiredArgs []mcp.PromptArgument) error {
	for _, arg := range requiredArgs {
		if arg.Required && args[arg.Name] == "" {
			return fmt.Errorf("missing required argument: %s", arg.Name)
		}
	}
	return nil
}
