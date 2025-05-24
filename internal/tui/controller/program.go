package controller

import (
	"envctl/internal/config"
	"envctl/internal/k8smanager"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"

	tea "github.com/charmbracelet/bubbletea"
)

// NewProgram initializes the entire TUI application, including the model and controller.
func NewProgram(
	mcName, wcName, currentKubeContext string,
	tuiDebugMode bool,
	envctlCfg config.EnvctlConfig,
	kubeMgr k8smanager.KubeManagerAPI,
	logChan <-chan logging.LogEntry,
) *tea.Program {
	// Initialize the core data model. ServiceManager is now created within InitialModel.
	m := model.InitialModel(
		mcName,
		wcName,
		currentKubeContext,
		tuiDebugMode,
		envctlCfg,
		kubeMgr,
		logChan,
	)

	// Setup AppModel which acts as the controller layer for Bubble Tea.
	// It takes the initialized model.
	appModel := NewAppModel(m, mcName, wcName)

	// Create and return the Bubble Tea program.
	// Program execution starts when p.Run() is called by the caller.
	return tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
}
