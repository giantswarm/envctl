package cmd

import (
	"context"
	"envctl/internal/color"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/managers"
	"envctl/internal/orchestrator"
	"envctl/internal/reporting"
	"envctl/internal/tui/controller"
	"envctl/pkg/logging"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	// For TUI program
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// MCP server specific types, variables, and init functions are now in internal/mcpserver

var noTUI bool        // Variable to store the value of the --no-tui flag
var tuiDebugMode bool // Variable to store the value of the --debug-tui flag for TUI
var debug bool        // General debug flag, distinct from tuiDebugMode for now if needed

// connectCmdDef defines the connect command structure
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
		managementClusterArg := args[0]
		workloadClusterArg := ""
		if len(args) == 2 {
			workloadClusterArg = args[1]
		}

		// Determine global log level based on debug flag
		appLogLevel := logging.LevelInfo
		if debug {
			appLogLevel = logging.LevelDebug
		}

		logging.InitForCLI(appLogLevel, os.Stdout)

		// Create kube manager
		kubeMgr := kube.NewManager(nil)

		// Get initial kube context
		initialKubeContext, err := kubeMgr.GetCurrentContext()
		if err != nil {
			logging.Warn("ConnectCmd", "Failed to get initial kube context: %v", err)
			initialKubeContext = "unknown"
		}
		// Log initial context using the appropriate logger if initialized
		logging.Info("CLI", "Initial Kubernetes context: %s", initialKubeContext)

		// Load configuration using the new central loader
		envctlCfg, err := config.LoadConfig(managementClusterArg, workloadClusterArg)
		if err != nil {
			logging.Error("CLI", err, "Failed to load envctl configuration")
			// It might be desirable to allow proceeding with defaults or minimal functionality
			// For now, strict failure.
			return fmt.Errorf("failed to load envctl configuration: %w", err)
		}

		// Check if we should use v2 architecture
		useV2 := os.Getenv("ENVCTL_V2") == "true"
		logging.Info("CLI", "ENVCTL_V2 environment variable: %s, useV2: %v", os.Getenv("ENVCTL_V2"), useV2)

		if noTUI {
			// logging.InitForCLI was already called above.
			logging.Info("CLI", "Running in no-TUI mode.")

			// ... (rest of CLI mode logic using logging.* for its own messages) ...
			if managementClusterArg != "" {
				logging.Info("CLI", "Attempting login to Management Cluster: %s", managementClusterArg)
				stdout, stderr, loginErr := kube.LoginToKubeCluster(managementClusterArg)
				if loginErr != nil {
					logging.Error("CLI", loginErr, "Login to %s failed. Continuing with setup if possible...", managementClusterArg)
				} else {
					// Get current kube context after login
					currentKubeContextAfterLogin, _ := kubeMgr.GetCurrentContext()
					logging.Info("ConnectCmd", "Current kube context after login: %s", currentKubeContextAfterLogin)
					initialKubeContext = currentKubeContextAfterLogin
				}
				if stdout != "" {
					logging.Debug("CLI", "Login stdout: %s", stdout)
				}
				if stderr != "" {
					logging.Debug("CLI", "Login stderr: %s", stderr)
				}
			}

			logging.Info("CLI", "--- Setting up orchestrator for service management ---")

			ctx := context.Background()

			if useV2 {
				logging.Info("CLI", "Using v2 service architecture")

				// Create v2 orchestrator
				orchConfig := orchestrator.ConfigV2{
					MCName:       managementClusterArg,
					WCName:       workloadClusterArg,
					PortForwards: envctlCfg.PortForwards,
					MCPServers:   envctlCfg.MCPServers,
				}
				orchV2 := orchestrator.NewV2(orchConfig)

				// Start orchestrator
				if err := orchV2.Start(ctx); err != nil {
					logging.Error("CLI", err, "Failed to start orchestrator v2")
					return err
				}

				logging.Info("CLI", "Services started with v2 architecture. Press Ctrl+C to stop all services and exit.")

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
				<-sigChan
				logging.Info("CLI", "\n--- Shutting down services ---")
				orchV2.Stop()
				time.Sleep(1 * time.Second)
			} else {
				// Use v1 orchestrator
				consoleReporter := reporting.NewConsoleReporter()
				serviceMgr := managers.NewServiceManager(consoleReporter)

				// Create orchestrator for health monitoring and dependency management
				orch := orchestrator.New(
					serviceMgr,
					consoleReporter,
					orchestrator.Config{
						MCName:              managementClusterArg,
						WCName:              workloadClusterArg,
						PortForwards:        envctlCfg.PortForwards,
						MCPServers:          envctlCfg.MCPServers,
						HealthCheckInterval: 15 * time.Second,
					},
				)

				// Start orchestrator
				if err := orch.Start(ctx); err != nil {
					logging.Error("CLI", err, "Failed to start orchestrator: %v", err)
					return err
				}

				logging.Info("CLI", "Services started with health monitoring. Press Ctrl+C to stop all services and exit.")

				sigChan := make(chan os.Signal, 1)
				signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
				<-sigChan
				logging.Info("CLI", "\n--- Shutting down services ---")
				orch.Stop()
				time.Sleep(1 * time.Second)
			}

		} else { // TUI Mode
			// This fmt.Println is pre-TUI initialization, so it's acceptable.
			logging.Info("CLI", "Starting TUI mode...")
			color.Initialize(true)

			logChan := logging.InitForTUI(appLogLevel)
			defer logging.CloseTUIChannel()

			// Check if we should use v2 architecture - already declared above

			var program *tea.Program
			if useV2 {
				logging.Info("CLI", "Using v2 service architecture")
				p, err := controller.NewProgramV2(
					managementClusterArg,
					workloadClusterArg,
					initialKubeContext,
					envctlCfg,
					logChan,
				)
				if err != nil {
					logging.Error("TUI-Lifecycle", err, "Error creating TUI program v2")
					return err
				}
				program = p
			} else {
				program = controller.NewProgram(
					managementClusterArg,
					workloadClusterArg,
					initialKubeContext,
					debug || tuiDebugMode, // Use either general debug or TUI-specific debug mode
					envctlCfg,
					logChan,
				)
			}

			if _, err := program.Run(); err != nil {
				// Log this error using the TUI logger if possible, or fallback
				logging.Error("TUI-Lifecycle", err, "Error running TUI program")
				return err
			}
			logging.Info("TUI-Lifecycle", "TUI exited.")
		}
		return nil
	},
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
			for _, cluster := range clusterInfo.ManagementClusters {
				candidates = append(candidates, cluster)
			}
		} else if len(args) == 1 {
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

// buildManagedServiceConfigs is a helper to create the config slice for ServiceManager.
func buildManagedServiceConfigs(pfConfigs []config.PortForwardDefinition, mcpConfigs []config.MCPServerDefinition) []managers.ManagedServiceConfig {
	var managedServiceConfigs []managers.ManagedServiceConfig
	for _, pfCfg := range pfConfigs {
		if !pfCfg.Enabled { // Only add enabled port-forwards
			continue
		}
		managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypePortForward,
			Label:  pfCfg.Name, // Using Name from new struct
			Config: pfCfg,
		})
	}
	for _, mcpCfg := range mcpConfigs {
		if !mcpCfg.Enabled { // Only add enabled MCP servers
			continue
		}
		managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeMCPServer,
			Label:  mcpCfg.Name, // Name is already correct
			Config: mcpCfg,
		})
	}
	return managedServiceConfigs
}

func init() {
	rootCmd.AddCommand(connectCmdDef)
	connectCmdDef.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI and run port forwarding in the background")
	// Flag for TUI specific debug features (e.g. showing debug panel in TUI)
	connectCmdDef.Flags().BoolVar(&tuiDebugMode, "debug-tui", false, "Enable TUI debug mode from startup (shows extra logs, debug panel)")
	// General debug flag for more verbose logging across the application, including non-TUI parts if applicable
	connectCmdDef.Flags().BoolVar(&debug, "debug", false, "Enable general debug logging")
}
