package kube

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestGetCurrentKubeContext(t *testing.T) {
	tests := []struct {
		name            string
		setupKubeconfig func(t *testing.T) (cleanup func())
		wantContext     string
		wantErr         bool
	}{
		{
			name: "valid kubeconfig with current context set",
			setupKubeconfig: func(t *testing.T) func() {
				tmpFile, err := ioutil.TempFile("", "kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp kubeconfig: %v", err)
				}
				config := api.Config{
					CurrentContext: "test-context",
					Contexts:       map[string]*api.Context{"test-context": {Cluster: "test-cluster"}},
					Clusters:       map[string]*api.Cluster{"test-cluster": {Server: "https://localhost:8080"}},
				}
				if err := clientcmd.WriteToFile(config, tmpFile.Name()); err != nil {
					t.Fatalf("Failed to write temp kubeconfig: %v", err)
				}
				t.Setenv("KUBECONFIG", tmpFile.Name())
				return func() { os.Remove(tmpFile.Name()) }
			},
			wantContext: "test-context",
			wantErr:     false,
		},
		{
			name: "kubeconfig with current context not set",
			setupKubeconfig: func(t *testing.T) func() {
				tmpFile, err := ioutil.TempFile("", "kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp kubeconfig: %v", err)
				}
				config := api.Config{
					Contexts: map[string]*api.Context{"another-context": {Cluster: "another-cluster"}},
					Clusters: map[string]*api.Cluster{"another-cluster": {Server: "https://localhost:8081"}},
					// CurrentContext is empty
				}
				if err := clientcmd.WriteToFile(config, tmpFile.Name()); err != nil {
					t.Fatalf("Failed to write temp kubeconfig: %v", err)
				}
				t.Setenv("KUBECONFIG", tmpFile.Name())
				return func() { os.Remove(tmpFile.Name()) }
			},
			wantContext: "",
			wantErr:     true, // Expect error as current context is not set
		},
		{
			name: "KUBECONFIG not set, default path does not exist (simulated by empty KUBECONFIG to non-existent file)",
			setupKubeconfig: func(t *testing.T) func() {
				// Point KUBECONFIG to a path that won't exist or is empty
				nonExistentPath := filepath.Join(t.TempDir(), "does_not_exist_kubeconfig")
				t.Setenv("KUBECONFIG", nonExistentPath)
				// For a more robust test of default path failure, we might need to manipulate user's home dir resolution,
				// but clientcmd.NewDefaultPathOptions() might also check KUBECONFIG first.
				// If KUBECONFIG is set (even to non-existent), it might take precedence over default ~/.kube/config.
				// This case effectively tests when GetStartingConfig fails to load anything.
				return func() {}
			},
			wantContext: "",
			wantErr:     true,
		},
		{
			name: "empty KUBECONFIG var, expect default path behavior (hard to test default path in isolation)",
			setupKubeconfig: func(t *testing.T) func() {
				originalKubeconfig := os.Getenv("KUBECONFIG")
				t.Setenv("KUBECONFIG", "") // Unset KUBECONFIG or set to empty
				// This test will now depend on the actual default kubeconfig of the test runner's environment
				// which is not ideal for a unit test. A true test would mock the filesystem or home dir.
				// For now, this tests that it *can* proceed if KUBECONFIG is empty, deferring to clientcmd's defaults.
				// We will just check it doesn't panic, error expectation depends on runner's env.
				return func() { t.Setenv("KUBECONFIG", originalKubeconfig) }
			},
			// wantContext: depends on runner's environment, cannot assert reliably
			// wantErr: depends on runner's environment
			// For this specific case, we'll just ensure it doesn't panic and returns *something* or an error.
			// A more sophisticated test would mock clientcmd.DefaultPathOptions.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupKubeconfig != nil {
				cleanup := tt.setupKubeconfig(t)
				defer cleanup()
			}

			gotContext, err := GetCurrentKubeContext()

			// Special handling for the unreliable default path test case
			if tt.name == "empty KUBECONFIG var, expect default path behavior (hard to test default path in isolation)" {
				// Just ensure it doesn't panic. Error or context value is env-dependent.
				t.Logf("Default path behavior test: gotContext=%q, err=%v (environment dependent)", gotContext, err)
				return
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentKubeContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotContext != tt.wantContext {
				t.Errorf("GetCurrentKubeContext() gotContext = %v, want %v", gotContext, tt.wantContext)
			}
		})
	}
}

