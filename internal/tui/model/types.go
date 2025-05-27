package model

import (
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/managers"
	"envctl/internal/orchestrator"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"time"

	"envctl/internal/api"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// AppMode defines the overall state or view of the application.
// Splitting these definitions into a dedicated file keeps the code base maintainable.
// NOTE: The ordering MUST stay in-sync with the String() method for a stable representation.
type AppMode int

const (
	// ModeInitializing is the initial state before essential data is loaded or UI is ready.
	ModeInitializing AppMode = iota
	// ModeMainDashboard is the primary view showing cluster health, port forwards, MCP servers, and logs.
	ModeMainDashboard
	// ModeNewConnectionInput is when the user is inputting MC/WC names for a new connection.
	ModeNewConnectionInput
	// ModeHelpOverlay is when the help screen is visible.
	ModeHelpOverlay
	// ModeLogOverlay is when the full-screen log viewer is active.
	ModeLogOverlay
	// ModeMcpConfigOverlay shows the MCP configuration overlay.
	ModeMcpConfigOverlay
	// ModeMcpToolsOverlay shows the MCP tools overlay.
	ModeMcpToolsOverlay
	// ModeQuitting is when the application is in the process of shutting down.
	ModeQuitting
	// ModeError an unrecoverable error state or a significant error message display.
	ModeError
)

// String makes AppMode satisfy the fmt.Stringer interface.
func (a AppMode) String() string {
	switch a {
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
	case ModeQuitting:
		return "Quitting"
	case ModeError:
		return "Error"
	default:
		return "Unknown"
	}
}

// MessageType defines the type of message for the status bar for styling.
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
	// Make sure this slice is ordered consistently with the const definitions.
	statuses := []string{"Initializing", "Up", "Connecting", "Degraded", "Failed", "Unknown"}
	if s < 0 || int(s) >= len(statuses)-1 { // -1 because Unknown is an extra fallback
		return statuses[len(statuses)-1] // Return "Unknown" for out-of-bounds
	}
	return statuses[s]
}

// FocusablePanelKeys defines string identifiers for UI panels that can receive focus.
const (
	McPaneFocusKey = "mc_pane"
	WcPaneFocusKey = "wc_pane"
	// Add other panel keys here as needed, e.g. for port-forwards or MCPs
)

// Misc. shared constants.
const (
	// MaxActivityLogLines defines the maximum number of lines to keep in the activityLog.
	// This prevents the log from growing indefinitely and consuming too much memory.
	MaxActivityLogLines = 10000

	// MaxPanelLogLines defines the maximum number of lines to keep in individual port-forward panel logs.
	MaxPanelLogLines = 100
)

// McpServerProcess holds the state for the MCP server process.
// It is kept here because several files (handlers, renderers, etc.) require the definition.

// KeyMap defines the keybindings for the application.
// Moved from controller to model package.
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
	ToggleLog       key.Binding
	CopyLogs        key.Binding
	ToggleMcpConfig key.Binding
	ToggleMcpTools  key.Binding
}

// FullHelp returns bindings for the main help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.ShiftTab},
		{k.NewCollection, k.Restart, k.Stop, k.SwitchContext, k.CopyLogs},
		{k.Help, k.ToggleLog, k.ToggleMcpConfig, k.ToggleMcpTools, k.ToggleDark, k.ToggleDebug, k.Quit},
	}
}

// ShortHelp returns a minimal set of bindings, often used for a status bar.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// InputModeHelp returns bindings specific to when in text input mode.
func (k KeyMap) InputModeHelp() [][]key.Binding {
	return [][]key.Binding{{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
		key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "submit")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel input")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "autocomplete")),
	}}
}

