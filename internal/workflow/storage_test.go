package workflow

import (
	"envctl/internal/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConfigurationLoader for testing
type mockConfigurationLoader struct {
	workflows []WorkflowDefinition
	err       error
}

func (m *mockConfigurationLoader) LoadAndParseYAML(subDir string, validator func(WorkflowDefinition) error) ([]WorkflowDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Validate each workflow
	var validWorkflows []WorkflowDefinition
	for _, wf := range m.workflows {
		if err := validator(wf); err == nil {
			validWorkflows = append(validWorkflows, wf)
		}
	}
	
	return validWorkflows, nil
}

func (m *mockConfigurationLoader) LoadAndParseYAMLWithErrors(subDir string, validator func(WorkflowDefinition) error) ([]WorkflowDefinition, *config.ConfigurationErrorCollection, error) {
	// For testing, we'll just use the simple version and return no errors
	workflows, err := m.LoadAndParseYAML(subDir, validator)
	if err != nil {
		return nil, nil, err
	}
	
	// Return empty error collection for successful case
	errorCollection := &config.ConfigurationErrorCollection{}
	return workflows, errorCollection, nil
}

func TestWorkflowStorage_LayeredLoading(t *testing.T) {
	// Create mock workflows
	projectWorkflow := WorkflowDefinition{
		Name:        "project_workflow", 
		Description: "Project workflow",
		Steps: []WorkflowStep{
			{ID: "step1", Tool: "exec_tool", Args: map[string]interface{}{"command": "{{ .command }}"}},
		},
	}

	overrideWorkflow := WorkflowDefinition{
		Name:        "user_workflow", // Same name as a user workflow - project version overrides
		Description: "Project override of user workflow",
		Steps: []WorkflowStep{
			{ID: "step1", Tool: "override_tool", Args: map[string]interface{}{"overridden": "{{ .overridden }}"}},
		},
	}

	// Mock loader that simulates layered loading: project overrides user
	mockLoader := &mockConfigurationLoader{
		workflows: []WorkflowDefinition{projectWorkflow, overrideWorkflow}, // Project version loaded last (wins)
	}

	// Create workflow storage with mock loader
	storage, err := NewWorkflowStorageWithLoader(t.TempDir(), mockLoader)
	require.NoError(t, err)

	// Test loaded workflows
	workflows := storage.ListWorkflows()
	assert.Len(t, workflows, 2) // Should have 2 workflows: project_workflow and overridden user_workflow

	// Find the workflows
	var userWf, projectWf *WorkflowDefinition
	for i := range workflows {
		switch workflows[i].Name {
		case "user_workflow":
			userWf = &workflows[i]
		case "project_workflow":
			projectWf = &workflows[i]
		}
	}

	// Verify project workflow exists
	require.NotNil(t, projectWf)
	assert.Equal(t, "project_workflow", projectWf.Name)
	assert.Equal(t, "Project workflow", projectWf.Description)
	assert.Equal(t, "exec_tool", projectWf.Steps[0].Tool)

	// Verify user workflow is overridden by project version
	require.NotNil(t, userWf)
	assert.Equal(t, "user_workflow", userWf.Name)
	assert.Equal(t, "Project override of user workflow", userWf.Description) // Should be project version
	assert.Equal(t, "override_tool", userWf.Steps[0].Tool)                   // Should be project version
}

func TestWorkflowStorage_ValidationFailure(t *testing.T) {
	// Create invalid workflow (missing steps)
	invalidWorkflow := WorkflowDefinition{
		Name:        "invalid_workflow",
		Description: "Invalid workflow with no steps",
		Steps:       []WorkflowStep{}, // Empty steps - should fail validation
	}

	// Mock loader with invalid workflow
	mockLoader := &mockConfigurationLoader{
		workflows: []WorkflowDefinition{invalidWorkflow},
	}

	// Create workflow storage (should load successfully but skip invalid workflow)
	storage, err := NewWorkflowStorageWithLoader(t.TempDir(), mockLoader)
	require.NoError(t, err)

	// Should have no workflows since the invalid one was skipped
	workflows := storage.ListWorkflows()
	assert.Len(t, workflows, 0)
}

func TestWorkflowStorage_LegacyCompatibility(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create legacy agent workflows file
	legacyAgentWorkflows := `workflows:
  - name: legacy_workflow
    description: "Legacy agent workflow"
    agentModifiable: true
    createdBy: agent
    inputSchema:
      type: object
      properties:
        action:
          type: string
          description: "Action to perform"
      required: ["action"]
    steps:
      - id: step1
        tool: legacy_tool
        args:
          action: "{{ .action }}"
`

	// Write legacy file
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "agent_workflows.yaml"), []byte(legacyAgentWorkflows), 0644))

	// Mock loader with no directory-based workflows
	mockLoader := &mockConfigurationLoader{
		workflows: []WorkflowDefinition{}, // No directory workflows
	}

	// Create workflow storage
	storage, err := NewWorkflowStorageWithLoader(tempDir, mockLoader)
	require.NoError(t, err)

	// Should have legacy workflow
	workflows := storage.ListWorkflows()
	assert.Len(t, workflows, 1)

	workflow := workflows[0]
	assert.Equal(t, "legacy_workflow", workflow.Name)
	assert.Equal(t, "Legacy agent workflow", workflow.Description)
	assert.True(t, workflow.AgentModifiable)
	assert.Equal(t, "agent", workflow.CreatedBy)
	assert.Equal(t, "legacy_tool", workflow.Steps[0].Tool)
}

func TestWorkflowStorage_DirectoryOverridesLegacy(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create directory-based workflow
	directoryWorkflow := WorkflowDefinition{
		Name:        "test_workflow",
		Description: "Directory-based workflow",
		Steps: []WorkflowStep{
			{ID: "step1", Tool: "directory_tool", Args: map[string]interface{}{"message": "{{ .message }}"}},
		},
	}

	// Create legacy agent workflow with same name
	legacyAgentWorkflows := `workflows:
  - name: test_workflow
    description: "Legacy agent workflow"
    agentModifiable: true
    createdBy: agent
    inputSchema:
      type: object
      properties:
        action:
          type: string
      required: ["action"]
    steps:
      - id: step1
        tool: legacy_tool
        args:
          action: "{{ .action }}"
`

	// Write legacy file
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "agent_workflows.yaml"), []byte(legacyAgentWorkflows), 0644))

	// Mock loader with directory-based workflow
	mockLoader := &mockConfigurationLoader{
		workflows: []WorkflowDefinition{directoryWorkflow},
	}

	// Create workflow storage
	storage, err := NewWorkflowStorageWithLoader(tempDir, mockLoader)
	require.NoError(t, err)

	// Should have only directory-based workflow (it overrides legacy)
	workflows := storage.ListWorkflows()
	assert.Len(t, workflows, 1)

	workflow := workflows[0]
	assert.Equal(t, "test_workflow", workflow.Name)
	assert.Equal(t, "Directory-based workflow", workflow.Description) // Should be directory version
	assert.Equal(t, "directory_tool", workflow.Steps[0].Tool)         // Should be directory version
} 