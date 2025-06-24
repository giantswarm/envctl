package cmd

import (
	"context"
	"envctl/internal/agent"
	"envctl/internal/cli"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	agentEndpoint  string
	agentTimeout   time.Duration
	agentVerbose   bool
	agentNoColor   bool
	agentJSONRPC   bool
	agentREPL      bool
	agentMCPServer bool
	agentTransport string
)

// agentCmd represents the agent command
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "MCP Client for the envctl aggregator server",
	Long: `The agent command connects to the MCP aggregator as a client agent, 
logs all JSON-RPC communication, and demonstrates dynamic tool updates.

This is useful for connecting the aggregator's behavior, filtering
tools, and ensuring that the agent can execute tools.

The agent command can run in three modes:
1. Normal mode (default): Connects, lists tools, and waits for notifications
2. REPL mode (--repl): Provides an interactive interface to explore and execute tools
3. MCP Server mode (--mcp-server): Runs an MCP server that exposes REPL functionality via stdio

Transport options:
- streamable-http (default): Fast HTTP-based transport, compatible with envctl serve
- sse: Server-Sent Events transport with real-time notification support

In REPL mode, you can:
- List available tools, resources, and prompts
- Get detailed information about specific items
- Execute tools interactively with JSON arguments
- View resources and retrieve their contents
- Execute prompts with arguments
- Toggle notification display

In MCP Server mode:
- The agent command acts as an MCP server using stdio transport
- It exposes all REPL functionality as MCP tools
- It's designed for integration with AI assistants like Claude or Cursor
- Configure it in your AI assistant's MCP settings

By default, it connects to the aggregator endpoint configured in your
envctl configuration file. You can override this with the --endpoint flag.

Note: The aggregator server must be running (use 'envctl serve') before using this command.`,
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(agentCmd)

	// Add flags
	agentCmd.Flags().StringVar(&agentEndpoint, "endpoint", "", "Aggregator MCP endpoint URL (default: from config)")
	agentCmd.Flags().DurationVar(&agentTimeout, "timeout", 5*time.Minute, "Timeout for waiting for notifications")
	agentCmd.Flags().BoolVar(&agentVerbose, "verbose", false, "Enable verbose logging (show keepalive messages)")
	agentCmd.Flags().BoolVar(&agentNoColor, "no-color", false, "Disable colored output")
	agentCmd.Flags().BoolVar(&agentJSONRPC, "json-rpc", false, "Enable full JSON-RPC message logging")
	agentCmd.Flags().BoolVar(&agentREPL, "repl", false, "Start interactive REPL mode")
	agentCmd.Flags().BoolVar(&agentMCPServer, "mcp-server", false, "Run as MCP server (stdio transport)")
	agentCmd.Flags().StringVar(&agentTransport, "transport", string(agent.TransportStreamableHTTP), "Transport to use (streamable-http, sse)")

	// Mark flags as mutually exclusive
	agentCmd.MarkFlagsMutuallyExclusive("repl", "mcp-server")
}

func runAgent(cmd *cobra.Command, args []string) error {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Handle interrupts gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if !agentMCPServer {
			fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		}
		cancel()
	}()

	// Determine endpoint using the same logic as CLI commands
	endpoint := agentEndpoint
	if endpoint == "" {
		// Use the same endpoint detection logic as CLI commands
		detectedEndpoint, err := cli.DetectAggregatorEndpoint()
		if err != nil {
			// Use fallback default that matches system defaults
			endpoint = "http://localhost:8090/mcp"
			if !agentMCPServer && agentVerbose {
				fmt.Printf("Warning: Could not detect endpoint (%v), using default: %s\n", err, endpoint)
			}
		} else {
			endpoint = detectedEndpoint
		}
	}

	// Parse transport type
	var transport agent.TransportType
	switch agentTransport {
	case "sse":
		transport = agent.TransportSSE
	case "streamable-http":
		transport = agent.TransportStreamableHTTP
	default:
		return fmt.Errorf("unsupported transport: %s (supported: streamable-http, sse)", agentTransport)
	}

	// Create logger
	logger := agent.NewLogger(agentVerbose, !agentNoColor, agentJSONRPC)

	// Create agent client
	client := agent.NewClient(endpoint, logger, transport)

	// Run in different modes
	if agentMCPServer {
		// MCP Server mode
		logger.Info("Connecting to aggregator at: %s using %s transport", endpoint, transport)
		server, err := agent.NewMCPServer(client, logger, false)
		if err != nil {
			return fmt.Errorf("failed to create MCP server: %w", err)
		}

		logger.Info("Starting envctl agent MCP server (stdio transport)...")

		if err := server.Start(ctx); err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}
		return nil
	} else if agentREPL {
		// REPL mode - let REPL handle its own connection and logging
		repl := agent.NewREPL(client, logger)
		if err := repl.Run(ctx); err != nil {
			return fmt.Errorf("REPL error: %w", err)
		}
		return nil
	} else {
		// Normal agent mode
		logger.Info("Connecting to aggregator at: %s using %s transport", endpoint, transport)
		timeoutCtx, timeoutCancel := context.WithTimeout(ctx, agentTimeout)
		defer timeoutCancel()

		if err := client.Run(timeoutCtx); err != nil {
			if err == context.DeadlineExceeded {
				logger.Info("Timeout reached after %v", agentTimeout)
				return nil
			}
			return fmt.Errorf("agent error: %w", err)
		}
		return nil
	}
}