// model represents the state of the entire TUI application. Keeping this definition
// close to the basic types makes it easier to reason about the data structure.
type Model struct {
	// Terminal dimensions
	Width  int
	Height int

	// Global application state
	QuitApp         bool
	IsLoading       bool
	CurrentAppMode  AppMode
	LastAppMode     AppMode // To return to after modal dialogs
	FocusedPanelKey string  // Tracks which major panel (e.g. port forward, mcp server, log) has focus
	DebugMode       bool
	ColorMode       string // Stores info about current color profile and dark mode

	// Cluster and Connection Info
	ManagementClusterName string
	WorkloadClusterName   string
	CurrentKubeContext    string
	QuittingMessage       string
	MCHealth              ClusterHealthInfo
	WCHealth              ClusterHealthInfo

	// --- Port Forwarding specific fields ---
	PortForwardingConfig []config.PortForwardDefinition
	PortForwards         map[string]*PortForwardProcess
	PortForwardOrder     []string

	// --- MCP Proxy specific fields ---
	McpServers        map[string]*McpServerProcess
	McpProxyOrder     []string
	MCPServerConfig   []config.MCPServerDefinition
	McpConfigViewport viewport.Model
	// McpToolsViewport is the viewport for displaying MCP server tools
	McpToolsViewport viewport.Model

	// --- UI State & Output ---
	ActivityLog              []string
	ActivityLogDirty         bool
	LogViewportLastWidth     int
	MainLogViewportLastWidth int
	LogViewport              viewport.Model
	MainLogViewport          viewport.Model
	Spinner                  spinner.Model
	NewConnectionInput       textinput.Model
	CurrentInputStep         InputStep
	StashedMcName            string
	ClusterInfo              *kube.ClusterInfo
	Keys                     KeyMap
	Help                     help.Model
	TUIChannel               chan tea.Msg
	DependencyGraph          *dependency.Graph
	StatusBarMessage         string
	StatusBarMessageType     MessageType
	StatusBarClearCancel     chan struct{}

	// --- Service Management ---
	ServiceManager managers.ServiceManagerAPI // Interface for managing services
	Reporter       reporting.ServiceReporter  // For sending updates to TUI/console
	Orchestrator   *orchestrator.Orchestrator

	// --- Logging ---
	LogChannel <-chan logging.LogEntry

	// --- API Layer ---
	APIs     *api.Provider            // API provider for accessing service functionality
	MCPTools map[string][]api.MCPTool // Cached MCP tools by server name
}

// Other structs that might need field export if used cross-package
type ClusterHealthInfo struct { // Renamed from clusterHealthInfo
	IsLoading   bool      // Exported
	ReadyNodes  int       // Exported
	TotalNodes  int       // Exported
	StatusError error     // Exported
	DebugLog    string    // Exported
	LastUpdated time.Time // ADDED Exported LastUpdated field
}

type PortForwardProcess struct { // Renamed from portForwardProcess
	Label       string                       // Exported
	Command     string                       // Exported
	LocalPort   int                          // Exported
	RemotePort  int                          // Exported
	TargetHost  string                       // Exported
	ContextName string                       // Exported
	StopChan    chan struct{}                // Exported
	Log         []string                     // Exported
	Active      bool                         // Exported
	Running     bool                         // Exported
	StatusMsg   string                       // Exported
	Err         error                        // Exported
	Config      config.PortForwardDefinition // Updated type
}

type McpServerProcess struct { // Renamed from mcpServerProcess
	Label     string                     // Exported, User-friendly label (e.g., "Kubernetes API").
	Pid       int                        // Exported, PID of the process.
	ProxyPort int                        // Exported, Port that mcp-proxy is listening on
	StopChan  chan struct{}              // Exported, Channel to signal the process to stop.
	Output    []string                   // Exported, Stores output or log messages.
	Err       error                      // Exported, Any error encountered by the process.
	Active    bool                       // Exported, Whether the server is configured to be active.
	StatusMsg string                     // Exported, Detailed status message for display.
	Config    config.MCPServerDefinition // Added Config field
}

