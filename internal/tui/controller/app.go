package controller

import (
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"
	"envctl/internal/utils"

	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// AppModel is the top-level tea.Model for the application.
// It holds the actual data model and coordinates updates via controller logic.
type AppModel struct {
	model *model.Model
	// If controller needs its own state or services, they go here
	// or are part of a separate Controller struct embedded/held here.
}

// NewAppModel creates a new AppModel.
func NewAppModel(m *model.Model, mcName, wcName string) *AppModel {
	app := &AppModel{model: m}

	// Configure initial port-forwards and dependency graph using controller functions
	// These were previously in model.InitialModel
	SetupPortForwards(app.model, mcName, wcName)
	app.model.DependencyGraph = BuildDependencyGraph(app.model)

	// Set initial focused panel key (logic moved from model.InitialModel)
	if len(app.model.PortForwardOrder) > 0 {
		app.model.FocusedPanelKey = app.model.PortForwardOrder[0]
	} else if mcName != "" { // mcName is the initial management cluster name
		app.model.FocusedPanelKey = model.McPaneFocusKey // McPaneFocusKey is a model constant
	} // Else, FocusedPanelKey remains empty or default from model.InitialModel

	return app
}

// Init initializes the AppModel.
func (a *AppModel) Init() tea.Cmd {
	var modelCmds tea.Cmd
	if a.model != nil {
		modelCmds = a.model.Init() // Call model's own Init first
	}

	var controllerCmds []tea.Cmd

	// Commands previously in model.Init, now dispatched by controller
	controllerCmds = append(controllerCmds, GetCurrentKubeContextCmd(a.model.Services.Cluster))
	controllerCmds = append(controllerCmds, FetchClusterListCmd())

	if a.model.ManagementClusterName != "" {
		mcTargetContext := utils.BuildMcContext(a.model.ManagementClusterName)
		controllerCmds = append(controllerCmds, FetchNodeStatusCmd(mcTargetContext, true, a.model.ManagementClusterName))
	}
	if a.model.WorkloadClusterName != "" && a.model.ManagementClusterName != "" {
		wcTargetContext := utils.BuildWcContext(a.model.ManagementClusterName, a.model.WorkloadClusterName)
		controllerCmds = append(controllerCmds, FetchNodeStatusCmd(wcTargetContext, false, a.model.WorkloadClusterName))
	}

	// SetupPortForwards and BuildDependencyGraph were already moved to NewAppModel for initial setup.
	// GetInitialPortForwardCmds should operate on the now configured model.
	controllerCmds = append(controllerCmds, GetInitialPortForwardCmds(a.model)...)

	if proxyCmds := StartMcpProxiesCmd(a.model.Services.Proxy, a.model.TUIChannel); len(proxyCmds) > 0 {
		controllerCmds = append(controllerCmds, proxyCmds...)
	}

	tickCmd := tea.Tick(HealthUpdateInterval, func(t time.Time) tea.Msg { return model.RequestClusterHealthUpdate{} })
	controllerCmds = append(controllerCmds, tickCmd)

	// Combine model's own init commands (if any) with controller's commands
	finalCmds := []tea.Cmd{modelCmds}
	finalCmds = append(finalCmds, controllerCmds...)

	return tea.Batch(finalCmds...)
}

// View renders the UI by delegating to the view package with the current model.
func (a *AppModel) View() string {
	if a.model != nil {
		return view.Render(a.model)
	}
	return "Error: model not initialized in AppModel"
}

// Update is the main message loop for the application.
// It uses controller logic (handlers) to update the model based on messages.
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.model == nil {
		// Should not happen if initialized correctly
		return a, tea.Quit
	}
	model.RecordMsgSample(msg) // Call model.RecordMsgSample

	// Delegate to a central dispatch function within the controller package
	// This dispatch function will contain the main switch statement for messages
	// and call the appropriate controller.handleXYZ functions.
	var cmd tea.Cmd
	// The handlers will modify a.model directly or return a new one.
	// For now, assume they modify in place and return updated model and cmd.
	// This mainControllerDispatch needs to be created.
	updatedModel, cmd := mainControllerDispatch(a.model, msg)
	a.model = updatedModel // Ensure our model reference is updated if a new one is returned

	return a, cmd
}

// mainControllerDispatch will be the new home for the switch from model.Update
// It will be defined in another controller file (e.g., controller_update.go or similar)
// For now, this is a placeholder for the edit tool.
// func mainControllerDispatch(m *model.Model, msg tea.Msg) (*model.Model, tea.Cmd) { /* ... */ }
