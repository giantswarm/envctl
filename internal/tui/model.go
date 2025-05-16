package tui

import (
	"envctl/internal/utils"
	"fmt" // Import os for stderr
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	combinedOutput    []string       // Log of messages and statuses displayed in the TUI.
	quitting          bool           // Flag indicating if the application is in the process of quitting.
	ready             bool           // Flag indicating if the TUI has received initial window size and is ready to render.
	width             int            // Current width of the terminal window.
	height            int            // Current height of the terminal window.
	debugMode         bool           // Flag to show or hide debug information
	colorMode         string         // Current color mode for debugging
	helpVisible       bool           // Flag to show or hide the help overlay
	logOverlayVisible bool           // Flag to show or hide the log overlay
	logViewport       viewport.Model // Viewport for scrollable log overlay
	mainLogViewport   viewport.Model // Viewport for the main, in-line log panel

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

// getManagementClusterContextIdentifier generates the MC part of a kube context name.
// For example, if m.managementCluster="myinstallation", this returns "myinstallation".
// This identifier is typically used to form the full context name, e.g., "teleport.giantswarm.io-myinstallation".
// Other parts of the codebase (e.g., in commands.go, handlers.go) should use this
// method when they need to construct or refer to the MC's context name.
func (m *model) getManagementClusterContextIdentifier() string {
	return m.managementCluster
}

// getWorkloadClusterContextIdentifier generates the WC context name or combined MC-WC, based on m.managementCluster and m.workloadCluster.
// Examples:
// - if m.managementCluster="myinstallation" and m.workloadCluster="myworkloadcluster", it returns "myinstallation-myworkloadcluster".
// - if m.managementCluster="myinstallation" and m.workloadCluster="myinstallation-myworkloadcluster", it returns "myinstallation-myworkloadcluster".
//
// This identifier is typically used to form the full context name, e.g., "teleport.giantswarm.io-myinstallation-myworkloadcluster".
// The function attempts to prevent accidental double prefixing of the MC name when constructing
// or match against the WC's context name. This will prevent errors like "myinstallation-myinstallation-myworkloadcluster".
func (m *model) getWorkloadClusterContextIdentifier() string {
	if m.workloadCluster == "" {
		return "" // No WC defined or selected
	}
	// If m.workloadCluster already starts with m.managementCluster + "-", it's likely the full name.
	if m.managementCluster != "" && strings.HasPrefix(m.workloadCluster, m.managementCluster+"-") {
		return m.workloadCluster
	}
	// If m.workloadCluster is a short name and m.managementCluster is present, combine them.
	if m.managementCluster != "" {
		return m.managementCluster + "-" + m.workloadCluster
	}
	// Otherwise, use m.workloadCluster as is (e.g., MC name is empty, or WC is standalone).
	return m.workloadCluster
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

	// Detect current color profile and set dark mode ON by default
	colorProfile := lipgloss.ColorProfile().String()
	lipgloss.SetHasDarkBackground(true) // Force dark mode by default
	isDarkBg := true                    // Set this explicitly since we're forcing dark mode
	colorMode := fmt.Sprintf("%s (Dark: %v)", colorProfile, isDarkBg)

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
		TUIChannel:         tuiMsgChannel,      // Assign the channel to the model
		debugMode:          false,              // Start with debug mode disabled by default
		colorMode:          colorMode,          // Store the detected color mode
		helpVisible:        false,              // Start with help overlay hidden
		logOverlayVisible:  false,              // Initialize log overlay as hidden
		logViewport:        viewport.New(0, 0), // Initialize viewport (size will be set in View)
		mainLogViewport:    viewport.New(0, 0), // Initialize main log viewport
	}

	m.logViewport.SetContent("Log overlay initialized...")  // Initial content
	m.mainLogViewport.SetContent("Main log initialized...") // Initial content for main log

	setupPortForwards(&m, mcName, wcName)

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
		mcIdentifier := m.getManagementClusterContextIdentifier()
		if mcIdentifier != "" {
			cmds = append(cmds, fetchNodeStatusCmd(mcIdentifier, true, m.managementCluster))
		}
	}
	if m.workloadCluster != "" {
		wcIdentifier := m.getWorkloadClusterContextIdentifier()
		if wcIdentifier != "" {
			// Pass m.workloadCluster (short name) as originalClusterShortName for the message tag.
			cmds = append(cmds, fetchNodeStatusCmd(wcIdentifier, false, m.workloadCluster))
		}
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
			// Handle special keys for overlay and mode toggling
			switch msg.String() {
			case "h":
				// Toggle help overlay
				m.helpVisible = !m.helpVisible
				return m, channelReaderCmd(m.TUIChannel)
			case "D":
				// Toggle dark mode and update color mode info
				isDark := lipgloss.HasDarkBackground()
				// Flip the dark background setting
				lipgloss.SetHasDarkBackground(!isDark)
				// Update the color mode status for display
				m.colorMode = fmt.Sprintf("%s (Dark: %v)", lipgloss.ColorProfile().String(), !isDark)
				return m, channelReaderCmd(m.TUIChannel)
			case "z":
				// Toggle debug mode
				m.debugMode = !m.debugMode
				return m, channelReaderCmd(m.TUIChannel)
			case "esc":
				// ESC key closes help overlay if it's open
				if m.helpVisible {
					m.helpVisible = false
					return m, channelReaderCmd(m.TUIChannel)
				}
				// Otherwise fall through to normal handling
			}

			// Handle log overlay toggle if no other specific key for overlays was pressed
			if !m.helpVisible && msg.String() == "L" { // Use 'L' for Log overlay
				m.logOverlayVisible = !m.logOverlayVisible
				if m.logOverlayVisible {
					// When opening, set viewport content and move to bottom
					m.logViewport.SetContent(strings.Join(m.combinedOutput, "\n"))
					m.logViewport.GotoBottom()
				}
				return m, channelReaderCmd(m.TUIChannel)
			}

			m, cmd = handleKeyMsgGlobal(m, msg, []tea.Cmd{})
		}
		return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))

	// Window size messages are handled by a function in handlers.go
	case tea.WindowSizeMsg:
		m, cmd := handleWindowSizeMsg(m, msg)
		// If log overlay is visible, update its size too
		if m.logOverlayVisible {
			// Example: 80% of screen width, 70% of screen height for the log overlay
			logOverlayWidth := int(float64(m.width) * 0.8)
			logOverlayHeight := int(float64(m.height) * 0.7)
			m.logViewport.Width = logOverlayWidth - logOverlayStyle.GetHorizontalFrameSize() // Use a new logOverlayStyle
			m.logViewport.Height = logOverlayHeight - logOverlayStyle.GetVerticalFrameSize()
		} else {
			// Update main log viewport size if overlay is not visible.
			// The actual dimensions will be driven by the View() function's layout calculations.
			// We can recalculate them here briefly or rely on View() to do it before rendering.
			// For simplicity, we'll let View() manage it, but ensure it has non-zero initial if possible.
			if m.ready { // only if model is ready and width/height are known
				contentWidth := m.width - appStyle.GetHorizontalFrameSize()
				totalAvailableHeight := m.height - appStyle.GetVerticalFrameSize()
				headerHeight := lipgloss.Height(renderHeader(m, contentWidth)) // Re-calc for current size

				maxRow1Height := int(float64(totalAvailableHeight-headerHeight) * 0.20)
				if maxRow1Height < 5 {
					maxRow1Height = 5
				} else if maxRow1Height > 7 {
					maxRow1Height = 7
				}
				row1Height := lipgloss.Height(renderContextPanesRow(m, contentWidth, maxRow1Height))

				maxRow2Height := int(float64(totalAvailableHeight-headerHeight) * 0.30)
				if maxRow2Height < 7 {
					maxRow2Height = 7
				} else if maxRow2Height > 9 {
					maxRow2Height = 9
				}
				row2Height := lipgloss.Height(renderPortForwardingRow(m, contentWidth, maxRow2Height))

				if m.height >= minHeightForMainLogView {
					numGaps := 3
					heightConsumedByElementsAndGaps := headerHeight + row1Height + row2Height + numGaps
					logSectionHeight := totalAvailableHeight - heightConsumedByElementsAndGaps
					if logSectionHeight < 0 {
						logSectionHeight = 0
					}

					m.mainLogViewport.Width = contentWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()
					m.mainLogViewport.Height = logSectionHeight - panelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(logPanelTitleStyle.Render(" ")) - 1
					if m.mainLogViewport.Height < 0 {
						m.mainLogViewport.Height = 0
					}
				}
			}
		}
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

	case tea.MouseMsg:
		var cmd tea.Cmd
		// If log overlay is visible, pass mouse events to it for scrolling
		if m.logOverlayVisible {
			m.logViewport, cmd = m.logViewport.Update(msg)
			return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
		} else {
			// If log overlay is NOT visible, pass mouse events to the main log viewport
			// (Assuming no other mouse-interactive components are active)
			m.mainLogViewport, cmd = m.mainLogViewport.Update(msg)
			return m, tea.Batch(cmd, channelReaderCmd(m.TUIChannel))
		}
		// If other mouse-interactive components are added later, handle them here.
		// For now, if not the log overlay, ignore other mouse events.
		return m, channelReaderCmd(m.TUIChannel) // Ensure channel reader continues

	default:
		// Handle text input updates if in new connection mode and input is focused,
		// but not a key press (which is handled by tea.KeyMsg case above).
		var finalCmd tea.Cmd
		if m.isConnectingNew && m.newConnectionInput.Focused() {
			var textInputCmd tea.Cmd
			m.newConnectionInput, textInputCmd = m.newConnectionInput.Update(msg)
			finalCmd = textInputCmd
		} else if m.logOverlayVisible { // Pass messages to viewport if log overlay is active
			var viewportCmd tea.Cmd
			m.logViewport, viewportCmd = m.logViewport.Update(msg)
			finalCmd = viewportCmd
		}
		return m, tea.Batch(finalCmd, channelReaderCmd(m.TUIChannel))
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
		return renderNewConnectionInputView(m, m.width) // Uses helper from view_helpers.go
	}

	// Regular view rendering
	// Use the full terminal width with no margins for perfect alignment
	contentWidth := m.width
	totalAvailableHeight := m.height

	// For extremely small windows, just show a header
	if totalAvailableHeight < 5 || contentWidth < 20 {
		return renderHeader(m, contentWidth)
	}

	// ----- GLOBAL HEADER SECTION -----
	headerRenderedView := renderHeader(m, contentWidth) // Uses helper from view_helpers.go
	headerHeight := lipgloss.Height(headerRenderedView)

	// Adjust layout approach for very small windows
	if totalAvailableHeight < 15 {
		// In small windows, just show header and cluster info
		row1FinalView := renderContextPanesRow(m, contentWidth, totalAvailableHeight-headerHeight-1)
		return lipgloss.JoinVertical(lipgloss.Left, headerRenderedView, row1FinalView)
	}

	// ----- Height Allocations -----
	maxRow1Height := int(float64(totalAvailableHeight-headerHeight) * 0.20) // Adjusted percentage slightly for better balance
	if maxRow1Height < 5 {
		maxRow1Height = 5
	} else if maxRow1Height > 7 {
		maxRow1Height = 7
	}

	maxRow2Height := int(float64(totalAvailableHeight-headerHeight) * 0.30) // Adjusted percentage slightly
	if maxRow2Height < 7 {
		maxRow2Height = 7
	} else if maxRow2Height > 9 {
		maxRow2Height = 9
	}

	// ----- ROW 1: MC/WC Info -----
	row1FinalView := renderContextPanesRow(m, contentWidth, maxRow1Height) // Uses helper from view_helpers.go
	row1Height := lipgloss.Height(row1FinalView)

	// ----- ROW 2: Port Forwarding -----
	row2FinalView := renderPortForwardingRow(m, contentWidth, maxRow2Height) // Uses helper from view_helpers.go
	row2Height := lipgloss.Height(row2FinalView)

	// ----- Main Content Assembly -----
	var finalViewLayout []string
	currentHeaderView := headerRenderedView

	finalViewLayout = append(finalViewLayout, currentHeaderView)
	finalViewLayout = append(finalViewLayout, row1FinalView)
	finalViewLayout = append(finalViewLayout, row2FinalView)

	if m.height >= minHeightForMainLogView { // minHeightForMainLogView is a constant from styles.go
		// Calculate log section height to take all remaining space
		numGaps := 3 // Gaps between header-row1, row1-row2, row2-logPanel
		heightConsumedByFixedElements := headerHeight + row1Height + row2Height + numGaps
		logSectionHeight := totalAvailableHeight - heightConsumedByFixedElements

		// Add debug info to see what's happening with height calculations
		debugHeightInfo := fmt.Sprintf(
			"DEBUG: total=%d fixed=%d log=%d | header=%d row1=%d row2=%d",
			totalAvailableHeight, heightConsumedByFixedElements, logSectionHeight,
			headerHeight, row1Height, row2Height)
		m.combinedOutput = append([]string{debugHeightInfo}, m.combinedOutput...)

		if logSectionHeight < 0 { // Ensure it's not negative if space is very constrained
			logSectionHeight = 0
		}

		// IMPORTANT: We need to force the log panel to take all remaining space
		// Set maximum log height - at least 30% of total height, or all remaining space
		if logSectionHeight < int(float64(totalAvailableHeight)*0.3) && totalAvailableHeight > 30 {
			// Ensure log panel takes at least 30% of screen
			logSectionHeight = int(float64(totalAvailableHeight) * 0.3)

			// Limit other sections if needed to make space
			if row2Height > 7 {
				row2Height = 7
			}
			if row1Height > 5 {
				row1Height = 5
			}
		}

		// Update log viewport size BEFORE rendering - forcing exact dimensions
		m.mainLogViewport.Width = contentWidth - panelStatusDefaultStyle.GetHorizontalFrameSize()

		// Viewport height must account for panel title and borders
		// Border top + title + gap + content + border bottom = log height
		borderAndTitleHeight := panelStatusDefaultStyle.GetVerticalFrameSize() + 1 // +1 for title
		viewportHeight := logSectionHeight - borderAndTitleHeight
		if viewportHeight < 0 {
			viewportHeight = 0
		}

		// Force viewport height to match the calculated space
		m.mainLogViewport.Height = viewportHeight

		// Set content AFTER setting dimensions
		m.mainLogViewport.SetContent(strings.Join(m.combinedOutput, "\n"))

		// Now render log panel with the properly sized viewport
		combinedLogViewString := renderCombinedLogPanel(&m, contentWidth, logSectionHeight)

		// Debug mode: Check if combined log view string starts with "Log [H=" and fix it
		if m.debugMode && strings.Contains(combinedLogViewString, "Log [H=") {
			// Replace the debug prefix with the regular title
			combinedLogViewString = strings.Replace(
				combinedLogViewString,
				"Log [H=",
				"Combined Activity Log",
				1)
		}

		finalViewLayout = append(finalViewLayout, combinedLogViewString)

	} else {
		// If main log view is hidden, update header to hint 'L' for log overlay
		if !strings.Contains(currentHeaderView, "L for Logs") {
			updatedHeaderStr := strings.Replace(currentHeaderView, "h for Help", "h for Help | L for Logs", 1)
			finalViewLayout[0] = updatedHeaderStr // Update the header in the layout
		}
		m.logViewport.SetContent(strings.Join(m.combinedOutput, "\n"))
	}

	// Join all layout elements vertically
	joinedView := lipgloss.JoinVertical(lipgloss.Left, finalViewLayout...)

	// Make sure the view fills the entire terminal width and height
	finalView := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left, // Align left horizontally
		lipgloss.Top,  // Align top vertically
		joinedView,
		lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#222222"}), // Match the terminal background
	)

	// ----- OVERLAYS (Help & Log) -----
	if m.helpVisible {
		helpOverlay := renderHelpOverlay(m, m.width, m.height) // Uses helper from view_helpers.go
		return lipgloss.Place(
			m.width, m.height, lipgloss.Center, lipgloss.Center, helpOverlay,
			lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}),
		)
	} else if m.logOverlayVisible {
		overlayWidth := int(float64(m.width) * 0.8)
		overlayHeight := int(float64(m.height) * 0.7)

		// Update viewport size before rendering it within the overlay
		m.logViewport.Width = overlayWidth - logOverlayStyle.GetHorizontalFrameSize()
		m.logViewport.Height = overlayHeight - logOverlayStyle.GetVerticalFrameSize()

		logOverlay := renderLogOverlay(m, overlayWidth, overlayHeight) // Uses helper from view_helpers.go
		return lipgloss.Place(
			m.width, m.height, lipgloss.Center, lipgloss.Center, logOverlay,
			lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}),
		)
	}

	return finalView
}
