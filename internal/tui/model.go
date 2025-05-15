package tui

import (
	"envctl/internal/utils"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// newInputStep defines the different stages of the new connection input process.
// This helps manage the UI flow when a user initiates a new connection.
type newInputStep int

const (
	mcInputStep newInputStep = iota // Represents the stage where the user inputs the Management Cluster name.
	wcInputStep                     // Represents the stage where the user inputs the Workload Cluster name.

	// maxCombinedOutputLines defines the maximum number of lines to keep in the combinedOutput log.
	// This prevents the log from growing indefinitely and consuming too much memory.
	maxCombinedOutputLines = 200
)

// model represents the state of the TUI application.
// It holds all the data necessary to render the UI and manage its behavior.
type model struct {
	// --- Cluster Information ---
	managementCluster  string // Name of the management cluster.
	workloadCluster    string // Name of the workload cluster (can be empty).
	kubeContext        string // Target Kubernetes context specified by the user (usually WC or MC if no WC).
	currentKubeContext string // Actual current Kubernetes context reported by `kubectl config current-context`.

	// --- Health Information ---
	MCHealth clusterHealthInfo // Health status of the management cluster.
	WCHealth clusterHealthInfo // Health status of the workload cluster.

	// --- Port Forwarding ---
	portForwards     map[string]*portForwardProcess // Map of active port-forwarding processes, keyed by label.
	portForwardOrder []string                       // Order in which port-forwarding panels (and MC/WC info panes) are displayed and navigated.
	focusedPanelKey  string                         // Key of the currently focused panel or pane for navigation.

	// --- UI State & Output ---
	combinedOutput []string // Log of messages and statuses displayed in the TUI.
	quitting       bool     // Flag indicating if the application is in the process of quitting.
	ready          bool     // Flag indicating if the TUI has received initial window size and is ready to render.
	width          int      // Current width of the terminal window.
	height         int      // Current height of the terminal window.

	// --- New Connection Input State ---
	isConnectingNew    bool               // True if the TUI is in 'new connection input' mode.
	newConnectionInput textinput.Model    // Bubbletea text input component for new cluster names.
	currentInputStep   newInputStep       // Current step in the new connection input flow (mcInputStep or wcInputStep).
	stashedMcName      string             // Temporarily stores the MC name while the WC name is being inputted.
	clusterInfo        *utils.ClusterInfo // Holds fetched cluster list for autocompletion during new connection input.

	// TUIChannel is a channel used by asynchronous operations (e.g., port forwarding, Kubernetes API calls)
	// to send messages (tea.Msg) back to the TUI's main update loop for processing.
	// This allows non-blocking operations and keeps the UI responsive.
	TUIChannel chan tea.Msg
}

// setupPortForwards initializes or re-initializes the port-forwarding configurations.
// It clears any existing port forwards and sets up new ones based on the provided
// management cluster (mcName) and workload cluster (wcName).
// This function defines the services to be port-forwarded (e.g., Prometheus, Grafana, Alloy Metrics)
// and their respective configurations.
func (m *model) setupPortForwards(mcName, wcName string) {
	// Clear existing port forwards before setting up new ones
	m.portForwards = make(map[string]*portForwardProcess)
	m.portForwardOrder = make([]string, 0)

	// Add context pane keys first for navigation order
	m.portForwardOrder = append(m.portForwardOrder, mcPaneFocusKey)
	if wcName != "" {
		m.portForwardOrder = append(m.portForwardOrder, wcPaneFocusKey)
	}

	// Prometheus for MC
	if mcName != "" {
		promLabel := "Prometheus (MC)"
		m.portForwardOrder = append(m.portForwardOrder, promLabel)
		m.portForwards[promLabel] = &portForwardProcess{
			label:     promLabel,
			port:      "8080:8080",
			isWC:      false,
			context:   "teleport.giantswarm.io-" + mcName,
			namespace: "mimir",
			service:   "service/mimir-query-frontend",
			active:    true,
			statusMsg: "Awaiting Setup...",
		}

		// Grafana for MC
		grafanaLabel := "Grafana (MC)"
		m.portForwardOrder = append(m.portForwardOrder, grafanaLabel)
		m.portForwards[grafanaLabel] = &portForwardProcess{
			label:     grafanaLabel,
			port:      "3000:3000",
			isWC:      false,
			context:   "teleport.giantswarm.io-" + mcName,
			namespace: "monitoring",
			service:   "service/grafana",
			active:    true,
			statusMsg: "Awaiting Setup...",
		}
	}

	// Alloy Metrics for WC
	if wcName != "" {
		alloyLabel := "Alloy Metrics (WC)"
		m.portForwardOrder = append(m.portForwardOrder, alloyLabel)

		// Construct the correct context name part for WC.
		// mcName is the short MC name (e.g., "alba")
		// wcName can be the short WC name (e.g., "apiel") or a full one (e.g., "alba-apiel" from CLI args)
		actualWcContextPart := wcName
		if mcName != "" && !strings.HasPrefix(wcName, mcName+"-") {
			// If wcName is a short name (e.g., "apiel") and doesn't already start with "alba-",
			// then prepend mcName to form "alba-apiel".
			actualWcContextPart = mcName + "-" + wcName
		}
		// If wcName was already "alba-apiel", it remains unchanged.
		// If mcName was empty, actualWcContextPart remains wcName.

		m.portForwards[alloyLabel] = &portForwardProcess{
			label:     alloyLabel,
			port:      "12345:12345",
			isWC:      true,
			context:   "teleport.giantswarm.io-" + actualWcContextPart,
			namespace: "kube-system",
			service:   "service/alloy-metrics-cluster",
			active:    true,
			statusMsg: "Awaiting Setup...",
		}
	}
}

// InitialModel creates the initial state of the TUI model.
// It takes the management cluster name, workload cluster name (optional),
// and the initial Kubernetes context as input.
// It sets up the initial port-forwarding configurations, text input for new connections,
// and initializes the TUI message channel.
func InitialModel(mcName, wcName, kubeCtx string) model {
	ti := textinput.New()
	ti.Placeholder = "Management Cluster"
	ti.CharLimit = 156 // Arbitrary limit
	ti.Width = 50      // Arbitrary width

	// Create the TUI message channel with a larger buffer
	tuiMsgChannel := make(chan tea.Msg, 100)

	m := model{
		managementCluster:  mcName,
		workloadCluster:    wcName,
		kubeContext:        kubeCtx,
		portForwards:       make(map[string]*portForwardProcess),
		portForwardOrder:   make([]string, 0),
		combinedOutput:     make([]string, 0),
		MCHealth:           clusterHealthInfo{IsLoading: true},
		isConnectingNew:    false,
		newConnectionInput: ti,
		currentInputStep:   mcInputStep,
		TUIChannel:         tuiMsgChannel, // Assign the channel to the model
	}

	m.setupPortForwards(mcName, wcName)

	if wcName != "" {
		m.WCHealth = clusterHealthInfo{IsLoading: true}
	}

	if len(m.portForwardOrder) > 0 {
		m.focusedPanelKey = m.portForwardOrder[0] // Default focus to the first item (MC pane)
	} else if mcName != "" {
		// Fallback if only MC exists and somehow portForwardOrder is empty (should not happen with current logic)
		m.focusedPanelKey = mcPaneFocusKey
	}
	return m
}

// channelReaderCmd creates a tea.Cmd that continuously listens for messages on the provided TUIChannel.
// When a message is received, it's sent to the Bubbletea update loop for processing.
// This is crucial for handling asynchronous events and updates within the TUI.
func channelReaderCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// Init is the first command executed when the TUI starts.
// It's responsible for initiating asynchronous operations like:
// - Fetching the current Kubernetes context.
// - Fetching the list of available clusters for autocompletion.
// - Performing initial health checks for the specified clusters.
// - Starting the configured port-forwarding processes.
// - Starting a ticker for periodic health updates.
// - Starting the listener for messages on the TUIChannel.
func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Get current kube context
	cmds = append(cmds, getCurrentKubeContextCmd())

	// Fetch cluster list for autocompletion
	cmds = append(cmds, fetchClusterListCmd())

	// Initial health checks
	if m.managementCluster != "" {
		cmds = append(cmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
	}
	if m.workloadCluster != "" {
		// When m.workloadCluster is from InitialModel, it might be the full "mc-wc" name.
		// m.managementCluster is the short MC name.
		// fetchNodeStatusCmd handles if clusterNameToFetchStatusFor is already "mc-wc".
		cmds = append(cmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
	}

	// Start port-forwarding processes
	initialPfCmds := getInitialPortForwardCmds(&m) // Pass model as a pointer
	cmds = append(cmds, initialPfCmds...)

	// Add a ticker for periodic health updates
	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	})
	cmds = append(cmds, tickCmd)

	// Add channel reader to process messages from TUIChannel
	cmds = append(cmds, channelReaderCmd(m.TUIChannel))

	return tea.Batch(cmds...)
}

