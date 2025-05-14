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

// Define an enum for the current step in the new connection input process.
type newInputStep int

const (
	mcInputStep newInputStep = iota
	wcInputStep
	maxCombinedOutputLines = 200 // Define the constant for log trimming
)

type model struct {
	managementCluster  string
	workloadCluster    string
	kubeContext        string // This is the target context from the command line, generally the WC context or MC if no WC
	currentKubeContext string // This is the actual current context from kubectl config current-context

	MCHealth clusterHealthInfo
	WCHealth clusterHealthInfo

	portForwards map[string]*portForwardProcess
	// portForwardOrder now includes MC/WC pane focus keys for unified navigation
	portForwardOrder []string
	focusedPanelKey  string
	combinedOutput   []string
	quitting         bool
	ready            bool // TUI ready (window size received)
	width            int
	height           int

	// --- New Connection Input State ---
	isConnectingNew    bool               // True if the TUI is in 'new connection input' mode
	newConnectionInput textinput.Model    // Text input field for new cluster names
	currentInputStep   newInputStep       // Tracks if we are inputting MC or WC name
	stashedMcName      string             // Temporarily stores MC name while WC name is being input
	clusterInfo        *utils.ClusterInfo // Holds fetched cluster list for autocompletion
}

// setupPortForwards populates the portForwards map and portForwardOrder slice.
// This is refactored from InitialModel to be reusable.
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
			context:   mcName,
			namespace: "mimir",
			service:   "service/mimir-query-frontend",
			active:    true,
			statusMsg: "Initializing...",
		}

		// Grafana for MC
		grafanaLabel := "Grafana (MC)"
		m.portForwardOrder = append(m.portForwardOrder, grafanaLabel)
		m.portForwards[grafanaLabel] = &portForwardProcess{
			label:     grafanaLabel,
			port:      "3000:3000",
			isWC:      false,
			context:   mcName,
			namespace: "monitoring",
			service:   "service/grafana",
			active:    true,
			statusMsg: "Initializing...",
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
			context:   actualWcContextPart, // Use the correctly formed context part
			namespace: "kube-system",
			service:   "service/alloy-metrics-cluster",
			active:    true,
			statusMsg: "Initializing...",
		}
	}
}

