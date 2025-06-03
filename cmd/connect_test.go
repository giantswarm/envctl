package cmd

import (
	"bytes"
	"context"
	"envctl/internal/kube"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// mockKubeManager implements kube.Manager interface for testing
type mockKubeManager struct {
	clusterInfo *kube.ClusterInfo
	err         error
}

func (m *mockKubeManager) Login(clusterName string) (string, string, error) {
	return "", "", nil
}

func (m *mockKubeManager) ListClusters() (*kube.ClusterInfo, error) {
	return m.clusterInfo, m.err
}

func (m *mockKubeManager) GetCurrentContext() (string, error) {
	return "test-context", nil
}

func (m *mockKubeManager) SwitchContext(targetContextName string) error {
	return nil
}

func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	return []string{}, nil
}

func (m *mockKubeManager) BuildMcContextName(mcName string) string {
	return "teleport.giantswarm.io-" + mcName
}

func (m *mockKubeManager) BuildWcContextName(mcName, wcName string) string {
	return "teleport.giantswarm.io-" + mcName + "-" + wcName
}

func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	return contextName
}

func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	return false
}

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (kube.NodeHealth, error) {
	return kube.NodeHealth{}, nil
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "", nil
}

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
	// Test the ValidArgsFunction integration
	cmd := connectCmdDef

	// The ValidArgsFunction should be set
	if cmd.ValidArgsFunction == nil {
		t.Error("Expected ValidArgsFunction to be set")
	}

	// Save the original factory and restore it after the test
	originalFactory := kubeManagerFactory
	defer func() { kubeManagerFactory = originalFactory }()

	// Mock cluster info for testing
	mockClusterInfo := &kube.ClusterInfo{
		ManagementClusters: []string{"testmc1", "testmc2"},
		WorkloadClusters: map[string][]string{
			"testmc1": {"testwc1", "testwc2"},
		},
	}

	// Inject a mock factory that returns our mock manager
	kubeManagerFactory = func(interface{}) kube.Manager {
		return &mockKubeManager{
			clusterInfo: mockClusterInfo,
			err:         nil,
		}
	}

	// Test with no args - should suggest management clusters
	suggestions, directive := cmd.ValidArgsFunction(cmd, []string{}, "")

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected NoFileComp directive, got %v", directive)
	}

	// Should return management clusters
	expectedMCs := []string{"testmc1", "testmc2"}
	if len(suggestions) != len(expectedMCs) {
		t.Errorf("Expected %d suggestions, got %d", len(expectedMCs), len(suggestions))
	}

	for i, expected := range expectedMCs {
		if i >= len(suggestions) || suggestions[i] != expected {
			t.Errorf("Expected suggestion %d to be %q, got %q", i, expected, suggestions[i])
		}
	}

	// Test with one arg - should suggest workload clusters
	suggestions, directive = cmd.ValidArgsFunction(cmd, []string{"testmc1"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("Expected NoFileComp directive, got %v", directive)
	}

	expectedWCs := []string{"testwc1", "testwc2"}
	if len(suggestions) != len(expectedWCs) {
		t.Errorf("Expected %d suggestions, got %d", len(expectedWCs), len(suggestions))
	}

	for i, expected := range expectedWCs {
		if i >= len(suggestions) || suggestions[i] != expected {
			t.Errorf("Expected suggestion %d to be %q, got %q", i, expected, suggestions[i])
		}
	}

	// Test error case
	kubeManagerFactory = func(interface{}) kube.Manager {
		return &mockKubeManager{
			err: errors.New("test error"),
		}
	}

	_, directive = cmd.ValidArgsFunction(cmd, []string{}, "")
	if directive != cobra.ShellCompDirectiveError {
		t.Errorf("Expected Error directive for error case, got %v", directive)
	}
}

func TestGetCompletionCandidates(t *testing.T) {
	// Test the extracted completion logic with mock data
	mockClusterInfo := &kube.ClusterInfo{
		ManagementClusters: []string{"mc1", "mc2"},
		WorkloadClusters: map[string][]string{
			"mc1": {"wc1", "wc2"},
			"mc2": {"wc3"},
		},
	}

	tests := []struct {
		name               string
		args               []string
		mockClusterInfo    *kube.ClusterInfo
		mockError          error
		expectedCandidates []string
		expectedDirective  cobra.ShellCompDirective
		expectError        bool
	}{
		{
			name:               "no args - suggest management clusters",
			args:               []string{},
			mockClusterInfo:    mockClusterInfo,
			expectedCandidates: []string{"mc1", "mc2"},
			expectedDirective:  cobra.ShellCompDirectiveNoFileComp,
			expectError:        false,
		},
		{
			name:               "one arg - suggest workload clusters for mc1",
			args:               []string{"mc1"},
			mockClusterInfo:    mockClusterInfo,
			expectedCandidates: []string{"wc1", "wc2"},
			expectedDirective:  cobra.ShellCompDirectiveNoFileComp,
			expectError:        false,
		},
		{
			name:               "one arg - suggest workload clusters for mc2",
			args:               []string{"mc2"},
			mockClusterInfo:    mockClusterInfo,
			expectedCandidates: []string{"wc3"},
			expectedDirective:  cobra.ShellCompDirectiveNoFileComp,
			expectError:        false,
		},
		{
			name:               "one arg - no workload clusters for unknown mc",
			args:               []string{"unknown"},
			mockClusterInfo:    mockClusterInfo,
			expectedCandidates: []string{},
			expectedDirective:  cobra.ShellCompDirectiveNoFileComp,
			expectError:        false,
		},
		{
			name:               "two args - no suggestions",
			args:               []string{"mc1", "wc1"},
			mockClusterInfo:    mockClusterInfo,
			expectedCandidates: []string{},
			expectedDirective:  cobra.ShellCompDirectiveNoFileComp,
			expectError:        false,
		},
		{
			name:              "error from ListClusters",
			args:              []string{},
			mockError:         errors.New("failed to list clusters"),
			expectedDirective: cobra.ShellCompDirectiveError,
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMgr := &mockKubeManager{
				clusterInfo: tt.mockClusterInfo,
				err:         tt.mockError,
			}

			candidates, directive, err := getCompletionCandidates(mockMgr, tt.args)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if directive != tt.expectedDirective {
				t.Errorf("Expected directive %v, got %v", tt.expectedDirective, directive)
			}

			if !tt.expectError {
				if len(candidates) != len(tt.expectedCandidates) {
					t.Errorf("Expected %d candidates, got %d", len(tt.expectedCandidates), len(candidates))
				}

				for i, expected := range tt.expectedCandidates {
					if i >= len(candidates) || candidates[i] != expected {
						t.Errorf("Expected candidate %d to be %q, got %q", i, expected, candidates[i])
					}
				}
			}
		})
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
