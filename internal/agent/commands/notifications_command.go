package commands

import (
	"context"
	"fmt"
	"strings"
)

// NotificationsCommand handles toggling notification display
type NotificationsCommand struct {
	*BaseCommand
}

// NewNotificationsCommand creates a new notifications command
func NewNotificationsCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *NotificationsCommand {
	return &NotificationsCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the notifications command
func (n *NotificationsCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", n.Usage())
	}

	setting := strings.ToLower(args[0])

	// First validate the setting is valid
	switch setting {
	case "on", "off":
		// Valid setting, continue
	default:
		return fmt.Errorf("invalid setting: %s. Use 'on' or 'off'", args[0])
	}

	// Check if transport supports notifications
	if !n.transport.SupportsNotifications() {
		fmt.Printf("Notifications are not supported with current transport. Use --transport=sse for notification support.\n")
		return nil
	}

	// Apply the setting
	switch setting {
	case "on":
		n.logger.SetVerbose(true)
		fmt.Println("Notifications enabled")
	case "off":
		n.logger.SetVerbose(false)
		fmt.Println("Notifications disabled")
	}
	return nil
}

// Usage returns the usage string
func (n *NotificationsCommand) Usage() string {
	return "notifications <on|off>"
}

// Description returns the command description
func (n *NotificationsCommand) Description() string {
	return "Enable/disable notification display"
}

// Completions returns possible completions
func (n *NotificationsCommand) Completions(input string) []string {
	return []string{"on", "off"}
}

// Aliases returns command aliases
func (n *NotificationsCommand) Aliases() []string {
	return []string{}
} 