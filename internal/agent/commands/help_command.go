package commands

import (
	"context"
	"fmt"
)

// HelpCommand handles displaying help information
type HelpCommand struct {
	*BaseCommand
	registry *Registry
}

// NewHelpCommand creates a new help command
func NewHelpCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface, registry *Registry) *HelpCommand {
	return &HelpCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
		registry:    registry,
	}
}

// Execute runs the help command
func (h *HelpCommand) Execute(ctx context.Context, args []string) error {
	// If specific command requested, show its help
	if len(args) > 0 {
		return h.showCommandHelp(args[0])
	}

	// Show general help
	return h.showGeneralHelp()
}

// Usage returns the usage string
func (h *HelpCommand) Usage() string {
	return "help [command]"
}

// Description returns the command description
func (h *HelpCommand) Description() string {
	return "Show help information for commands"
}

// Completions returns possible completions
func (h *HelpCommand) Completions(input string) []string {
	// Return all command names for completion
	return h.registry.AllCompletions()
}

// Aliases returns command aliases
func (h *HelpCommand) Aliases() []string {
	return []string{"?"}
}

// showGeneralHelp displays general help information
func (h *HelpCommand) showGeneralHelp() error {
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

// showCommandHelp displays help for a specific command
func (h *HelpCommand) showCommandHelp(commandName string) error {
	cmd, exists := h.registry.Get(commandName)
	if !exists {
		return fmt.Errorf("unknown command: %s", commandName)
	}

	fmt.Printf("Command: %s\n", commandName)
	fmt.Printf("Description: %s\n", cmd.Description())
	fmt.Printf("Usage: %s\n", cmd.Usage())

	aliases := cmd.Aliases()
	if len(aliases) > 0 {
		fmt.Printf("Aliases: %v\n", aliases)
	}

	return nil
}
