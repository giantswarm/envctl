package cmd

import (
	"github.com/giantswarm/envctl/internal/tui"
	"github.com/giantswarm/envctl/internal/utils"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Variable to hold the background port-forward process

var noTUI bool // Variable to store the value of the --no-tui flag

// connectCmdDef defines the connect command structure
var connectCmdDef = &cobra.Command{
	Use:   "connect <management-cluster> [workload-cluster-shortname]",
	Short: "Connect to Giant Swarm K8s and managed services with an interactive TUI or CLI mode.",
	Long: `Connects to Giant Swarm Kubernetes clusters, sets the kubectl context, and manages port-forwarding for essential services.
It can run in two modes:

1. Interactive TUI Mode (default):
   - Launches a terminal user interface to monitor connections, cluster health, and manage port-forwards.
   - Logs into the specified Management Cluster (MC) and, if provided, the Workload Cluster (WC).
   - Sets the kubectl context (to WC if specified, otherwise MC).
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
		managementCluster := args[0]
		shortWorkloadClusterName := ""
		fullWorkloadClusterName := ""

		if len(args) == 2 {
			shortWorkloadClusterName = args[1]
			fullWorkloadClusterName = managementCluster + "-" + shortWorkloadClusterName
		}

		// --- Login Logic ---
		fmt.Println("--- Kubernetes Login ---")

		// Initial login to the management cluster. The output of this initial login
		// is printed directly to the console, as the TUI is not yet running.
		mcLoginStdout, mcLoginStderr, err := utils.LoginToKubeCluster(managementCluster)
		if mcLoginStdout != "" {
			fmt.Print(mcLoginStdout) // Print stdout to console
		}
		if mcLoginStderr != "" {
			fmt.Fprint(os.Stderr, mcLoginStderr) // Print stderr to console
		}
		if err != nil {
			return fmt.Errorf("failed to log into management cluster '%s': %w", managementCluster, err)
		}

		teleportContextToUse := "teleport.giantswarm.io-" + managementCluster

		if fullWorkloadClusterName != "" {
			wcLoginStdout, wcLoginStderr, wcErr := utils.LoginToKubeCluster(fullWorkloadClusterName)
			if wcLoginStdout != "" {
				fmt.Print(wcLoginStdout)
			}
			if wcLoginStderr != "" {
				fmt.Fprint(os.Stderr, wcLoginStderr)
			}
			if wcErr != nil {
				return fmt.Errorf("failed to log into workload cluster '%s' (short name '%s'): %w", fullWorkloadClusterName, shortWorkloadClusterName, wcErr)
			}
			teleportContextToUse = "teleport.giantswarm.io-" + fullWorkloadClusterName
		}

		fmt.Printf("Current Kubernetes context set to: %s\n", teleportContextToUse)
		fmt.Println("--------------------------")

		if noTUI {
			fmt.Println("Skipping TUI. Setting up port forwarding in the background...")
			// Placeholder for non-TUI port forwarding logic
			// This will involve calling a modified version of port forwarding setup

			// Get port forwarding configurations
			configs := getPortForwardConfigs(managementCluster, fullWorkloadClusterName, teleportContextToUse)
			if len(configs) == 0 {
				fmt.Println("No port forwarding configurations found. Exiting.")
				return nil
			}

			var wg sync.WaitGroup
			stopChannels := make([]chan struct{}, 0)
			allStopChan := make(chan struct{}) // Single channel to signal all goroutines

			for _, pfConfig := range configs {
				wg.Add(1)
				// Use a local copy of pfConfig for the goroutine
				config := pfConfig
				go func() {
					defer wg.Done()
					fmt.Printf("Attempting to start port-forward for %s on %s to %s:%s (context: %s)...\n",
						config.label, config.service, config.localPort, config.remotePort, config.kubeContext)

					// Simple console logger for updates
					sendUpdateFunc := func(status, outputLog string, isError, isReady bool) {
						logPrefix := fmt.Sprintf("[%s] ", config.label)
						if isError {
							fmt.Printf("%sERROR: %s %s\n", logPrefix, status, outputLog)
						} else if isReady {
							fmt.Printf("%sREADY: %s %s\n", logPrefix, status, outputLog)
						} else if outputLog != "" {
							fmt.Printf("%sLOG: %s\n", logPrefix, outputLog)
						} else if status != "" {
							fmt.Printf("%sSTATUS: %s\n", logPrefix, status)
						}
					}

					// StartPortForwardClientGo expects localPort:remotePort format
					portSpec := fmt.Sprintf("%s:%s", config.localPort, config.remotePort)

					// Start the port-forwarding
					// Note: StartPortForwardClientGo returns (stopChan, initialStatus, initialError)
					// We need to handle the initialStatus and initialError appropriately.
					individualStopChan, initialStatus, initialErr := utils.StartPortForwardClientGo(
						config.kubeContext,
						config.namespace,
						config.service, // Service name e.g. "service/mimir-query-frontend"
						portSpec,
						config.label,
						sendUpdateFunc,
					)

					if initialErr != nil {
						fmt.Fprintf(os.Stderr, "[%s] Failed to start port-forward: %v. Initial Status: %s\n", config.label, initialErr, initialStatus)
						return // Don't try to manage stopChan if setup failed
					}
					if individualStopChan == nil && initialErr == nil {
						// This case should ideally be covered by initialErr, but as a safeguard:
						fmt.Fprintf(os.Stderr, "[%s] Port-forward setup returned no error but stop channel is nil. Initial Status: %s\n", config.label, initialStatus)
						return
					}

					fmt.Printf("[%s] Port-forwarding setup initiated. Initial TUI status: %s\n", config.label, initialStatus)
					stopChannels = append(stopChannels, individualStopChan) // Add to a shared slice (needs mutex if accessed by main goroutine concurrently, but here it's fine)

					// Wait for either the individual stop or the global stop signal
					select {
					case <-individualStopChan: // If the port-forward stops on its own (e.g. error)
						fmt.Printf("[%s] Port-forwarding stopped (individual signal).\n", config.label)
					case <-allStopChan: // If global shutdown is triggered
						fmt.Printf("[%s] Stopping port-forwarding (global signal)...\n", config.label)
						close(individualStopChan) // Signal the specific port-forward to stop
						fmt.Printf("[%s] Port-forwarding stopped (global signal processed).\n", config.label)
					}
				}()
			}

			fmt.Println("All port-forwarding processes initiated. Press Ctrl+C to stop.")

			// Wait for interrupt signal
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			// Block until a signal is received or all port forwards complete (less likely for long-running PFs)
			select {
			case <-sigChan:
				fmt.Println("\nReceived interrupt signal. Shutting down port forwards...")
				close(allStopChan) // Signal all goroutines to stop
			}

			wg.Wait() // Wait for all port-forwarding goroutines to finish
			fmt.Println("All port forwards gracefully shut down.")
			return nil

		} else {
			fmt.Println("Setup complete. Starting TUI...") // Updated message

			_ = lipgloss.HasDarkBackground()

			initialModel := tui.InitialModel(managementCluster, fullWorkloadClusterName, teleportContextToUse)
			p := tea.NewProgram(initialModel, tea.WithAltScreen(), tea.WithMouseAllMotion())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				return err
			}
		}
		return nil
	},
	// ValidArgsFunction provides dynamic command-line completion for cluster names.
	// It fetches available management and workload clusters to suggest to the user.
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		clusterInfo, err := utils.GetClusterInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Completion error: %v\n", err)
			return nil, cobra.ShellCompDirectiveError
		}

		var candidates []string
		if len(args) == 0 {
			for _, cluster := range clusterInfo.ManagementClusters {
				// Provide all fetched management clusters as completion candidates.
				candidates = append(candidates, cluster)
			}
		} else if len(args) == 1 {
			managementClusterName := args[0]
			if wcShortNames, ok := clusterInfo.WorkloadClusters[managementClusterName]; ok {
				for _, shortName := range wcShortNames {
					// Provide all short names for the given management cluster as candidates.
					candidates = append(candidates, shortName)
				}
			}
		}
		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
}

// newConnectCmd creates and returns the connect command
// This function encapsulates the command definition for better organization.
func newConnectCmd() *cobra.Command {
	// Add the --no-tui flag
	connectCmdDef.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI and run port forwarding in the background")
	return connectCmdDef
}

// portForwardConfig holds the necessary details for a single port-forwarding operation
// when running without the TUI.
type portForwardConfig struct {
	label       string
	localPort   string
	remotePort  string
	kubeContext string
	namespace   string
	service     string // e.g., "service/mimir-query-frontend" or "mimir-query-frontend" if utils expects that
}

// getPortForwardConfigs defines the port forwarding configurations.
// This is similar to what setupPortForwards does in the TUI, but adapted for non-TUI mode.
func getPortForwardConfigs(mcName, wcName, baseKubeContext string) []portForwardConfig {
	configs := make([]portForwardConfig, 0)

	mcKubeContext := "teleport.giantswarm.io-" + mcName
	var wcKubeContext string
	if wcName != "" {
		wcKubeContext = "teleport.giantswarm.io-" + wcName // wcName is already full here e.g. mc-wc
	}

	// Prometheus for MC
	if mcName != "" {
		configs = append(configs, portForwardConfig{
			label:       "Prometheus (MC)",
			localPort:   "8080",
			remotePort:  "8080",
			kubeContext: mcKubeContext,
			namespace:   "mimir",
			service:     "service/mimir-query-frontend",
		})
		// Grafana for MC
		configs = append(configs, portForwardConfig{
			label:       "Grafana (MC)",
			localPort:   "3000",
			remotePort:  "3000",
			kubeContext: mcKubeContext,
			namespace:   "monitoring",
			service:     "service/grafana",
		})
	}

	// Alloy Metrics for WC (if wcName is provided) or MC (if wcName is not provided)
	alloyLabel := "Alloy Metrics"
	alloyContext := mcKubeContext // Default to MC context

	if wcName != "" {
		alloyLabel += " (WC)"
		alloyContext = wcKubeContext
	} else {
		alloyLabel += " (MC)"
	}

	configs = append(configs, portForwardConfig{
		label:       alloyLabel,
		localPort:   "12345",
		remotePort:  "12345",
		kubeContext: alloyContext,
		namespace:   "kube-system",
		service:     "service/alloy-metrics-cluster",
	})

	return configs
}
