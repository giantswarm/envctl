package cmd

import (
	"context"
	"envctl/internal/color"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/orchestrator"
	"envctl/internal/tui/controller"
	"envctl/pkg/logging"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	// For TUI program

	"github.com/spf13/cobra"
)

// MCP server specific types, variables, and init functions are now in internal/mcpserver

// noTUI controls whether to run in CLI mode (true) or TUI mode (false).
// CLI mode is useful for scripting and CI/CD environments where interactive UI is not desired.
var noTUI bool

// debug enables verbose logging across the application.
// This helps troubleshoot connection issues and understand service behavior.
var debug bool

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
	RunE: func(cmd *cobra.Command, args []string) error {
		// Extract cluster names from command arguments
		managementClusterArg := args[0]
		workloadClusterArg := ""
		if len(args) == 2 {
			workloadClusterArg = args[1]
		}

		// Configure logging based on debug flag
		// Debug mode provides detailed information about service operations
		appLogLevel := logging.LevelInfo
		if debug {
			appLogLevel = logging.LevelDebug
		}

		// Initialize logging for CLI output (will be replaced for TUI mode)
		logging.InitForCLI(appLogLevel, os.Stdout)

		// Create kube manager to handle Kubernetes operations
		kubeMgr := kube.NewManager(nil)

		// Capture the initial Kubernetes context before any modifications
		// This helps users understand what context they started with
		initialKubeContext, err := kubeMgr.GetCurrentContext()
		if err != nil {
			logging.Warn("ConnectCmd", "Failed to get initial kube context: %v", err)
			initialKubeContext = "unknown"
		}
		logging.Info("CLI", "Initial Kubernetes context: %s", initialKubeContext)

		// Load configuration from multiple sources (default, user, project)
		// This provides flexibility in how users configure envctl
		envctlCfg, err := config.LoadConfig(managementClusterArg, workloadClusterArg)
		if err != nil {
			logging.Error("CLI", err, "Failed to load envctl configuration")
			// Configuration is essential for proper operation
			return fmt.Errorf("failed to load envctl configuration: %w", err)
		}

		if noTUI {
			// CLI Mode: Non-interactive operation suitable for scripts and automation
			logging.Info("CLI", "Running in no-TUI mode.")

			// Attempt to log into the management cluster first
			// This establishes the foundation for all other operations
			if managementClusterArg != "" {
				logging.Info("CLI", "Attempting login to Management Cluster: %s", managementClusterArg)
				stdout, stderr, loginErr := kube.LoginToKubeCluster(managementClusterArg)
				if loginErr != nil {
					// Continue setup even if login fails - user might already be logged in
					logging.Error("CLI", loginErr, "Login to %s failed. Continuing with setup if possible...", managementClusterArg)
				} else {
					// Update context after successful login
					currentKubeContextAfterLogin, _ := kubeMgr.GetCurrentContext()
					logging.Info("ConnectCmd", "Current kube context after login: %s", currentKubeContextAfterLogin)
					initialKubeContext = currentKubeContextAfterLogin
				}
				// Log command output for debugging
				if stdout != "" {
					logging.Debug("CLI", "Login stdout: %s", stdout)
				}
				if stderr != "" {
					logging.Debug("CLI", "Login stderr: %s", stderr)
				}
			}

			logging.Info("CLI", "--- Setting up orchestrator for service management ---")

			ctx := context.Background()

			// Create orchestrator to manage all services
			// The orchestrator handles dependencies and ensures services start in the correct order
			orchConfig := orchestrator.Config{
				MCName:       managementClusterArg,
				WCName:       workloadClusterArg,
				PortForwards: envctlCfg.PortForwards,
				MCPServers:   envctlCfg.MCPServers,
			}
			orch := orchestrator.New(orchConfig)

			// Start all configured services
			if err := orch.Start(ctx); err != nil {
				logging.Error("CLI", err, "Failed to start orchestrator")
				return err
			}

			logging.Info("CLI", "Services started. Press Ctrl+C to stop all services and exit.")

			// Wait for interrupt signal to gracefully shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan

			// Graceful shutdown sequence
			logging.Info("CLI", "\n--- Shutting down services ---")
			orch.Stop()
			// Allow time for services to clean up
			time.Sleep(1 * time.Second)

		} else {
			// TUI Mode: Interactive terminal interface for monitoring and control
			logging.Info("CLI", "Starting TUI mode...")

			// Initialize color scheme for TUI (dark mode by default)
			color.Initialize(true)

			// Switch logging to channel-based system for TUI integration
			logChan := logging.InitForTUI(appLogLevel)
			defer logging.CloseTUIChannel()

			// Create and configure the TUI program
			p, err := controller.NewProgram(
				managementClusterArg,
				workloadClusterArg,
				initialKubeContext,
				debug,
				envctlCfg,
				logChan,
			)
			if err != nil {
				logging.Error("TUI-Lifecycle", err, "Error creating TUI program")
				return err
			}

			// Run the TUI until user exits
			if _, err := p.Run(); err != nil {
				logging.Error("TUI-Lifecycle", err, "Error running TUI program")
				return err
			}
			logging.Info("TUI-Lifecycle", "TUI exited.")
		}
		return nil
	},
	// ValidArgsFunction provides shell completion for cluster names
	// This enhances user experience by suggesting valid cluster names during tab completion
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Create a temporary kube manager for completion
		tempKubeMgr := kube.NewManager(nil)
		clusterInfo, err := tempKubeMgr.ListClusters()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Completion error: %v\n", err)
			return nil, cobra.ShellCompDirectiveError
		}

		var candidates []string
		if len(args) == 0 {
			// First argument: suggest management cluster names
			for _, cluster := range clusterInfo.ManagementClusters {
				candidates = append(candidates, cluster)
			}
		} else if len(args) == 1 {
			// Second argument: suggest workload cluster short names for the selected MC
			managementClusterName := args[0]
			if wcShortNames, ok := clusterInfo.WorkloadClusters[managementClusterName]; ok {
				for _, shortName := range wcShortNames {
					candidates = append(candidates, shortName)
				}
			}
		}
		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
}

// init registers the connect command and its flags with the root command.
// This is called automatically when the package is imported.
func init() {
	rootCmd.AddCommand(connectCmdDef)

	// Register command flags
	connectCmdDef.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI and run port forwarding in the background")
	connectCmdDef.Flags().BoolVar(&debug, "debug", false, "Enable general debug logging")
}
