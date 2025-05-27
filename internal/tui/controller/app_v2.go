package controller

import (
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"

	tea "github.com/charmbracelet/bubbletea"
)

// AppModelV2 wraps ModelV2 and provides the controller logic
type AppModelV2 struct {
	Model *model.ModelV2
}

// NewAppModelV2 creates a new AppModelV2
func NewAppModelV2(m *model.ModelV2) *AppModelV2 {
	return &AppModelV2{
		Model: m,
	}
}

// Init implements tea.Model
func (a *AppModelV2) Init() tea.Cmd {
	return a.Model.Init()
}

// Update implements tea.Model
func (a *AppModelV2) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Use UpdateV2 to handle the update logic
	updatedModel, cmd := UpdateV2(msg, a.Model)
	a.Model = updatedModel
	return a, cmd
}

// View implements tea.Model
func (a *AppModelV2) View() string {
	// Use the view package to render the model
	return view.RenderV2(a.Model)
}
