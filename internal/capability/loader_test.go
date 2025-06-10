package capability

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// mockToolChecker implements ToolAvailabilityChecker for testing
type mockToolChecker struct {
	availableTools map[string]bool
}

func (m *mockToolChecker) IsToolAvailable(toolName string) bool {
	return m.availableTools[toolName]
}

func (m *mockToolChecker) GetAvailableTools() []string {
	tools := make([]string, 0, len(m.availableTools))
	for tool, available := range m.availableTools {
		if available {
			tools = append(tools, tool)
		}
	}
	return tools
}

func TestCapabilityDefinitionParsing(t *testing.T) {
	// Test parsing a capability definition with embedded workflows
	yamlContent := `
name: teleport_auth
type: auth_provider
version: "1.0.0"
description: "Teleport authentication provider"
operations:
  login:
    description: "Authenticate to a cluster"
    parameters:
      cluster:
        type: string
        required: true
        description: "Target cluster"
    requires:
      - x_teleport_kube
      - x_teleport_status
    workflow:
      name: teleport_auth_login
      description: "Login workflow"
      agentModifiable: false
      inputSchema:
        type: object
        properties:
          cluster:
            type: string
      steps:
        - id: check_status
          tool: x_teleport_status
          args: {}
          store: status
        - id: login
          tool: x_teleport_kube
          args:
            cluster: "{{ .cluster }}"
          store: result
  status:
    description: "Check status"
    requires:
      - x_teleport_status
    workflow:
      name: teleport_auth_status
      description: "Status workflow"
      steps:
        - id: get_status
          tool: x_teleport_status
          args: {}
          store: status
metadata:
  provider: teleport
  icon: "🔐"
`

	var def CapabilityDefinition
	err := yaml.Unmarshal([]byte(yamlContent), &def)
	require.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "teleport_auth", def.Name)
	assert.Equal(t, "auth_provider", def.Type)
	assert.Equal(t, "1.0.0", def.Version)
	assert.Equal(t, "Teleport authentication provider", def.Description)

	// Verify operations
	assert.Len(t, def.Operations, 2)

	// Check login operation
	loginOp, exists := def.Operations["login"]
	assert.True(t, exists)
	assert.Equal(t, "Authenticate to a cluster", loginOp.Description)
	assert.Equal(t, []string{"x_teleport_kube", "x_teleport_status"}, loginOp.Requires)
	assert.NotNil(t, loginOp.Workflow)

	// Verify parameters
	assert.Len(t, loginOp.Parameters, 1)
	clusterParam, exists := loginOp.Parameters["cluster"]
	assert.True(t, exists)
	assert.Equal(t, "string", clusterParam.Type)
	assert.True(t, clusterParam.Required)

	// Verify embedded workflow
	workflowMap, ok := loginOp.Workflow.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "teleport_auth_login", workflowMap["name"])
	assert.Equal(t, "Login workflow", workflowMap["description"])
	assert.Equal(t, false, workflowMap["agentModifiable"])

	// Check status operation
	statusOp, exists := def.Operations["status"]
	assert.True(t, exists)
	assert.Equal(t, "Check status", statusOp.Description)
	assert.Equal(t, []string{"x_teleport_status"}, statusOp.Requires)

	// Verify metadata
	assert.Equal(t, "teleport", def.Metadata["provider"])
	assert.Equal(t, "🔐", def.Metadata["icon"])
}

func TestCapabilityLoader(t *testing.T) {
	// Create a temporary directory for test definitions
	tmpDir, err := os.MkdirTemp("", "capability-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test capability definition file
	capabilityYAML := `
name: test_auth
type: auth_provider
version: "1.0.0"
description: "Test authentication provider"
operations:
  login:
    description: "Test login"
    requires:
      - x_test_login
      - x_test_status
    workflow:
      name: test_login
      steps:
        - id: login
          tool: x_test_login
          args:
            test: true
          store: result
  status:
    description: "Test status"
    requires:
      - x_test_status
    workflow: test_status_workflow
metadata:
  test: "true"
`

	capFile := filepath.Join(tmpDir, "test_auth.yaml")
	err = os.WriteFile(capFile, []byte(capabilityYAML), 0644)
	require.NoError(t, err)

	// Create mock tool checker
	mockChecker := &mockToolChecker{
		availableTools: map[string]bool{
			"x_test_login":  true,
			"x_test_status": true,
		},
	}

	// Create loader
	registry := NewRegistry()
	loader := NewCapabilityLoader(tmpDir, mockChecker, registry)

	// Load definitions
	err = loader.LoadDefinitions()
	require.NoError(t, err)

	// Verify capability was loaded
	def, exists := loader.GetCapabilityDefinition("test_auth")
	assert.True(t, exists)
	assert.NotNil(t, def)
	assert.Equal(t, "test_auth", def.Name)

	// Verify available capability tools
	availableTools := loader.GetAvailableCapabilityTools()
	assert.Contains(t, availableTools, "x_auth_provider_login")
	assert.Contains(t, availableTools, "x_auth_provider_status")

	// Test GetOperationForTool
	op, capDef, err := loader.GetOperationForTool("x_auth_provider_login")
	require.NoError(t, err)
	assert.NotNil(t, op)
	assert.NotNil(t, capDef)
	assert.Equal(t, "Test login", op.Description)

	// Test with missing tools
	mockChecker.availableTools["x_test_login"] = false
	loader.RefreshAvailability()

	// Login should no longer be available
	availableTools = loader.GetAvailableCapabilityTools()
	assert.NotContains(t, availableTools, "x_auth_provider_login")
	assert.Contains(t, availableTools, "x_auth_provider_status") // Status still available
}

func TestCapabilityLoaderWithMissingFile(t *testing.T) {
	// Test loading from non-existent directory
	loader := NewCapabilityLoader("/non/existent/path", &mockToolChecker{}, NewRegistry())
	err := loader.LoadDefinitions()
	assert.NoError(t, err) // Should not error, just have no definitions

	// Verify no capabilities loaded
	availableTools := loader.GetAvailableCapabilityTools()
	assert.Empty(t, availableTools)
}

func TestCapabilityLoaderValidation(t *testing.T) {
	testCases := []struct {
		name        string
		yaml        string
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing name",
			yaml: `
type: auth_provider
operations:
  login:
    description: "Login"
`,
			expectError: true,
			errorMsg:    "capability name is required",
		},
		{
			name: "missing type",
			yaml: `
name: test
operations:
  login:
    description: "Login"
`,
			expectError: true,
			errorMsg:    "capability type is required",
		},
		{
			name: "missing operations",
			yaml: `
name: test
type: auth_provider
`,
			expectError: true,
			errorMsg:    "at least one operation is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "cap-validation-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Write test file
			testFile := filepath.Join(tmpDir, "test.yaml")
			err = os.WriteFile(testFile, []byte(tc.yaml), 0644)
			require.NoError(t, err)

			// Create loader and load definitions
			loader := NewCapabilityLoader(tmpDir, &mockToolChecker{}, NewRegistry())
			err = loader.LoadDefinitions()

			if tc.expectError {
				// The error is logged, but LoadDefinitions continues
				// Check that the capability was not loaded
				assert.Empty(t, loader.definitions)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, loader.definitions)
			}
		})
	}
}
