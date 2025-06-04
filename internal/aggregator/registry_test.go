package aggregator

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestNameTracker_SmartPrefixing(t *testing.T) {
	tests := []struct {
		name     string
		servers  map[string]*ServerInfo
		expected map[string]string // tool/prompt name -> expected exposed name
	}{
		{
			name: "No conflicts - tools keep original names",
			servers: map[string]*ServerInfo{
				"serverA": {
					Name:      "serverA",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "read_file"},
						{Name: "write_file"},
					},
				},
				"serverB": {
					Name:      "serverB",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "search"},
						{Name: "analyze"},
					},
				},
			},
			expected: map[string]string{
				"serverA.read_file":  "read_file",
				"serverA.write_file": "write_file",
				"serverB.search":     "search",
				"serverB.analyze":    "analyze",
			},
		},
		{
			name: "With conflicts - only conflicting tools get prefixed",
			servers: map[string]*ServerInfo{
				"serverA": {
					Name:      "serverA",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "read_file"},
						{Name: "search"}, // conflicts with serverB
					},
				},
				"serverB": {
					Name:      "serverB",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "search"}, // conflicts with serverA
						{Name: "analyze"},
					},
				},
			},
			expected: map[string]string{
				"serverA.read_file": "read_file",      // no conflict
				"serverA.search":    "serverA.search", // conflict
				"serverB.search":    "serverB.search", // conflict
				"serverB.analyze":   "analyze",        // no conflict
			},
		},
		{
			name: "Multiple servers with same tool",
			servers: map[string]*ServerInfo{
				"serverA": {
					Name:      "serverA",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "common_tool"},
					},
				},
				"serverB": {
					Name:      "serverB",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "common_tool"},
					},
				},
				"serverC": {
					Name:      "serverC",
					Connected: true,
					Tools: []mcp.Tool{
						{Name: "common_tool"},
						{Name: "unique_tool"},
					},
				},
			},
			expected: map[string]string{
				"serverA.common_tool": "serverA.common_tool",
				"serverB.common_tool": "serverB.common_tool",
				"serverC.common_tool": "serverC.common_tool",
				"serverC.unique_tool": "unique_tool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewNameTracker()
			tracker.RebuildMappings(tt.servers)

			for key, expectedName := range tt.expected {
				parts := splitKey(key)
				serverName := parts[0]
				toolName := parts[1]

				actualName := tracker.GetExposedToolName(serverName, toolName)
				assert.Equal(t, expectedName, actualName,
					"Tool %s on server %s should be exposed as %s, but got %s",
					toolName, serverName, expectedName, actualName)
			}
		})
	}
}

func TestNameTracker_ResolveName(t *testing.T) {
	servers := map[string]*ServerInfo{
		"serverA": {
			Name:      "serverA",
			Connected: true,
			Tools: []mcp.Tool{
				{Name: "unique_tool"},
				{Name: "shared_tool"},
			},
			Prompts: []mcp.Prompt{
				{Name: "unique_prompt"},
			},
		},
		"serverB": {
			Name:      "serverB",
			Connected: true,
			Tools: []mcp.Tool{
				{Name: "shared_tool"},
			},
			Prompts: []mcp.Prompt{
				{Name: "shared_prompt"},
			},
		},
		"serverC": {
			Name:      "serverC",
			Connected: true,
			Prompts: []mcp.Prompt{
				{Name: "shared_prompt"},
			},
		},
	}

	tracker := NewNameTracker()
	tracker.RebuildMappings(servers)

	tests := []struct {
		exposedName      string
		expectedServer   string
		expectedOriginal string
		expectedItemType string
		expectError      bool
	}{
		// Unique tool - no prefix
		{"unique_tool", "serverA", "unique_tool", "tool", false},
		// Shared tool - prefixed
		{"serverA.shared_tool", "serverA", "shared_tool", "tool", false},
		{"serverB.shared_tool", "serverB", "shared_tool", "tool", false},
		// Unique prompt - no prefix
		{"unique_prompt", "serverA", "unique_prompt", "prompt", false},
		// Shared prompt - prefixed
		{"serverB.shared_prompt", "serverB", "shared_prompt", "prompt", false},
		{"serverC.shared_prompt", "serverC", "shared_prompt", "prompt", false},
		// Non-existent name
		{"non_existent", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.exposedName, func(t *testing.T) {
			serverName, originalName, itemType, err := tracker.ResolveName(tt.exposedName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedServer, serverName)
				assert.Equal(t, tt.expectedOriginal, originalName)
				assert.Equal(t, tt.expectedItemType, itemType)
			}
		})
	}
}

// Helper function to split "server.tool" into ["server", "tool"]
func splitKey(key string) []string {
	for i := 0; i < len(key); i++ {
		if key[i] == '.' {
			return []string{key[:i], key[i+1:]}
		}
	}
	return []string{key}
}
