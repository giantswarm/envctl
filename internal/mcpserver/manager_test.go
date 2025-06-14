package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPServerManager(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.loader)
	assert.NotNil(t, manager.definitions)
	assert.Empty(t, manager.definitions)
}

func TestMCPServerManager_validateDefinition(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	tests := []struct {
		name    string
		def     *MCPServerDefinition
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid local command server",
			def: &MCPServerDefinition{
				Name:    "test-server",
				Type:    MCPServerTypeLocalCommand,
				Command: []string{"echo", "hello"},
			},
			wantErr: false,
		},
		{
			name: "valid container server",
			def: &MCPServerDefinition{
				Name:  "test-container",
				Type:  MCPServerTypeContainer,
				Image: "nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			def: &MCPServerDefinition{
				Type:    MCPServerTypeLocalCommand,
				Command: []string{"echo", "hello"},
			},
			wantErr: true,
			errMsg:  "MCP server name cannot be empty",
		},
		{
			name: "invalid type",
			def: &MCPServerDefinition{
				Name: "test-server",
				Type: "invalid-type",
			},
			wantErr: true,
			errMsg:  "invalid MCP server type",
		},
		{
			name: "local command without command",
			def: &MCPServerDefinition{
				Name: "test-server",
				Type: MCPServerTypeLocalCommand,
			},
			wantErr: true,
			errMsg:  "command is required for local command MCP servers",
		},
		{
			name: "container without image",
			def: &MCPServerDefinition{
				Name: "test-server",
				Type: MCPServerTypeContainer,
			},
			wantErr: true,
			errMsg:  "image is required for container MCP servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateDefinition(tt.def)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMCPServerManager_GetDefinition(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	// Test getting non-existent definition
	def, exists := manager.GetDefinition("non-existent")
	assert.False(t, exists)
	assert.Equal(t, MCPServerDefinition{}, def)

	// Add a definition directly to test getting it
	testDef := &MCPServerDefinition{
		Name:    "test-server",
		Type:    MCPServerTypeLocalCommand,
		Command: []string{"echo", "hello"},
		Enabled: true,
	}
	manager.definitions["test-server"] = testDef

	// Test getting existing definition
	def, exists = manager.GetDefinition("test-server")
	assert.True(t, exists)
	assert.Equal(t, *testDef, def)
}

func TestMCPServerManager_ListDefinitions(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	// Test empty list
	defs := manager.ListDefinitions()
	assert.Empty(t, defs)

	// Add some definitions
	testDef1 := &MCPServerDefinition{
		Name:    "server1",
		Type:    MCPServerTypeLocalCommand,
		Command: []string{"echo", "hello"},
		Enabled: true,
	}
	testDef2 := &MCPServerDefinition{
		Name:  "server2",
		Type:  MCPServerTypeContainer,
		Image: "nginx:latest",
		Enabled: false,
	}

	manager.definitions["server1"] = testDef1
	manager.definitions["server2"] = testDef2

	// Test list with definitions
	defs = manager.ListDefinitions()
	assert.Len(t, defs, 2)

	// Check that both definitions are included
	names := make(map[string]bool)
	for _, def := range defs {
		names[def.Name] = true
	}
	assert.True(t, names["server1"])
	assert.True(t, names["server2"])
}

func TestMCPServerManager_ListAvailableDefinitions(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	// Add some definitions
	testDef1 := &MCPServerDefinition{
		Name:    "server1",
		Type:    MCPServerTypeLocalCommand,
		Command: []string{"echo", "hello"},
		Enabled: true,
	}
	testDef2 := &MCPServerDefinition{
		Name:  "server2",
		Type:  MCPServerTypeContainer,
		Image: "nginx:latest",
		Enabled: false,
	}

	manager.definitions["server1"] = testDef1
	manager.definitions["server2"] = testDef2

	// For MCP servers, all definitions should be considered available
	availableDefs := manager.ListAvailableDefinitions()
	assert.Len(t, availableDefs, 2)

	// Should be the same length as ListDefinitions since no tool checking is done
	allDefs := manager.ListDefinitions()
	assert.Len(t, allDefs, len(availableDefs))

	// Check that both definitions are included by name
	availableNames := make(map[string]bool)
	for _, def := range availableDefs {
		availableNames[def.Name] = true
	}
	assert.True(t, availableNames["server1"])
	assert.True(t, availableNames["server2"])
}

func TestMCPServerManager_IsAvailable(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	// Test non-existent server
	assert.False(t, manager.IsAvailable("non-existent"))

	// Add a definition
	testDef := &MCPServerDefinition{
		Name:    "test-server",
		Type:    MCPServerTypeLocalCommand,
		Command: []string{"echo", "hello"},
		Enabled: true,
	}
	manager.definitions["test-server"] = testDef

	// Test existing server
	assert.True(t, manager.IsAvailable("test-server"))
}

func TestMCPServerManager_RefreshAvailability(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	// This should not panic and should be a no-op for MCP servers
	manager.RefreshAvailability()
}

func TestMCPServerManager_GetDefinitionsPath(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	path := manager.GetDefinitionsPath()
	assert.NotEmpty(t, path)
	assert.Contains(t, path, "mcpservers")
	assert.Contains(t, path, "User:")
	assert.Contains(t, path, "Project:")
}

func TestMCPServerManager_GetAllDefinitions(t *testing.T) {
	manager, err := NewMCPServerManager()
	require.NoError(t, err)

	// Test empty map
	allDefs := manager.GetAllDefinitions()
	assert.Empty(t, allDefs)

	// Add some definitions
	testDef1 := &MCPServerDefinition{
		Name:    "server1",
		Type:    MCPServerTypeLocalCommand,
		Command: []string{"echo", "hello"},
		Enabled: true,
	}
	testDef2 := &MCPServerDefinition{
		Name:  "server2",
		Type:  MCPServerTypeContainer,
		Image: "nginx:latest",
		Enabled: false,
	}

	manager.definitions["server1"] = testDef1
	manager.definitions["server2"] = testDef2

	// Test with definitions
	allDefs = manager.GetAllDefinitions()
	assert.Len(t, allDefs, 2)
	assert.Equal(t, testDef1, allDefs["server1"])
	assert.Equal(t, testDef2, allDefs["server2"])

	// Verify it's a copy (modifying returned map shouldn't affect original)
	delete(allDefs, "server1")
	assert.Len(t, manager.definitions, 2) // Original should still have both
} 