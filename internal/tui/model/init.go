package model

import (
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
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
	kubeMgr k8smanager.KubeManagerAPI,
) *Model {
	ti := textinput.New()
	ti.Placeholder = "Management Cluster"
	ti.CharLimit = 156
	ti.Width = 50

	// Buffered channel to avoid blocking goroutines.
	tuiMsgChannel := make(chan tea.Msg, 1000)

	// Force dark background for lipgloss; helps with colour-consistency.
	colorProfile := lipgloss.ColorProfile().String()
	// lipgloss.SetHasDarkBackground(true) // MOVED to internal/color/Initialize
	colorMode := fmt.Sprintf("%s (Dark: %v)", colorProfile, true) // This might need adjustment based on how dark mode is determined globally

	// Spinner setup.
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create TUIReporter first, as ServiceManager needs it.
	tuiReporter := reporting.NewTUIReporter(tuiMsgChannel)

	// Create ServiceManager, now passing the reporter.
	serviceMgr := managers.NewServiceManager(tuiReporter)

	m := Model{
		ManagementClusterName:    mcName,
		WorkloadClusterName:      wcName,
		CurrentKubeContext:       kubeContext,
		MCPServerConfig:          mcpServerConfig,
		PortForwardingConfig:     portForwardingConfig,
		ServiceManager:           serviceMgr,
		KubeMgr:                  kubeMgr,
		Reporter:                 tuiReporter,
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

// ChannelReaderCmd returns a Bubbletea command that forwards messages from the given channel.
func ChannelReaderCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg := <-ch
		return msg
	}
}

// Init implements tea.Model and starts asynchronous bootstrap tasks.
// It now also starts the port forwarding and MCP services using the ServiceManager.
func (m *Model) Init() tea.Cmd {
	// fmt.Println("DEBUG_TUI_LIFECYCLE: model.Model.Init() called") // REMOVED
	var cmds []tea.Cmd

	if m.ServiceManager == nil {
		errMsg := "ServiceManager not initialized in TUI model"
		// Use reporter if available, otherwise direct append (should not happen)
		if m.Reporter != nil {
			m.Reporter.Report(reporting.ManagedServiceUpdate{SourceType: reporting.ServiceTypeSystem, SourceLabel: "ModelInit", Level: reporting.LogLevelFatal, Message: errMsg, IsError: true})
		} else {
			m.ActivityLog = append(m.ActivityLog, errMsg) // Fallback
		}
		m.QuittingMessage = errMsg
		return tea.Quit
	}

	var managedServiceConfigs []managers.ManagedServiceConfig
	for _, pfCfg := range m.PortForwardingConfig {
		managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypePortForward, // Use reporting type
			Label:  pfCfg.Label,
			Config: pfCfg,
		})
	}
	for _, mcpCfg := range m.MCPServerConfig {
		managedServiceConfigs = append(managedServiceConfigs, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeMCPServer, // Use reporting type
			Label:  mcpCfg.Name,
			Config: mcpCfg,
		})
	}

	if len(managedServiceConfigs) > 0 {
		startServicesCmd := func() tea.Msg {
			var wg sync.WaitGroup
			// Call StartServices without the updateCb
			_, startupErrors := m.ServiceManager.StartServices(managedServiceConfigs, &wg)
			return AllServicesStartedMsg{InitialStartupErrors: startupErrors}
		}
		cmds = append(cmds, startServicesCmd)
	}

	if m.TUIChannel != nil {
		cmds = append(cmds, ChannelReaderCmd(m.TUIChannel))
	}

	cmds = append(cmds, m.Spinner.Tick)

	return tea.Batch(cmds...)
}
