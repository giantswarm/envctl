package cmd

import (
	"envctl/internal/cli"

	"github.com/spf13/cobra"
)

var (
	serviceOutputFormat string
	serviceQuiet        bool
)

// serviceCmd represents the service command
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage services",
	Long: `Manage services in the envctl environment.

Services are the core components that provide functionality to envctl,
including MCP servers, port forwarding, and other infrastructure services.

Available commands:
  list     - List all services with their status
  start    - Start a service
  stop     - Stop a service
  restart  - Restart a service
  status   - Get detailed status of a service

Note: The aggregator server must be running (use 'envctl serve') before using these commands.`,
}

// serviceListCmd lists all services
var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all services",
	Long: `List all services with their current status.

This command shows all registered services, their health status,
and basic information about each service.`,
	RunE: runServiceList,
}

// serviceStartCmd starts a service
var serviceStartCmd = &cobra.Command{
	Use:   "start <service-label>",
	Short: "Start a service",
	Long: `Start a specific service by its label.

The service label is the unique identifier for the service.
Use 'envctl service list' to see available services and their labels.`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceStart,
}

// serviceStopCmd stops a service
var serviceStopCmd = &cobra.Command{
	Use:   "stop <service-label>",
	Short: "Stop a service",
	Long: `Stop a specific service by its label.

The service label is the unique identifier for the service.
Use 'envctl service list' to see available services and their labels.`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceStop,
}

// serviceRestartCmd restarts a service
var serviceRestartCmd = &cobra.Command{
	Use:   "restart <service-label>",
	Short: "Restart a service",
	Long: `Restart a specific service by its label.

This will stop the service if it's running and then start it again.
The service label is the unique identifier for the service.
Use 'envctl service list' to see available services and their labels.`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceRestart,
}

// serviceStatusCmd gets detailed status of a service
var serviceStatusCmd = &cobra.Command{
	Use:   "status <service-label>",
	Short: "Get detailed status of a service",
	Long: `Get detailed status information for a specific service by its label.

This shows comprehensive information about the service including
health status, configuration, and runtime details.

The service label is the unique identifier for the service.
Use 'envctl service list' to see available services and their labels.`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceStatus,
}

func init() {
	rootCmd.AddCommand(serviceCmd)

	// Add subcommands
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceStatusCmd)

	// Add flags to the parent command
	serviceCmd.PersistentFlags().StringVarP(&serviceOutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	serviceCmd.PersistentFlags().BoolVarP(&serviceQuiet, "quiet", "q", false, "Suppress non-essential output")
}

func runServiceList(cmd *cobra.Command, args []string) error {
	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceOutputFormat),
		Quiet:  serviceQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	return executor.Execute(ctx, "core_service_list", nil)
}

func runServiceStart(cmd *cobra.Command, args []string) error {
	serviceLabel := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceOutputFormat),
		Quiet:  serviceQuiet,
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
		"label": serviceLabel,
	}

	return executor.Execute(ctx, "core_service_start", args_map)
}

func runServiceStop(cmd *cobra.Command, args []string) error {
	serviceLabel := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceOutputFormat),
		Quiet:  serviceQuiet,
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
		"label": serviceLabel,
	}

	return executor.Execute(ctx, "core_service_stop", args_map)
}

func runServiceRestart(cmd *cobra.Command, args []string) error {
	serviceLabel := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceOutputFormat),
		Quiet:  serviceQuiet,
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
		"label": serviceLabel,
	}

	return executor.Execute(ctx, "core_service_restart", args_map)
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	serviceLabel := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(serviceOutputFormat),
		Quiet:  serviceQuiet,
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
		"label": serviceLabel,
	}

	return executor.Execute(ctx, "core_service_status", args_map)
}
