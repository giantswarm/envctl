package cmd

import (
	"envctl/internal/tui"
	"envctl/internal/utils"
	"fmt"
	"os"

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

		// The TUI will handle subsequent 'tsh' output for connections made via the UI.
		// This initial login output is explicitly handled before the TUI starts.

		fmt.Printf("Current Kubernetes context set to: %s\n", teleportContextToUse)

		// --- Print Initial Setup Info (can be displayed in TUI later) ---
		fmt.Println("--------------------------")
		fmt.Println("Setup complete. Starting TUI...") // Updated message
		
		initialModel := tui.InitialModel(managementCluster, fullWorkloadClusterName, teleportContextToUse)
		p := tea.NewProgram(initialModel, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\\n", err)
			return err
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
	return connectCmdDef
}
