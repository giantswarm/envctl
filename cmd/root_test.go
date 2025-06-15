package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSetVersion(t *testing.T) {
	// Test setting version
	testVersion := "1.2.3-test"
	SetVersion(testVersion)

	if rootCmd.Version != testVersion {
		t.Errorf("Expected version to be %s, got %s", testVersion, rootCmd.Version)
	}
}

func TestRootCommand(t *testing.T) {
	// Test root command properties
	if rootCmd.Use != "envctl" {
		t.Errorf("Expected Use to be 'envctl', got %s", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if rootCmd.Long == "" {
		t.Error("Expected Long description to be set")
	}

	if !rootCmd.SilenceUsage {
		t.Error("Expected SilenceUsage to be true")
	}
}

func TestVersionTemplate(t *testing.T) {
	// Create a new command to test version template
	testCmd := &cobra.Command{
		Use:     "test",
		Version: "1.0.0",
	}

	// Set the same version template as in Execute()
	testCmd.SetVersionTemplate(`{{printf "envctl version %s\n" .Version}}`)

	// Capture output
	var buf bytes.Buffer
	testCmd.SetOut(&buf)

	// Execute version command
	testCmd.SetArgs([]string{"--version"})
	err := testCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing version command: %v", err)
	}

	output := buf.String()
	expected := "envctl version 1.0.0\n"
	if output != expected {
		t.Errorf("Expected version output %q, got %q", expected, output)
	}
}

func TestSubcommands(t *testing.T) {
	// Test that subcommands are added
	commands := rootCmd.Commands()

	expectedCommands := []string{"version", "self-update", "serve"}
	foundCommands := make(map[string]bool)

	for _, cmd := range commands {
		foundCommands[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !foundCommands[expected] {
			t.Errorf("Expected subcommand %s to be registered", expected)
		}
	}
}

func TestRootCommandHelp(t *testing.T) {
	// Test that help can be generated without error
	var buf bytes.Buffer

	// Create a new command to avoid affecting the global one
	testRootCmd := &cobra.Command{
		Use:   "envctl",
		Short: "Connect your environment to Giant Swarm clusters",
		Long: `envctl simplifies connecting your local development environment
(e.g., MCP servers in Cursor) to Giant Swarm clusters via Teleport
and setting up necessary connections like Prometheus port-forwarding.`,
		SilenceUsage: true,
	}

	testRootCmd.SetOut(&buf)
	testRootCmd.SetArgs([]string{"--help"})

	err := testRootCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing help command: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "envctl") {
		t.Errorf("Help output should contain 'envctl'. Got: %q", output)
	}

	if !strings.Contains(output, "simplifies connecting") {
		t.Errorf("Help output should contain the long description. Got: %q", output)
	}
}
