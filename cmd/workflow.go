package cmd

import (
	"envctl/internal/cli"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var (
	workflowOutputFormat string
	workflowQuiet        bool
)

// workflowCmd represents the workflow command
var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Manage workflow definitions",
	Long: `Manage workflow definitions in the envctl environment.

Workflows define sequences of operations that can be executed
to automate complex tasks and processes.

Available commands:
  list     - List all workflow definitions
  get      - Get detailed information about a specific workflow
  create   - Create a new workflow from a definition file
  update   - Update an existing workflow
  delete   - Delete a workflow
  validate - Validate a workflow definition

Note: The aggregator server must be running (use 'envctl serve') before using these commands.`,
}

// workflowListCmd lists all workflow definitions
var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflow definitions",
	Long: `List all available workflow definitions.

This command shows all registered workflow definitions
and basic information about each one.`,
	RunE: runWorkflowList,
}

// workflowGetCmd gets detailed information about a workflow
var workflowGetCmd = &cobra.Command{
	Use:   "get <workflow-name>",
	Short: "Get detailed information about a workflow",
	Long: `Get detailed information about a specific workflow by name.

This shows comprehensive information about the workflow including
its definition, steps, and configuration.`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowGet,
}

// workflowCreateCmd creates a new workflow
var workflowCreateCmd = &cobra.Command{
	Use:   "create <workflow-file>",
	Short: "Create a new workflow from a definition file",
	Long: `Create a new workflow from a YAML definition file.

The workflow file should contain a valid workflow definition
in YAML format. Use '-' to read from stdin.`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowCreate,
}

// workflowUpdateCmd updates an existing workflow
var workflowUpdateCmd = &cobra.Command{
	Use:   "update <workflow-name> <workflow-file>",
	Short: "Update an existing workflow",
	Long: `Update an existing workflow with a new definition from a YAML file.

The workflow file should contain a valid workflow definition
in YAML format. Use '-' to read from stdin.`,
	Args: cobra.ExactArgs(2),
	RunE: runWorkflowUpdate,
}

// workflowDeleteCmd deletes a workflow
var workflowDeleteCmd = &cobra.Command{
	Use:   "delete <workflow-name>",
	Short: "Delete a workflow",
	Long: `Delete a specific workflow by name.

This will permanently remove the workflow definition.`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowDelete,
}

// workflowValidateCmd validates a workflow definition
var workflowValidateCmd = &cobra.Command{
	Use:   "validate <workflow-file>",
	Short: "Validate a workflow definition",
	Long: `Validate a workflow definition file without creating it.

The workflow file should contain a valid workflow definition
in YAML format. Use '-' to read from stdin.`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflowValidate,
}

func init() {
	rootCmd.AddCommand(workflowCmd)

	// Add subcommands
	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowGetCmd)
	workflowCmd.AddCommand(workflowCreateCmd)
	workflowCmd.AddCommand(workflowUpdateCmd)
	workflowCmd.AddCommand(workflowDeleteCmd)
	workflowCmd.AddCommand(workflowValidateCmd)

	// Add flags to the parent command
	workflowCmd.PersistentFlags().StringVarP(&workflowOutputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	workflowCmd.PersistentFlags().BoolVarP(&workflowQuiet, "quiet", "q", false, "Suppress non-essential output")
}

func runWorkflowList(cmd *cobra.Command, args []string) error {
	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(workflowOutputFormat),
		Quiet:  workflowQuiet,
	})
	if err != nil {
		return err
	}
	defer executor.Close()

	ctx := cmd.Context()
	if err := executor.Connect(ctx); err != nil {
		return err
	}

	return executor.Execute(ctx, "core_workflow_list", nil)
}

func runWorkflowGet(cmd *cobra.Command, args []string) error {
	workflowName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(workflowOutputFormat),
		Quiet:  workflowQuiet,
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
		"name": workflowName,
	}

	return executor.Execute(ctx, "core_workflow_get", args_map)
}

func runWorkflowCreate(cmd *cobra.Command, args []string) error {
	workflowFile := args[0]

	// Read workflow definition
	definition, err := readWorkflowFile(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(workflowOutputFormat),
		Quiet:  workflowQuiet,
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
		"definition": definition,
	}

	return executor.Execute(ctx, "core_workflow_create", args_map)
}

func runWorkflowUpdate(cmd *cobra.Command, args []string) error {
	workflowName := args[0]
	workflowFile := args[1]

	// Read workflow definition
	definition, err := readWorkflowFile(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(workflowOutputFormat),
		Quiet:  workflowQuiet,
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
		"name":       workflowName,
		"definition": definition,
	}

	return executor.Execute(ctx, "core_workflow_update", args_map)
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
	workflowName := args[0]

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(workflowOutputFormat),
		Quiet:  workflowQuiet,
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
		"name": workflowName,
	}

	return executor.Execute(ctx, "core_workflow_delete", args_map)
}

func runWorkflowValidate(cmd *cobra.Command, args []string) error {
	workflowFile := args[0]

	// Read workflow definition
	definition, err := readWorkflowFile(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %w", err)
	}

	executor, err := cli.NewToolExecutor(cli.ExecutorOptions{
		Format: cli.OutputFormat(workflowOutputFormat),
		Quiet:  workflowQuiet,
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
		"definition": definition,
	}

	return executor.Execute(ctx, "core_workflow_validate", args_map)
}

// readWorkflowFile reads a workflow definition from a file or stdin
func readWorkflowFile(filename string) (string, error) {
	var reader io.Reader

	if filename == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return "", err
		}
		defer file.Close()
		reader = file
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
