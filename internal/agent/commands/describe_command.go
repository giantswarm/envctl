package commands

import (
	"context"
	"fmt"
	"strings"
)

// DescribeCommand handles describing various resources
type DescribeCommand struct {
	*BaseCommand
}

// NewDescribeCommand creates a new describe command
func NewDescribeCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *DescribeCommand {
	return &DescribeCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the describe command
func (d *DescribeCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s", d.Usage())
	}

	targetType := strings.ToLower(args[0])
	name := d.joinArgsFrom(args, 1)

	switch targetType {
	case "tool":
		return d.describeTool(name)
	case "resource":
		return d.describeResource(name)
	case "prompt":
		return d.describePrompt(name)
	default:
		return fmt.Errorf("unknown describe target: %s. Use 'tool', 'resource', or 'prompt'", args[0])
	}
}

// Usage returns the usage string
func (d *DescribeCommand) Usage() string {
	return "describe <tool|resource|prompt> <name>"
}

// Description returns the command description
func (d *DescribeCommand) Description() string {
	return "Show detailed information about a tool, resource, or prompt"
}

// Completions returns possible completions
func (d *DescribeCommand) Completions(input string) []string {
	parts := strings.Fields(input)

	// If no args yet, suggest targets
	if len(parts) <= 1 {
		return []string{"tool", "resource", "prompt"}
	}

	// If we have the target type, suggest items of that type
	targetType := strings.ToLower(parts[1])
	switch targetType {
	case "tool":
		return d.getToolCompletions()
	case "resource":
		return d.getResourceCompletions()
	case "prompt":
		return d.getPromptCompletions()
	default:
		return []string{}
	}
}

// Aliases returns command aliases
func (d *DescribeCommand) Aliases() []string {
	return []string{"desc", "info"}
}

// describeTool shows detailed information about a tool
func (d *DescribeCommand) describeTool(name string) error {
	tools := d.client.GetToolCache()
	tool := d.getFormatters().FindTool(tools, name)
	if tool == nil {
		return fmt.Errorf("tool not found: %s", name)
	}

	fmt.Println(d.getFormatters().FormatToolDetail(*tool))
	return nil
}

// describeResource shows detailed information about a resource
func (d *DescribeCommand) describeResource(uri string) error {
	resources := d.client.GetResourceCache()
	resource := d.getFormatters().FindResource(resources, uri)
	if resource == nil {
		return fmt.Errorf("resource not found: %s", uri)
	}

	fmt.Println(d.getFormatters().FormatResourceDetail(*resource))
	return nil
}

// describePrompt shows detailed information about a prompt
func (d *DescribeCommand) describePrompt(name string) error {
	prompts := d.client.GetPromptCache()
	prompt := d.getFormatters().FindPrompt(prompts, name)
	if prompt == nil {
		return fmt.Errorf("prompt not found: %s", name)
	}

	fmt.Println(d.getFormatters().FormatPromptDetail(*prompt))
	return nil
}