// SetStatusMessage updates the status bar message and schedules clearing it after the given duration.
func (m *Model) SetStatusMessage(message string, msgType MessageType, clearAfter time.Duration) tea.Cmd {
	// Estimate available width for the center message part of the status bar
	estimatedLeftW := int(float64(m.Width) * 0.25)
	estimatedRightW := int(float64(m.Width) * 0.35)
	estimatedCenterW := m.Width - estimatedLeftW - estimatedRightW

	const iconBuffer = 2     // Approximate for icon and a space
	const ellipsisBuffer = 3 // For "..."
	const paddingBuffer = 2  // General padding
	totalBuffer := iconBuffer + ellipsisBuffer + paddingBuffer

	maxLen := estimatedCenterW - totalBuffer
	if maxLen < 0 {
		maxLen = 0
	}

	// Using simple len() for byte length, not visual width. This is a simplification.
	actualMessageByteLength := len(message)
	truncatedMessage := message

	if actualMessageByteLength > maxLen {
		if maxLen <= 0 {
			truncatedMessage = ""
		} else if maxLen <= ellipsisBuffer { // Not enough space for ellipsis itself
			// Take as much of the start of the message as fits
			if len(message) > maxLen {
				truncatedMessage = message[:maxLen]
			} // else message is already shorter or equal, no truncation needed
		} else {
			// Can fit message part and ellipsis
			// Simple byte slice truncation, may cut multi-byte runes.
			// A rune-aware approach would be: string([]rune(message)[:someRuneCount])
			// but calculating someRuneCount to fit maxLen bytes is complex without rune width metrics.
			truncateAt := maxLen - ellipsisBuffer
			if truncateAt < 0 {
				truncateAt = 0
			} // Should not happen if maxLen > ellipsisBuffer

			// Ensure we don't slice beyond the actual length of the message if it's short but still needs ellipsis space
			if len(message) > truncateAt {
				truncatedMessage = message[:truncateAt] + "..."
			} else {
				// This case means message is shorter than (maxLen - ellipsisBuffer), but longer than maxLen overall.
				// This implies maxLen is very small, just show what fits without ellipsis.
				if len(message) > maxLen {
					truncatedMessage = message[:maxLen]
				}
				// else message is already short enough, no truncation or ellipsis needed (covered by outer if)
			}
		}
	}

	m.StatusBarMessage = truncatedMessage
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

// GetServiceState returns the current state of a service from the StateStore
func (m *Model) GetServiceState(label string) reporting.ServiceState {
	if m.Reporter != nil && m.Reporter.GetStateStore() != nil {
		snapshot, exists := m.Reporter.GetStateStore().GetServiceState(label)
		if exists {
			return snapshot.State
		}
	}
	return reporting.StateUnknown
}

// GetServiceSnapshot returns the complete state snapshot of a service from the StateStore
func (m *Model) GetServiceSnapshot(label string) (reporting.ServiceStateSnapshot, bool) {
	if m.Reporter != nil && m.Reporter.GetStateStore() != nil {
		return m.Reporter.GetStateStore().GetServiceState(label)
	}
	return reporting.ServiceStateSnapshot{}, false
}

// IsServiceReady returns whether a service is ready based on StateStore
func (m *Model) IsServiceReady(label string) bool {
	snapshot, exists := m.GetServiceSnapshot(label)
	return exists && snapshot.IsReady
}

// GetAllServiceStates returns all service states from the StateStore
func (m *Model) GetAllServiceStates() map[string]reporting.ServiceStateSnapshot {
	if m.Reporter != nil && m.Reporter.GetStateStore() != nil {
		return m.Reporter.GetStateStore().GetAllServiceStates()
	}
	return make(map[string]reporting.ServiceStateSnapshot)
}

// GetServicesByType returns all services of a specific type from the StateStore
func (m *Model) GetServicesByType(serviceType reporting.ServiceType) map[string]reporting.ServiceStateSnapshot {
	if m.Reporter != nil && m.Reporter.GetStateStore() != nil {
		return m.Reporter.GetStateStore().GetServicesByType(serviceType)
	}
	return make(map[string]reporting.ServiceStateSnapshot)
}

// ReconcileState synchronizes the TUI model state with the StateStore
func (m *Model) ReconcileState() {
	if m.Reporter == nil || m.Reporter.GetStateStore() == nil {
		return
	}

	states := m.Reporter.GetStateStore().GetAllServiceStates()

	// Update PortForwards and McpServers from StateStore
	for label, snapshot := range states {
		switch snapshot.SourceType {
		case reporting.ServiceTypePortForward:
			if pf, exists := m.PortForwards[label]; exists {
				// Update the port forward state from the snapshot
				pf.StatusMsg = string(snapshot.State)
				pf.Running = snapshot.IsReady
				m.PortForwards[label] = pf
			}
		case reporting.ServiceTypeMCPServer:
			if mcp, exists := m.McpServers[label]; exists {
				// Update the MCP server state from the snapshot
				mcp.StatusMsg = string(snapshot.State)
				// Don't overwrite Active with IsReady - Active means the service is configured to run
				// mcp.Active = snapshot.IsReady  // REMOVED - this was the bug
				if snapshot.ProxyPort > 0 {
					mcp.ProxyPort = snapshot.ProxyPort
				}
				if snapshot.PID > 0 {
					mcp.Pid = snapshot.PID
				}
				m.McpServers[label] = mcp
			}
		}
	}
}

// GetMCContext returns the full Kubernetes context name for the current MC
func (m *Model) GetMCContext() string {
	if m.ManagementClusterName == "" {
		return ""
	}
	mcContext := kube.BuildMcContext(m.ManagementClusterName)
	return mcContext
}

// GetWCContext returns the full Kubernetes context name for the current WC
func (m *Model) GetWCContext() string {
	if m.ManagementClusterName == "" || m.WorkloadClusterName == "" {
		return ""
	}
	wcContext := kube.BuildWcContext(m.ManagementClusterName, m.WorkloadClusterName)
	return wcContext
}