func InitialModel(mcName, wcName, kubeCtx string) model {
	ti := textinput.New()
	ti.Placeholder = "Management Cluster"
	ti.CharLimit = 156 // Arbitrary limit
	ti.Width = 50      // Arbitrary width

	m := model{
		managementCluster: mcName,
		workloadCluster:   wcName,
		kubeContext:       kubeCtx,
		portForwards:      make(map[string]*portForwardProcess),
		// Initialize portForwardOrder with context pane keys first for navigation order
		portForwardOrder: make([]string, 0),
		combinedOutput:   make([]string, 0),
		MCHealth:         clusterHealthInfo{IsLoading: true},
		// New connection input fields
		isConnectingNew:    false,
		newConnectionInput: ti,
		currentInputStep:   mcInputStep,
	}

	m.setupPortForwards(mcName, wcName) // Use the refactored method

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

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd // Holds commands to be batched IF NOT handled by a specific case returning a cmd.

	switch msg := msg.(type) {
	// Key messages are handled by functions in handlers.go
	case tea.KeyMsg:
		var cmd tea.Cmd
		if m.isConnectingNew && m.newConnectionInput.Focused() {
			// Pass m and msg. The handler will return the updated model and any command.
			m, cmd = handleKeyMsgInputMode(m, msg)
		} else {
			// Pass m, msg, and the current cmds slice.
			// The handler will return the updated model and any command.
			// Note: handleKeyMsgGlobal might append to cmds or return a new tea.Cmd.
			// If it returns a tea.Cmd, that should be the one used.
			// If it appends to the passed cmds, then the batch at the end will pick it up.
			// For simplicity and consistency, let's assume it returns a command.
			m, cmd = handleKeyMsgGlobal(m, msg, []tea.Cmd{}) // Pass empty slice, expect direct cmd return
		}
		return m, cmd

	// Window size messages are handled by a function in handlers.go
	case tea.WindowSizeMsg:
		return handleWindowSizeMsg(m, msg)

	// Port Forwarding Messages (handlers in portforward_handlers.go)
	case portForwardStartedMsg:
		return handlePortForwardStartedMsg(m, msg)
	case portForwardOutputMsg:
		return handlePortForwardOutputMsg(m, msg)
	case portForwardErrorMsg:
		return handlePortForwardErrorMsg(m, msg)
	case portForwardStreamEndedMsg:
		return handlePortForwardStreamEndedMsg(m, msg)

	// New Connection Flow Messages (handlers in connection_flow.go)
	// These handlers take the existing cmds []tea.Cmd, append to it, and return tea.Batch.
	// So, we pass the local cmds and let the handler return the final batched command.
	case submitNewConnectionMsg:
		return handleSubmitNewConnectionMsg(m, msg, cmds)
	case kubeLoginResultMsg:
		return handleKubeLoginResultMsg(m, msg, cmds)
	case contextSwitchAndReinitializeResultMsg:
		return handleContextSwitchAndReinitializeResultMsg(m, msg, cmds)

	// Other System/Async Messages (handlers in handlers.go)
	case kubeContextResultMsg:
		m = handleKubeContextResultMsg(m, msg) // Modifies model, returns no cmd
		return m, nil
	case requestClusterHealthUpdate:
		// This handler returns (model, tea.Cmd)
		return handleRequestClusterHealthUpdate(m)
	case kubeContextSwitchedMsg:
		// This handler returns (model, tea.Cmd)
		return handleKubeContextSwitchedMsg(m, msg)
	case nodeStatusMsg:
		m = handleNodeStatusMsg(m, msg) // Modifies model, returns no cmd
		return m, nil
	case clusterListResultMsg:
		m = handleClusterListResultMsg(m, msg) // Modifies model, returns no cmd
		return m, nil

	default:
		// Handle text input updates if in new connection mode and input is focused,
		// but not a key press (which is handled by tea.KeyMsg case above).
		if m.isConnectingNew && m.newConnectionInput.Focused() {
			var textInputCmd tea.Cmd
			m.newConnectionInput, textInputCmd = m.newConnectionInput.Update(msg)
			return m, textInputCmd
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
	return m, tea.Batch(cmds...)
}

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
		// Add help text for current input step
		if m.currentInputStep == mcInputStep {
			inputPrompt.WriteString("\n\n[Input: Management Cluster Name]")
		} else {
			inputPrompt.WriteString(fmt.Sprintf("\n\n[Input: Workload Cluster Name for MC: %s (optional)]", m.stashedMcName))
		}
		inputViewStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).Width(m.width - 4).Align(lipgloss.Center)
		return inputViewStyle.Render(inputPrompt.String())
	}

	// Regular view rendering (existing logic)
	availableWidth := m.width - appStyle.GetHorizontalFrameSize()
	availableHeight := m.height - appStyle.GetVerticalFrameSize()

	headerTitleString := "envctl TUI - Quit: 'q'/Ctrl+C | Navigate: Tab/Shift+Tab | Restart PF: 'r' | Switch Ctx: 's' | New Conn: N"
	headerTitleView := headerStyle.Copy().Width(availableWidth).Render(headerTitleString)

	var contextPanesView string
	if m.workloadCluster != "" {
		separatorWidth := 1 // For the space between MC and WC panes
		mcPaneWidth := (availableWidth - separatorWidth) / 2
		wcPaneWidth := availableWidth - separatorWidth - mcPaneWidth // Ensures total width is availableWidth

		renderedMcPane := renderMcPane(m, mcPaneWidth)
		renderedWcPane := renderWcPane(m, wcPaneWidth)
		contextPanesView = lipgloss.JoinHorizontal(lipgloss.Top, renderedMcPane, lipgloss.NewStyle().Width(separatorWidth).Render(" "), renderedWcPane)
	} else {
		contextPanesView = renderMcPane(m, availableWidth)
	}

	topSection := lipgloss.JoinVertical(lipgloss.Left,
		headerTitleView,
		lipgloss.NewStyle().MarginTop(1).Render(contextPanesView),
	)
	topSectionHeight := lipgloss.Height(topSection)

	var portForwardPanelViews []string
	numPanels := 0
	for _, key := range m.portForwardOrder {
		if key != mcPaneFocusKey && key != wcPaneFocusKey {
			numPanels++
		}
	}

	maxPfPanelHeight := 0
	if numPanels > 0 {
		individualPanelFrameSize := panelStatusDefaultStyle.GetHorizontalFrameSize() // Frame size of a single panel

		baseOuterWidthPerPanel := availableWidth / numPanels
		remainderOuterWidth := availableWidth % numPanels

		panelIndex := 0
		for _, pfKey := range m.portForwardOrder {
			if pfKey == mcPaneFocusKey || pfKey == wcPaneFocusKey {
				continue
			}
			pf := m.portForwards[pfKey]

			currentPanelOuterWidth := baseOuterWidthPerPanel
			if panelIndex < remainderOuterWidth {
				currentPanelOuterWidth++
			}

			currentPanelContentWidth := currentPanelOuterWidth - individualPanelFrameSize
			if currentPanelContentWidth < 0 { // Ensure content width isn't negative
				currentPanelContentWidth = 0
			}

			renderedPanel := renderPortForwardPanel(pf, m, currentPanelOuterWidth, currentPanelContentWidth)
			portForwardPanelViews = append(portForwardPanelViews, renderedPanel)
			if lipgloss.Height(renderedPanel) > maxPfPanelHeight {
				maxPfPanelHeight = lipgloss.Height(renderedPanel)
			}
			panelIndex++
		}
	} else {
		// Handle case where numPanels is 0 - portForwardPanelViews remains empty
	}

	portForwardsView := lipgloss.JoinHorizontal(lipgloss.Top, portForwardPanelViews...)
	pfSectionHeight := maxPfPanelHeight

	logPanelMinHeight := 5
	logSectionHeight := availableHeight - topSectionHeight - pfSectionHeight - appStyle.GetVerticalFrameSize() - lipgloss.NewStyle().MarginTop(1).GetVerticalFrameSize() - lipgloss.NewStyle().MarginBottom(1).GetVerticalFrameSize()
	if logSectionHeight < logPanelMinHeight {
		logSectionHeight = logPanelMinHeight
	}

	combinedLogView := renderCombinedLogPanel(m, availableWidth, logSectionHeight) // Use helper

	finalView := lipgloss.JoinVertical(lipgloss.Left,
		topSection,
		lipgloss.NewStyle().MarginTop(1).Width(availableWidth).Render(portForwardsView),
		lipgloss.NewStyle().MarginTop(1).Render(combinedLogView),
	)

	return appStyle.Render(finalView)
}
