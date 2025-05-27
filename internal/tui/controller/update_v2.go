package controller

import (
	"envctl/internal/api"
	"envctl/internal/kube"
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateV2 handles messages for the new service architecture
func UpdateV2(msg tea.Msg, m *model.ModelV2) (*model.ModelV2, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		// Update viewport sizes
		m.LogViewport.Width = msg.Width
		m.LogViewport.Height = msg.Height - 10 // Leave room for header/footer
		m.MainLogViewport.Width = msg.Width
		m.MainLogViewport.Height = msg.Height / 3
		return m, nil

	case spinner.TickMsg:
		// Update spinner
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case model.InitializationCompleteMsg:
		// Orchestrator initialization is complete, switch to main dashboard
		m.CurrentAppMode = model.ModeMainDashboard
		// Start periodic refresh ticker
		cmds = append(cmds, tickCmd())
		// Add a small delay to ensure services have started
		cmds = append(cmds, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return refreshServiceDataMsg{}
		}))
		return m, tea.Batch(cmds...)

	case tickMsg:
		// Periodic refresh
		cmds = append(cmds, refreshServiceData(m))
		// Continue ticking
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case refreshServiceDataMsg:
		// Refresh service data after delay
		cmds = append(cmds, refreshServiceData(m))
		return m, tea.Batch(cmds...)

	case api.ServiceStateChangedEvent:
		// Handle service state changes
		cmds = append(cmds, handleServiceStateChange(m, msg))

	case model.ServiceStartedMsg:
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Service %s started", msg.Label),
			model.StatusBarSuccess,
			3*time.Second,
		))
		// Refresh service data
		cmds = append(cmds, refreshServiceData(m))

	case model.ServiceStoppedMsg:
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Service %s stopped", msg.Label),
			model.StatusBarInfo,
			3*time.Second,
		))
		// Refresh service data
		cmds = append(cmds, refreshServiceData(m))

	case model.ServiceRestartedMsg:
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Service %s restarted", msg.Label),
			model.StatusBarSuccess,
			3*time.Second,
		))
		// Refresh service data
		cmds = append(cmds, refreshServiceData(m))

	case model.ServiceErrorMsg:
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Error with service %s: %v", msg.Label, msg.Err),
			model.StatusBarError,
			5*time.Second,
		))

	case model.ClearStatusBarMsg:
		m.StatusBarMessage = ""
		m.StatusBarMessageType = model.StatusBarInfo

	case model.NewLogEntryMsg:
		// Format and add log entry to activity log
		logLine := fmt.Sprintf("[%s] [%s] %s: %s",
			msg.Entry.Timestamp.Format("15:04:05"),
			msg.Entry.Level.String(),
			msg.Entry.Subsystem,
			msg.Entry.Message,
		)
		m.ActivityLog = append(m.ActivityLog, logLine)
		m.ActivityLogDirty = true

		// Limit log size
		if len(m.ActivityLog) > model.MaxActivityLogLines {
			m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-model.MaxActivityLogLines:]
		}

		// Re-queue the log listener
		return m, m.ListenForLogs()

	case tea.KeyMsg:
		cmds = append(cmds, handleKeyPressV2(m, msg))

	case model.KubeContextSwitchedMsg:
		// Handle context switch result
		if msg.Err != nil {
			cmds = append(cmds, m.SetStatusMessage(
				fmt.Sprintf("Failed to switch context: %v", msg.Err),
				model.StatusBarError,
				5*time.Second,
			))
		} else {
			m.CurrentKubeContext = msg.TargetContext
			cmds = append(cmds, m.SetStatusMessage(
				fmt.Sprintf("Switched to context: %s", msg.TargetContext),
				model.StatusBarSuccess,
				3*time.Second,
			))
		}
	}

	// Re-queue listeners for continuous operation
	if _, ok := msg.(api.ServiceStateChangedEvent); ok {
		cmds = append(cmds, m.ListenForStateChanges())
	}

	// Re-queue channel reader
	if msg != nil {
		cmds = append(cmds, model.ChannelReaderCmd(m.TUIChannel))
	}

	return m, tea.Batch(cmds...)
}

