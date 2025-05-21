package controller

import (
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/tui/model"

	tea "github.com/charmbracelet/bubbletea"
)

// NewProgram assembles the model and returns a ready-to-run *tea.Program.
// View and Controller aspects are handled by the model's Init/Update/View methods
// and the functions/methods they call in their respective packages.
// It now accepts a ServiceManagerAPI to handle background service lifecycles.
func NewProgram(
	mcName, wcName, kubeContext string, 
	tuiDebug bool, 
	mcpServerConfig []mcpserver.MCPServerConfig, // Kept for now, TUI model will use these to create ManagedServiceConfig
	portForwardingConfig []portforwarding.PortForwardingConfig, // Kept for now
	serviceMgr managers.ServiceManagerAPI, // Added ServiceManagerAPI
	kubeMgr k8smanager.KubeManagerAPI, // ADDED kubeMgr
) *tea.Program {
	// Pass serviceMgr and kubeMgr to InitialModel.
	initialMdl := model.InitialModel(mcName, wcName, kubeContext, tuiDebug, mcpServerConfig, portForwardingConfig, serviceMgr, kubeMgr)
	app := NewAppModel(initialMdl, mcName, wcName) // NewAppModel might also need serviceMgr if it directly uses it.
	                                               // For now, assuming initialMdl handles it.
	return tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())
}
