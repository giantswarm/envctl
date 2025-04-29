package cmd

import (
	"envctl/internal/utils"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// Variable to hold the background port-forward process
var portForwardCmd *exec.Cmd

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect <management-cluster> [workload-cluster-shortname]",
	Short: "Connect to Giant Swarm K8s and Prometheus",
	Long: `Connects Kubernetes context and Prometheus for MCP servers.

Provide the management cluster name and, optionally, the short name
of the workload cluster (e.g., 've5v6' for 'enigma-ve5v6').

- If only <management-cluster> is provided:
  Logs into the management cluster.
  Starts Prometheus port-forwarding using the management cluster context.
  Sets the current Kubernetes context to the management cluster.

- If <management-cluster> and [workload-cluster-shortname] are provided:
  Logs into both the management cluster and the full workload cluster (e.g., 'enigma-ve5v6').
  Starts Prometheus port-forwarding using the management cluster context in the background.
  Sets the current Kubernetes context to the full workload cluster name.`,
	Args: cobra.RangeArgs(1, 2), // Accepts 1 or 2 arguments
	RunE: func(cmd *cobra.Command, args []string) error {
		managementCluster := args[0]
		shortWorkloadClusterName := ""
		fullWorkloadClusterName := ""

		if len(args) == 2 {
			shortWorkloadClusterName = args[1]
			// Construct the full name for internal use (login, context setting)
			fullWorkloadClusterName = managementCluster + "-" + shortWorkloadClusterName
		}

		// Stop previous port-forward if any
		if portForwardCmd != nil && portForwardCmd.Process != nil {
			fmt.Println("Stopping previous port-forward process...")
			err := utils.StopProcess(portForwardCmd.Process)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not stop previous port-forward: %v\n", err)
			}
			portForwardCmd = nil
		}

		// --- Login Logic ---
		fmt.Println("--- Kubernetes Login ---")
		err := utils.LoginToKubeCluster(managementCluster)
		if err != nil {
			return fmt.Errorf("failed to log into management cluster '%s': %w", managementCluster, err)
		}

		kubeContextToUse := managementCluster
		if fullWorkloadClusterName != "" {
			// Log into the *full* workload cluster name.
			err = utils.LoginToKubeCluster(fullWorkloadClusterName)
			if err != nil {
				// Include the short name in the error message for user clarity
				return fmt.Errorf("failed to log into workload cluster '%s' (short name '%s'): %w", fullWorkloadClusterName, shortWorkloadClusterName, err)
			}
			kubeContextToUse = fullWorkloadClusterName // Context is the full name
		}

		fmt.Printf("Current Kubernetes context set to: %s\n", kubeContextToUse)

		// --- Prometheus Port-Forward ---
		fmt.Println("--- Prometheus Connection ---")
		var pfErr error
		portForwardCmd, pfErr = utils.StartPrometheusPortForward(managementCluster)
		if pfErr != nil {
			return fmt.Errorf("failed to start Prometheus port-forward using context '%s': %w", managementCluster, pfErr)
		}

		fmt.Println("--------------------------")
		fmt.Println("Setup complete:")
		fmt.Printf("- Kubernetes context: %s", kubeContextToUse)
		if fullWorkloadClusterName != "" {
			fmt.Printf(" (connected via short name: %s)\n", shortWorkloadClusterName)
		} else {
			fmt.Println()
		}
		fmt.Printf("- Prometheus port-forward: Active (via %s context)\n", managementCluster)
		fmt.Println("--------------------------")

		return nil
	},
	// Add dynamic completion for cluster names
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		clusterInfo, err := utils.GetClusterInfo()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Completion error: %v\n", err)
			return nil, cobra.ShellCompDirectiveError
		}

		var candidates []string
		if len(args) == 0 {
			// Completing the first argument (management cluster)
			for _, cluster := range clusterInfo.ManagementClusters {
				if strings.HasPrefix(cluster, toComplete) {
					candidates = append(candidates, cluster)
				}
			}
		} else if len(args) == 1 {
			// Completing the second argument (workload cluster short name)
			managementCluster := args[0]
			if wcShortNames, ok := clusterInfo.WorkloadClusters[managementCluster]; ok {
				for _, shortName := range wcShortNames {
					if strings.HasPrefix(shortName, toComplete) {
						candidates = append(candidates, shortName)
					}
				}
			}
		}

		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
}
