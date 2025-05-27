package controller

import (
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppModel(t *testing.T) {
	// Create a test model
	m := &model.Model{
		Width:  80,
		Height: 24,
	}

	// Create app model
	app := NewAppModel(m)

	// Verify the model is wrapped correctly
	assert.NotNil(t, app.model)
	assert.Equal(t, m, app.model)
}

func TestAppModel_Init(t *testing.T) {
	// Create a test model
	m := &model.Model{
		TUIChannel:        make(chan tea.Msg, 1),
		LogChannel:        make(<-chan logging.LogEntry),             // Closed channel
		StateChangeEvents: make(<-chan api.ServiceStateChangedEvent), // Closed channel
	}
	app := NewAppModel(m)

	// Call Init - this calls the model's Init which starts goroutines
	// We should not actually call this in tests as it starts background goroutines
	// Instead, we'll just verify the method exists and returns a command
	cmd := app.Init()

	// The Init should return a command
	assert.NotNil(t, cmd)
}

func TestAppModel_Update_WindowSizeMsg(t *testing.T) {
	// Create a test model
	m := &model.Model{
		Width:  80,
		Height: 24,
	}
	app := NewAppModel(m)

	// Create a window size message
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 30,
	}

	// Update the model
	updatedModel, cmd := app.Update(msg)

	// Verify the model was updated
	updatedApp, ok := updatedModel.(AppModel)
	require.True(t, ok)
	assert.Equal(t, 100, updatedApp.model.Width)
	assert.Equal(t, 30, updatedApp.model.Height)
	assert.Nil(t, cmd)
}

func TestAppModel_Update_OtherMessages(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.Msg
	}{
		{
			name: "key message",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
		},
		{
			name: "custom message",
			msg:  model.ServiceStartedMsg{Label: "test-service"},
		},
		{
			name: "nil message",
			msg:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test model with a buffered channel
			tuiChan := make(chan tea.Msg, 1)
			defer close(tuiChan)

			m := &model.Model{
				TUIChannel: tuiChan,
			}
			app := NewAppModel(m)

			// Update with the message
			updatedModel, cmd := app.Update(tt.msg)

			// Verify the model is returned
			updatedApp, ok := updatedModel.(AppModel)
			require.True(t, ok)
			assert.NotNil(t, updatedApp.model)

			// For non-window size messages, Update function should be called
			// The command returned depends on the Update function's logic
			if tt.msg != nil {
				// We expect at least the channel reader command
				assert.NotNil(t, cmd)
			}
		})
	}
}

func TestAppModel_View(t *testing.T) {
	// Create a test model with some state
	m := &model.Model{
		Width:                 80,
		Height:                24,
		CurrentAppMode:        model.ModeMainDashboard,
		ManagementClusterName: "test-mc",
		WorkloadClusterName:   "test-wc",
		CurrentKubeContext:    "test-context",
	}
	app := NewAppModel(m)

	// Call View
	view := app.View()

	// Verify we get a non-empty view
	assert.NotEmpty(t, view)
	// The view should contain some expected content based on the model state
	// This depends on the view.Render implementation
}

func TestAppModel_UpdateIntegration(t *testing.T) {
	// Test a sequence of updates to ensure the app model handles them correctly
	tuiChan := make(chan tea.Msg, 10)
	defer close(tuiChan)

	m := &model.Model{
		Width:      80,
		Height:     24,
		TUIChannel: tuiChan,
	}
	app := NewAppModel(m)

	// First, resize the window
	updatedModel, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = updatedModel.(AppModel)
	assert.Equal(t, 120, app.model.Width)
	assert.Equal(t, 40, app.model.Height)

	// Then send a key message
	updatedModel, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	app = updatedModel.(AppModel)
	// The actual behavior depends on the Update function

	// Send a service message
	updatedModel, _ = app.Update(model.ServiceStartedMsg{Label: "test-service"})
	app = updatedModel.(AppModel)
	// Verify the model is still valid
	assert.NotNil(t, app.model)
}
