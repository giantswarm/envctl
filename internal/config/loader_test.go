package config

import (
	"envctl/internal/kube"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	// For direct use in tests if needed, or mocking
)

// Helper function to create a temporary config file
func createTempConfigFile(t *testing.T, dir string, filename string, content EnvctlConfig) string {
	t.Helper()
	tempFilePath := filepath.Join(dir, filename)
	data, err := yaml.Marshal(&content)
	assert.NoError(t, err)
	err = os.WriteFile(tempFilePath, data, 0644)
	assert.NoError(t, err)
	return tempFilePath
}

func TestLoadConfig_DefaultOnly(t *testing.T) {
	tc := GetDefaultConfigWithRoles("test-mc", "test-wc")

	loadedConfig, err := LoadConfig("test-mc", "test-wc")
	assert.NoError(t, err)

	// DeepEqual might be too strict if order changes in slices, but for default config it should be stable.
	// For MCPServers and PortForwards, we might need to compare them in a more order-insensitive way
	// if the merge logic or GetDefaultConfig doesn't guarantee order.
	assert.True(t, reflect.DeepEqual(tc.GlobalSettings, loadedConfig.GlobalSettings), "GlobalSettings should match default")
	assert.ElementsMatch(t, tc.MCPServers, loadedConfig.MCPServers, "MCPServers should match default")
	assert.ElementsMatch(t, tc.PortForwards, loadedConfig.PortForwards, "PortForwards should match default")
	assert.ElementsMatch(t, tc.Clusters, loadedConfig.Clusters, "Clusters should match default")
	assert.Equal(t, tc.ActiveClusters, loadedConfig.ActiveClusters, "ActiveClusters should match default")
}

func TestLoadConfig_UserOverride(t *testing.T) {
	tempDir := t.TempDir()

	// Mock user config dir
	originalGetUserConfigPath := getUserConfigPath
	originalOsUserHomeDir := osUserHomeDir // Mock our package-level variable
	defer func() {
		getUserConfigPath = originalGetUserConfigPath
		osUserHomeDir = originalOsUserHomeDir // Restore
	}()

	osUserHomeDir = func() (string, error) { return tempDir, nil } // Assign to our var
	getUserConfigPath = func() (string, error) {
		// This can now also use the mocked osUserHomeDir if needed, or be self-contained
		// For this test, it directly returns the temp path based on the mocked home dir.
		return filepath.Join(tempDir, userConfigDir, configFileName), nil
	}

	// Create a user config file
	userConfDir := filepath.Join(tempDir, userConfigDir)
	err := os.MkdirAll(userConfDir, 0755)
	assert.NoError(t, err)

	userOverride := EnvctlConfig{
		GlobalSettings: GlobalSettings{
			DefaultContainerRuntime: "podman",
		},
		MCPServers: []MCPServerDefinition{
			{
				Name:    "kubernetes", // Override existing
				Type:    MCPServerTypeContainer,
				Image:   "my-custom-kube-api:latest",
				Enabled: true,
			},
			{
				Name:    "new-server", // Add new
				Type:    MCPServerTypeLocalCommand,
				Command: []string{"echo", "hello"},
				Enabled: true,
			},
		},
		PortForwards: []PortForwardDefinition{
			{
				Name:              "mc-prometheus", // Override existing
				KubeContextTarget: "override-mc-context",
				Namespace:         "override-ns",
				TargetType:        "pod",
				TargetName:        "override-pod",
				LocalPort:         "9090",
				RemotePort:        "9090",
				Enabled:           true,
			},
		},
	}
	createTempConfigFile(t, userConfDir, configFileName, userOverride)

	loadedConfig, err := LoadConfig("test-mc", "test-wc")
	assert.NoError(t, err)

	// Check global settings override
	assert.Equal(t, "podman", loadedConfig.GlobalSettings.DefaultContainerRuntime)

	// Check MCPServers override and addition
	assert.Len(t, loadedConfig.MCPServers, 7) // Default has 6 (teleport, kubernetes, capi, flux, prometheus, grafana), 1 overridden (kubernetes), 1 new (new-server)
	foundKube := false
	foundNewServer := false
	for _, srv := range loadedConfig.MCPServers {
		if srv.Name == "kubernetes" {
			assert.Equal(t, MCPServerTypeContainer, srv.Type)
			assert.Equal(t, "my-custom-kube-api:latest", srv.Image)
			foundKube = true
		}
		if srv.Name == "new-server" {
			foundNewServer = true
		}
	}
	assert.True(t, foundKube, "Overridden kubernetes server not found")
	assert.True(t, foundNewServer, "New server not found")

	// Check PortForwards override
	foundProm := false
	for _, pf := range loadedConfig.PortForwards {
		if pf.Name == "mc-prometheus" {
			assert.Equal(t, "override-mc-context", pf.KubeContextTarget) // Placeholder not resolved yet by this stage of test, resolution is last step in LoadConfig
			assert.Equal(t, "9090", pf.LocalPort)
			foundProm = true
		}
	}
	assert.True(t, foundProm, "Overridden mc-prometheus port-forward not found")
}

