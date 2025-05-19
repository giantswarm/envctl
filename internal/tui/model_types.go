package tui

import (
	"envctl/internal/dependency"
	"envctl/internal/utils"

	"github.com/charmbracelet/bubbles/help"
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

// newInputStep defines the different stages of the new connection input process.
// This helps manage the UI flow when a user initiates a new connection.
// These constants are used by the connection flow logic (see handler_connection.go).
type newInputStep int

const (
    mcInputStep newInputStep = iota // User is entering the Management Cluster name.
    wcInputStep                     // User is entering the Workload Cluster name.
)

// Misc. shared constants.
const (
    // maxCombinedOutputLines defines the maximum number of lines to keep in the combinedOutput log.
    // This prevents the log from growing indefinitely and consuming too much memory.
    maxCombinedOutputLines = 200

    // maxPanelLogLines defines the maximum number of lines to keep in individual port-forward panel logs.
    maxPanelLogLines = 100
)

// mcpServerProcess holds the state for the MCP server process.
// It is kept here because several files (handlers, renderers, etc.) require the definition.
type mcpServerProcess struct {
    label     string        // User-friendly label (e.g., "Kubernetes API").
    pid       int           // PID of the process.
    stopChan  chan struct{} // Channel to signal the process to stop.
    output    []string      // Stores output or log messages.
    err       error         // Any error encountered by the process.
    active    bool          // Whether the server is configured to be active.
    statusMsg string        // Detailed status message for display.
}

// model represents the state of the entire TUI application. Keeping this definition
// close to the basic types makes it easier to reason about the data structure.
type model struct {
    // --- Cluster Information ---
    managementCluster  string // Name of the management cluster.
    workloadCluster    string // Name of the workload cluster (can be empty).
    kubeContext        string // Target Kubernetes context specified by the user.
    currentKubeContext string // Actual current Kubernetes context.
    quittingMessage    string // Message to display when quitting.

    // --- Health Information ---
    MCHealth clusterHealthInfo // Health status of the management cluster.
    WCHealth clusterHealthInfo // Health status of the workload cluster.

    // --- Port Forwarding ---
    portForwards     map[string]*portForwardProcess // Active port-forwarding processes by label.
    portForwardOrder []string                       // Display / navigation order of port-forwarding panels.
    focusedPanelKey  string                         // Currently focused panel key.

    // --- MCP Proxies ---
    mcpProxyOrder []string                       // Navigation order for MCP proxy panels.
    mcpServers    map[string]*mcpServerProcess   // State of multiple MCP proxy processes.

    // --- UI State & Output ---
    combinedOutput     []string       // Global combined log lines.
    width, height      int            // Terminal dimensions.
    debugMode          bool           // Show/hide debug info.
    colorMode          string         // Current color profile description.
    logViewport        viewport.Model // Viewport for the full-screen log overlay.
    mainLogViewport    viewport.Model // Viewport for the inline log panel.
    mcpConfigViewport  viewport.Model // Viewport for the MCP JSON config overlay.

    // --- Status Bar ---
    statusBarMessage     string        // Status bar text.
    statusBarMessageType MessageType   // Status bar message type for styling.
    statusBarClearCancel chan struct{} // Cancel channel for deferred clear.

    // --- Loading State ---
    isLoading bool          // True while background operation is running.
    spinner   spinner.Model // Spinner for loading indication.

    // --- Application Mode ---
    currentAppMode AppMode // Current application mode (view).

    // --- Dependency Graph ---
    dependencyGraph *dependency.Graph // Tracks semantic dependencies.

    // --- New Connection Input State ---
    newConnectionInput textinput.Model    // Text input component for new cluster names.
    currentInputStep   newInputStep       // Current step in the input flow.
    stashedMcName      string             // Temporarily stored MC name while entering WC.
    clusterInfo        *utils.ClusterInfo // Cluster lists for autocompletion.

    // --- Key Map & Help ---
    keys KeyMap     // Keybindings.
    help help.Model // Help model.

    // TUIChannel is used by background goroutines to send messages back to the Bubbletea update loop.
    TUIChannel chan tea.Msg
} 
