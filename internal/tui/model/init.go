package model

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"errors"
	"time"

	"envctl/internal/orchestrator"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"envctl/internal/api"
	"envctl/internal/kube"
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
		ToggleMcpTools: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "show MCP tools"),
		),
	}
}

// InitialModel constructs the initial model with sensible defaults.
func InitialModel(
	mcName string,
	wcName string,
	currentContext string,
	debugMode bool,
	cfg config.EnvctlConfig,
	logChannel <-chan logging.LogEntry,
) *Model {
	// Create event bus and state store
	eventBus := reporting.NewEventBus()
	stateStore := reporting.NewStateStore()

	// Create the event bus adapter
	eventBusAdapter := reporting.NewEventBusAdapter(eventBus, stateStore)

	// Create the TUI reporter with the same state store
	tuiChannel := make(chan tea.Msg, 100)
	tuiReporterConfig := reporting.DefaultTUIReporterConfig()
	tuiReporterConfig.StateStore = stateStore // Use the same state store!
	tuiReporter := reporting.NewTUIReporterWithConfig(tuiChannel, tuiReporterConfig)

	// Subscribe TUI to service state events from the event bus
	// This ensures the TUI receives all state updates
	serviceStateFilter := reporting.FilterByType(
		reporting.EventTypeServiceStarting,
		reporting.EventTypeServiceRunning,
		reporting.EventTypeServiceStopping,
		reporting.EventTypeServiceStopped,
		reporting.EventTypeServiceFailed,
		reporting.EventTypeServiceRetrying,
		reporting.EventTypeHealthCheck,
	)

	eventBus.Subscribe(serviceStateFilter, func(event reporting.Event) {
		// Convert event to TUI message
		if stateEvent, ok := event.(*reporting.ServiceStateEvent); ok {
			// Send the event directly to the TUI channel
			select {
			case tuiChannel <- *stateEvent:
				// Event sent successfully
			default:
				// Channel full, log warning
				logging.Warn("TUIModel", "TUI channel full, dropping service state event for %s", stateEvent.SourceLabel)
			}
		}
	})

	// Create service manager with the reporter
	serviceMgr := managers.NewServiceManager(eventBusAdapter)

	// Create API provider first (needed by orchestrator)
	kubeMgr := kube.NewManager(eventBusAdapter)
	apiProvider := api.NewProvider(eventBus, stateStore, kubeMgr)

	// Create the orchestrator
	orch := orchestrator.New(
		serviceMgr,
		eventBusAdapter,
		orchestrator.Config{
			MCName:              mcName,
			WCName:              wcName,
			PortForwards:        cfg.PortForwards,
			MCPServers:          cfg.MCPServers,
			HealthCheckInterval: 15 * time.Second,
		},
	)

	// Create port forward processes map
	portForwards := make(map[string]*PortForwardProcess)
	portForwardOrder := []string{}
	for _, pf := range cfg.PortForwards {
		if pf.Enabled {
			portForwards[pf.Name] = &PortForwardProcess{
				Label:     pf.Name,
				Config:    pf,
				Active:    true,
				StatusMsg: "Not started",
			}
			portForwardOrder = append(portForwardOrder, pf.Name)
		}
	}

	// Create MCP server processes map
	mcpServers := make(map[string]*McpServerProcess)
	mcpProxyOrder := []string{}
	for _, mcp := range cfg.MCPServers {
		if mcp.Enabled {
			mcpServers[mcp.Name] = &McpServerProcess{
				Label:     mcp.Name,
				Config:    mcp,
				Active:    true,
				StatusMsg: "Not started",
			}
			mcpProxyOrder = append(mcpProxyOrder, mcp.Name)
		}
	}

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create viewports
	logVp := viewport.New(80, 10)
	logVp.SetContent("Activity log will appear here...")

	mainLogVp := viewport.New(80, 10)
	mainLogVp.SetContent("Main log will appear here...")

	mcpConfigVp := viewport.New(80, 20)
	mcpToolsVp := viewport.New(80, 20)

	// Create text input for new connection
	ti := textinput.New()
	ti.Placeholder = "Enter management cluster name"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return &Model{
		CurrentAppMode:        ModeInitializing,
		ManagementClusterName: mcName,
		WorkloadClusterName:   wcName,
		CurrentKubeContext:    currentContext,
		DebugMode:             debugMode,
		PortForwardingConfig:  cfg.PortForwards,
		PortForwards:          portForwards,
		PortForwardOrder:      portForwardOrder,
		McpServers:            mcpServers,
		McpProxyOrder:         mcpProxyOrder,
		MCPServerConfig:       cfg.MCPServers,
		ActivityLog:           []string{},
		LogViewport:           logVp,
		MainLogViewport:       mainLogVp,
		McpConfigViewport:     mcpConfigVp,
		McpToolsViewport:      mcpToolsVp,
		Spinner:               s,
		NewConnectionInput:    ti,
		CurrentInputStep:      McInputStep,
		Keys:                  DefaultKeyMap(),
		Help:                  help.New(),
		TUIChannel:            tuiChannel,
		ServiceManager:        serviceMgr,
		Reporter:              tuiReporter,
		Orchestrator:          orch,
		LogChannel:            logChannel,
		APIs:                  apiProvider,
		MCPTools:              make(map[string][]api.MCPTool),
		StatusBarMessage:      "",
		StatusBarMessageType:  StatusBarInfo,
	}
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

		// Reconcile state after orchestrator starts to ensure UI consistency
		m.ReconcileState()

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