func TestLoadConfig_ProjectOverride(t *testing.T) {
	tempDir := t.TempDir()

	originalGetProjectConfigPath := getProjectConfigPath
	originalOsGetwd := osGetwd // Mock our package-level variable
	defer func() {
		getProjectConfigPath = originalGetProjectConfigPath
		osGetwd = originalOsGetwd // Restore
	}()

	osGetwd = func() (string, error) { return tempDir, nil } // Assign to our var
	getProjectConfigPath = func() (string, error) {
		// Similar to user config, this can use mocked osGetwd or be self-contained.
		return filepath.Join(tempDir, projectConfigDir, configFileName), nil
	}

	projectConfDir := filepath.Join(tempDir, projectConfigDir)
	err := os.MkdirAll(projectConfDir, 0755)
	assert.NoError(t, err)

	projectOverride := EnvctlConfig{
		GlobalSettings: GlobalSettings{DefaultContainerRuntime: "cri-o"},
		MCPServers: []MCPServerDefinition{
			{Name: "kubernetes", Command: []string{"kubectl", "proxy"}, Type: MCPServerTypeLocalCommand, Enabled: true},
		},
	}
	createTempConfigFile(t, projectConfDir, configFileName, projectOverride)

	loadedConfig, err := LoadConfig("test-mc", "test-wc")
	assert.NoError(t, err)
	assert.Equal(t, "cri-o", loadedConfig.GlobalSettings.DefaultContainerRuntime)

	foundKubeProject := false
	for _, srv := range loadedConfig.MCPServers {
		if srv.Name == "kubernetes" {
			assert.Contains(t, strings.Join(srv.Command, " "), "kubectl proxy")
			foundKubeProject = true
		}
	}
	assert.True(t, foundKubeProject, "Project overridden kubernetes server not found or incorrect")
}

