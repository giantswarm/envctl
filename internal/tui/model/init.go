package model

import (
	"envctl/internal/mcpserver"
	"envctl/internal/service"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// getManagementClusterContextIdentifier generates the MC part of a kube context name.
// func (m *model) getManagementClusterContextIdentifier() string {
//     return m.managementCluster
// }

// getWorkloadClusterContextIdentifier generates the WC context identifier based on MC and WC names.
// func (m *model) getWorkloadClusterContextIdentifier() string {
//     if m.workloadCluster == "" {
//         return ""
//     }
//     if m.managementCluster != "" && strings.HasPrefix(m.workloadCluster, m.managementCluster+"-") {
//         return m.workloadCluster
//     }
//     if m.managementCluster != "" {
//         return m.managementCluster + "-" + m.workloadCluster
//     }
//     return m.workloadCluster
// }

// DefaultKeyMap returns a KeyMap with the default bindings used by the TUI.
// Moved from controller package.
func DefaultKeyMap() KeyMap { // Returns model.KeyMap (KeyMap is in this package)
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "navigate up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "navigate down"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous panel"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select/confirm"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel/back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "toggle help"),
		),
		NewCollection: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new connection"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart forwarder"),
		),
		SwitchContext: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "switch k8s context"),
		),
		ToggleDark: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "toggle dark/light mode"),
		),
		ToggleDebug: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "toggle debug info"),
		),
		CopyLogs: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy logs"),
		),
		ToggleLog: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "toggle log overlay"),
		),
		ToggleMcpConfig: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "show MCP config"),
		),
	}
}

// InitialModel constructs the initial model with sensible defaults.
func InitialModel(mcName, wcName, kubeContext string, tuiDebug bool) *Model {
	ti := textinput.New()
	ti.Placeholder = "Management Cluster"
	ti.CharLimit = 156
	ti.Width = 50

	// Buffered channel to avoid blocking goroutines.
	tuiMsgChannel := make(chan tea.Msg, 100)

	// Force dark background for lipgloss; helps with colour-consistency.
	colorProfile := lipgloss.ColorProfile().String()
	// lipgloss.SetHasDarkBackground(true) // MOVED to internal/color/Initialize
	colorMode := fmt.Sprintf("%s (Dark: %v)", colorProfile, true) // This might need adjustment based on how dark mode is determined globally

	// Spinner setup.
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := Model{
		ManagementClusterName:    mcName,
		WorkloadClusterName:      wcName,
		CurrentKubeContext:       kubeContext,
		PortForwards:             make(map[string]*PortForwardProcess),
		PortForwardOrder:         make([]string, 0),
		McpServers:               make(map[string]*McpServerProcess),
		ActivityLog:              make([]string, 0),
		ActivityLogDirty:         true,
		LogViewportLastWidth:     0,
		MainLogViewportLastWidth: 0,
		MCHealth:                 ClusterHealthInfo{IsLoading: true},
		CurrentAppMode:           ModeInitializing,
		NewConnectionInput:       ti,
		CurrentInputStep:         McInputStep,
		TUIChannel:               tuiMsgChannel,
		DebugMode:                tuiDebug,
		ColorMode:                colorMode,
		LogViewport:              viewport.New(0, 0),
		MainLogViewport:          viewport.New(0, 0),
		Spinner:                  s,
		IsLoading:                true,
		Keys:                     DefaultKeyMap(),
		Help:                     help.New(),
		McpConfigViewport:        viewport.New(0, 0),
		Services:                 service.Default(),
		StashedMcName:            "",
		ClusterInfo:              nil,
		DependencyGraph:          nil,
	}

	// m.Help.ShowAll = true // Help styling removed for now

	// Basic initialization that CAN be done within model package:
	if wcName != "" {
		m.WCHealth = ClusterHealthInfo{IsLoading: true}
	}

	// Build navigation order for MCP proxies - this uses mcpserver, which is fine for model to know about.
	// This might be better if controller sets it up if McpProxyOrder is purely for controller navigation.
	// For now, leave as it only depends on mcpserver package, not controller or view.
	m.McpProxyOrder = nil // Initialize explicitly
	for _, cfg := range mcpserver.PredefinedMcpServers {
		m.McpProxyOrder = append(m.McpProxyOrder, cfg.Name)
	}

	// Initial focused panel can be set here if it's a sensible default not requiring controller logic
	if len(m.PortForwardOrder) > 0 { // PortForwardOrder will be empty now initially
		// m.FocusedPanelKey = m.PortForwardOrder[0] // This will need to be set by controller after SetupPortForwards
	} else if mcName != "" {
		m.FocusedPanelKey = McPaneFocusKey // McPaneFocusKey is a model constant
	} // Else, FocusedPanelKey remains empty, controller can set it.

	return &m
}

// channelReaderCmd returns a Bubbletea command that forwards messages from the given channel.
func channelReaderCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// Init implements tea.Model and starts asynchronous bootstrap tasks for the model itself.
// Controller-level command dispatch will be handled by controller.AppModel.Init().
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Listen for async messages on the model's TUIChannel (if model directly uses it for internal ops)
	if m.TUIChannel != nil { // Check if TUIChannel is part of model's direct responsibility
		cmds = append(cmds, channelReaderCmd(m.TUIChannel))
	}

	// Spinner tick is closely tied to model's IsLoading state view
	cmds = append(cmds, m.Spinner.Tick)

	// Initial commands related to fetching cluster info, health, port-forwards, MCPs
	// are now dispatched by controller.AppModel.Init() after this model.Init() completes.

	return tea.Batch(cmds...)
}
