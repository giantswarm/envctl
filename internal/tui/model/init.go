package model

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/kube"
	"envctl/pkg/logging"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// InitializeModel creates and initializes a new TUI model with the new architecture
func InitializeModel(cfg TUIConfig, logChannel <-chan logging.LogEntry) (*Model, error) {

	// Get current kube context if not provided
	currentContext, _ := kube.GetCurrentKubeContext()

	// Create the model
	m := &Model{
		// Service Architecture
		Orchestrator:    cfg.Orchestrator,
		OrchestratorAPI: cfg.OrchestratorAPI,
		MCPServiceAPI:   cfg.MCPServiceAPI,
		PortForwardAPI:  cfg.PortForwardAPI,
		K8sServiceAPI:   cfg.K8sServiceAPI,

		// Cluster info
		ManagementClusterName: cfg.ManagementClusterName,
		WorkloadClusterName:   cfg.WorkloadClusterName,
		CurrentKubeContext:    currentContext,

		// Configuration
		PortForwardingConfig: cfg.PortForwardingConfig,
		MCPServerConfig:      cfg.MCPServerConfig,
		AggregatorConfig:     cfg.AggregatorConfig,

		// UI State
		CurrentAppMode: ModeInitializing,
		ColorMode:      cfg.ColorMode,
		DebugMode:      cfg.DebugMode,

		// Data structures
		K8sConnections: make(map[string]*api.K8sConnectionInfo),
		PortForwards:   make(map[string]*api.PortForwardServiceInfo),
		MCPServers:     make(map[string]*api.MCPServerInfo),
		MCPTools:       make(map[string][]api.MCPTool),

		// UI Components
		Spinner:            spinner.New(),
		LogViewport:        viewport.New(80, 20),
		MainLogViewport:    viewport.New(80, 10),
		McpConfigViewport:  viewport.New(80, 20),
		McpToolsViewport:   viewport.New(80, 20),
		NewConnectionInput: textinput.New(),
		Help:               help.New(),
		Keys:               DefaultKeyMap(),

		// Channels
		TUIChannel: make(chan tea.Msg, 100),
		LogChannel: logChannel,

		// Activity log
		ActivityLog: []string{},
	}

	// Subscribe to state changes
	if m.OrchestratorAPI != nil {
		m.StateChangeEvents = m.OrchestratorAPI.SubscribeToStateChanges()
	}

	// Configure spinner
	m.Spinner.Spinner = spinner.Dot

	// Configure text input
	m.NewConnectionInput.Placeholder = "Enter MC name"
	m.NewConnectionInput.Focus()
	m.NewConnectionInput.CharLimit = 50
	m.NewConnectionInput.Width = 30

	return m, nil
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
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
			key.WithKeys("M"),
			key.WithHelp("M", "show MCP tools"),
		),
	}
}

// Init implements the tea.Model interface
func (m *Model) Init() tea.Cmd {
	// Immediately transition to main dashboard
	m.CurrentAppMode = ModeMainDashboard

	return tea.Batch(
		m.Spinner.Tick,
		m.startOrchestrator(),
		m.ListenForStateChanges(),
		m.ListenForLogs(),
		ChannelReaderCmd(m.TUIChannel),
	)
}

// Update implements the tea.Model interface
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// For now, just return the model unchanged
	// The actual update logic will be handled by a wrapper
	return m, nil
}

// View implements the tea.Model interface
func (m *Model) View() string {
	// For now, return a simple view
	// The actual view will be handled by a wrapper
	return fmt.Sprintf("envctl - Services: %d K8s, %d Port Forwards, %d MCP Servers\n\nPress ? for help, q to quit",
		len(m.K8sConnections),
		len(m.PortForwards),
		len(m.MCPServers),
	)
}

// startOrchestrator starts the orchestrator
func (m *Model) startOrchestrator() tea.Cmd {
	return func() tea.Msg {
		// Skip if orchestrator is nil (e.g., in tests)
		if m.Orchestrator == nil {
			return nil
		}

		ctx := context.Background()
		if err := m.Orchestrator.Start(ctx); err != nil {
			return ServiceErrorMsg{
				Label: "orchestrator",
				Err:   err,
			}
		}

		// Initial data refresh
		if err := m.RefreshServiceData(); err != nil {
			return ServiceErrorMsg{
				Label: "refresh",
				Err:   err,
			}
		}

		// Return nil - data refresh is complete
		return nil
	}
}

// ListenForStateChanges listens for service state change events
func (m *Model) ListenForStateChanges() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.StateChangeEvents
		if !ok {
			return nil
		}
		return event
	}
}

// ListenForLogs listens for log entries
func (m *Model) ListenForLogs() tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-m.LogChannel
		if !ok {
			return nil
		}
		return NewLogEntryMsg{Entry: entry}
	}
}

// ChannelReaderCmd reads messages from a channel and returns them as tea.Msg
func ChannelReaderCmd(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}
