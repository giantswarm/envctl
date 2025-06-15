package cmd

import (
	"envctl/internal/cli"

	"github.com/spf13/cobra"
)

var (
	capabilityOutputFormat string
	capabilityQuiet        bool
)

// capabilityCmd represents the capability command
var capabilityCmd = &cobra.Command{
	Use:   "capability",
	Short: "Manage capability definitions",
	Long: `Manage capability definitions in the envctl environment.

Capabilities define specific functionalities that can be enabled
or disabled based on configuration and dependencies.

Available commands:
  list      - List all capability definitions
  get       - Get detailed information about a specific capability
  available - Check if a capability is available for use

Note: The aggregator server must be running (use 'envctl serve') before using these commands.`,
}

// capabilityListCmd lists all capability definitions
var capabilityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all capability definitions",
	Long: `List all available capability definitions.

This command shows all registered capability definitions,
their availability status, and basic information about each one.`,
	RunE: runCapabilityList,
}

// capabilityGetCmd gets detailed information about a capability
var capabilityGetCmd = &cobra.Command{
	Use:   "get <capability-name>",
	Short: "Get detailed information about a capability",
	Long: `Get detailed information about a specific capability by name.

This shows comprehensive information about the capability including
its definition, dependencies, and configuration requirements.`,
	Args: cobra.ExactArgs(1),
	RunE: runCapabilityGet,
}

// capabilityAvailableCmd checks if a capability is available
var capabilityAvailableCmd = &cobra.Command{
	Use:   "available <capability-name>",
	Short: "Check if a capability is available",
	Long: `Check if a specific capability is available for use.

This verifies that the capability is properly configured and
all its dependencies are satisfied.`,
	Args: cobra.ExactArgs(1),
	RunE: runCapabilityAvailable,
}

func init() {
	rootCmd.AddCommand(capabilityCmd)

	// Add subcommands
	capabilityCmd.AddCommand(capabilityListCmd)
	capabilityCmd.AddCommand(capabilityGetCmd)
	capabilityCmd.AddCommand(capabilityAvailableCmd)

	// Add flags to the parent command
	capabilityCmd.PersistentFlags().StringVarP(&capabilityOutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	capabilityCmd.PersistentFlags().BoolVarP(&capabilityQuiet, "quiet", "q", false, "Suppress non-essential output")
}

func runCapabilityList(cmd *cobra.Command, args []string) error {
	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(capabilityOutputFormat),
		Quiet:  capabilityQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	return executor.Execute(ctx, "core_capability_list", nil)
}

func runCapabilityGet(cmd *cobra.Command, args []string) error {
	capabilityName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(capabilityOutputFormat),
		Quiet:  capabilityQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	args_map := map[string]interface{}{
		"name": capabilityName,
	}

	return executor.Execute(ctx, "core_capability_get", args_map)
}

func runCapabilityAvailable(cmd *cobra.Command, args []string) error {
	capabilityName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(capabilityOutputFormat),
		Quiet:  capabilityQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	args_map := map[string]interface{}{
		"name": capabilityName,
	}

	return executor.Execute(ctx, "core_capability_available", args_map)
} 