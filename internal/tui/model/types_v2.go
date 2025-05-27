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
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ModelV2 represents the state of the TUI application using the new service architecture
type ModelV2 struct {
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

	// Cluster and Connection Info
	ManagementClusterName string
	WorkloadClusterName   string
	CurrentKubeContext    string
	QuittingMessage       string

	// Service Architecture Components
	Orchestrator    *orchestrator.OrchestratorV2
	OrchestratorAPI api.OrchestratorAPI
	MCPServiceAPI   api.MCPServiceAPI
	PortForwardAPI  api.PortForwardServiceAPI
	K8sServiceAPI   api.K8sServiceAPI

	// Cached service data for display
	K8sConnections map[string]*api.K8sConnectionInfo
	PortForwards   map[string]*api.PortForwardServiceInfo
	MCPServers     map[string]*api.MCPServerInfo
	MCPTools       map[string][]api.MCPTool

	// Service ordering for display
	K8sConnectionOrder []string
	PortForwardOrder   []string
	MCPServerOrder     []string

	// Configuration
	PortForwardingConfig []config.PortForwardDefinition
	MCPServerConfig      []config.MCPServerDefinition

	// UI State & Output
	ActivityLog              []string
	ActivityLogDirty         bool
	LogViewportLastWidth     int
	MainLogViewportLastWidth int
	LogViewport              viewport.Model
	MainLogViewport          viewport.Model
	McpConfigViewport        viewport.Model
	McpToolsViewport         viewport.Model
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

	// Logging
	LogChannel <-chan logging.LogEntry

	// Event subscription
	StateChangeEvents <-chan api.ServiceStateChangedEvent
}

// RefreshServiceData fetches the latest service data from APIs
func (m *ModelV2) RefreshServiceData() error {
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

	// Only update order if it's empty (first time)
	if len(m.K8sConnectionOrder) == 0 {
		m.K8sConnectionOrder = []string{}
		for _, conn := range k8sConns {
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

	// Only update order if it's empty (first time)
	if len(m.PortForwardOrder) == 0 {
		m.PortForwardOrder = []string{}
		for _, pf := range portForwards {
			m.PortForwardOrder = append(m.PortForwardOrder, pf.Label)
		}
	}
	m.PortForwards = newPortForwards

	// Refresh MCP servers
	mcpServers, err := m.MCPServiceAPI.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list MCP servers: %w", err)
	}

	// Update MCP servers while preserving order
	newMCPServers := make(map[string]*api.MCPServerInfo)
	for _, mcp := range mcpServers {
		newMCPServers[mcp.Name] = mcp
	}

	// Only update order if it's empty (first time)
	if len(m.MCPServerOrder) == 0 {
		m.MCPServerOrder = []string{}
		for _, mcp := range mcpServers {
			m.MCPServerOrder = append(m.MCPServerOrder, mcp.Name)
		}
	}
	m.MCPServers = newMCPServers

	return nil
}

// GetMCPServerPort returns the port for an MCP server
func (m *ModelV2) GetMCPServerPort(name string) int {
	if mcp, exists := m.MCPServers[name]; exists {
		return mcp.Port
	}
	return 0
}

// GetK8sConnectionHealth returns the health info for a K8s connection
func (m *ModelV2) GetK8sConnectionHealth(label string) (ready int, total int, healthy bool) {
	if conn, exists := m.K8sConnections[label]; exists {
		healthy = conn.Health == "healthy"
		return conn.ReadyNodes, conn.TotalNodes, healthy
	}
	return 0, 0, false
}

// GetPortForwardStatus returns the status of a port forward
func (m *ModelV2) GetPortForwardStatus(label string) (running bool, localPort int, remotePort int) {
	if pf, exists := m.PortForwards[label]; exists {
		running = pf.State == "running"
		return running, pf.LocalPort, pf.RemotePort
	}
	return false, 0, 0
}

// StartService starts a service through the orchestrator API
func (m *ModelV2) StartService(label string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.OrchestratorAPI.StartService(ctx, label); err != nil {
			return ServiceErrorMsg{
				Label: label,
				Err:   err,
			}
		}
		return ServiceStartedMsg{Label: label}
	}
}

// StopService stops a service through the orchestrator API
func (m *ModelV2) StopService(label string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.OrchestratorAPI.StopService(ctx, label); err != nil {
			return ServiceErrorMsg{
				Label: label,
				Err:   err,
			}
		}
		return ServiceStoppedMsg{Label: label}
	}
}

// RestartService restarts a service through the orchestrator API
func (m *ModelV2) RestartService(label string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.OrchestratorAPI.RestartService(ctx, label); err != nil {
			return ServiceErrorMsg{
				Label: label,
				Err:   err,
			}
		}
		return ServiceRestartedMsg{Label: label}
	}
}

// SetStatusMessage updates the status bar message
func (m *ModelV2) SetStatusMessage(message string, msgType MessageType, clearAfter time.Duration) tea.Cmd {
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

// Message types for service operations
type ServiceStartedMsg struct {
	Label string
}

type ServiceStoppedMsg struct {
	Label string
}

// ServiceRestartedMsg indicates a service was restarted
type ServiceRestartedMsg struct {
	Label string
}

// InitializationCompleteMsg indicates that the orchestrator initialization is complete
type InitializationCompleteMsg struct{}
