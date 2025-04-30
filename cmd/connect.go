package cmd

import (
	"envctl/internal/utils"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Variable to hold the background port-forward process

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect <management-cluster> [workload-cluster-shortname]",
	Short: "Connect to Giant Swarm K8s and Prometheus",
	Long: `Connects Kubernetes context and Prometheus for MCP servers.

Provide the management cluster name and, optionally, the short name
of the workload cluster (e.g., 've5v6' for 'enigma-ve5v6').

- If only <management-cluster> is provided:
  Logs into the management cluster.
  Sets the current Kubernetes context to the management cluster.
  Starts Prometheus port-forwarding using the management cluster context,
  running in the foreground until interrupted (Ctrl+C).

- If <management-cluster> and [workload-cluster-shortname] are provided:
  Logs into both the management cluster and the full workload cluster (e.g., 'enigma-ve5v6').
  Sets the current Kubernetes context to the full workload cluster name.
  Starts Prometheus port-forwarding using the management cluster context in the foreground
  until interrupted (Ctrl+C).`, // Updated help text
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

		err := utils.LoginToKubeCluster(managementCluster)
		if err != nil {
			return fmt.Errorf("failed to log into management cluster '%s': %w", managementCluster, err)
		}

		teleportContextToUse := "teleport.giantswarm.io-" + managementCluster

		if fullWorkloadClusterName != "" {
			err = utils.LoginToKubeCluster(fullWorkloadClusterName)
			if err != nil {
				return fmt.Errorf("failed to log into workload cluster '%s' (short name '%s'): %w", fullWorkloadClusterName, shortWorkloadClusterName, err)
			}
			teleportContextToUse = "teleport.giantswarm.io-" + fullWorkloadClusterName
		}

		fmt.Printf("Current Kubernetes context set to: %s\n", teleportContextToUse)

		// --- Print Setup Info Before Starting Blocking Port-Forward ---
		fmt.Println("--------------------------")
		fmt.Println("Setup complete. Starting Prometheus port-forward...") // Moved messages
		fmt.Printf("- Kubernetes context: %s", teleportContextToUse)
		if fullWorkloadClusterName != "" {
			fmt.Printf(" (connected via short name: %s)\n", shortWorkloadClusterName)
		} else {
			fmt.Println()
		}
		fmt.Printf("- Prometheus port-forward will run via %s context (full name: teleport.giantswarm.io-%s)\n", managementCluster, managementCluster)
		fmt.Println("--------------------------")

		// --- Prometheus Port-Forward (Now Blocking) ---
		fmt.Println("--- Prometheus Connection ---")
		pfErr := utils.StartPrometheusPortForward(managementCluster) // New call, blocks here
		if pfErr != nil {
			// Error is already printed within StartPrometheusPortForward,
			// but we return it to signal failure to cobra.
			// Could add more context here if needed.
			return fmt.Errorf("prometheus port-forward failed: %w", pfErr)
		}

		// Code here will only be reached if port-forward exits cleanly (rarely happens without error)
		fmt.Println("Prometheus port-forwarding finished.")
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
