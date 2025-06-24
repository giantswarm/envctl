package agent

import (
	"context"
	"fmt"
	"strings"
)

// executeCommand parses and executes a command
func (r *REPL) executeCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "help", "?":
		return r.showHelp()

	case "list":
		if len(parts) < 2 {
			return fmt.Errorf("usage: list <tools|resources|prompts|core-tools>")
		}
		return r.handleList(ctx, parts[1])

	case "filter":
		if len(parts) < 2 {
			return fmt.Errorf("usage: filter <tools> [pattern] [description_filter]")
		}
		if parts[1] == "tools" {
			return r.handleFilterTools(ctx, parts[2:]...)
		}
		return fmt.Errorf("filter only supports 'tools' currently")

	case "describe":
		if len(parts) < 3 {
			return fmt.Errorf("usage: describe <tool|resource|prompt> <name>")
		}
		return r.handleDescribe(ctx, parts[1], strings.Join(parts[2:], " "))

	case "exit", "quit":
		return fmt.Errorf("exit")

	case "notifications":
		if len(parts) < 2 {
			return fmt.Errorf("usage: notifications <on|off>")
		}
		return r.handleNotifications(parts[1])

	case "call":
		if len(parts) < 2 {
			return fmt.Errorf("usage: call <tool-name> [args...]")
		}
		return r.handleCallTool(ctx, parts[1], strings.Join(parts[2:], " "))

	case "get":
		if len(parts) < 2 {
			return fmt.Errorf("usage: get <resource-uri>")
		}
		return r.handleGetResource(ctx, parts[1])

	case "prompt":
		if len(parts) < 2 {
			return fmt.Errorf("usage: prompt <prompt-name> [args...]")
		}
		return r.handleGetPrompt(ctx, parts[1], strings.Join(parts[2:], " "))

	default:
		return fmt.Errorf("unknown command: %s. Type 'help' for available commands", command)
	}
}

// showHelp displays available commands
func (r *REPL) showHelp() error {
	fmt.Println("Available commands:")
	fmt.Println("  help, ?                      - Show this help message")
	fmt.Println("  list tools                   - List all available tools")
	fmt.Println("  list resources               - List all available resources")
	fmt.Println("  list prompts                 - List all available prompts")
	fmt.Println("  list core-tools              - List core envctl tools (built-in functionality)")
	fmt.Println("  filter tools [pattern] [desc] - Filter tools by name pattern or description")
	fmt.Println("  describe tool <name>         - Show detailed information about a tool")
	fmt.Println("  describe resource <uri>      - Show detailed information about a resource")
	fmt.Println("  describe prompt <name>       - Show detailed information about a prompt")
	fmt.Println("  call <tool> {json}           - Execute a tool with JSON arguments")
	fmt.Println("  get <resource-uri>           - Retrieve a resource")
	fmt.Println("  prompt <name> {json}         - Get a prompt with JSON arguments")
	fmt.Println("  notifications <on|off>       - Enable/disable notification display")
	fmt.Println("  exit, quit                   - Exit the REPL")
	fmt.Println()
	fmt.Println("Keyboard shortcuts:")
	fmt.Println("  TAB                          - Auto-complete commands and arguments")
	fmt.Println("  ↑/↓ (arrow keys)             - Navigate command history")
	fmt.Println("  Ctrl+R                       - Search command history")
	fmt.Println("  Ctrl+C                       - Cancel current line")
	fmt.Println("  Ctrl+D                       - Exit REPL")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  call calculate {\"operation\": \"add\", \"x\": 5, \"y\": 3}")
	fmt.Println("  get docs://readme")
	fmt.Println("  prompt greeting {\"name\": \"Alice\"}")
	fmt.Println("  filter tools *workflow*      - Find tools with 'workflow' in name")
	fmt.Println("  filter tools \"\" \"kubernetes\" - Find tools with 'kubernetes' in description")
	return nil
}

// handleList handles list commands
func (r *REPL) handleList(ctx context.Context, target string) error {
	switch strings.ToLower(target) {
	case "tools", "tool":
		return r.listTools(ctx)
	case "resources", "resource":
		return r.listResources(ctx)
	case "prompts", "prompt":
		return r.listPrompts(ctx)
	case "core-tools", "coretools":
		return r.listCoreTools(ctx)
	default:
		return fmt.Errorf("unknown list target: %s. Use 'tools', 'resources', 'prompts', or 'core-tools'", target)
	}
}

// handleDescribe handles describe commands
func (r *REPL) handleDescribe(ctx context.Context, targetType, name string) error {
	switch strings.ToLower(targetType) {
	case "tool":
		return r.describeTool(ctx, name)
	case "resource":
		return r.describeResource(ctx, name)
	case "prompt":
		return r.describePrompt(ctx, name)
	default:
		return fmt.Errorf("unknown describe target: %s. Use 'tool', 'resource', or 'prompt'", targetType)
	}
}

// handleNotifications toggles notification display
func (r *REPL) handleNotifications(setting string) error {
	// First validate the setting is valid
	switch strings.ToLower(setting) {
	case "on", "off":
		// Valid setting, continue
	default:
		return fmt.Errorf("invalid setting: %s. Use 'on' or 'off'", setting)
	}

	// Check if transport supports notifications
	if r.client.transport != TransportSSE {
		fmt.Printf("Notifications are not supported with %s transport. Use --transport=sse for notification support.\n", r.client.transport)
		return nil
	}

	// Apply the setting
	switch strings.ToLower(setting) {
	case "on":
		r.logger.SetVerbose(true)
		fmt.Println("Notifications enabled")
	case "off":
		r.logger.SetVerbose(false)
		fmt.Println("Notifications disabled")
	}
	return nil
} 