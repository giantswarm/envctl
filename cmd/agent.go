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
	agentEndpoint string
	agentTimeout  time.Duration
	agentVerbose  bool
	agentNoColor  bool
	agentJSONRPC  bool
)

// agentCmd represents the agent command
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Act as an MCP client to debug the aggregator SSE server",
	Long: `The agent command connects to the MCP aggregator as a client agent, 
logs all JSON-RPC communication, and demonstrates dynamic tool updates.

This is useful for debugging the aggregator's behavior, verifying that
tools are properly aggregated, and ensuring that notifications work correctly
when tools are added or removed.

By default, it connects to the aggregator endpoint configured in your
envctl configuration file. You can override this with the --endpoint flag.`,
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(agentCmd)

	// Add flags
	agentCmd.Flags().StringVar(&agentEndpoint, "endpoint", "", "SSE endpoint URL (default: from config)")
	agentCmd.Flags().DurationVar(&agentTimeout, "timeout", 5*time.Minute, "Timeout for waiting for notifications")
	agentCmd.Flags().BoolVar(&agentVerbose, "verbose", false, "Enable verbose logging (show keepalive messages)")
	agentCmd.Flags().BoolVar(&agentNoColor, "no-color", false, "Disable colored output")
	agentCmd.Flags().BoolVar(&agentJSONRPC, "json-rpc", false, "Enable full JSON-RPC message logging")
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
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Determine endpoint
	endpoint := agentEndpoint
	if endpoint == "" {
		// Load configuration to get aggregator settings
		cfg, err := config.LoadConfig("", "")
		if err != nil {
			// Use default if config cannot be loaded
			endpoint = "http://localhost:8090/sse"
			fmt.Printf("Warning: Could not load config (%v), using default endpoint: %s\n", err, endpoint)
		} else {
			// Build endpoint from config
			host := cfg.Aggregator.Host
			if host == "" {
				host = "localhost"
			}
			port := cfg.Aggregator.Port
			if port == 0 {
				port = 8090
			}
			endpoint = fmt.Sprintf("http://%s:%d/sse", host, port)
		}
	}

	// Create logger
	logger := agent.NewLogger(agentVerbose, !agentNoColor, agentJSONRPC)

	// Create and run agent client
	client := agent.NewClient(endpoint, logger)

	// Create timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, agentTimeout)
	defer timeoutCancel()

	// Run the agent
	if err := client.Run(timeoutCtx); err != nil {
		if err == context.DeadlineExceeded {
			logger.Info("Timeout reached after %v", agentTimeout)
			return nil
		}
		return fmt.Errorf("agent error: %w", err)
	}

	return nil
}
