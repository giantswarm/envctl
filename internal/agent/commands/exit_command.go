package commands

import (
	"context"
	"fmt"
)

// ExitCommand handles exiting the REPL
type ExitCommand struct {
	*BaseCommand
}

// NewExitCommand creates a new exit command
func NewExitCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *ExitCommand {
	return &ExitCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the exit command
func (e *ExitCommand) Execute(ctx context.Context, args []string) error {
	// Return special error that REPL recognizes as exit signal
	return fmt.Errorf("exit")
}

// Usage returns the usage string
func (e *ExitCommand) Usage() string {
	return "exit"
}

// Description returns the command description
func (e *ExitCommand) Description() string {
	return "Exit the REPL"
}

// Completions returns possible completions
func (e *ExitCommand) Completions(input string) []string {
	return []string{}
}

// Aliases returns command aliases
func (e *ExitCommand) Aliases() []string {
	return []string{"quit"}
}
