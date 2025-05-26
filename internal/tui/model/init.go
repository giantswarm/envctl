package model

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"errors"
	"fmt"
	"time"

	"envctl/internal/orchestrator"

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
			key.WithHelp("r", "restart service"),
		),
		Stop: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "stop service"),
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
	managementClusterName, workloadClusterName, initialKubeContext string,
	debugMode bool,
	envctlCfg config.EnvctlConfig,
	kubeMgr k8smanager.KubeManagerAPI,
	logChannel <-chan logging.LogEntry,
) *Model {
	// Create TUI reporter and manager
	tuiChannel := make(chan tea.Msg, 1000)
	tuiReporter := reporting.NewTUIReporter(tuiChannel)
	// Ensure kubeMgr uses the TUI reporter
	kubeMgr.SetReporter(tuiReporter)
	serviceMgr := managers.NewServiceManager(tuiReporter)

	// Create orchestrator for health monitoring and service lifecycle
	orch := orchestrator.New(
		kubeMgr,
		serviceMgr,
		tuiReporter,
		orchestrator.Config{
			MCName:              managementClusterName,
			WCName:              workloadClusterName,
			PortForwards:        envctlCfg.PortForwards,
			MCPServers:          envctlCfg.MCPServers,
			HealthCheckInterval: 15 * time.Second,
		},
	)

	m := &Model{
		Width:                 80, // Default, will be updated
		Height:                24, // Default, will be updated
		QuitApp:               false,
		CurrentAppMode:        ModeInitializing,
		FocusedPanelKey:       "",
		ActivityLog:           []string{},
		ManagementClusterName: managementClusterName,
		WorkloadClusterName:   workloadClusterName,
		CurrentKubeContext:    initialKubeContext,
		KubeMgr:               kubeMgr,
		ServiceManager:        serviceMgr,
		Reporter:              tuiReporter,
		Orchestrator:          orch,
		TUIChannel:            tuiChannel,
		PortForwards:          make(map[string]*PortForwardProcess),
		McpServers:            make(map[string]*McpServerProcess),
		ClusterInfo:           &k8smanager.ClusterList{},
		DebugMode:             debugMode,
		PortForwardingConfig:  envctlCfg.PortForwards,
		MCPServerConfig:       envctlCfg.MCPServers,
		// Initialize K8sStateManager with the orchestrator's instance
		K8sStateManager:      orch.GetK8sStateManager(),
		DependencyGraph:      nil, // Will be set by orchestrator
		LogChannel:           logChannel,
		StatusBarMessage:     "",
		StatusBarMessageType: StatusBarInfo,
	}

	// Initialize UI components
	ti := textinput.New()
	ti.Placeholder = "Management Cluster"
	ti.CharLimit = 156
	ti.Width = 50
	m.NewConnectionInput = ti
	m.CurrentInputStep = McInputStep

	// Initialize viewports
	m.LogViewport = viewport.New(0, 0)
	m.MainLogViewport = viewport.New(0, 0)
	m.McpConfigViewport = viewport.New(0, 0)

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m.Spinner = s

	// Initialize other UI state
	m.IsLoading = true
	m.Keys = DefaultKeyMap()
	m.Help = help.New()
	m.ColorMode = fmt.Sprintf("%s (Dark: %v)", lipgloss.ColorProfile().String(), true)
	m.MCHealth = ClusterHealthInfo{IsLoading: true}

	// Populate PortForwards and McpServers with initial placeholder data
	// This ensures that when status updates arrive, the map entries exist.
	for _, pfCfg := range m.PortForwardingConfig {
		if !pfCfg.Enabled {
			continue
		}
		m.PortForwardOrder = append(m.PortForwardOrder, pfCfg.Name)
		m.PortForwards[pfCfg.Name] = &PortForwardProcess{
			Label:     pfCfg.Name,
			StatusMsg: "Initializing...",
			Active:    false,
			Running:   false,
			Config:    pfCfg,
		}
	}

	for _, mcpCfg := range m.MCPServerConfig {
		if !mcpCfg.Enabled {
			continue
		}
		m.McpProxyOrder = append(m.McpProxyOrder, mcpCfg.Name)
		m.McpServers[mcpCfg.Name] = &McpServerProcess{
			Label:     mcpCfg.Name,
			StatusMsg: "Initializing...",
			Active:    false,
			Config:    mcpCfg,
			ProxyPort: mcpCfg.ProxyPort,
			Pid:       0,
		}
	}

	// m.Help.ShowAll = true // Help styling removed for now

	// Basic initialization that CAN be done within model package:
	if workloadClusterName != "" {
		m.WCHealth = ClusterHealthInfo{IsLoading: true}
	}

	// McpProxyOrder will be initialized by the controller.
	m.McpProxyOrder = nil // Initialize explicitly

	// Initial focused panel can be set here if it's a sensible default not requiring controller logic
	if len(m.PortForwardOrder) > 0 { // PortForwardOrder will be empty now initially
		// m.FocusedPanelKey = m.PortForwardOrder[0] // This will need to be set by controller after SetupPortForwards
	} else if managementClusterName != "" {
		m.FocusedPanelKey = McPaneFocusKey // McPaneFocusKey is a model constant
	} // Else, FocusedPanelKey remains empty, controller can set it.

	return m
}

// ChannelReaderCmd returns a Bubbletea command that forwards messages from the given channel.
func ChannelReaderCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg := <-ch
		return msg
	}
}

// Init implements tea.Model and starts asynchronous bootstrap tasks.
// It now starts the orchestrator which handles service lifecycle and health monitoring.
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	if m.Orchestrator == nil {
		errMsg := "Orchestrator not initialized in TUI model"
		logging.Error("ModelInit", errors.New(errMsg), "%s", errMsg)
		m.QuittingMessage = errMsg
		return tea.Quit
	}

	// Start the orchestrator (which will start services and monitor health)
	startOrchestratorCmd := func() tea.Msg {
		ctx := context.Background()
		if err := m.Orchestrator.Start(ctx); err != nil {
			logging.Error("ModelInit", err, "Failed to start orchestrator")
			return AllServicesStartedMsg{InitialStartupErrors: []error{err}}
		}
		// Update dependency graph from orchestrator
		m.DependencyGraph = m.Orchestrator.GetDependencyGraph()
		logging.Info("ModelInit", "Orchestrator started successfully")
		return AllServicesStartedMsg{InitialStartupErrors: nil}
	}
	cmds = append(cmds, startOrchestratorCmd)

	if m.TUIChannel != nil {
		cmds = append(cmds, ChannelReaderCmd(m.TUIChannel))
	}

	if m.LogChannel != nil {
		cmds = append(cmds, ListenForLogEntriesCmd(m.LogChannel))
	}

	cmds = append(cmds, m.Spinner.Tick)

	return tea.Batch(cmds...)
}

// ListenForLogEntriesCmd returns a Bubbletea command that listens on the LogChannel
// and forwards new log entries as NewLogEntryMsg.
func ListenForLogEntriesCmd(logChan <-chan logging.LogEntry) tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-logChan
		if !ok {
			// Channel has been closed, perhaps return a specific nil message or a special "closed" message
			// For now, returning nil will stop this command from re-queueing if Bubble Tea handles it that way.
			return nil
		}
		return NewLogEntryMsg{Entry: entry}
	}
}
