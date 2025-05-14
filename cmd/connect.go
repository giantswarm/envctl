package cmd

import (
	"envctl/internal/tui"
	"envctl/internal/utils"
	"fmt"
	"os"

	// "strings" // No longer needed directly here
	// "sync" // No longer directly managing goroutines here

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// Variable to hold the background port-forward process

// connectCmdDef defines the connect command structure
var connectCmdDef = &cobra.Command{
	Use:   "connect <management-cluster> [workload-cluster-shortname]",
	Short: "Connect to Giant Swarm K8s and Prometheus with an interactive TUI",
	Long: `Connects Kubernetes context and sets up port-forwarding for MCP servers.
It provides an interactive terminal user interface to monitor connections.

Provide the management cluster name and, optionally, the short name
of the workload cluster (e.g., 've5v6' for 'enigma-ve5v6').

- Logs into the specified cluster(s).
- Sets the Kubernetes context.
- Starts port-forwarding for Prometheus (management cluster) and 
  AlloyDB (workload cluster, if specified) in the background.
- Displays connection status and logs in an interactive TUI.`,
	Args: cobra.RangeArgs(1, 2),
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

		// Call LoginToKubeCluster and handle its output for the initial non-TUI phase.
		// We want its stdout/stderr to go to the console here.
		mcLoginStdout, mcLoginStderr, err := utils.LoginToKubeCluster(managementCluster)
		if mcLoginStdout != "" {
			fmt.Print(mcLoginStdout) // Print to console
		}
		if mcLoginStderr != "" {
			fmt.Fprint(os.Stderr, mcLoginStderr) // Print to console stderr
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

		// Note: The original `tsh` output is now manually printed above.
		// The TUI will later capture and display `tsh` output for *new* connections made via the UI.

		fmt.Printf("Current Kubernetes context set to: %s\n", teleportContextToUse)

		// --- Print Initial Setup Info (can be displayed in TUI later) ---
		fmt.Println("--------------------------")
		fmt.Println("Setup complete. Initializing TUI...")
		// Provider determination can be moved into the TUI model or passed if needed
		// For now, keeping it simple and focusing on port-forward display

		// Initialize the TUI model
		tuiModel := tui.InitialModel(managementCluster, fullWorkloadClusterName, teleportContextToUse)
		p := tea.NewProgram(tuiModel, tea.WithAltScreen()) // tea.WithOutput(os.Stderr) can be useful for debugging

		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			return err
		}

		fmt.Println("Exited envctl.")
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
			for _, cluster := range clusterInfo.ManagementClusters {
				// This check was commented out in a previous step, ensuring it's still intended or fixing if not.
				// For now, assuming it should include all clusters, not just prefixed ones.
				candidates = append(candidates, cluster)
			}
		} else if len(args) == 1 {
			managementClusterName := args[0]
			if wcShortNames, ok := clusterInfo.WorkloadClusters[managementClusterName]; ok {
				for _, shortName := range wcShortNames {
					// Similar to above, assuming all short names are candidates.
					candidates = append(candidates, shortName)
				}
			}
		}
		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
}

// newConnectCmd creates and returns the connect command
func newConnectCmd() *cobra.Command {
	return connectCmdDef
}