// handleServiceStateChange processes service state change events
func handleServiceStateChange(m *model.ModelV2, event api.ServiceStateChangedEvent) tea.Cmd {
	// Log the state change
	logMsg := fmt.Sprintf("[%s] %s: %s → %s",
		time.Now().Format("15:04:05"),
		event.Label,
		event.OldState,
		event.NewState,
	)

	if event.Error != nil {
		logMsg += fmt.Sprintf(" (error: %v)", event.Error)
	}

	m.ActivityLog = append(m.ActivityLog, logMsg)
	m.ActivityLogDirty = true

	// Limit activity log size
	if len(m.ActivityLog) > model.MaxActivityLogLines {
		m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-model.MaxActivityLogLines:]
	}

	// Refresh service data to get latest state
	return refreshServiceData(m)
}

// refreshServiceData returns a command to refresh all service data
func refreshServiceData(m *model.ModelV2) tea.Cmd {
	return func() tea.Msg {
		if err := m.RefreshServiceData(); err != nil {
			return model.ServiceErrorMsg{
				Label: "refresh",
				Err:   err,
			}
		}
		return serviceDataRefreshedMsg{}
	}
}

// handleKeyPressV2 handles keyboard input for the new model
func handleKeyPressV2(m *model.ModelV2, key tea.KeyMsg) tea.Cmd {
	// Handle overlay-specific keys first
	switch m.CurrentAppMode {
	case model.ModeHelpOverlay:
		switch key.String() {
		case "esc", "?", "h", "q":
			m.CurrentAppMode = m.LastAppMode
		}
		return nil

	case model.ModeLogOverlay:
		switch key.String() {
		case "L", "esc", "q":
			m.CurrentAppMode = m.LastAppMode
		case "y":
			// Copy logs to clipboard
			if err := clipboard.WriteAll(strings.Join(m.ActivityLog, "\n")); err != nil {
				return m.SetStatusMessage("Copy logs failed", model.StatusBarError, 3*time.Second)
			}
			return m.SetStatusMessage("Logs copied to clipboard", model.StatusBarSuccess, 3*time.Second)
		default:
			// Pass other keys to viewport for scrolling
			var vpCmd tea.Cmd
			m.LogViewport, vpCmd = m.LogViewport.Update(key)
			return vpCmd
		}
		return nil

	case model.ModeMcpConfigOverlay:
		switch key.String() {
		case "C", "esc", "q":
			m.CurrentAppMode = m.LastAppMode
		case "y":
			// Copy MCP config to clipboard
			configStr := GenerateMcpConfigJsonV2(m.MCPServerConfig, m.MCPServers)
			if err := clipboard.WriteAll(configStr); err != nil {
				return m.SetStatusMessage("Copy MCP config failed", model.StatusBarError, 3*time.Second)
			}
			return m.SetStatusMessage("MCP config copied", model.StatusBarSuccess, 3*time.Second)
		default:
			// Pass other keys to viewport for scrolling
			var vpCmd tea.Cmd
			m.McpConfigViewport, vpCmd = m.McpConfigViewport.Update(key)
			return vpCmd
		}
		return nil

	case model.ModeMcpToolsOverlay:
		switch key.String() {
		case "M", "esc", "q":
			m.CurrentAppMode = m.LastAppMode
		default:
			// Pass other keys to viewport for scrolling
			var vpCmd tea.Cmd
			m.McpToolsViewport, vpCmd = m.McpToolsViewport.Update(key)
			return vpCmd
		}
		return nil

	case model.ModeMainDashboard:
		return handleMainDashboardKeysV2(m, key)
	}

	return nil
}

