package model

import (
	"context"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/orchestrator"
	"envctl/pkg/logging"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// AppMode represents the current mode of the application
type AppMode int

const (
	ModeInitializing AppMode = iota
	ModeMainDashboard
	ModeNewConnectionInput
	ModeHelpOverlay
	ModeLogOverlay
	ModeMcpConfigOverlay
	ModeMcpToolsOverlay
	ModeAgentREPLOverlay
	ModeQuitting
)

// TUI configuration struct
type TUIConfig struct {
	DebugMode        bool
	ColorMode        string
	MCPServerConfig  []api.MCPServerDefinition
	AggregatorConfig config.AggregatorConfig
	Orchestrator     *orchestrator.Orchestrator
	OrchestratorAPI  api.OrchestratorAPI
	PortForwardAPI   api.PortForwardServiceAPI
	K8sServiceAPI    api.K8sServiceAPI
	AggregatorAPI    api.AggregatorAPI
}

// InputStep represents the current step in the new connection input flow
type InputStep int

const (
	InputStepMC InputStep = iota
	InputStepWC
)

// MessageType represents the type of status bar message
type MessageType int

const (
	StatusBarInfo MessageType = iota
	StatusBarSuccess
	StatusBarError
	StatusBarWarning
)

// OverallAppStatus defines the high-level operational status of the application.
type OverallAppStatus int

const (
	AppStatusUnknown OverallAppStatus = iota // Or AppStatusInitializing
	AppStatusUp
	AppStatusConnecting
	AppStatusDegraded
	AppStatusFailed
)

// String provides a human-readable representation of the OverallAppStatus.
func (s OverallAppStatus) String() string {
	switch s {
	case AppStatusUp:
		return "Up"
	case AppStatusConnecting:
		return "Connecting"
	case AppStatusDegraded:
		return "Degraded"
	case AppStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// String provides a human-readable representation of the AppMode.
func (m AppMode) String() string {
	switch m {
	case ModeInitializing:
		return "Initializing"
	case ModeMainDashboard:
		return "MainDashboard"
	case ModeNewConnectionInput:
		return "NewConnectionInput"
	case ModeHelpOverlay:
		return "HelpOverlay"
	case ModeLogOverlay:
		return "LogOverlay"
	case ModeMcpConfigOverlay:
		return "McpConfigOverlay"
	case ModeMcpToolsOverlay:
		return "McpToolsOverlay"
	case ModeAgentREPLOverlay:
		return "AgentREPLOverlay"
	case ModeQuitting:
		return "Quitting"
	default:
		return "Unknown"
	}
}

// Constants for UI
const (
	MaxActivityLogLines = 1000
	McPaneFocusKey      = "mc-pane"
	WcPaneFocusKey      = "wc-pane"
)

// KeyMap defines all the key bindings for the application
type KeyMap struct {
	Up              key.Binding
	Down            key.Binding
	Tab             key.Binding
	ShiftTab        key.Binding
	Enter           key.Binding
	Esc             key.Binding
	Quit            key.Binding
	Help            key.Binding
	NewCollection   key.Binding
	Restart         key.Binding
	Stop            key.Binding
	SwitchContext   key.Binding
	ToggleDark      key.Binding
	ToggleDebug     key.Binding
	CopyLogs        key.Binding
	ToggleLog       key.Binding
	ToggleMcpConfig key.Binding
	ToggleMcpTools  key.Binding
	ToggleAgentREPL key.Binding
}

// Model represents the state of the TUI application using the new service architecture
type Model struct {
	// Terminal dimensions
	Width  int
	Height int

	// Global application state
	QuitApp         bool
	IsLoading       bool
	CurrentAppMode  AppMode
	LastAppMode     AppMode
	FocusedPanelKey string
	DebugMode       bool
	ColorMode       string

	// Connection Info
	CurrentKubeContext string
	QuittingMessage    string

	// Service Architecture Components
	Orchestrator    *orchestrator.Orchestrator
	OrchestratorAPI api.OrchestratorAPI
	PortForwardAPI  api.PortForwardServiceAPI
	K8sServiceAPI   api.K8sServiceAPI
	AggregatorAPI   api.AggregatorAPI

	// Cached service data for display
	K8sConnections map[string]*api.K8sConnectionInfo
	PortForwards   map[string]*api.PortForwardServiceInfo
	MCPServers     map[string]*api.MCPServerInfo
	MCPTools       map[string][]api.MCPTool
	AggregatorInfo *api.AggregatorInfo

	// MCP Items from aggregator
	MCPToolsWithStatus []api.ToolWithStatus
	MCPResources       []api.MCPResource
	MCPPrompts         []api.MCPPrompt

	// Service ordering for display
	K8sConnectionOrder []string
	PortForwardOrder   []string
	MCPServerOrder     []string

	// Configuration
	MCPServerConfig  []api.MCPServerDefinition
	AggregatorConfig config.AggregatorConfig

	// UI State & Output
	ActivityLog              []string
	ActivityLogDirty         bool
	LogViewportLastWidth     int
	MainLogViewportLastWidth int
	LogViewport              viewport.Model
	MainLogViewport          viewport.Model
	McpConfigViewport        viewport.Model
	McpToolsViewport         viewport.Model
	AgentREPLViewport        viewport.Model
	AgentREPLInput           textinput.Model
	AgentREPLHistory         []string
	AgentREPLHistoryIndex    int
	AgentREPLOutput          []string
	AgentClient              interface{} // Will be *agent.Client, using interface{} to avoid circular import
	Spinner                  spinner.Model
	NewConnectionInput       textinput.Model
	CurrentInputStep         InputStep
	StashedMcName            string
	Keys                     KeyMap
	Help                     help.Model
	TUIChannel               chan tea.Msg
	DependencyGraph          *dependency.Graph
	StatusBarMessage         string
	StatusBarMessageType     MessageType
	StatusBarClearCancel     chan struct{}
	PeriodicTickerStarted    bool

	// Logging
	LogChannel <-chan logging.LogEntry

	// Event subscription
	StateChangeEvents <-chan api.ServiceStateChangedEvent

	// List models for the new UI (stored as interface{} to avoid circular import)
	ClustersList     interface{}
	PortForwardsList interface{}
	MCPServersList   interface{}
	MCPToolsList     interface{} // List for MCP tools display
}

// RefreshServiceData fetches the latest service data from APIs
func (m *Model) RefreshServiceData() error {
	// Skip if APIs are nil (e.g., in tests)
	if m.K8sServiceAPI == nil || m.PortForwardAPI == nil || m.OrchestratorAPI == nil {
		return nil
	}

	ctx := context.Background()

	// Refresh K8s connections
	k8sConns, err := m.K8sServiceAPI.ListConnections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list K8s connections: %w", err)
	}

	// Update K8s connections while preserving order
	newK8sConnections := make(map[string]*api.K8sConnectionInfo)
	for _, conn := range k8sConns {
		newK8sConnections[conn.Label] = conn
	}

	// Update order - add any new connections that aren't in the order yet
	existingInOrder := make(map[string]bool)
	for _, label := range m.K8sConnectionOrder {
		existingInOrder[label] = true
	}
	for _, conn := range k8sConns {
		if !existingInOrder[conn.Label] {
			m.K8sConnectionOrder = append(m.K8sConnectionOrder, conn.Label)
		}
	}
	m.K8sConnections = newK8sConnections

	// Refresh port forwards
	portForwards, err := m.PortForwardAPI.ListForwards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list port forwards: %w", err)
	}

	// Update port forwards while preserving order
	newPortForwards := make(map[string]*api.PortForwardServiceInfo)
	for _, pf := range portForwards {
		newPortForwards[pf.Label] = pf
	}

	// Update order - add any new port forwards that aren't in the order yet
	existingPFInOrder := make(map[string]bool)
	for _, label := range m.PortForwardOrder {
		existingPFInOrder[label] = true
	}
	for _, pf := range portForwards {
		if !existingPFInOrder[pf.Label] {
			m.PortForwardOrder = append(m.PortForwardOrder, pf.Label)
		}
	}
	m.PortForwards = newPortForwards

	// Refresh MCP servers - get them from the service registry instead
	if registry := api.GetServiceRegistry(); registry != nil {
		mcpServices := registry.GetByType(api.TypeMCPServer)

		// Convert service info to MCPServerInfo
		newMCPServers := make(map[string]*api.MCPServerInfo)
		for _, service := range mcpServices {
			mcpInfo := &api.MCPServerInfo{
				Label:   service.GetLabel(),
				State:   string(service.GetState()),
				Health:  string(service.GetHealth()),
				Enabled: true, // Assume enabled if it's in the registry
			}

			// Get additional info from service data if available
			if data := service.GetServiceData(); data != nil {
				if name, ok := data["name"].(string); ok {
					mcpInfo.Name = name
				}
				if icon, ok := data["icon"].(string); ok {
					mcpInfo.Icon = icon
				}
				if enabled, ok := data["enabled"].(bool); ok {
					mcpInfo.Enabled = enabled
				}
			}

			// Get error if any
			if err := service.GetLastError(); err != nil {
				mcpInfo.Error = err.Error()
			}

			newMCPServers[service.GetLabel()] = mcpInfo
		}

		// Update order - add any new MCP servers that aren't in the order yet
		existingMCPInOrder := make(map[string]bool)
		for _, name := range m.MCPServerOrder {
			existingMCPInOrder[name] = true
		}
		for _, service := range mcpServices {
			if !existingMCPInOrder[service.GetLabel()] {
				m.MCPServerOrder = append(m.MCPServerOrder, service.GetLabel())
			}
		}
		m.MCPServers = newMCPServers
	}

	// Refresh aggregator info
	if m.AggregatorAPI != nil {
		aggInfo, err := m.AggregatorAPI.GetAggregatorInfo(ctx)
		if err != nil {
			// Log error but don't fail the entire refresh
			logging.Debug("Model", "Failed to get aggregator info: %v", err)
		} else {
			m.AggregatorInfo = aggInfo
		}

		// Fetch tools with status
		tools, err := m.AggregatorAPI.GetAllToolsWithStatus(ctx)
		if err != nil {
			logging.Debug("Model", "Failed to get tools with status: %v", err)
		} else {
			m.MCPToolsWithStatus = tools
		}

		// Fetch resources
		resources, err := m.AggregatorAPI.GetAllResources(ctx)
		if err != nil {
			logging.Debug("Model", "Failed to get resources: %v", err)
		} else {
			m.MCPResources = resources
		}

		// Fetch prompts
		prompts, err := m.AggregatorAPI.GetAllPrompts(ctx)
		if err != nil {
			logging.Debug("Model", "Failed to get prompts: %v", err)
		} else {
			m.MCPPrompts = prompts
		}
	}

	return nil
}

// GetK8sConnectionHealth returns the health info for a K8s connection
func (m *Model) GetK8sConnectionHealth(label string) (ready int, total int, healthy bool) {
	if conn, exists := m.K8sConnections[label]; exists {
		healthy = conn.Health == "healthy"
		return conn.ReadyNodes, conn.TotalNodes, healthy
	}
	return 0, 0, false
}

// GetMCPServerStatus returns the status of an mcp server
func (m *Model) GetMCPServerStatus(label string) (running bool) {
	if mcp, exists := m.MCPServers[label]; exists {
		running = mcp.State == "running"
		return running
	}
	return false
}

// GetPortForwardStatus returns the status of a port forward
func (m *Model) GetPortForwardStatus(label string) (running bool, localPort int, remotePort int) {
	if pf, exists := m.PortForwards[label]; exists {
		running = pf.State == "running"
		return running, pf.LocalPort, pf.RemotePort
	}
	return false, 0, 0
}

// StartService starts a service through the orchestrator API
func (m *Model) StartService(label string) tea.Cmd {
	return func() tea.Msg {
		// Skip if API is nil (e.g., in tests)
		if m.OrchestratorAPI == nil {
			return nil
		}

		if err := m.OrchestratorAPI.StartService(label); err != nil {
			return ServiceErrorMsg{
				Label: label,
				Err:   err,
			}
		}
		return ServiceStartedMsg{Label: label}
	}
}

// StopService stops a service through the orchestrator API
func (m *Model) StopService(label string) tea.Cmd {
	return func() tea.Msg {
		// Skip if API is nil (e.g., in tests)
		if m.OrchestratorAPI == nil {
			return nil
		}

		if err := m.OrchestratorAPI.StopService(label); err != nil {
			return ServiceErrorMsg{
				Label: label,
				Err:   err,
			}
		}
		return ServiceStoppedMsg{Label: label}
	}
}

// RestartService restarts a service through the orchestrator API
func (m *Model) RestartService(label string) tea.Cmd {
	return func() tea.Msg {
		// Skip if API is nil (e.g., in tests)
		if m.OrchestratorAPI == nil {
			return nil
		}

		if err := m.OrchestratorAPI.RestartService(label); err != nil {
			return ServiceErrorMsg{
				Label: label,
				Err:   err,
			}
		}
		return ServiceRestartedMsg{Label: label}
	}
}

// SetStatusMessage updates the status bar message
func (m *Model) SetStatusMessage(message string, msgType MessageType, clearAfter time.Duration) tea.Cmd {
	// Implementation similar to original but simplified
	m.StatusBarMessage = message
	m.StatusBarMessageType = msgType

	if m.StatusBarClearCancel != nil {
		close(m.StatusBarClearCancel)
	}

	m.StatusBarClearCancel = make(chan struct{})
	captured := m.StatusBarClearCancel

	return tea.Tick(clearAfter, func(t time.Time) tea.Msg {
		select {
		case <-captured:
			return nil
		default:
			return ClearStatusBarMsg{}
		}
	})
}

// Utility Functions
// -----------------

// stringInSlice checks if a string is in a slice
func stringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
