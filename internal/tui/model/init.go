package model

import (
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"fmt"
	"sync"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
func InitialModel(
	mcName, wcName, kubeContext string, 
	tuiDebug bool, 
	mcpServerConfig []mcpserver.MCPServerConfig, 
	portForwardingConfig []portforwarding.PortForwardingConfig, 
	serviceMgr managers.ServiceManagerAPI,
	kubeMgr k8smanager.KubeManagerAPI,
) *Model {
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
		MCPServerConfig:          mcpServerConfig,
		PortForwardingConfig:     portForwardingConfig,
		ServiceManager:           serviceMgr,
		KubeMgr:                  kubeMgr,
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
		StashedMcName:            "",
		ClusterInfo:              nil,
		DependencyGraph:          nil,
	}

	// m.Help.ShowAll = true // Help styling removed for now

	// Basic initialization that CAN be done within model package:
	if wcName != "" {
		m.WCHealth = ClusterHealthInfo{IsLoading: true}
	}

	// McpProxyOrder will be initialized by the controller.
	m.McpProxyOrder = nil // Initialize explicitly

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

// Init implements tea.Model and starts asynchronous bootstrap tasks.
// It now also starts the port forwarding and MCP services using the ServiceManager.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.ServiceManager == nil {
		// This should not happen if InitialModel and NewProgram are correctly wired
		errMsg := "ServiceManager not initialized in TUI model"
		m.ActivityLog = append(m.ActivityLog, errMsg)
		m.QuittingMessage = errMsg
		return tea.Quit // or some error message
	}

	// 1. Prepare ManagedServiceConfig slice
	var managedServiceConfigs []managers.ManagedServiceConfig
	for _, pfCfg := range m.PortForwardingConfig { // These are populated by InitialModel
		managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
			Type:   managers.ServiceTypePortForward,
			Label:  pfCfg.Label,
			Config: pfCfg,
		})
	}
	for _, mcpCfg := range m.MCPServerConfig { // These are populated by InitialModel
		managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
			Type:   managers.ServiceTypeMCPServer,
			Label:  mcpCfg.Name,
			Config: mcpCfg,
		})
	}

	// 2. Define the TUI service update callback
	tuiServiceUpdateCb := func(update managers.ManagedServiceUpdate) {
		if m.TUIChannel != nil {
			// Send the update to the TUI's main channel to be processed in Model.Update
			// This runs in the goroutine of the service manager, so it must not block.
			// Ensure TUIChannel is buffered or consumed promptly.
			m.TUIChannel <- ServiceUpdateMsg{Update: update}
		}
	}

	// 3. Create a command to start all services
	if len(managedServiceConfigs) > 0 {
		startServicesCmd := func() tea.Msg {
			// The WaitGroup here is for the ServiceManager's internal goroutines.
			// The TUI model might have its own WaitGroup for other async tasks if needed.
			var wg sync.WaitGroup 
			_, startupErrors := m.ServiceManager.StartServices(managedServiceConfigs, tuiServiceUpdateCb, &wg)
			
			// We need a way to signal the TUI that all services have been *attempted* to start
			// and to pass any initial startup errors. Using AllServicesStartedMsg for this.
			// Note: This message is sent after StartServices returns. Individual updates
			// will arrive via tuiServiceUpdateCb.
			return AllServicesStartedMsg{InitialStartupErrors: startupErrors}
		}
		cmds = append(cmds, startServicesCmd)
	}

	// Listen for async messages on the model's TUIChannel
	if m.TUIChannel != nil {
		cmds = append(cmds, channelReaderCmd(m.TUIChannel))
	}

	// Spinner tick
	cmds = append(cmds, m.Spinner.Tick)

	// Any other initial commands previously dispatched by controller.AppModel.Init() 
	// or specific to the model's setup would go here.
	// For example, fetching initial cluster health, etc. (These might also become services managed by ServiceManager if complex)

	return tea.Batch(cmds...)
}
