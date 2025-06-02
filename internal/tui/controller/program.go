package controller

import (
	"envctl/internal/tui/model"
	"envctl/pkg/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// NewProgram creates a new Bubble Tea program with the new service architecture
func NewProgram(
	cfg model.TUIConfig,
	logChannel <-chan logging.LogEntry,
) (*tea.Program, error) {
	// Initialize the model with the new architecture
	m, err := model.InitializeModel(cfg, logChannel)
	if err != nil {
		return nil, err
	}

	// Create the app wrapper
	app := NewAppModel(m)

	// Create and return the program
	p := tea.NewProgram(app, tea.WithAltScreen())
	return p, nil
}
