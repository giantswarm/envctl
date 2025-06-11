package cmd

import (
	"context"
	"envctl/internal/app"
	"fmt"

	"github.com/spf13/cobra"
)

// noTUI controls whether to run in CLI mode (true) or TUI mode (false).
// CLI mode is useful for scripting and CI/CD environments where interactive UI is not desired.
var noTUI bool

// debug enables verbose logging across the application.
// This helps troubleshoot connection issues and understand service behavior.
var debug bool

// yolo disables the denylist for destructive tool calls.
// When enabled, all MCP tools can be executed without restrictions.
var yolo bool

// connectCmdDef defines the connect command structure.
// This is the main command of envctl that establishes connections to Giant Swarm clusters
// and sets up the necessary port forwards and MCP servers for development.
var connectCmdDef = &cobra.Command{
	Use:   "connect <management-cluster> [workload-cluster-shortname]",
	Short: "Connect to Giant Swarm K8s and managed services with an interactive TUI or CLI mode.",
	Long: `Connects to Giant Swarm Kubernetes clusters, sets the Kubernetes context, and manages port-forwarding for essential services.
It can run in two modes:

1. Interactive TUI Mode (default):
   - Launches a terminal user interface to monitor connections, cluster health, and manage port-forwards.
   - Logs into the specified Management Cluster (MC) and, if provided, the Workload Cluster (WC).
   - Sets the Kubernetes context (to WC if specified, otherwise MC).
   - Automatically starts and manages port-forwarding for:
     - Prometheus (MC) on localhost:8080
     - Grafana (MC) on localhost:3000
     - Alloy Metrics (on localhost:12345):
       - For the Workload Cluster (WC) if specified.
       - For the Management Cluster (MC) if only an MC is specified.
   - Provides real-time status and allows interactive control over port-forwards and context switching.

2. Non-TUI / CLI Mode (using --no-tui flag):
   - Performs logins and context switching as in TUI mode.
   - Starts the same set of port-forwards (Prometheus (MC), Grafana (MC), Alloy Metrics (WC if specified, otherwise MC))
     but runs them in the background.
   - Prints a summary of actions and connection details to the console, then exits.
   - Useful for scripting or when a TUI is not desired. Port-forwards continue to run
     until the 'envctl' process initiated by 'connect --no-tui' is terminated (e.g., Ctrl+C).

Arguments:
  <management-cluster>: (Required) The name of the Giant Swarm management cluster (e.g., "myinstallation", "mycluster").
  [workload-cluster-shortname]: (Optional) The *short* name of the workload cluster (e.g., "myworkloadcluster" for "myinstallation-myworkloadcluster", "customerprod" for "mycluster-customerprod").`,
	Args: cobra.RangeArgs(1, 2), // Accepts 1 or 2 arguments
	RunE: runConnect,
}

// runConnect is the main entry point for the connect command
func runConnect(cmd *cobra.Command, args []string) error {
	// Extract cluster names from command arguments
	managementClusterArg := args[0]
	workloadClusterArg := ""
	if len(args) == 2 {
		workloadClusterArg = args[1]
	}

	// Create application configuration
	cfg := app.NewConfig(managementClusterArg, workloadClusterArg, noTUI, debug, yolo)

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

// init registers the connect command and its flags with the root command.
// This is called automatically when the package is imported.
func init() {
	rootCmd.AddCommand(connectCmdDef)

	// Register command flags
	connectCmdDef.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI and run port forwarding in the background")
	connectCmdDef.Flags().BoolVar(&debug, "debug", false, "Enable general debug logging")
	connectCmdDef.Flags().BoolVar(&yolo, "yolo", false, "Disable denylist for destructive tool calls (use with caution)")
}
