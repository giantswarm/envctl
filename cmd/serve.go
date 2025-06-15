package cmd

import (
	"context"
	"envctl/internal/app"
	"fmt"

	"github.com/spf13/cobra"
)

// noTUI controls whether to run in CLI mode (true) or TUI mode (false).
// CLI mode is useful for scripting and CI/CD environments where interactive UI is not desired.
var serveNoTUI bool

// debug enables verbose logging across the application.
// This helps troubleshoot connection issues and understand service behavior.
var serveDebug bool

// yolo disables the denylist for destructive tool calls.
// When enabled, all MCP tools can be executed without restrictions.
var serveYolo bool

// serveCmd defines the serve command structure.
// This is the main command of envctl that starts the aggregator server
// and sets up the necessary MCP servers for development.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the envctl aggregator server with an interactive TUI or CLI mode.",
	Long: `Starts the envctl aggregator server and manages MCP servers for AI assistant access.
It can run in two modes:

1. Interactive TUI Mode (default):
   - Launches a terminal user interface to monitor service connections and manage MCP servers.
   - Automatically starts configured MCP servers for AI assistant access.
   - Provides real-time status and allows interactive control over services.

2. Non-TUI / CLI Mode (using --no-tui flag):
   - Starts configured MCP servers and services in the background.
   - Prints a summary of actions and connection details to the console, then exits.
   - Useful for scripting or when a TUI is not desired. Services continue to run
     until the 'envctl' process initiated by 'serve --no-tui' is terminated (e.g., Ctrl+C).

The aggregator server provides a unified MCP interface that other envctl commands can connect to.
Use 'envctl service', 'envctl workflow', etc. to interact with the running server.

Configuration:
  envctl loads configuration from .envctl/config.yaml in the current directory or user config directory.
  Use 'envctl serve --help' for more information about configuration options.`,
	Args: cobra.NoArgs, // No arguments required
	RunE: runServe,
}

// runServe is the main entry point for the serve command
func runServe(cmd *cobra.Command, args []string) error {
	// Create application configuration without cluster arguments
	cfg := app.NewConfig(serveNoTUI, serveDebug, serveYolo)

	// Create and initialize the application
	application, err := app.NewApplication(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// Run the application
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return application.Run(ctx)
}

// init registers the serve command and its flags with the root command.
// This is called automatically when the package is imported.
func init() {
	rootCmd.AddCommand(serveCmd)

	// Register command flags
	serveCmd.Flags().BoolVar(&serveNoTUI, "no-tui", false, "Disable TUI and run services in the background")
	serveCmd.Flags().BoolVar(&serveDebug, "debug", false, "Enable general debug logging")
	serveCmd.Flags().BoolVar(&serveYolo, "yolo", false, "Disable denylist for destructive tool calls (use with caution)")
} 