package cmd

import (
	"envctl/internal/cli"

	"github.com/spf13/cobra"
)

var (
	serviceclassOutputFormat string
	serviceclassQuiet        bool
)

// serviceclassCmd represents the serviceclass command
var serviceclassCmd = &cobra.Command{
	Use:   "serviceclass",
	Short: "Manage ServiceClass definitions",
	Long: `Manage ServiceClass definitions in the envctl environment.

ServiceClasses define templates for creating service instances with
specific configurations and capabilities.

Available commands:
  list      - List all ServiceClass definitions
  get       - Get detailed information about a specific ServiceClass
  available - Check if a ServiceClass is available for use

Note: The aggregator server must be running (use 'envctl serve') before using these commands.`,
}

// serviceclassListCmd lists all ServiceClass definitions
var serviceclassListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all ServiceClass definitions",
	Long: `List all available ServiceClass definitions.

This command shows all registered ServiceClass definitions,
their availability status, and basic information about each one.`,
	RunE: runServiceClassList,
}

// serviceclassGetCmd gets detailed information about a ServiceClass
var serviceclassGetCmd = &cobra.Command{
	Use:   "get <serviceclass-name>",
	Short: "Get detailed information about a ServiceClass",
	Long: `Get detailed information about a specific ServiceClass by name.

This shows comprehensive information about the ServiceClass including
its definition, parameters, and configuration options.`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceClassGet,
}

// serviceclassAvailableCmd checks if a ServiceClass is available
var serviceclassAvailableCmd = &cobra.Command{
	Use:   "available <serviceclass-name>",
	Short: "Check if a ServiceClass is available",
	Long: `Check if a specific ServiceClass is available for use.

This verifies that the ServiceClass is properly configured and
all its dependencies are satisfied.`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceClassAvailable,
}

func init() {
	rootCmd.AddCommand(serviceclassCmd)

	// Add subcommands
	serviceclassCmd.AddCommand(serviceclassListCmd)
	serviceclassCmd.AddCommand(serviceclassGetCmd)
	serviceclassCmd.AddCommand(serviceclassAvailableCmd)

	// Add flags to the parent command
	serviceclassCmd.PersistentFlags().StringVarP(&serviceclassOutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	serviceclassCmd.PersistentFlags().BoolVarP(&serviceclassQuiet, "quiet", "q", false, "Suppress non-essential output")
}

func runServiceClassList(cmd *cobra.Command, args []string) error {
	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceclassOutputFormat),
		Quiet:  serviceclassQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	return executor.Execute(ctx, "core_serviceclass_list", nil)
}

func runServiceClassGet(cmd *cobra.Command, args []string) error {
	serviceclassName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceclassOutputFormat),
		Quiet:  serviceclassQuiet,
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
		"name": serviceclassName,
	}

	return executor.Execute(ctx, "core_serviceclass_get", args_map)
}

func runServiceClassAvailable(cmd *cobra.Command, args []string) error {
	serviceclassName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceclassOutputFormat),
		Quiet:  serviceclassQuiet,
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
		"name": serviceclassName,
	}

	return executor.Execute(ctx, "core_serviceclass_available", args_map)
} 