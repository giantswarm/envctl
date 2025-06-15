package cmd

import (
	"envctl/internal/cli"

	"github.com/spf13/cobra"
)

var (
	mcpserverOutputFormat string
	mcpserverQuiet        bool
)

// mcpserverCmd represents the mcpserver command
var mcpserverCmd = &cobra.Command{
	Use:   "mcpserver",
	Short: "Manage MCP server definitions",
	Long: `Manage MCP server definitions in the envctl environment.

MCP servers provide tools and capabilities that can be accessed
through the Model Context Protocol (MCP).

Available commands:
  list      - List all MCP server definitions
  get       - Get detailed information about a specific MCP server
  available - Check if an MCP server is available for use

Note: The aggregator server must be running (use 'envctl serve') before using these commands.`,
}

// mcpserverListCmd lists all MCP server definitions
var mcpserverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all MCP server definitions",
	Long: `List all available MCP server definitions.

This command shows all registered MCP server definitions,
their availability status, and basic information about each one.`,
	RunE: runMCPServerList,
}

// mcpserverGetCmd gets detailed information about an MCP server
var mcpserverGetCmd = &cobra.Command{
	Use:   "get <mcpserver-name>",
	Short: "Get detailed information about an MCP server",
	Long: `Get detailed information about a specific MCP server by name.

This shows comprehensive information about the MCP server including
its definition, configuration, and available tools.`,
	Args: cobra.ExactArgs(1),
	RunE: runMCPServerGet,
}

// mcpserverAvailableCmd checks if an MCP server is available
var mcpserverAvailableCmd = &cobra.Command{
	Use:   "available <mcpserver-name>",
	Short: "Check if an MCP server is available",
	Long: `Check if a specific MCP server is available for use.

This verifies that the MCP server is properly configured and
all its dependencies are satisfied.`,
	Args: cobra.ExactArgs(1),
	RunE: runMCPServerAvailable,
}

func init() {
	rootCmd.AddCommand(mcpserverCmd)

	// Add subcommands
	mcpserverCmd.AddCommand(mcpserverListCmd)
	mcpserverCmd.AddCommand(mcpserverGetCmd)
	mcpserverCmd.AddCommand(mcpserverAvailableCmd)

	// Add flags to the parent command
	mcpserverCmd.PersistentFlags().StringVarP(&mcpserverOutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	mcpserverCmd.PersistentFlags().BoolVarP(&mcpserverQuiet, "quiet", "q", false, "Suppress non-essential output")
}

func runMCPServerList(cmd *cobra.Command, args []string) error {
	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(mcpserverOutputFormat),
		Quiet:  mcpserverQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	return executor.Execute(ctx, "core_mcpserver_list", nil)
}

func runMCPServerGet(cmd *cobra.Command, args []string) error {
	mcpserverName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(mcpserverOutputFormat),
		Quiet:  mcpserverQuiet,
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
		"name": mcpserverName,
	}

	return executor.Execute(ctx, "core_mcpserver_get", args_map)
}

func runMCPServerAvailable(cmd *cobra.Command, args []string) error {
	mcpserverName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(mcpserverOutputFormat),
		Quiet:  mcpserverQuiet,
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
		"name": mcpserverName,
	}

	return executor.Execute(ctx, "core_mcpserver_available", args_map)
} 