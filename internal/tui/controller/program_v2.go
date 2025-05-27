package controller

import (
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// NewProgramV2 creates a new Bubble Tea program with the new service architecture
func NewProgramV2(
	mcName, wcName, currentKubeContext string,
	cfg config.EnvctlConfig,
	logChannel <-chan logging.LogEntry,
) (*tea.Program, error) {
	// Initialize the model with the new architecture
	m, err := model.InitializeModelV2(
		mcName,
		wcName,
		currentKubeContext,
		cfg,
		logChannel,
	)
	if err != nil {
		return nil, err
	}

	// Create the app wrapper
	app := NewAppModelV2(m)

	// Create and return the program
	p := tea.NewProgram(app, tea.WithAltScreen())
	return p, nil
}
