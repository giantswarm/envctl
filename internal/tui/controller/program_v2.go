package controller

import (
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// NewProgramV2 initializes the TUI application using the new service architecture
func NewProgramV2(
	mcName, wcName, currentKubeContext string,
	tuiDebugMode bool,
	envctlCfg config.EnvctlConfig,
	logChan <-chan logging.LogEntry,
) (*tea.Program, error) {
	// Initialize the ModelV2
	m, err := model.InitializeModelV2(mcName, wcName, envctlCfg, logChan)
	if err != nil {
		return nil, err
	}

	// Set debug mode
	m.DebugMode = tuiDebugMode

	// Create the AppModelV2 wrapper
	appModel := NewAppModelV2(m)

	// Create and return the Bubble Tea program
	return tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion()), nil
}
