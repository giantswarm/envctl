package cmd

import (
	"strings"
	"testing"
)

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

	// Check yolo flag
	yoloFlag := cmd.Flags().Lookup("yolo")
	if yoloFlag == nil {
		t.Error("Expected yolo flag to be defined")
	} else {
		if yoloFlag.DefValue != "false" {
			t.Errorf("Expected yolo default value to be false, got %s", yoloFlag.DefValue)
		}
		if yoloFlag.Usage != "Disable denylist for destructive tool calls (use with caution)" {
			t.Errorf("Unexpected yolo usage text: %s", yoloFlag.Usage)
		}
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
