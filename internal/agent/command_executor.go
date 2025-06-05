package agent

import (
	"context"
	"fmt"
)

// CommandExecutor defines the interface for executing REPL commands
type CommandExecutor interface {
	// Execute runs a command and returns the output
	Execute(ctx context.Context, command string) (string, error)

	// GetCompletions returns possible completions for a partial command
	GetCompletions(partial string) []string

	// IsConnected returns whether the executor is connected to the MCP
	IsConnected() bool
}

// SimpleCommandExecutor is a simple implementation for testing
type SimpleCommandExecutor struct {
	connected bool
}

// NewSimpleCommandExecutor creates a new simple command executor
func NewSimpleCommandExecutor() *SimpleCommandExecutor {
	return &SimpleCommandExecutor{
		connected: true,
	}
}

// Execute implements CommandExecutor
func (e *SimpleCommandExecutor) Execute(ctx context.Context, command string) (string, error) {
	// For now, just return help text for any command
	if command == "help" || command == "?" {
		return `Available commands:
  help, ?                      - Show this help message
  list tools                   - List all available tools
  list resources               - List all available resources
  list prompts                 - List all available prompts
  describe tool <name>         - Show detailed information about a tool
  describe resource <uri>      - Show detailed information about a resource
  describe prompt <name>       - Show detailed information about a prompt
  call <tool> {json}           - Execute a tool with JSON arguments
  get <resource-uri>           - Retrieve a resource
  prompt <name> {json}         - Get a prompt with JSON arguments
  exit, quit                   - Exit the REPL`, nil
	}

	// Placeholder response for other commands
	return fmt.Sprintf("Command '%s' execution not yet implemented", command), nil
}

// GetCompletions implements CommandExecutor
func (e *SimpleCommandExecutor) GetCompletions(partial string) []string {
	// Basic command completions
	commands := []string{
		"help", "list", "describe", "call", "get", "prompt", "exit", "quit",
		"list tools", "list resources", "list prompts",
		"describe tool", "describe resource", "describe prompt",
	}

	var completions []string
	for _, cmd := range commands {
		if len(partial) == 0 || (len(cmd) >= len(partial) && cmd[:len(partial)] == partial) {
			completions = append(completions, cmd)
		}
	}

	return completions
}

// IsConnected implements CommandExecutor
func (e *SimpleCommandExecutor) IsConnected() bool {
	return e.connected
}
