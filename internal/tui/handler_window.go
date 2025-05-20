package tui

import tea "github.com/charmbracelet/bubbletea"

// handleWindowSizeMsg updates the model with the new terminal dimensions when the window is resized.
// It also transitions from ModeInitializing → ModeMainDashboard once we know the size.
func handleWindowSizeMsg(m model, msg tea.WindowSizeMsg) (model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	// Leaving the initial state once we have dimensions.
	if m.currentAppMode == ModeInitializing {
		m.currentAppMode = ModeMainDashboard
		m.isLoading = false
	}
	return m, nil
}
