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

	portForwards     map[string]*portForwardProcess
	// portForwardOrder now includes MC/WC pane focus keys for unified navigation
	portForwardOrder []string
	focusedPanelKey  string
	combinedOutput   []string
	quitting         bool
	ready            bool // TUI ready (window size received)
	width            int
	height           int

	// --- New Connection Input State ---
	isConnectingNew    bool             // True if the TUI is in 'new connection input' mode
	newConnectionInput textinput.Model  // Text input field for new cluster names
	currentInputStep   newInputStep     // Tracks if we are inputting MC or WC name
	stashedMcName      string           // Temporarily stores MC name while WC name is being input
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
		portForwardOrder:  make([]string, 0),
		combinedOutput:    make([]string, 0),
		MCHealth:          clusterHealthInfo{IsLoading: true},
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
	for _, label := range m.portForwardOrder {
		// Check if the label corresponds to an actual port-forward process
		// and not a special focus key like mcPaneFocusKey or wcPaneFocusKey.
		pf, isActualPortForward := m.portForwards[label]

		if isActualPortForward && pf.active { // Only proceed if it's a defined and active port-forward
			pf_loop := pf // Capture loop variable for closure
			cmd, stdout, stderr, err := utils.StartPortForward(pf_loop.context, pf_loop.namespace, pf_loop.service, pf_loop.port, pf_loop.label)
			if err != nil {
				m.portForwards[pf_loop.label].err = err
				m.portForwards[pf_loop.label].statusMsg = "Failed to start"
				m.portForwards[pf_loop.label].stdoutClosed = true
				m.portForwards[pf_loop.label].stderrClosed = true
				cmds = append(cmds, func() tea.Msg { return portForwardErrorMsg{label: pf_loop.label, streamType: "general", err: fmt.Errorf("failed to start %s: %w", pf_loop.label, err)} })
			} else {
				// cmd is from this iteration and StartPortForward succeeded, so cmd.Process should be valid.
				processID := cmd.Process.Pid // Evaluate and capture the PID now.
				m.portForwards[pf_loop.label].cmd = cmd
				m.portForwards[pf_loop.label].stdout = stdout
				m.portForwards[pf_loop.label].stderr = stderr
				m.portForwards[pf_loop.label].statusMsg = "Starting..."
				cmds = append(cmds,
					waitForPortForwardActivity(pf_loop.label, "stdout", stdout),
					waitForPortForwardActivity(pf_loop.label, "stderr", stderr),
					func() tea.Msg { return portForwardStartedMsg{label: pf_loop.label, pid: processID} },
				)
			}
		}
	}

	// Add a ticker for periodic health updates
	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
		return requestClusterHealthUpdate{}
	})
	cmds = append(cmds, tickCmd)

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Default handling when not in new connection input mode (or for any message type)
	switch msg := msg.(type) {
	case tea.KeyMsg: // msg is of type tea.KeyMsg here
		if m.isConnectingNew && m.newConnectionInput.Focused() {
			// Input mode is active
			switch msg.String() {
			case "ctrl+s": // Submit new connection (MC or WC)
				if m.currentInputStep == mcInputStep {
					mcName := m.newConnectionInput.Value()
					if mcName == "" {
						// Optionally: m.newConnectionInput.Err = errors.New("MC name cannot be empty")
						return m, nil
					}
					m.stashedMcName = mcName
					m.currentInputStep = wcInputStep
					m.newConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName) // Added Tab Complete
					m.newConnectionInput.SetValue("")
					m.newConnectionInput.Focus() // Keep focus
					return m, nil
				} else if m.currentInputStep == wcInputStep {
					wcName := m.newConnectionInput.Value()
					m.isConnectingNew = false
					m.newConnectionInput.Blur()
					m.newConnectionInput.Reset() // Clear input field for next time
					// Restore focus to previously focused panel or default
					if len(m.portForwardOrder) > 0 { // Basic fallback
						m.focusedPanelKey = m.portForwardOrder[0]
					}
					return m, func() tea.Msg { return submitNewConnectionMsg{mc: m.stashedMcName, wc: wcName} }
				}

			case "enter": // Confirm MC input and move to WC, or submit WC input
				if m.currentInputStep == mcInputStep {
					mcName := m.newConnectionInput.Value()
					if mcName == "" {
						// Optionally: m.newConnectionInput.Err = errors.New("MC name cannot be empty")
						return m, nil
					}
					m.stashedMcName = mcName
					m.currentInputStep = wcInputStep
					m.newConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName) // Added Tab Complete
					m.newConnectionInput.SetValue("")
					m.newConnectionInput.Focus() // Keep focus
					return m, nil
				} else if m.currentInputStep == wcInputStep {
					wcName := m.newConnectionInput.Value()
					m.isConnectingNew = false
					m.newConnectionInput.Blur()
					m.newConnectionInput.Reset() // Clear input field for next time
					if len(m.portForwardOrder) > 0 { // Basic fallback
						m.focusedPanelKey = m.portForwardOrder[0]
					}
					return m, func() tea.Msg { return submitNewConnectionMsg{mc: m.stashedMcName, wc: wcName} }
				}

			case "esc": // Cancel new connection input
				m.isConnectingNew = false
				m.newConnectionInput.Blur()
				m.newConnectionInput.Reset()
				m.currentInputStep = mcInputStep // Reset for next time
				m.stashedMcName = ""
				if len(m.portForwardOrder) > 0 { // Basic fallback
					m.focusedPanelKey = m.portForwardOrder[0]
				}
				return m, nil

			case "tab": // Autocompletion
				currentInput := m.newConnectionInput.Value()
				if m.clusterInfo != nil && currentInput != "" {
					var suggestions []string
					normalizedCurrentInput := strings.ToLower(currentInput)
					if m.currentInputStep == mcInputStep {
						for _, mcSuggestion := range m.clusterInfo.ManagementClusters {
							if strings.HasPrefix(strings.ToLower(mcSuggestion), normalizedCurrentInput) {
								suggestions = append(suggestions, mcSuggestion)
							}
						}
					} else if m.currentInputStep == wcInputStep && m.stashedMcName != "" {
						if wcs, ok := m.clusterInfo.WorkloadClusters[m.stashedMcName]; ok {
							for _, wcSuggestion := range wcs {
								if strings.HasPrefix(strings.ToLower(wcSuggestion), normalizedCurrentInput) {
									suggestions = append(suggestions, wcSuggestion)
								}
							}
						}
					}
					if len(suggestions) > 0 {
						m.newConnectionInput.SetValue(suggestions[0])
						m.newConnectionInput.SetCursor(len(suggestions[0]))
					}
				}
				return m, nil // Tab consumed for autocompletion

			default:
				// Let the textinput handle other keys (arrows, chars, backspace, etc.)
				var inputCmd tea.Cmd
				m.newConnectionInput, inputCmd = m.newConnectionInput.Update(msg)
				cmds = append(cmds, inputCmd) // Collect command from textinput
				return m, tea.Batch(cmds...)   // Batch commands
			}
		} else {
			// Not in input mode OR input not focused -> Handle global keybindings
			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				var quitCmds []tea.Cmd
				for _, pf := range m.portForwards {
					if pf.cmd != nil && pf.cmd.Process != nil {
						pfToKill := pf // Capture loop variable for closure
						quitCmds = append(quitCmds, func() tea.Msg {
							pfToKill.cmd.Process.Kill() //nolint:errcheck // Best effort
							return nil
						})
					}
				}
				quitCmds = append(quitCmds, tea.Quit)
				return m, tea.Batch(quitCmds...)

			case "n": // Start new connection
				if !m.isConnectingNew { // Prevent re-triggering if somehow 'n' is pressed while already connecting
					m.isConnectingNew = true
					m.currentInputStep = mcInputStep
					m.newConnectionInput.Prompt = "Enter Management Cluster (Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): "
					m.newConnectionInput.Focus()
					// TODO: Stash m.focusedPanelKey to restore on Esc and set it to an "" to prevent panel actions
					return m, textinput.Blink
				}

			case "tab": // Panel focus
				if len(m.portForwardOrder) > 0 {
					currentIndex := -1
					for i, key := range m.portForwardOrder {
						if key == m.focusedPanelKey {
							currentIndex = i
							break
						}
					}
					if currentIndex != -1 {
						nextIndex := (currentIndex + 1) % len(m.portForwardOrder)
						m.focusedPanelKey = m.portForwardOrder[nextIndex]
					} else { // If no panel was focused, or focus was lost, focus the first one.
						m.focusedPanelKey = m.portForwardOrder[0]
					}
				}
				return m, nil

			case "shift+tab": // Panel focus (reverse)
				if len(m.portForwardOrder) > 0 {
					currentIndex := -1
					for i, key := range m.portForwardOrder {
						if key == m.focusedPanelKey {
							currentIndex = i
							break
						}
					}
					if currentIndex != -1 {
						nextIndex := (currentIndex - 1 + len(m.portForwardOrder)) % len(m.portForwardOrder)
						m.focusedPanelKey = m.portForwardOrder[nextIndex]
					} else { // If no panel was focused, or focus was lost, focus the last one.
						m.focusedPanelKey = m.portForwardOrder[len(m.portForwardOrder)-1]
					}
				}
				return m, nil

			case "k", "up":
				if len(m.portForwardOrder) > 0 {
					currentIndex := -1
					for i, key := range m.portForwardOrder {
						if key == m.focusedPanelKey {
							currentIndex = i
							break
						}
					}
					if currentIndex != -1 {
						nextIndex := (currentIndex - 1 + len(m.portForwardOrder)) % len(m.portForwardOrder)
						m.focusedPanelKey = m.portForwardOrder[nextIndex]
					} else if len(m.portForwardOrder) > 0 {
						m.focusedPanelKey = m.portForwardOrder[len(m.portForwardOrder)-1] // Focus last
					}
				}
				return m, nil

			case "j", "down":
				if len(m.portForwardOrder) > 0 {
					currentIndex := -1
					for i, key := range m.portForwardOrder {
						if key == m.focusedPanelKey {
							currentIndex = i
							break
						}
					}
					if currentIndex != -1 {
						nextIndex := (currentIndex + 1) % len(m.portForwardOrder)
						m.focusedPanelKey = m.portForwardOrder[nextIndex]
					} else if len(m.portForwardOrder) > 0 {
						m.focusedPanelKey = m.portForwardOrder[0] // Focus first
					}
				}
				return m, nil

			case "r": // Restart focused port-forward
				if m.focusedPanelKey != "" {
					if pf, ok := m.portForwards[m.focusedPanelKey]; ok {
						if pf.cmd != nil && pf.cmd.Process != nil {
							pf.cmd.Process.Kill() //nolint:errcheck // Best effort
						}
						pf.cmd = nil
						pf.stdout = nil
						pf.stderr = nil
						pf.err = nil
						pf.output = []string{}
						pf.statusMsg = "Restarting..."
						pf.stdoutClosed = false
						pf.stderrClosed = false
						pf.active = true
						pf.forwardingEstablished = false

						m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Attempting restart...", pf.label))
						if len(m.combinedOutput) > maxCombinedOutputLines {
							m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
						}
						
						// pf_loop capture for the closure
						pf_loop := pf
						cmdToRun, stdout, stderr, err := utils.StartPortForward(pf_loop.context, pf_loop.namespace, pf_loop.service, pf_loop.port, pf_loop.label)
						if err != nil {
							pf_loop.err = err
							pf_loop.statusMsg = "Restart failed"
							pf_loop.stdoutClosed = true
							pf_loop.stderrClosed = true
							pf_loop.active = false // Mark as inactive on failed restart
							cmds = append(cmds, func() tea.Msg {
								return portForwardErrorMsg{label: pf_loop.label, streamType: "general", err: fmt.Errorf("failed to restart %s: %w", pf_loop.label, err)}
							})
						} else {
							pf_loop.cmd = cmdToRun
							pf_loop.stdout = stdout
							pf_loop.stderr = stderr
							pf_loop.statusMsg = "Starting..." // Will be updated by portForwardStartedMsg
							// Evaluate and capture the PID now.
							processID := cmdToRun.Process.Pid 
							cmds = append(cmds,
								waitForPortForwardActivity(pf_loop.label, "stdout", stdout),
								waitForPortForwardActivity(pf_loop.label, "stderr", stderr),
								func() tea.Msg { return portForwardStartedMsg{label: pf_loop.label, pid: processID} },
							)
						}
					}
				}

			case "s": // Switch kubectl context to focused MC/WC pane
				var targetContextToSwitch string
				var clusterShortNameForContext string // This will be "mc" or "mc-wc"
				var paneNameForLog string

				if m.focusedPanelKey == mcPaneFocusKey && m.managementCluster != "" {
					clusterShortNameForContext = m.managementCluster
					paneNameForLog = "MC"
				} else if m.focusedPanelKey == wcPaneFocusKey && m.workloadCluster != "" {
					if m.managementCluster != "" { // Should always be true if wc is set
						clusterShortNameForContext = m.managementCluster + "-" + m.workloadCluster
					} else {
						clusterShortNameForContext = m.workloadCluster // Fallback, less ideal
					}
					paneNameForLog = "WC"
				}

				if clusterShortNameForContext != "" {
					if !strings.HasPrefix(clusterShortNameForContext, "teleport.giantswarm.io-") {
						targetContextToSwitch = "teleport.giantswarm.io-" + clusterShortNameForContext
					} else {
						targetContextToSwitch = clusterShortNameForContext
					}
					m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Attempting to switch kubectl context to: %s (Pane: %s)", targetContextToSwitch, paneNameForLog))
					cmds = append(cmds, performSwitchKubeContextCmd(targetContextToSwitch))
				} else {
					m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Cannot switch context: Focus a valid MC/WC pane with a defined cluster name.")
				}
				if len(m.combinedOutput) > maxCombinedOutputLines {
					m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
				}
			}
		}
		// Batch any commands accumulated from non-input mode or from textinput.Update
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		return handleWindowSizeMsg(m, msg)

	case portForwardStartedMsg:
		if pf, ok := m.portForwards[msg.label]; ok {
			if pf.statusMsg != "Restart failed" {
				pf.statusMsg = fmt.Sprintf("Running (PID: %d)", msg.pid)
			}
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Process started (PID: %d)", msg.label, msg.pid))
		}

	case portForwardOutputMsg:
		if pf, ok := m.portForwards[msg.label]; ok {
			line := fmt.Sprintf("[%s %s] %s", msg.label, msg.streamType, msg.line)
			pf.output = append(pf.output, msg.line)
			m.combinedOutput = append(m.combinedOutput, line)

			if !pf.forwardingEstablished && strings.Contains(msg.line, "Forwarding from") {
				pf.forwardingEstablished = true
				parts := strings.Fields(msg.line)
				localPort := "unknown"
				for i, p := range parts {
					if (p == "from" || p == "From") && i+1 < len(parts) {
						addressAndPort := parts[i+1]
						lastColon := strings.LastIndex(addressAndPort, ":")
						if lastColon != -1 && lastColon+1 < len(addressAndPort) {
							localPort = addressAndPort[lastColon+1:]
							break
						}
					}
				}
				pf.statusMsg = fmt.Sprintf("Forwarding Active (Local Port: %s)", localPort)
				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s] Confirmed: %s", msg.label, pf.statusMsg))
			}

			if len(m.combinedOutput) > maxCombinedOutputLines {
				m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
			}
			if len(pf.output) > maxCombinedOutputLines {
				pf.output = pf.output[len(pf.output)-maxCombinedOutputLines:]
			}

			if msg.streamType == "stdout" && !pf.stdoutClosed {
				cmds = append(cmds, waitForPortForwardActivity(msg.label, "stdout", pf.stdout))
			} else if msg.streamType == "stderr" && !pf.stderrClosed {
				cmds = append(cmds, waitForPortForwardActivity(msg.label, "stderr", pf.stderr))
			}
		}

	case portForwardErrorMsg:
		if pf, ok := m.portForwards[msg.label]; ok {
			errMsgText := fmt.Sprintf("[%s %s ERROR] %s", msg.label, msg.streamType, msg.err.Error())
			pf.err = msg.err
			pf.output = append(pf.output, "ERROR: "+msg.err.Error())
			m.combinedOutput = append(m.combinedOutput, errMsgText)
			if pf.statusMsg != "Failed to start" && pf.statusMsg != "Restart failed" {
				pf.statusMsg = "Error"
			}
			if len(m.combinedOutput) > maxCombinedOutputLines {
				m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
			}
			if len(pf.output) > maxCombinedOutputLines {
				pf.output = pf.output[len(pf.output)-maxCombinedOutputLines:]
			}
			if msg.streamType == "stdout" {
				pf.stdoutClosed = true
			} else if msg.streamType == "stderr" {
				pf.stderrClosed = true
			}
		}

	case portForwardStreamEndedMsg:
		if pf, ok := m.portForwards[msg.label]; ok {
			logMsg := fmt.Sprintf("[%s %s] Stream closed.", msg.label, msg.streamType)
			m.combinedOutput = append(m.combinedOutput, logMsg)
			if msg.streamType == "stdout" {
				pf.stdoutClosed = true
			} else if msg.streamType == "stderr" {
				pf.stderrClosed = true
			}
			if pf.stdoutClosed && pf.stderrClosed && pf.active {
				if pf.statusMsg != "Killed" && pf.statusMsg != "Error" && pf.statusMsg != "Failed to start" && pf.statusMsg != "Restart failed" {
					pf.statusMsg = "Exited"
				}
			}
		}

	case kubeContextResultMsg:
		if msg.err != nil {
			m.currentKubeContext = "Error fetching context"
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Error getting current kube context: %s", msg.err.Error()))
		} else {
			m.currentKubeContext = msg.context
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Current kubectl context: %s", msg.context))
		}
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}

	case requestClusterHealthUpdate:
		logMsg := fmt.Sprintf("[SYSTEM] Requesting cluster health updates at %s", time.Now().Format("15:04:05"))
		m.combinedOutput = append(m.combinedOutput, logMsg)
		if len(m.combinedOutput) > maxCombinedOutputLines { m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:] }

		if m.managementCluster != "" {
			m.MCHealth.IsLoading = true
			cmds = append(cmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
		}
		if m.workloadCluster != "" {
			m.WCHealth.IsLoading = true
			// Pass the current MC name for context construction if it's a WC health check
			cmds = append(cmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
		}
		// Re-tick for next update
		cmds = append(cmds, tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
			return requestClusterHealthUpdate{}
		}))

	case kubeContextSwitchedMsg:
		if msg.err != nil {
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Failed to switch kubectl context to '%s': %s", msg.TargetContext, msg.err.Error()))
		} else {
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Successfully switched kubectl context. Target was: %s", msg.TargetContext))
			// Refresh current context display and health information
			cmds = append(cmds, getCurrentKubeContextCmd())
			if m.managementCluster != "" {
				m.MCHealth.IsLoading = true
				cmds = append(cmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
			}
			if m.workloadCluster != "" {
				m.WCHealth.IsLoading = true
				cmds = append(cmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
			}
		}
		if len(m.combinedOutput) > maxCombinedOutputLines {
			m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
		}

	case nodeStatusMsg:
		var targetHealth *clusterHealthInfo
		clusterNameForLog := "" // This will be m.managementCluster or m.workloadCluster based on msg.forMC

		// Determine which cluster this health update is for based on msg.forMC and msg.clusterShortName
		// and ensure it matches the model's current cluster names.
		if msg.forMC && msg.clusterShortName == m.managementCluster {
			targetHealth = &m.MCHealth
			clusterNameForLog = m.managementCluster
		} else if !msg.forMC && msg.clusterShortName == m.workloadCluster {
			targetHealth = &m.WCHealth
			clusterNameForLog = m.workloadCluster
		} else {
			// This status message is for a cluster that is no longer current (e.g., stale message)
			// Or msg.clusterShortName doesn't match, which could indicate an issue.
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[HEALTH STALE/MISMATCH] Received status for '%s' (isMC: %v), current MC: '%s', WC: '%s'. Discarding.", msg.clusterShortName, msg.forMC, m.managementCluster, m.workloadCluster))
			if len(m.combinedOutput) > maxCombinedOutputLines { m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:] }
			return m, tea.Batch(cmds...) // No further processing for this stale/mismatched message
		}

		targetHealth.IsLoading = false
		targetHealth.LastUpdated = time.Now()
		if msg.err != nil {
			targetHealth.StatusError = msg.err
			targetHealth.ReadyNodes = 0
			targetHealth.TotalNodes = 0
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[HEALTH %s] Error: %s", clusterNameForLog, msg.err.Error()))
		} else {
			targetHealth.StatusError = nil
			targetHealth.ReadyNodes = msg.readyNodes
			targetHealth.TotalNodes = msg.totalNodes
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[HEALTH %s] Nodes: %d/%d", clusterNameForLog, msg.readyNodes, msg.totalNodes))
		}
		if len(m.combinedOutput) > maxCombinedOutputLines { m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:] }

	case submitNewConnectionMsg:
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Initiating new connection to MC: %s, WC: %s", msg.mc, msg.wc))
		
		// 1. Stop existing port forwards
		for _, pfKey := range m.portForwardOrder {
			if pf, ok := m.portForwards[pfKey]; ok {
				if pf.cmd != nil && pf.cmd.Process != nil {
					pf.cmd.Process.Kill()
				}
			}
		}

		// Store desired names temporarily, will be applied to model upon full success
		m.stashedMcName = msg.mc // Re-using stashedMcName, though its purpose shifts here
		// Add a new field in model if stashedMcName is confusing: e.g., m.pendingNewWcName = msg.wc

		if msg.mc == "" {
			m.combinedOutput = append(m.combinedOutput, "[SYSTEM ERROR] Management Cluster name cannot be empty.")
			return m, nil
		}
		// Start the sequence: Login to MC first.
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Step 1: Logging into Management Cluster: %s...", msg.mc))
		cmds = append(cmds, performKubeLoginCmd(msg.mc, true, msg.wc)) // Pass desired WC to carry through
		return m, tea.Batch(cmds...)

	case kubeLoginResultMsg:
		// Append login output to the combined log first, regardless of error
		if strings.TrimSpace(msg.loginStdout) != "" {
			m.combinedOutput = append(m.combinedOutput, strings.Split(strings.TrimRight(msg.loginStdout, "\n"), "\n")...)
		}
		if strings.TrimSpace(msg.loginStderr) != "" {
			// Prefix stderr lines to distinguish them, or style them if possible later
			for _, line := range strings.Split(strings.TrimRight(msg.loginStderr, "\n"), "\n") {
				m.combinedOutput = append(m.combinedOutput, "[tsh stderr] "+line)
			}
		}

		if msg.err != nil {
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Login failed for %s: %v", msg.clusterName, msg.err))
			// Potentially reset isConnectingNew = false here or offer retry?
			return m, nil
		}
		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Login successful for: %s", msg.clusterName))

		if msg.isMC {
			// MC Login was successful. Now, check if WC login is needed.
			desiredMcForNextStep := msg.clusterName // This is the confirmed MC name
			desiredWcForNextStep := msg.desiredWcShortName // WC name from original user input

			if desiredWcForNextStep != "" {
				fullDesiredWcName := desiredMcForNextStep + "-" + desiredWcForNextStep
				m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Step 2: Logging into Workload Cluster: %s...", fullDesiredWcName))
				// For WC login, we don't need to carry forward wcShortName further.
				cmds = append(cmds, performKubeLoginCmd(fullDesiredWcName, false, ""))
			} else {
				// No WC specified, proceed to context switch and re-initialize for MC only.
				m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Step 2: No Workload Cluster specified. Proceeding to context switch for MC.")
				targetKubeContext := "teleport.giantswarm.io-" + desiredMcForNextStep
				cmds = append(cmds, performPostLoginOperationsCmd(targetKubeContext, desiredMcForNextStep, ""))
			}
		} else {
			// WC Login was successful. Proceed to context switch and re-initialize for MC + WC.
			// At this point, MC login already succeeded. msg.clusterName is the full WC name.
			// We need the MC name from when MC login succeeded. It was stashed in m.stashedMcName by submitNewConnectionMsg.
			// And the WC name (short form) was msg.desiredWcShortName from the mcLoginResultMsg.
			// This is getting complicated. Let's ensure stashedMcName is reliable or pass it through messages.
			// For this path, mcLoginResultMsg.isMC is false, meaning msg.clusterName is full WC name.
			// The stashedMcName should hold the MC that was successfully logged into.
			finalMcName := m.stashedMcName // From initial submitNewConnectionMsg
			// Extract short WC name from full WC name (msg.clusterName)
			shortWcName := ""
			if strings.HasPrefix(msg.clusterName, finalMcName+"-") {
				shortWcName = strings.TrimPrefix(msg.clusterName, finalMcName+"-")
			}
			m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Step 3: Workload Cluster login successful. Proceeding to context switch for WC.")
			targetKubeContext := "teleport.giantswarm.io-" + msg.clusterName // Full WC name for context
			cmds = append(cmds, performPostLoginOperationsCmd(targetKubeContext, finalMcName, shortWcName))
		}
		return m, tea.Batch(cmds...)

	case contextSwitchAndReinitializeResultMsg:
		// Log diagnostics first
		if msg.diagnosticLog != "" {
			m.combinedOutput = append(m.combinedOutput, "--- Diagnostic Log (Context Switch Phase) ---")
			m.combinedOutput = append(m.combinedOutput, strings.Split(strings.TrimSpace(msg.diagnosticLog), "\n")...)
			m.combinedOutput = append(m.combinedOutput, "--- End Diagnostic Log ---")
		}
		if msg.err != nil {
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Context switch/re-init failed: %v", msg.err))
			return m, nil
		}

		m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM] Successfully switched context to: %s. Re-initializing TUI.", msg.switchedContext))

		// Apply new cluster names to the model
		m.managementCluster = msg.desiredMcName
		m.workloadCluster = msg.desiredWcName
		m.currentKubeContext = msg.switchedContext

		// Reset health info
		m.MCHealth = clusterHealthInfo{IsLoading: true}
		if m.workloadCluster != "" {
			m.WCHealth = clusterHealthInfo{IsLoading: true}
		} else {
			m.WCHealth = clusterHealthInfo{}
		}

		m.setupPortForwards(m.managementCluster, m.workloadCluster)

		if len(m.portForwardOrder) > 0 {
			m.focusedPanelKey = m.portForwardOrder[0]
		} else if m.managementCluster != "" {
			m.focusedPanelKey = mcPaneFocusKey
		} else {
			m.focusedPanelKey = ""
		}

		var newInitCmds []tea.Cmd
		newInitCmds = append(newInitCmds, getCurrentKubeContextCmd()) // Verify context
		if m.managementCluster != "" {
			newInitCmds = append(newInitCmds, fetchNodeStatusCmd(m.managementCluster, true, ""))
		}
		if m.workloadCluster != "" {
			newInitCmds = append(newInitCmds, fetchNodeStatusCmd(m.workloadCluster, false, m.managementCluster))
		}

		for _, label := range m.portForwardOrder {
			pf, isActualPortForward := m.portForwards[label]
			if isActualPortForward && pf.active {
				pf_loop := pf
				startCmd, stdout, stderr, err := utils.StartPortForward(pf_loop.context, pf_loop.namespace, pf_loop.service, pf_loop.port, pf_loop.label)
				if err != nil {
					m.portForwards[pf_loop.label].err = err
					m.portForwards[pf_loop.label].statusMsg = "Failed to start"
					m.portForwards[pf_loop.label].stdoutClosed = true
					m.portForwards[pf_loop.label].stderrClosed = true
					newInitCmds = append(newInitCmds, func() tea.Msg { return portForwardErrorMsg{label: pf_loop.label, streamType: "general", err: fmt.Errorf("failed to start %s: %w", pf_loop.label, err)} })
				} else {
					processID := startCmd.Process.Pid
					m.portForwards[pf_loop.label].cmd = startCmd
					m.portForwards[pf_loop.label].stdout = stdout
					m.portForwards[pf_loop.label].stderr = stderr
					newInitCmds = append(newInitCmds,
						waitForPortForwardActivity(pf_loop.label, "stdout", stdout),
						waitForPortForwardActivity(pf_loop.label, "stderr", stderr),
						func() tea.Msg { return portForwardStartedMsg{label: pf_loop.label, pid: processID} },
					)
				}
			}
		}
		newInitCmds = append(newInitCmds, tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg {
			return requestClusterHealthUpdate{}
		}))
		cmds = append(cmds, tea.Batch(newInitCmds...))
		return m, tea.Batch(cmds...)

	case clusterListResultMsg:
		if msg.err != nil {
			m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[SYSTEM ERROR] Failed to fetch cluster list: %v", msg.err))
			// Potentially log this to a file or handle more gracefully
			// For now, autocompletion will simply not work if this fails.
		} else {
			m.clusterInfo = msg.info
			// Optionally log success or number of clusters found
			// m.combinedOutput = append(m.combinedOutput, "[SYSTEM] Cluster list fetched for autocompletion.")
		}
		return m, nil // No further command needed from here
	}

	if m.quitting {
		return m, tea.Batch(cmds...)
	}

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
		inputViewStyle := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).Width(m.width -4).Align(lipgloss.Center)
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
