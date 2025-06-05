package controller

import (
	"strings"

	"envctl/internal/tui/model"
)

// TUIAgentWriter captures agent output and sends it to the TUI viewport
type TUIAgentWriter struct {
	model *model.Model
}

// NewTUIAgentWriter creates a new TUI agent writer
func NewTUIAgentWriter(m *model.Model) *TUIAgentWriter {
	return &TUIAgentWriter{model: m}
}

// Write implements io.Writer
func (w *TUIAgentWriter) Write(p []byte) (n int, err error) {
	// Split the output into lines
	output := string(p)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	// Add non-empty lines to the agent REPL output
	for _, line := range lines {
		if line != "" {
			w.model.AgentREPLOutput = append(w.model.AgentREPLOutput, line)
		}
	}

	return len(p), nil
}
