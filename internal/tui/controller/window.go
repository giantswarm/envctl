package controller

import (
	"envctl/internal/tui/model"

	tea "github.com/charmbracelet/bubbletea"
)

// handleWindowSizeMsg updates the model with the new terminal dimensions when the window is resized.
// It also transitions from ModeInitializing → ModeMainDashboard once we know the size.
func handleWindowSizeMsg(m *model.Model, msg tea.WindowSizeMsg) (*model.Model, tea.Cmd) {
	m.Width = msg.Width
	m.Height = msg.Height

	if m.CurrentAppMode == model.ModeInitializing {
		m.CurrentAppMode = model.ModeMainDashboard
		m.IsLoading = false
	}
	return m, nil
}
