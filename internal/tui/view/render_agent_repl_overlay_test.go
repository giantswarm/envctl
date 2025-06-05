package view

import (
	"envctl/internal/tui/model"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/assert"
)

func TestRenderAgentREPLOverlay(t *testing.T) {
	tests := []struct {
		name               string
		model              *model.Model
		expectedContains   []string
		unexpectedContains []string
	}{
		{
			name: "empty agent REPL",
			model: &model.Model{
				Width:             100,
				Height:            30,
				CurrentAppMode:    model.ModeAgentREPLOverlay,
				AgentREPLViewport: viewport.New(80, 20),
				AgentREPLInput:    textinput.New(),
				AgentREPLOutput:   []string{},
			},
			expectedContains: []string{
				"Agent REPL",
				"Welcome to the Agent REPL",
				"MCP>",
			},
		},
		{
			name: "agent REPL with history",
			model: &model.Model{
				Width:             100,
				Height:            30,
				CurrentAppMode:    model.ModeAgentREPLOverlay,
				AgentREPLViewport: viewport.New(80, 20),
				AgentREPLInput:    textinput.New(),
				AgentREPLOutput: []string{
					"MCP> list tools",
					"Available tools (3):",
					"  1. calculate - Perform mathematical calculations",
					"  2. weather - Get weather information",
					"  3. translate - Translate text between languages",
					"",
					"MCP> help",
					"Available commands:",
					"  help, ?                      - Show this help message",
					"  list tools                   - List all available tools",
					"  list resources               - List all available resources",
					"  list prompts                 - List all available prompts",
				},
			},
			expectedContains: []string{
				"Agent REPL",
				"MCP> list tools",
				"Available tools (3):",
				"calculate",
				"weather",
				"translate",
				"MCP> help",
				"Available commands:",
			},
			unexpectedContains: []string{
				"Welcome to the Agent REPL", // Should not show welcome when there's output
			},
		},
		{
			name: "agent REPL with input text",
			model: func() *model.Model {
				m := &model.Model{
					Width:             100,
					Height:            30,
					CurrentAppMode:    model.ModeAgentREPLOverlay,
					AgentREPLViewport: viewport.New(80, 20),
					AgentREPLInput:    textinput.New(),
					AgentREPLOutput:   []string{"MCP> list tools", "No tools available"},
				}
				m.AgentREPLInput.SetValue("call translate")
				return m
			}(),
			expectedContains: []string{
				"Agent REPL",
				"call translate", // The current input
				"MCP>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize the input field
			tt.model.AgentREPLInput.Focus()
			tt.model.AgentREPLInput.Placeholder = "Enter command..."

			// Set viewport content
			content := PrepareAgentREPLContent(tt.model.AgentREPLOutput, tt.model.AgentREPLViewport.Width)
			tt.model.AgentREPLViewport.SetContent(content)

			// Render the overlay
			output := renderAgentREPLOverlay(tt.model)

			// Check expected content
			for _, expected := range tt.expectedContains {
				assert.Contains(t, output, expected, "Expected output to contain: %s", expected)
			}

			// Check unexpected content
			for _, unexpected := range tt.unexpectedContains {
				assert.NotContains(t, output, unexpected, "Expected output NOT to contain: %s", unexpected)
			}

			// Basic structure checks
			assert.Contains(t, output, "↑/↓ scroll or history", "Should contain navigation help")
			assert.Contains(t, output, "Tab complete", "Should contain tab completion help")
			assert.Contains(t, output, "Enter execute", "Should contain execution help")
			assert.Contains(t, output, "Esc close", "Should contain close help")
		})
	}
}

func TestPrepareAgentREPLContent(t *testing.T) {
	tests := []struct {
		name     string
		output   []string
		width    int
		expected []string
	}{
		{
			name:     "empty output shows welcome",
			output:   []string{},
			width:    80,
			expected: []string{"Welcome to the Agent REPL"},
		},
		{
			name: "simple output",
			output: []string{
				"MCP> help",
				"Available commands:",
				"  help - Show help",
			},
			width: 80,
			expected: []string{
				"MCP> help",
				"Available commands:",
				"  help - Show help",
			},
		},
		{
			name: "long lines are wrapped",
			output: []string{
				"This is a very long line that should be wrapped when displayed in the agent REPL viewport",
			},
			width: 30,
			expected: []string{
				"This is a very long line that",
				"should be wrapped when",
				"displayed in the agent REPL",
				"viewport",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareAgentREPLContent(tt.output, tt.width)
			lines := strings.Split(result, "\n")

			for _, expected := range tt.expected {
				found := false
				for _, line := range lines {
					if strings.Contains(line, expected) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected to find line containing: %s", expected)
			}
		})
	}
}
