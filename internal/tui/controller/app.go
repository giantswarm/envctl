package controller

import (
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"

	tea "github.com/charmbracelet/bubbletea"
)

// AppModel wraps the model to handle updates and views
type AppModel struct {
	model *model.Model
}

// NewAppModel creates a new app wrapper
func NewAppModel(m *model.Model) AppModel {
	return AppModel{model: m}
}

// Init implements tea.Model
func (a AppModel) Init() tea.Cmd {
	return a.model.Init()
}

// Update implements tea.Model
func (a AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size updates
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		a.model.Width = msg.Width
		a.model.Height = msg.Height
		return a, nil
	}

	// Let the update function handle all other messages
	updatedModel, cmd := Update(msg, a.model)
	a.model = updatedModel
	return a, cmd
}

// View implements tea.Model
func (a AppModel) View() string {
	return view.Render(a.model)
}
