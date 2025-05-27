package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConnectCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectError   bool
		errorContains string
	}{
		{
			name:          "no arguments",
			args:          []string{},
			expectError:   true,
			errorContains: "accepts between 1 and 2 arg(s)",
		},
		{
			name:          "too many arguments",
			args:          []string{"mc1", "wc1", "extra"},
			expectError:   true,
			errorContains: "accepts between 1 and 2 arg(s)",
		},
		// Skip tests that would actually try to connect
		// These tests need proper mocking of kube manager and config loading
		// {
		// 	name:        "valid MC only",
		// 	args:        []string{"mymc"},
		// 	expectError: true, // Will fail due to missing config/kube setup
		// },
		// {
		// 	name:        "valid MC and WC",
		// 	args:        []string{"mymc", "mywc"},
		// 	expectError: true, // Will fail due to missing config/kube setup
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new command for each test to avoid state issues
			cmd := &cobra.Command{
				Use:   connectCmdDef.Use,
				Short: connectCmdDef.Short,
				Long:  connectCmdDef.Long,
				Args:  connectCmdDef.Args,
				RunE:  connectCmdDef.RunE,
			}

			// Add flags
			cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Disable TUI")
			cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug")

			// Set no-tui flag to avoid TUI initialization in tests
			cmd.SetArgs(append([]string{"--no-tui"}, tt.args...))

			// Capture output
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			err := cmd.Execute()

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.errorContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestConnectCmd_Flags(t *testing.T) {
	// Test that flags are properly defined
	cmd := connectCmdDef

	// Check no-tui flag
	noTUIFlag := cmd.Flags().Lookup("no-tui")
	if noTUIFlag == nil {
		t.Error("Expected no-tui flag to be defined")
	} else {
		if noTUIFlag.DefValue != "false" {
			t.Errorf("Expected no-tui default value to be false, got %s", noTUIFlag.DefValue)
		}
		if noTUIFlag.Usage != "Disable TUI and run port forwarding in the background" {
			t.Errorf("Unexpected no-tui usage text: %s", noTUIFlag.Usage)
		}
	}

	// Check debug flag
	debugFlag := cmd.Flags().Lookup("debug")
	if debugFlag == nil {
		t.Error("Expected debug flag to be defined")
	} else {
		if debugFlag.DefValue != "false" {
			t.Errorf("Expected debug default value to be false, got %s", debugFlag.DefValue)
		}
		if debugFlag.Usage != "Enable general debug logging" {
			t.Errorf("Unexpected debug usage text: %s", debugFlag.Usage)
		}
	}
}

func TestConnectCmd_ValidArgsFunction(t *testing.T) {
	// Test the ValidArgsFunction
	cmd := connectCmdDef

	// Test with no args (should suggest management clusters)
	suggestions, directive := cmd.ValidArgsFunction(cmd, []string{}, "")

	// We expect NoFileComp directive since the function returns empty suggestions
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Test with one arg (should suggest workload clusters)
	suggestions, directive = cmd.ValidArgsFunction(cmd, []string{"mymc"}, "")

	// We expect NoFileComp directive since the function returns empty suggestions
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Test with two args (should not suggest anything)
	suggestions, directive = cmd.ValidArgsFunction(cmd, []string{"mymc", "mywc"}, "")

	// Should return empty suggestions
	if len(suggestions) != 0 {
		t.Errorf("Expected no suggestions with 2 args, got %v", suggestions)
	}
}

func TestConnectCmd_Usage(t *testing.T) {
	cmd := connectCmdDef

	// Test Use string
	expectedUse := "connect <management-cluster> [workload-cluster-shortname]"
	if cmd.Use != expectedUse {
		t.Errorf("Expected Use %q, got %q", expectedUse, cmd.Use)
	}

	// Test Short description
	expectedShort := "Connect to Giant Swarm K8s and managed services with an interactive TUI or CLI mode."
	if cmd.Short != expectedShort {
		t.Errorf("Expected Short %q, got %q", expectedShort, cmd.Short)
	}

	// Test that Long description contains key information
	longDesc := cmd.Long
	expectedPhrases := []string{
		"Interactive TUI Mode",
		"Non-TUI / CLI Mode",
		"Prometheus",
		"Grafana",
		"Alloy Metrics",
		"--no-tui flag",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(longDesc, phrase) {
			t.Errorf("Expected Long description to contain %q", phrase)
		}
	}
}

func TestInit_ConnectCmd(t *testing.T) {
	// Test that connect command is added to root command
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "connect" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected connect command to be added to root command")
	}
}