// handleMainDashboardKeysV2 handles keys in the main dashboard
func handleMainDashboardKeysV2(m *model.ModelV2, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "q", "ctrl+c":
		m.QuitApp = true
		m.CurrentAppMode = model.ModeQuitting
		m.QuittingMessage = "Shutting down services..."
		// Stop the orchestrator to clean up all services
		if m.Orchestrator != nil {
			go func() {
				m.Orchestrator.Stop()
			}()
		}
		// Give services a moment to stop gracefully
		return tea.Sequence(
			tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return tea.Quit()
			}),
		)

	case "?", "h":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeHelpOverlay

	case "L":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeLogOverlay
		// Prepare log content for viewport
		if m.LogViewport.Width > 0 {
			preparedContent := PrepareLogContentV2(m.ActivityLog, m.LogViewport.Width)
			m.LogViewport.SetContent(preparedContent)
			m.LogViewport.GotoBottom()
		}

	case "C":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeMcpConfigOverlay
		// Populate the viewport content when entering the mode
		configJSON := GenerateMcpConfigJsonV2(m.MCPServerConfig, m.MCPServers)
		m.McpConfigViewport.SetContent(configJSON)
		m.McpConfigViewport.GotoTop()

	case "M":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeMcpToolsOverlay
		// Generate and set tools content
		toolsContent := view.GenerateMcpToolsContentV2(m)
		m.McpToolsViewport.SetContent(toolsContent)
		m.McpToolsViewport.GotoTop()

	case "D":
		// Toggle dark mode
		currentIsDark := lipgloss.HasDarkBackground()
		lipgloss.SetHasDarkBackground(!currentIsDark)
		colorProfile := lipgloss.ColorProfile().String()
		m.ColorMode = fmt.Sprintf("%s (Dark: %v)", colorProfile, !currentIsDark)

	case "z":
		// Toggle debug mode
		m.DebugMode = !m.DebugMode

	case "r":
		// Restart focused service
		if m.FocusedPanelKey != "" {
			return m.RestartService(m.FocusedPanelKey)
		}

	case "x":
		// Stop focused service
		if m.FocusedPanelKey != "" {
			return m.StopService(m.FocusedPanelKey)
		}

	case "enter":
		// Start focused service if stopped
		if m.FocusedPanelKey != "" {
			// Check if service is stopped
			if svc, exists := m.MCPServers[m.FocusedPanelKey]; exists && svc.State != "running" {
				return m.StartService(m.FocusedPanelKey)
			}
			if pf, exists := m.PortForwards[m.FocusedPanelKey]; exists && pf.State != "running" {
				return m.StartService(m.FocusedPanelKey)
			}
		}

	case "s":
		// Context switch for K8s connections
		if m.FocusedPanelKey == model.McPaneFocusKey && m.ManagementClusterName != "" {
			// Switch to MC context
			kubeMgr := kube.NewManager(nil)
			target := kubeMgr.BuildMcContextName(m.ManagementClusterName)
			return PerformSwitchKubeContextCmd(target)
		} else if m.FocusedPanelKey == model.WcPaneFocusKey && m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
			// Switch to WC context
			kubeMgr := kube.NewManager(nil)
			target := kubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName)
			return PerformSwitchKubeContextCmd(target)
		}

	case "tab":
		// Cycle through focusable panels
		cycleFocus(m, 1)

	case "shift+tab":
		// Cycle backwards through focusable panels
		cycleFocus(m, -1)

	case "k", "up":
		// Move focus up
		cycleFocus(m, -1)

	case "j", "down":
		// Move focus down
		cycleFocus(m, 1)
	}

	return nil
}

// cycleFocus moves focus between panels
func cycleFocus(m *model.ModelV2, direction int) {
	// Build list of focusable items
	var focusableItems []string

	// Add MC pane
	if m.ManagementClusterName != "" {
		focusableItems = append(focusableItems, model.McPaneFocusKey)
	}

	// Add WC pane
	if m.WorkloadClusterName != "" {
		focusableItems = append(focusableItems, model.WcPaneFocusKey)
	}

	// Add port forwards
	focusableItems = append(focusableItems, m.PortForwardOrder...)

	// Add MCP servers
	focusableItems = append(focusableItems, m.MCPServerOrder...)

	if len(focusableItems) == 0 {
		return
	}

	// Find current index
	currentIdx := -1
	for i, item := range focusableItems {
		if item == m.FocusedPanelKey {
			currentIdx = i
			break
		}
	}

	// Calculate next index
	nextIdx := currentIdx + direction
	if nextIdx < 0 {
		nextIdx = len(focusableItems) - 1
	} else if nextIdx >= len(focusableItems) {
		nextIdx = 0
	}

	m.FocusedPanelKey = focusableItems[nextIdx]
}

// serviceDataRefreshedMsg indicates service data has been refreshed
type serviceDataRefreshedMsg struct{}

// refreshServiceDataMsg indicates we should refresh service data
type refreshServiceDataMsg struct{}

// tickMsg is sent periodically to refresh service data
type tickMsg struct{}

// tickCmd returns a command that sends a tick message after a delay
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}