func TestLoadConfig_ContextResolution(t *testing.T) {
	tempDir := t.TempDir()
	originalGetUserConfigPath := getUserConfigPath
	originalOsUserHomeDir := osUserHomeDir
	defer func() {
		getUserConfigPath = originalGetUserConfigPath
		osUserHomeDir = originalOsUserHomeDir
	}()

	osUserHomeDir = func() (string, error) { return tempDir, nil }
	getUserConfigPath = func() (string, error) {
		return filepath.Join(tempDir, userConfigDir, configFileName), nil
	}

	userConfDir := filepath.Join(tempDir, userConfigDir)
	err := os.MkdirAll(userConfDir, 0755)
	assert.NoError(t, err)

	// This config will be written to the mock user config file path
	confWithPlaceholders := EnvctlConfig{
		PortForwards: []PortForwardDefinition{
			// Order matters for testing which error is hit first
			{Name: "pf-mc", KubeContextTarget: "mc", Namespace: "ns1", TargetType: "svc", TargetName: "s1", LocalPort: "1111", RemotePort: "1111", Enabled: true},
			{Name: "pf-wc", KubeContextTarget: "wc", Namespace: "ns2", TargetType: "pod", TargetName: "p1", LocalPort: "2222", RemotePort: "2222", Enabled: true},
			{Name: "pf-explicit", KubeContextTarget: "explicit-context", Namespace: "ns3", TargetType: "svc", TargetName: "s2", LocalPort: "3333", RemotePort: "3333", Enabled: true},
			{Name: "pf-another-mc-needing-name", KubeContextTarget: "mc", Namespace: "ns4", TargetType: "svc", TargetName: "s3", LocalPort: "4444", RemotePort: "4444", Enabled: true},
			{Name: "pf-another-wc-needing-names", KubeContextTarget: "wc", Namespace: "ns5", TargetType: "svc", TargetName: "s4", LocalPort: "5555", RemotePort: "5555", Enabled: true},
		},
	}
	createTempConfigFile(t, userConfDir, configFileName, confWithPlaceholders)

	// Case 1: mc and wc provided - successful resolution
	loadedMcWc, errMcWc := LoadConfig("my-mc", "my-wc")
	assert.NoError(t, errMcWc)
	expectedMcContext := kube.BuildMcContext("my-mc")
	expectedWcContext := kube.BuildWcContext("my-mc", "my-wc")
	resolvedPortForwards := make(map[string]string)
	for _, pf := range loadedMcWc.PortForwards {
		resolvedPortForwards[pf.Name] = pf.KubeContextTarget
	}
	assert.Equal(t, expectedMcContext, resolvedPortForwards["pf-mc"])
	assert.Equal(t, expectedWcContext, resolvedPortForwards["pf-wc"])
	assert.Equal(t, "explicit-context", resolvedPortForwards["pf-explicit"])
	assert.Equal(t, expectedMcContext, resolvedPortForwards["pf-another-mc-needing-name"])
	assert.Equal(t, expectedWcContext, resolvedPortForwards["pf-another-wc-needing-names"])

	// Case 2: Only mc provided. "wc" placeholder should cause an error for pf-wc.
	// The user config file (confWithPlaceholders) is still in effect.
	_, errOnlyMc := LoadConfig("some-mc-for-case2", "")
	assert.Error(t, errOnlyMc)
	// Due to map iteration order, we might get error for either "pf-wc" or "pf-another-wc-needing-names"
	assert.Contains(t, errOnlyMc.Error(), "requires WC context, but wcName is not provided (mcName: some-mc-for-case2)")
	assert.True(t,
		strings.Contains(errOnlyMc.Error(), "port-forward 'pf-wc'") ||
			strings.Contains(errOnlyMc.Error(), "port-forward 'pf-another-wc-needing-names'"),
		"Error should mention one of the WC-requiring port-forwards")

	// Case 3: No mc or wc provided. Either "mc" or "wc" placeholder error could occur first.
	// The user config file (confWithPlaceholders) is still in effect.
	_, errNoMcWc := LoadConfig("", "")
	assert.Error(t, errNoMcWc)
	// Since map iteration order is not guaranteed, we could get either MC or WC error
	// Just verify it's a context resolution error
	assert.Contains(t, errNoMcWc.Error(), "error resolving KubeContextTarget placeholders")
	assert.True(t,
		strings.Contains(errNoMcWc.Error(), "requires MC context, but mcName is not provided") ||
			strings.Contains(errNoMcWc.Error(), "requires WC context, but wcName is not provided"),
		"Error should be about missing MC or WC context")

}

func TestResolveKubeContextPlaceholders(t *testing.T) {
	tests := []struct {
		name      string
		config    EnvctlConfig
		mcName    string
		wcName    string
		expectErr bool
		expected  EnvctlConfig
	}{
		{
			name: "resolve mc placeholder",
			config: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: "mc"},
				},
			},
			mcName:    "my-mc",
			wcName:    "",
			expectErr: false,
			expected: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: kube.BuildMcContext("my-mc")},
				},
			},
		},
		{
			name: "resolve wc placeholder",
			config: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: "wc"},
				},
			},
			mcName:    "my-mc",
			wcName:    "my-wc",
			expectErr: false,
			expected: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: kube.BuildWcContext("my-mc", "my-wc")},
				},
			},
		},
		{
			name: "error when mc required but not provided",
			config: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: "mc"},
				},
			},
			mcName:    "",
			wcName:    "",
			expectErr: true,
		},
		{
			name: "error when wc required but not provided",
			config: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: "wc"},
				},
			},
			mcName:    "my-mc",
			wcName:    "",
			expectErr: true,
		},
		{
			name: "explicit context unchanged",
			config: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: "explicit-context"},
				},
			},
			mcName:    "my-mc",
			wcName:    "my-wc",
			expectErr: false,
			expected: EnvctlConfig{
				PortForwards: []PortForwardDefinition{
					{Name: "pf1", KubeContextTarget: "explicit-context"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolveKubeContextPlaceholders(&tt.config, tt.mcName, tt.wcName)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, tt.config)
			}
		})
	}
}

// TODO: Add more tests:
// - Test with user config but no project config (covered by TestLoadConfig_UserOverride if project path doesn't exist).
// - Test with project config but no user config (covered by TestLoadConfig_ProjectOverride if user path doesn't exist).
// - Test with both user and project config, ensuring project takes precedence for same-name items.
// - Test with empty config files.
// - Test with malformed YAML in config files.
// - Test merge strategy for GlobalSettings more thoroughly if more fields are added.

// </rewritten_file>
