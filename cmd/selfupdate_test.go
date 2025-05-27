package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewSelfUpdateCmd(t *testing.T) {
	// Test self-update command creation
	selfUpdateCmd := newSelfUpdateCmd()

	if selfUpdateCmd.Use != "self-update" {
		t.Errorf("Expected Use to be 'self-update', got %s", selfUpdateCmd.Use)
	}

	if selfUpdateCmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if selfUpdateCmd.Long == "" {
		t.Error("Expected Long description to be set")
	}

	if selfUpdateCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func TestRunSelfUpdateWithDevVersion(t *testing.T) {
	// Test self-update with development version
	originalVersion := rootCmd.Version
	defer func() { rootCmd.Version = originalVersion }()

	// Test with "dev" version
	rootCmd.Version = "dev"

	err := runSelfUpdate(nil, []string{})
	if err == nil {
		t.Error("Expected error for dev version")
	}

	if !strings.Contains(err.Error(), "cannot self-update a development version") {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestRunSelfUpdateWithEmptyVersion(t *testing.T) {
	// Test self-update with empty version
	originalVersion := rootCmd.Version
	defer func() { rootCmd.Version = originalVersion }()

	rootCmd.Version = ""

	err := runSelfUpdate(nil, []string{})
	if err == nil {
		t.Error("Expected error for empty version")
	}

	if !strings.Contains(err.Error(), "cannot self-update a development version") {
		t.Errorf("Expected specific error message, got: %s", err.Error())
	}
}

func TestSelfUpdateCommandHelp(t *testing.T) {
	// Test self-update command help
	selfUpdateCmd := newSelfUpdateCmd()
	var buf bytes.Buffer
	selfUpdateCmd.SetOut(&buf)
	selfUpdateCmd.SetErr(&buf) // Also capture stderr for help
	selfUpdateCmd.SetArgs([]string{"--help"})

	err := selfUpdateCmd.Execute()
	if err != nil {
		t.Fatalf("Error executing self-update help: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Checks for the latest release") {
		t.Errorf("Help output should contain long description. Got: %q", output)
	}

	if !strings.Contains(output, "self-update") {
		t.Errorf("Help output should contain command name. Got: %q", output)
	}
}

func TestGithubRepoSlug(t *testing.T) {
	// Test that the GitHub repo slug is set correctly
	expected := "giantswarm/envctl"
	if githubRepoSlug != expected {
		t.Errorf("Expected githubRepoSlug to be %s, got %s", expected, githubRepoSlug)
	}
}

// Note: We don't test the actual update functionality as it requires network access
// and would attempt to download and replace the binary. In a real-world scenario,
// you might want to create integration tests that run in a controlled environment
// or mock the selfupdate library for more comprehensive testing.
