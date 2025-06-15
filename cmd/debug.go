package cmd

import (
	"context"
	"envctl/internal/agent"
	"envctl/internal/config"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	debugEndpoint  string
	debugTimeout   time.Duration
	debugVerbose   bool
	debugNoColor   bool
	debugJSONRPC   bool
	debugREPL      bool
	debugMCPServer bool
)

// debugCmd represents the debug command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug the envctl aggregator server using an MCP client",
	Long: `The debug command connects to the MCP aggregator as a client agent, 
logs all JSON-RPC communication, and demonstrates dynamic tool updates.

This is useful for debugging the aggregator's behavior, verifying that
tools are properly aggregated, and ensuring that notifications work correctly
when tools are added or removed.

The debug command can run in three modes:
1. Normal mode (default): Connects, lists tools, and waits for notifications
2. REPL mode (--repl): Provides an interactive interface to explore and execute tools
3. MCP Server mode (--mcp-server): Runs an MCP server that exposes REPL functionality via stdio

In REPL mode, you can:
- List available tools, resources, and prompts
- Get detailed information about specific items
- Execute tools interactively with JSON arguments
- View resources and retrieve their contents
- Execute prompts with arguments
- Toggle notification display

In MCP Server mode:
- The debug command acts as an MCP server using stdio transport
- It exposes all REPL functionality as MCP tools
- It's designed for integration with AI assistants like Claude or Cursor
- Configure it in your AI assistant's MCP settings

By default, it connects to the aggregator endpoint configured in your
envctl configuration file. You can override this with the --endpoint flag.

Note: The aggregator server must be running (use 'envctl serve') before using this command.`,
	RunE: runDebug,
}

func init() {
	rootCmd.AddCommand(debugCmd)

	// Add flags
	debugCmd.Flags().StringVar(&debugEndpoint, "endpoint", "", "SSE endpoint URL (default: from config)")
	debugCmd.Flags().DurationVar(&debugTimeout, "timeout", 5*time.Minute, "Timeout for waiting for notifications")
	debugCmd.Flags().BoolVar(&debugVerbose, "verbose", false, "Enable verbose logging (show keepalive messages)")
	debugCmd.Flags().BoolVar(&debugNoColor, "no-color", false, "Disable colored output")
	debugCmd.Flags().BoolVar(&debugJSONRPC, "json-rpc", false, "Enable full JSON-RPC message logging")
	debugCmd.Flags().BoolVar(&debugREPL, "repl", false, "Start interactive REPL mode")
	debugCmd.Flags().BoolVar(&debugMCPServer, "mcp-server", false, "Run as MCP server (stdio transport)")

	// Mark flags as mutually exclusive
	debugCmd.MarkFlagsMutuallyExclusive("repl", "mcp-server")
}

func runDebug(cmd *cobra.Command, args []string) error {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Handle interrupts gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if !debugMCPServer {
			fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		}
		cancel()
	}()

	// Determine endpoint
	endpoint := debugEndpoint
	if endpoint == "" {
		// Load configuration to get aggregator settings
		cfg, err := config.LoadConfig()
		if err != nil {
			// Use default if config cannot be loaded
			endpoint = "http://localhost:8080/sse"
			if !debugMCPServer {
				fmt.Printf("Warning: Could not load config (%v), using default endpoint: %s\n", err, endpoint)
			}
		} else {
			// Build endpoint from config
			host := cfg.Aggregator.Host
			if host == "" {
				host = "localhost"
			}
			port := cfg.Aggregator.Port
			if port == 0 {
				port = 8080
			}
			endpoint = fmt.Sprintf("http://%s:%d/sse", host, port)
		}
	}

	// Create logger
	logger := agent.NewLogger(debugVerbose, !debugNoColor, debugJSONRPC)

	// Run in MCP Server mode if requested
	if debugMCPServer {
		server, err := agent.NewMCPServer(endpoint, logger, false)
		if err != nil {
			return fmt.Errorf("failed to create MCP server: %w", err)
		}

		logger.Info("Starting envctl debug MCP server (stdio transport)...")
		logger.Info("Connecting to aggregator at: %s", endpoint)

		if err := server.Start(ctx); err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}
		return nil
	}

	// Create and run agent client
	client := agent.NewClient(endpoint, logger)

	// Run in REPL mode if requested
	if debugREPL {
		// REPL mode doesn't use timeout
		repl := agent.NewREPL(client, logger)
		if err := repl.Run(ctx); err != nil {
			return fmt.Errorf("REPL error: %w", err)
		}
		return nil
	}

	// Create timeout context for non-REPL mode
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, debugTimeout)
	defer timeoutCancel()

	// Run the agent in normal mode
	if err := client.Run(timeoutCtx); err != nil {
		if err == context.DeadlineExceeded {
			logger.Info("Timeout reached after %v", debugTimeout)
			return nil
		}
		return fmt.Errorf("debug error: %w", err)
	}

	return nil
}
