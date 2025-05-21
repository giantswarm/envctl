package cmd

import (
	"envctl/internal/color"
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
	"envctl/internal/tui/controller"
	"envctl/internal/utils"
	"fmt"
	"os"
	"os/signal"
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

		kubeMgr := k8smanager.NewKubeManager()

		fmt.Println("--- Kubernetes Login ---")

		if managementClusterArg != "" {
			fmt.Printf("Attempting to login to Management Cluster: %s\n", managementClusterArg)
			mcLoginStdout, mcLoginStderr, err := kubeMgr.Login(managementClusterArg)
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

		if fullWorkloadClusterIdentifier != "" {
			fmt.Printf("Attempting to login to Workload Cluster: %s\n", fullWorkloadClusterIdentifier)
			wcLoginStdout, wcLoginStderr, wcErr := kubeMgr.Login(fullWorkloadClusterIdentifier)
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

		currentKubeContextAfterLogin, ctxErr := kubeMgr.GetCurrentContext()
		if ctxErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get current Kubernetes context after login: %v\n", ctxErr)
			currentKubeContextAfterLogin = ""
		}
		fmt.Printf("Actual Kubernetes context after login: %s\n", currentKubeContextAfterLogin)
		fmt.Println("--------------------------")

		portForwardingConfig := portforwarding.GetPortForwardConfig(managementClusterArg, workloadClusterArg)
		mcpServerConfig := mcpserver.GetMCPServerConfig()

		if noTUI {
			fmt.Println("Skipping TUI. Setting up services in the background...")

			var wg sync.WaitGroup
			consoleReporter := reporting.NewConsoleReporter()
			serviceMgr := managers.NewServiceManager(consoleReporter)

			var managedServiceConfigs []managers.ManagedServiceConfig
			for _, pfCfg := range portForwardingConfig {
				managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
					Type:   reporting.ServiceTypePortForward,
					Label:  pfCfg.Label,
					Config: pfCfg,
				})
			}
			for _, mcpCfg := range mcpServerConfig {
				managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
					Type:   reporting.ServiceTypeMCPServer,
					Label:  mcpCfg.Name,
					Config: mcpCfg,
				})
			}

			if len(managedServiceConfigs) > 0 {
				fmt.Println("--- Starting Background Services ---")
				_, startupErrors := serviceMgr.StartServices(managedServiceConfigs, &wg)
				if len(startupErrors) > 0 {
					consoleReporter.Report(reporting.ManagedServiceUpdate{
						Timestamp:   time.Now(),
						SourceType:  reporting.ServiceTypeSystem,
						SourceLabel: "ServiceManagerInit",
						Level:       reporting.LogLevelError,
						Message:     "Errors during initial service startup configuration phase:",
					})
					for _, err := range startupErrors {
						consoleReporter.Report(reporting.ManagedServiceUpdate{
							Timestamp:   time.Now(),
							SourceType:  reporting.ServiceTypeSystem,
							SourceLabel: "ServiceStartupError",
							Level:       reporting.LogLevelError,
							Message:     err.Error(),
							ErrorDetail: err,
						})
					}
				}
				fmt.Println("All background services initiated. Press Ctrl+C to stop.")
			} else {
				fmt.Println("No background services (port-forwards or MCP proxies) were configured. Exiting.")
				return nil
			}

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			<-sigChan

			fmt.Println("\nReceived interrupt signal. Shutting down services...")
			serviceMgr.StopAllServices()

			waitGroupDone := make(chan struct{})
			go func() {
				wg.Wait()
				close(waitGroupDone)
			}()

			select {
			case <-waitGroupDone:
				fmt.Println("All background services gracefully shut down.")
			case <-time.After(10 * time.Second):
				fmt.Println("Timeout waiting for background services to shut down. Forcing exit.")
			}

			return nil

		} else {
			fmt.Println("Setup complete. Starting TUI...")
			color.Initialize(true)

			p := controller.NewProgram(
				managementClusterArg,
				workloadClusterArg,
				currentKubeContextAfterLogin,
				tuiDebugMode,
				mcpServerConfig,
				portForwardingConfig,
				kubeMgr,
			)
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
