package model

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/pkg/logging"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// InitializeModelV2 creates and initializes a new TUI model with the new architecture
func InitializeModelV2(mcName, wcName string, cfg config.EnvctlConfig, logChannel <-chan logging.LogEntry) (*ModelV2, error) {
	// Create the orchestrator
	orchConfig := orchestrator.ConfigV2{
		MCName:       mcName,
		WCName:       wcName,
		PortForwards: cfg.PortForwards,
		MCPServers:   cfg.MCPServers,
	}
	orch := orchestrator.NewV2(orchConfig)
	
	// Get the service registry
	registry := orch.GetServiceRegistry()
	
	// Create APIs
	orchestratorAPI := api.NewOrchestratorAPI(orch, registry)
	mcpAPI := api.NewMCPServiceAPI(registry)
	portForwardAPI := api.NewPortForwardServiceAPI(registry)
	k8sAPI := api.NewK8sServiceAPI(registry)
	
	// Create the model
	m := &ModelV2{
		// Service Architecture
		Orchestrator:    orch,
		OrchestratorAPI: orchestratorAPI,
		MCPServiceAPI:   mcpAPI,
		PortForwardAPI:  portForwardAPI,
		K8sServiceAPI:   k8sAPI,
		
		// Cluster info
		ManagementClusterName: mcName,
		WorkloadClusterName:   wcName,
		
		// Configuration
		PortForwardingConfig: cfg.PortForwards,
		MCPServerConfig:      cfg.MCPServers,
		
		// UI State
		CurrentAppMode: ModeInitializing,
		ColorMode:      "auto",
		
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
		Keys:               DefaultKeyMapV2(),
		
		// Channels
		TUIChannel: make(chan tea.Msg, 100),
		LogChannel: logChannel,
		
		// Activity log
		ActivityLog: []string{},
	}
	
	// Subscribe to state changes
	m.StateChangeEvents = m.OrchestratorAPI.SubscribeToStateChanges()
	
	// Configure spinner
	m.Spinner.Spinner = spinner.Dot
	
	// Configure text input
	m.NewConnectionInput.Placeholder = "Enter MC name"
	m.NewConnectionInput.Focus()
	m.NewConnectionInput.CharLimit = 50
	m.NewConnectionInput.Width = 30
	
	return m, nil
}

// DefaultKeyMapV2 returns the default key bindings
func DefaultKeyMapV2() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev panel"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select/start"),
		),
		Esc: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
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
			key.WithKeys("s"),
			key.WithHelp("s", "stop service"),
		),
		SwitchContext: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "switch context"),
		),
		ToggleDark: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "toggle dark mode"),
		),
		ToggleDebug: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "toggle debug"),
		),
		ToggleLog: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "toggle log"),
		),
		CopyLogs: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy logs"),
		),
		ToggleMcpConfig: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "MCP config"),
		),
		ToggleMcpTools: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "MCP tools"),
		),
	}
}

// Init implements the tea.Model interface
func (m *ModelV2) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		m.startOrchestrator(),
		m.listenForStateChanges(),
		m.listenForLogs(),
	)
}

// Update implements the tea.Model interface
func (m *ModelV2) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// For now, just return the model unchanged
	// The actual update logic will be in the controller
	return m, nil
}

// View implements the tea.Model interface
func (m *ModelV2) View() string {
	// For now, return a simple view
	// This will be replaced with the actual view implementation
	return fmt.Sprintf("envctl v2 - Services: %d K8s, %d Port Forwards, %d MCP Servers\n\nPress ? for help, q to quit",
		len(m.K8sConnections),
		len(m.PortForwards),
		len(m.MCPServers),
	)
}

// startOrchestrator starts the orchestrator
func (m *ModelV2) startOrchestrator() tea.Cmd {
	return func() tea.Msg {
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
		
		m.CurrentAppMode = ModeMainDashboard
		return nil
	}
}

// listenForStateChanges listens for service state change events
func (m *ModelV2) listenForStateChanges() tea.Cmd {
	return func() tea.Msg {
		for event := range m.StateChangeEvents {
			m.TUIChannel <- event
		}
		return nil
	}
}

// listenForLogs listens for log entries
func (m *ModelV2) listenForLogs() tea.Cmd {
	return func() tea.Msg {
		for entry := range m.LogChannel {
			logLine := fmt.Sprintf("[%s] %s: %s",
				entry.Timestamp.Format("15:04:05"),
				entry.Subsystem,
				entry.Message,
			)
			
			m.ActivityLog = append(m.ActivityLog, logLine)
			m.ActivityLogDirty = true
			
			// Limit log size
			if len(m.ActivityLog) > MaxActivityLogLines {
				m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-MaxActivityLogLines:]
			}
		}
		return nil
	}
} 