func TestSwitchKubeContext(t *testing.T) {
	tests := []struct {
		name              string
		initialKubeconfig *api.Config // nil for non-existent or error cases initially
		contextToSwitch   string
		setupFS           func(t *testing.T) (kubeconfigPath string, cleanup func()) // For non-existent file test
		wantErr           bool
		verifySwitch      func(t *testing.T, kubeconfigPath string) // Check CurrentContext after switch
	}{
		{
			name: "switch to existing context",
			initialKubeconfig: &api.Config{
				CurrentContext: "ctx1",
				Contexts: map[string]*api.Context{
					"ctx1": {Cluster: "cluster1"},
					"ctx2": {Cluster: "cluster2"},
				},
				Clusters: map[string]*api.Cluster{
					"cluster1": {Server: "server1"},
					"cluster2": {Server: "server2"},
				},
			},
			contextToSwitch: "ctx2",
			wantErr:         false,
			verifySwitch: func(t *testing.T, kubeconfigPath string) {
				config, err := clientcmd.LoadFromFile(kubeconfigPath)
				if err != nil {
					t.Fatalf("Failed to load kubeconfig for verification: %v", err)
				}
				if config.CurrentContext != "ctx2" {
					t.Errorf("Expected current context to be 'ctx2', got '%s'", config.CurrentContext)
				}
			},
		},
		{
			name: "switch to non-existent context",
			initialKubeconfig: &api.Config{
				CurrentContext: "ctx1",
				Contexts:       map[string]*api.Context{"ctx1": {Cluster: "cluster1"}},
				Clusters:       map[string]*api.Cluster{"cluster1": {Server: "server1"}},
			},
			contextToSwitch: "non-existent-ctx",
			wantErr:         true,
		},
		{
			name: "kubeconfig file does not exist",
			setupFS: func(t *testing.T) (string, func()) {
				kubeconfigPath := filepath.Join(t.TempDir(), "non_existent_kubeconfig")
				// Ensure it doesn't exist (though TempDir is new, this is explicit)
				os.Remove(kubeconfigPath)
				return kubeconfigPath, func() {}
			},
			contextToSwitch: "any-ctx",
			wantErr:         true, // clientcmd.LoadFromFile called by GetStartingConfig will fail
		},
		{
			name: "empty kubeconfig file (invalid)",
			setupFS: func(t *testing.T) (string, func()) {
				tmpFile, err := ioutil.TempFile("", "empty-kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				// Leave it empty
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			contextToSwitch: "any-ctx",
			wantErr:         true, // clientcmd.LoadFromFile will likely fail or return empty config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var kubeconfigPath string
			cleanup := func() {}

			if tt.setupFS != nil {
				kubeconfigPath, cleanup = tt.setupFS(t)
			} else if tt.initialKubeconfig != nil {
				tmpFile, err := ioutil.TempFile("", "kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp kubeconfig: %v", err)
				}
				kubeconfigPath = tmpFile.Name()
				if err := clientcmd.WriteToFile(*tt.initialKubeconfig, kubeconfigPath); err != nil {
					t.Fatalf("Failed to write temp kubeconfig: %v", err)
				}
				cleanup = func() { os.Remove(kubeconfigPath) }
			} else {
				// Should not happen if test cases are well-defined
				t.Fatal("Test case misconfigured: either initialKubeconfig or setupFS must be provided")
			}
			defer cleanup()
			t.Setenv("KUBECONFIG", kubeconfigPath)

			err := SwitchKubeContext(tt.contextToSwitch)

			if (err != nil) != tt.wantErr {
				t.Errorf("SwitchKubeContext() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.verifySwitch != nil {
				tt.verifySwitch(t, kubeconfigPath)
			}
		})
	}
}
