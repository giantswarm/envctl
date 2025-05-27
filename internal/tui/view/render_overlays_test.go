package view

import (
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
)

func TestGenerateMcpToolsContent(t *testing.T) {
	tests := []struct {
		name     string
		model    *model.Model
		expected []string
	}{
		{
			name: "no MCP servers",
			model: &model.Model{
				MCPTools:         map[string][]api.MCPTool{},
				McpToolsViewport: viewport.Model{Width: 80},
			},
			expected: []string{"No MCP servers with tools available"},
		},
		{
			name: "single server with tools",
			model: &model.Model{
				MCPTools: map[string][]api.MCPTool{
					"test-server": {
						{Name: "tool1", Description: "Tool 1 description"},
						{Name: "tool2", Description: "Tool 2 description"},
					},
				},
				McpToolsViewport: viewport.Model{Width: 80},
			},
			expected: []string{
				"=== test-server ===",
				"  • tool1",
				"    Tool 1 description",
				"  • tool2",
				"    Tool 2 description",
			},
		},
		{
			name: "single server without tools",
			model: &model.Model{
				MCPTools: map[string][]api.MCPTool{
					"empty-server": {},
				},
				McpToolsViewport: viewport.Model{Width: 80},
			},
			expected: []string{
				"=== empty-server ===",
				"  No tools available",
			},
		},
		{
			name: "multiple servers with mixed tools",
			model: &model.Model{
				MCPTools: map[string][]api.MCPTool{
					"server1": {
						{Name: "search", Description: "Search the web"},
						{Name: "calculate", Description: "Perform calculations"},
					},
					"server2": {},
					"server3": {
						{Name: "translate", Description: "Translate text"},
					},
				},
				McpToolsViewport: viewport.Model{Width: 80},
			},
			expected: []string{
				"=== server",
				"  • search",
				"    Search the web",
				"  • calculate",
				"    Perform calculations",
				"No tools available",
				"  • translate",
				"    Translate text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := GenerateMcpToolsContent(tt.model)

			for _, expectedLine := range tt.expected {
				assert.Contains(t, content, expectedLine, "Expected content to contain: %s", expectedLine)
			}

			// Additional checks for structure
			if len(tt.model.MCPTools) > 0 {
				// Check that each server has a section
				for serverName := range tt.model.MCPTools {
					assert.Contains(t, content, serverName)
				}
			}
		})
	}
}

func TestGenerateMcpToolsContent_Formatting(t *testing.T) {
	// Test specific formatting requirements
	m := &model.Model{
		MCPTools: map[string][]api.MCPTool{
			"format-test": {
				{Name: "test-tool", Description: "A test tool"},
			},
		},
		McpToolsViewport: viewport.Model{Width: 80},
	}

	content := GenerateMcpToolsContent(m)

	// Check for proper formatting
	lines := strings.Split(content, "\n")

	// Should have at least 5 lines (header, tool name, description, empty line, empty line)
	assert.GreaterOrEqual(t, len(lines), 5)

	// Check header format
	assert.Contains(t, lines[0], "=== format-test ===")

	// Check tool format (should be indented with bullet)
	foundName := false
	foundDesc := false
	for i, line := range lines {
		if strings.Contains(line, "test-tool") {
			assert.True(t, strings.HasPrefix(line, "  • "), "Tool line should start with '  • '")
			assert.Equal(t, "  • test-tool", line)
			foundName = true
			// Check that description follows on next line
			if i+1 < len(lines) {
				assert.Equal(t, "    A test tool", lines[i+1])
				foundDesc = true
			}
		}
	}
	assert.True(t, foundName, "Tool name line not found")
	assert.True(t, foundDesc, "Tool description line not found")
}

func TestGenerateMcpToolsContent_LongDescription(t *testing.T) {
	// Test wrapping of long descriptions
	m := &model.Model{
		MCPTools: map[string][]api.MCPTool{
			"wrap-test": {
				{
					Name:        "long-tool",
					Description: "This is a very long description that should be wrapped to multiple lines when displayed in the viewport to ensure readability",
				},
			},
		},
		McpToolsViewport: viewport.Model{Width: 50}, // Narrow viewport to force wrapping
	}

	content := GenerateMcpToolsContent(m)
	lines := strings.Split(content, "\n")

	// Find the description lines
	var descLines []string
	inDesc := false
	for _, line := range lines {
		if strings.Contains(line, "long-tool") {
			inDesc = true
			continue
		}
		if inDesc && strings.HasPrefix(line, "    ") {
			descLines = append(descLines, line)
		} else if inDesc && line == "" {
			break
		}
	}

	// Should have multiple description lines due to wrapping
	assert.Greater(t, len(descLines), 1, "Long description should be wrapped to multiple lines")

	// Each wrapped line should be properly indented
	for _, line := range descLines {
		assert.True(t, strings.HasPrefix(line, "    "), "Description lines should be indented with 4 spaces")
		// Line length should not exceed viewport width minus indentation
		assert.LessOrEqual(t, len(line), 50, "Wrapped lines should not exceed viewport width")
	}
}
