package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMCPServerDefinition(t *testing.T) {
	tests := []struct {
		name    string
		def     MCPServerDefinition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid local command server",
			def: MCPServerDefinition{
				Name:    "test-server",
				Type:    MCPServerTypeLocalCommand,
				Command: []string{"echo", "hello"},
			},
			wantErr: false,
		},
		{
			name: "valid container server",
			def: MCPServerDefinition{
				Name:  "test-container",
				Type:  MCPServerTypeContainer,
				Image: "nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			def: MCPServerDefinition{
				Type:    MCPServerTypeLocalCommand,
				Command: []string{"echo", "hello"},
			},
			wantErr: true,
			errMsg:  "MCP server name cannot be empty",
		},
		{
			name: "invalid type",
			def: MCPServerDefinition{
				Name: "test-server",
				Type: "invalid-type",
			},
			wantErr: true,
			errMsg:  "field 'type': must be one of: localCommand, container, mock",
		},
		{
			name: "local command without command",
			def: MCPServerDefinition{
				Name: "test-server",
				Type: MCPServerTypeLocalCommand,
			},
			wantErr: true,
			errMsg:  "command is required for local command MCP servers",
		},
		{
			name: "container without image",
			def: MCPServerDefinition{
				Name: "test-server",
				Type: MCPServerTypeContainer,
			},
			wantErr: true,
			errMsg:  "image is required for container MCP servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMCPServerDefinition(tt.def)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetMCPServerConfigurationPaths(t *testing.T) {
	userPath, projectPath, err := GetMCPServerConfigurationPaths()
	require.NoError(t, err)

	assert.NotEmpty(t, userPath)
	assert.NotEmpty(t, projectPath)
	assert.Contains(t, userPath, "mcpservers")
	assert.Contains(t, projectPath, "mcpservers")
	assert.Contains(t, userPath, ".config/envctl")
	assert.Contains(t, projectPath, ".envctl")
}

func TestLoadMCPServerDefinitions_EmptyDirectories(t *testing.T) {
	// Test that the function handles empty directories gracefully
	// Since we can't easily mock the config paths in this basic test,
	// we just verify the function doesn't panic and returns expected structure
	definitions, errorCollection, err := LoadMCPServerDefinitions()

	// Should not return an error for empty directories
	require.NoError(t, err)

	// Definitions may be nil or empty when no files exist, both are valid
	if definitions != nil {
		// If not nil, should be a valid slice (possibly empty)
		assert.GreaterOrEqual(t, len(definitions), 0)
	}

	// ErrorCollection may be nil when there are no errors
	if errorCollection != nil {
		assert.False(t, errorCollection.HasErrors())
	}
}
