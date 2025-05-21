package cmd

import (
	"envctl/internal/color"
	"envctl/internal/kube"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/tui/controller"
	"envctl/internal/utils"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// MCP server specific types, variables, and init functions are now in internal/mcpserver

var noTUI bool        // Variable to store the value of the --no-tui flag
var tuiDebugMode bool // Variable to store the value of the --debug-tui flag for TUI

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
		var fullWorkloadClusterIdentifier string
		if len(args) == 2 {
			workloadClusterArg = args[1]
			fullWorkloadClusterIdentifier = managementClusterArg + "-" + workloadClusterArg
		}

		fmt.Println("--- Kubernetes Login ---")

		// Login to Management Cluster if specified
		if managementClusterArg != "" {
			fmt.Printf("Attempting to login to Management Cluster: %s\n", managementClusterArg)
			mcLoginStdout, mcLoginStderr, err := utils.LoginToKubeCluster(managementClusterArg)
			if mcLoginStdout != "" {
				fmt.Print(mcLoginStdout)
			}
			if mcLoginStderr != "" {
				fmt.Fprint(os.Stderr, mcLoginStderr)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to log into management cluster '%s': %v\n", managementClusterArg, err)
			}
		}

		// Login to Workload Cluster if specified
		if fullWorkloadClusterIdentifier != "" {
			fmt.Printf("Attempting to login to Workload Cluster: %s\n", fullWorkloadClusterIdentifier)
			wcLoginStdout, wcLoginStderr, wcErr := utils.LoginToKubeCluster(fullWorkloadClusterIdentifier)
			if wcLoginStdout != "" {
				fmt.Print(wcLoginStdout)
			}
			if wcLoginStderr != "" {
				fmt.Fprint(os.Stderr, wcLoginStderr)
			}
			if wcErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: tsh kube login for %s failed: %v. Stderr: %s\n", fullWorkloadClusterIdentifier, wcErr, wcLoginStderr)
			}
		}

		currentKubeContextAfterLogin, ctxErr := kube.GetCurrentKubeContext()
		if ctxErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get current Kubernetes context after login: %v\n", ctxErr)
			currentKubeContextAfterLogin = ""
		}
		fmt.Printf("Actual Kubernetes context after login: %s\n", currentKubeContextAfterLogin)
		fmt.Println("--------------------------")

		if noTUI {
			fmt.Println("Skipping TUI. Setting up port forwarding and MCP proxies in the background...")

			portForwardingConfig := portforwarding.GetPortForwardConfig(managementClusterArg, workloadClusterArg)

			var wg sync.WaitGroup
			allPortForwardsStopChan := make(chan struct{})
			mcpStopChans := make(map[string]chan struct{})

			portForwardsStarted := false
			if len(portForwardingConfig) > 0 {
				fmt.Println("--- Port Forwarding ---")
				portForwardsStarted = true
				for _, pfCfg := range portForwardingConfig {
					wg.Add(1)
					config := pfCfg
					go func() {
						defer wg.Done()
						fmt.Printf("Attempting to start port-forward for %s on %s to %s:%s (context: %s)...\n",
							config.Label, config.ServiceName, config.LocalPort, config.RemotePort, config.KubeContext)

						sendUpdateFunc := func(status, outputLog string, isError, isReady bool) {
							logPrefix := fmt.Sprintf("[%s] ", config.Label)
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

						portSpec := fmt.Sprintf("%s:%s", config.LocalPort, config.RemotePort)
						individualStopChan, initialStatus, initialErr := kube.StartPortForwardClientGo(
							config.KubeContext,
							config.Namespace,
							config.ServiceName,
							portSpec,
							config.Label,
							sendUpdateFunc,
						)

						if initialErr != nil {
							fmt.Fprintf(os.Stderr, "[%s] Failed to start port-forward: %v. Initial Status: %s\n", config.Label, initialErr, initialStatus)
							return
						}
						if individualStopChan == nil && initialErr == nil {
							fmt.Fprintf(os.Stderr, "[%s] Port-forward setup returned no error but stop channel is nil. Initial Status: %s\n", config.Label, initialStatus)
							return
						}

						fmt.Printf("[%s] Port-forwarding setup initiated. Initial TUI status: %s\n", config.Label, initialStatus)

						select {
						case <-individualStopChan:
							fmt.Printf("[%s] Port-forwarding stopped (individual signal).\n", config.Label)
						case <-allPortForwardsStopChan:
							fmt.Printf("[%s] Stopping port-forwarding (global signal)...\n", config.Label)
							close(individualStopChan)
							fmt.Printf("[%s] Port-forwarding stopped (global signal processed).\n", config.Label)
						}
					}()
				}
			} else {
				fmt.Println("No port forwarding configurations found or defined.")
			}

			fmt.Println("--- MCP Proxies ---")

			// Define the console update function for MCP servers
			consoleMcpUpdateFn := func(update mcpserver.McpProcessUpdate) {
				if update.OutputLog != "" {
					// OutputLog from StartAndManageIndividualMcpServer includes prefixes and full messages
					if strings.Contains(update.OutputLog, "STDERR]") || update.IsError {
						fmt.Fprintln(os.Stderr, update.OutputLog)
					} else {
						fmt.Println(update.OutputLog)
					}
				}
			}

			mcpServerConfig := mcpserver.GetMCPServerConfig()
			managedMcpChan := mcpserver.StartAllMCPServers(mcpServerConfig, consoleMcpUpdateFn, &wg)
			mcpServersAttempted := false
			if len(mcpServerConfig) > 0 {
				mcpServersAttempted = true
			}

			hasSuccessfullyStartedMcps := false
			for serverInfo := range managedMcpChan {
				hasSuccessfullyStartedMcps = true
				if serverInfo.Err != nil {
					fmt.Fprintf(os.Stderr, "[MCP Proxy %s] Failed to initialize: %v\n", serverInfo.Label, serverInfo.Err)
				} else if serverInfo.StopChan != nil {
					mcpStopChans[serverInfo.Label] = serverInfo.StopChan
				} else {
					fmt.Fprintf(os.Stderr, "[MCP Proxy %s] Started without error but StopChan is nil.\n", serverInfo.Label)
				}
			}
			if mcpServersAttempted && !hasSuccessfullyStartedMcps {
				mcpServersAttempted = false
			}

			if portForwardsStarted || mcpServersAttempted {
				fmt.Println("All background processes initiated. Press Ctrl+C to stop.")
			} else {
				fmt.Println("No background processes (port-forwards or MCP proxies) were started. Exiting.")
				return nil
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			select {
			case <-sigChan:
				fmt.Println("\nReceived interrupt signal. Shutting down...")
				if portForwardsStarted {
					fmt.Println("Stopping port forwards...")
					close(allPortForwardsStopChan)
				}
				if mcpServersAttempted {
					fmt.Println("Stopping MCP proxies...")
					for name, stopChan := range mcpStopChans {
						fmt.Printf("[MCP Proxy %s] Signaling proxy to stop...\n", name)
						close(stopChan)
					}
				}
			}

			waitGroupDone := make(chan struct{})
			go func() {
				wg.Wait()
				close(waitGroupDone)
			}()

			select {
			case <-waitGroupDone:
				fmt.Println("All background processes gracefully shut down.")
			case <-time.After(5 * time.Second):
				fmt.Println("Timeout waiting for background processes to shut down. Forcing exit.")
			}

			return nil

		} else {
			fmt.Println("Setup complete. Starting TUI...")

			// Initialize color profile for TUI (force dark mode for now)
			color.Initialize(true)

			mcpServerConfig := mcpserver.GetMCPServerConfig()
			portForwardingConfig := portforwarding.GetPortForwardConfig(managementClusterArg, workloadClusterArg)

			p := controller.NewProgram(managementClusterArg, workloadClusterArg, currentKubeContextAfterLogin, tuiDebugMode, mcpServerConfig, portForwardingConfig)
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
				return err
			}
		}
		return nil
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		clusterInfo, err := utils.GetClusterInfo()
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

func newConnectCmd() *cobra.Command {
	connectCmdDef.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI and run port forwarding in the background")
	connectCmdDef.Flags().BoolVar(&tuiDebugMode, "debug-tui", false, "Enable TUI debug mode from startup (shows extra logs)")
	return connectCmdDef
}

// Removed init() function as MCP server config is no longer initialized here.
