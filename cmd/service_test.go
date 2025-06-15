package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCmd(t *testing.T) {
	cmd := serviceCmd

	assert.NotNil(t, cmd)
	assert.Equal(t, "service", cmd.Use)
	assert.Contains(t, cmd.Short, "Manage services")
	assert.True(t, cmd.HasSubCommands())

	// Check that all expected subcommands exist
	subcommands := []string{"list", "start", "stop", "restart", "status"}
	for _, subcmd := range subcommands {
		found := false
		for _, child := range cmd.Commands() {
			if child.Use == subcmd || child.Use == subcmd+" <service-label>" {
				found = true
				break
			}
		}
		assert.True(t, found, "Subcommand %s not found", subcmd)
	}
}

func TestServiceListCmd(t *testing.T) {
	cmd := serviceListCmd

	assert.NotNil(t, cmd)
	assert.Equal(t, "list", cmd.Use)
	assert.Contains(t, cmd.Short, "List all services")

	// Test that the command has the correct structure
	assert.NotNil(t, cmd.RunE)
}

func TestServiceStartCmd(t *testing.T) {
	cmd := serviceStartCmd

	assert.NotNil(t, cmd)
	assert.Equal(t, "start <service-label>", cmd.Use)
	assert.Contains(t, cmd.Short, "Start a service")

	// Test that it has the correct args validation
	assert.NotNil(t, cmd.Args)
	assert.NotNil(t, cmd.RunE)
}

func TestServiceStopCmd(t *testing.T) {
	cmd := serviceStopCmd

	assert.NotNil(t, cmd)
	assert.Equal(t, "stop <service-label>", cmd.Use)
	assert.Contains(t, cmd.Short, "Stop a service")

	// Test that it has the correct args validation
	assert.NotNil(t, cmd.Args)
	assert.NotNil(t, cmd.RunE)
}

func TestServiceRestartCmd(t *testing.T) {
	cmd := serviceRestartCmd

	assert.NotNil(t, cmd)
	assert.Equal(t, "restart <service-label>", cmd.Use)
	assert.Contains(t, cmd.Short, "Restart a service")

	// Test that it has the correct args validation
	assert.NotNil(t, cmd.Args)
	assert.NotNil(t, cmd.RunE)
}

func TestServiceStatusCmd(t *testing.T) {
	cmd := serviceStatusCmd

	assert.NotNil(t, cmd)
	assert.Equal(t, "status <service-label>", cmd.Use)
	assert.Contains(t, cmd.Short, "Get detailed status")

	// Test that it has the correct args validation
	assert.NotNil(t, cmd.Args)
	assert.NotNil(t, cmd.RunE)
}

func TestServiceCmd_Flags(t *testing.T) {
	cmd := serviceCmd

	// Test that global flags are available
	outputFlag := cmd.PersistentFlags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "table", outputFlag.DefValue)

	quietFlag := cmd.PersistentFlags().Lookup("quiet")
	require.NotNil(t, quietFlag)
	assert.Equal(t, "false", quietFlag.DefValue)
}

func TestServiceCmd_Integration(t *testing.T) {
	// Test the full command structure
	rootCmd := &cobra.Command{Use: "envctl"}
	rootCmd.AddCommand(serviceCmd)

	// Test help for main service command
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"service", "--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Manage services")
	assert.Contains(t, output, "Available Commands:")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "start")
	assert.Contains(t, output, "stop")
	assert.Contains(t, output, "restart")
	assert.Contains(t, output, "status")
}