// Update handles incoming messages (tea.Msg) and updates the model accordingly.
// Messages can be key presses, window size changes, results from asynchronous operations,
// or custom messages defined by the application.
// It's the core logic loop of the TUI, determining how the application state changes in response to events.
// Each message type is typically handled by a specific helper function (e.g., handleKeyMsg, handlePortForwardStatusUpdateMsg).
// After processing a message, it returns the updated model and potentially a new command (tea.Cmd) to be executed.
// Crucially, after every message processing step, it re-subscribes to the TUIChannel via channelReaderCmd
// to ensure continuous processing of asynchronous messages.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd // Holds commands to be batched IF NOT handled by a specific case returning a cmd.

	switch msg := msg.(type) {
	// Key messages are handled by functions in handlers.go
	case tea.KeyMsg:
		var cmd tea.Cmd
		if m.isConnectingNew && m.newConnectionInput.Focused() {
			m, cmd = handleKeyMsgInputMode(m, msg)
		} else {
			m, cmd = handleKeyMsgGlobal(m, msg, []tea.Cmd{})
		}
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))

	// Window size messages are handled by a function in handlers.go
	case tea.WindowSizeMsg:
		m, cmd := handleWindowSizeMsg(m, msg)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))

	// Port Forwarding Messages (handlers in portforward_handlers.go)
	case portForwardSetupCompletedMsg:
		m, cmd := handlePortForwardSetupCompletedMsg(m, msg)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
	case portForwardStatusUpdateMsg:
		// Pass directly to the handler without extra debugging output
		m, cmd := handlePortForwardStatusUpdateMsg(m, msg)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))

	// New Connection Flow Messages (handlers in connection_flow.go)
	case submitNewConnectionMsg:
		m, cmd := handleSubmitNewConnectionMsg(m, msg, cmds)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
	case kubeLoginResultMsg:
		m, cmd := handleKubeLoginResultMsg(m, msg, cmds)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
	case contextSwitchAndReinitializeResultMsg:
		m, cmd := handleContextSwitchAndReinitializeResultMsg(m, msg, cmds)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))

	// Other System/Async Messages (handlers in handlers.go)
	case kubeContextResultMsg:
		m = handleKubeContextResultMsg(m, msg) // Modifies model, returns no cmd
		return m, channelReaderCmd(m.TUIChannel)
	case requestClusterHealthUpdate:
		// This handler returns (model, tea.Cmd)
		m, cmd := handleRequestClusterHealthUpdate(m)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
	case kubeContextSwitchedMsg:
		// This handler returns (model, tea.Cmd)
		m, cmd := handleKubeContextSwitchedMsg(m, msg)
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
	case nodeStatusMsg:
		m = handleNodeStatusMsg(m, msg) // Modifies model, returns no cmd
		return m, channelReaderCmd(m.TUIChannel)
	case clusterListResultMsg:
		m = handleClusterListResultMsg(m, msg) // Modifies model, returns no cmd
		return m, channelReaderCmd(m.TUIChannel)

	default:
		// Handle text input updates if in new connection mode and input is focused,
		// but not a key press (which is handled by tea.KeyMsg case above).
		if m.isConnectingNew && m.newConnectionInput.Focused() {
			var textInputCmd tea.Cmd
			m.newConnectionInput, textInputCmd = m.newConnectionInput.Update(msg)
			return m, tea.Batch(textInputCmd, channelReaderCmd(m.TUIChannel))
		}
		// If no other case matched, no specific command is returned here.
		// Any accumulated cmds in the local `cmds` slice would be batched at the end.
		// However, most handlers now return directly.
	}

	// Trim combinedOutput (general operation after message processing)
	if len(m.combinedOutput) > maxCombinedOutputLines+50 {
		m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
	}

	// If the switch statement fell through without returning a specific command,
	// batch any commands that might have been accumulated in the `cmds` slice.
	// Most cases now return directly, so `cmds` will often be empty here.
	cmds = append(cmds, channelReaderCmd(m.TUIChannel))
	return m, tea.Batch(cmds...)
}

