package controller

import (
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/tui/model"

	tea "github.com/charmbracelet/bubbletea"
)

// NewProgram assembles the model and returns a ready-to-run *tea.Program.
// View and Controller aspects are handled by the model's Init/Update/View methods
// and the functions/methods they call in their respective packages.
func NewProgram(mcName, wcName, kubeContext string, tuiDebug bool, mcpServerConfig []mcpserver.MCPServerConfig, portForwardingConfig []portforwarding.PortForwardingConfig) *tea.Program {
	initialMdl := model.InitialModel(mcName, wcName, kubeContext, tuiDebug, mcpServerConfig, portForwardingConfig)
	app := NewAppModel(initialMdl, mcName, wcName)
	return tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())
}