// View renders the current state of the model as a string, which is then displayed in the terminal.
// It constructs the UI layout by arranging different components (header, cluster info panes,
// port-forwarding panels, activity log) based on the model's data.
// Lipgloss is used for styling and layout.
// If the application is quitting or not yet ready, it displays a status message.
// If in 'new connection input' mode, it renders the input UI.
func (m model) View() string {
	if m.quitting {
		return statusStyle.Render("Cleaning up and quitting...")
	}
	if !m.ready {
		return statusStyle.Render("Initializing...")
	}

	// If in new connection input mode, render the input UI
	if m.isConnectingNew {
		var inputPrompt strings.Builder
		inputPrompt.WriteString("Enter new cluster information (ESC to cancel, Enter to confirm/next)\n\n")
		inputPrompt.WriteString(m.newConnectionInput.View()) // Renders the text input bubble
		if m.currentInputStep == mcInputStep {
			inputPrompt.WriteString("\n\n[Input: Management Cluster Name]")
		} else {
			inputPrompt.WriteString(fmt.Sprintf("\n\n[Input: Workload Cluster Name for MC: %s (optional)]", m.stashedMcName))
		}
		inputViewStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).Width(m.width - 4).Align(lipgloss.Center)
		return inputViewStyle.Render(inputPrompt.String())
	}

	// Regular view rendering - use consistent width for all sections
	totalWidth := m.width
	contentWidth := totalWidth - appStyle.GetHorizontalFrameSize()
	marginTop := lipgloss.NewStyle().MarginTop(1)

	// ----- GLOBAL HEADER SECTION -----
	headerTitleString := "envctl TUI - Quit: 'q'/Ctrl+C | Navigate: Tab/Shift+Tab | Restart PF: 'r' | Switch Ctx: 's' | New Conn: N"
	headerContentAreaWidth := contentWidth - headerStyle.GetHorizontalFrameSize()
	if headerContentAreaWidth < 0 {
		headerContentAreaWidth = 0
	}
	headerTitleView := headerStyle.Copy().Width(headerContentAreaWidth).Render(headerTitleString)
	headerHeight := lipgloss.Height(headerTitleView)

	// ----- NEW ROW 1: MC/WC Info (Target: 2 columns or 1 if no WC) -----
	var row1View string
	if m.workloadCluster != "" {
		mcPaneWidth := contentWidth / 2
		wcPaneWidth := contentWidth - mcPaneWidth
		renderedMcPane := renderMcPane(m, mcPaneWidth)
		renderedWcPane := renderWcPane(m, wcPaneWidth)
		row1View = lipgloss.JoinHorizontal(lipgloss.Top, renderedMcPane, renderedWcPane)
	} else {
		row1View = renderMcPane(m, contentWidth)
	}
	// Ensure row1View itself is contentWidth wide, aligning its internal content left.
	row1FinalView := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).Render(row1View)
	row1Height := lipgloss.Height(row1FinalView)

	// ----- NEW ROW 2: Port Forwarding (Target: 3 fixed columns) -----
	numFixedColumnsForRow2 := 3
	row2PanelBaseWidth := contentWidth / numFixedColumnsForRow2
	row2RemainderPixels := contentWidth % numFixedColumnsForRow2

	row2CellsRendered := make([]string, numFixedColumnsForRow2)
	pfPanelKeysToShow := []string{}
	for _, key := range m.portForwardOrder {
		if key != mcPaneFocusKey && key != wcPaneFocusKey {
			pfPanelKeysToShow = append(pfPanelKeysToShow, key)
		}
	}

	maxPfPanelHeightRow2 := 0
	for i := 0; i < numFixedColumnsForRow2; i++ {
		currentCellWidth := row2PanelBaseWidth
		if i < row2RemainderPixels {
			currentCellWidth++
		}

		if i < len(pfPanelKeysToShow) {
			pfKey := pfPanelKeysToShow[i]
			pf := m.portForwards[pfKey]
			renderedPfCell := renderPortForwardPanel(pf, m, currentCellWidth)
			row2CellsRendered[i] = renderedPfCell
			if lipgloss.Height(renderedPfCell) > maxPfPanelHeightRow2 {
				maxPfPanelHeightRow2 = lipgloss.Height(renderedPfCell)
			}
		} else {
			// Render an empty placeholder panel to maintain the 3-column structure
			// Use panelStyle and ensure it respects its frame for width calculation.
			placeholderContentWidth := currentCellWidth - panelStyle.GetHorizontalFrameSize()
			if placeholderContentWidth < 0 {
				placeholderContentWidth = 0
			}
			// For consistent height, try to use maxPfPanelHeightRow2 if already determined, or a sensible min.
			// However, lipgloss.JoinHorizontal Top alignment handles varying heights.
			// For simplicity, a basic empty panel.
			emptyCell := panelStyle.Copy().Width(placeholderContentWidth).Render("")
			// If we want empty cells to attempt to match height of actual panels for visual balance:
			// if maxPfPanelHeightRow2 > 0 { // Requires panelStyle to have Height settable or content to force it.
			//  emptyCell = panelStyle.Copy().Width(placeholderContentWidth).Height(maxPfPanelHeightRow2).Render("")
			// } else { // if no panels yet, use a default min height for empty cells
			//  minH := 7 // example
			//  emptyCell = panelStyle.Copy().Width(placeholderContentWidth).Height(minH).Render("")
			// }
			row2CellsRendered[i] = emptyCell
		}
	}
	row2JoinedPanels := lipgloss.JoinHorizontal(lipgloss.Top, row2CellsRendered...)
	// Ensure row2View itself is contentWidth wide, aligning its internal content left.
	row2FinalView := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Left).Render(row2JoinedPanels)
	row2Height := lipgloss.Height(row2FinalView) // Get height directly from the new overall row view

	// Fallback for row2Height if it's still minimal (e.g., all placeholders resulted in a very short row)
	if len(pfPanelKeysToShow) == 0 { // If there were no actual PF panels, only placeholders
		// Ensure the row has a minimum sensible height to represent empty panel slots.
		// A typical panel might be around 7 lines tall (title, blank, 3-4 lines info, status + padding/border).
		const pragmaticMinRow2HeightWhenEmpty = 7
		if row2Height < pragmaticMinRow2HeightWhenEmpty {
			row2Height = pragmaticMinRow2HeightWhenEmpty
		}
	}

	// ----- NEW ROW 3: Activity Log (1 column) -----
	logPanelMinHeight := 5
	// Calculate available height for log section, accounting for all rows and margins
	// Total vertical space for margins between header, row1, row2, row3 (3 margins)
	verticalMarginSpace := marginTop.GetVerticalFrameSize() * 3

	logSectionHeight := m.height - appStyle.GetVerticalFrameSize() - headerHeight - row1Height - row2Height - verticalMarginSpace
	if logSectionHeight < logPanelMinHeight {
		logSectionHeight = logPanelMinHeight
	}
	combinedLogView := renderCombinedLogPanel(m, contentWidth, logSectionHeight)

	// ----- FINAL ASSEMBLY -----
	finalView := lipgloss.JoinVertical(lipgloss.Left,
		headerTitleView,
		marginTop.Render(row1FinalView), // Use the new width-enforced view
		marginTop.Render(row2FinalView), // Use the new width-enforced view
		marginTop.Render(combinedLogView),
	)

	return appStyle.Render(finalView)
}
