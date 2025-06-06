package controller

import (
	"context"
	"envctl/internal/agent"
	"envctl/internal/api"
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Update handles messages for the new service architecture
func Update(msg tea.Msg, m *model.Model) (*model.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle different message types
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		// Update viewport sizes
		m.LogViewport.Width = msg.Width
		m.LogViewport.Height = msg.Height - 10 // Leave room for header/footer
		m.MainLogViewport.Width = msg.Width
		m.MainLogViewport.Height = msg.Height / 3

		// If we're in main dashboard and haven't started the ticker yet, start it
		if m.CurrentAppMode == model.ModeMainDashboard && !m.PeriodicTickerStarted {
			m.PeriodicTickerStarted = true
			// Immediate data refresh
			cmds = append(cmds, refreshServiceData(m))
			// Start periodic refresh ticker
			cmds = append(cmds, tickCmd())
		}

		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		// Update spinner
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Periodic refresh
		cmds = append(cmds, refreshServiceData(m))
		// Continue ticking
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case refreshServiceDataMsg:
		// Refresh service data after delay
		logging.Debug("TUI", "Refreshing service data")
		cmds = append(cmds, refreshServiceData(m))
		return m, tea.Batch(cmds...)

	case serviceDataRefreshedMsg:
		// Service data has been refreshed, update list items
		updateListItems(m)
		return m, nil

	case api.ServiceStateChangedEvent:
		// Handle service state changes
		logging.Debug("TUI", "Service state changed: %s", msg.Label)
		cmds = append(cmds, handleServiceStateChange(m, msg))

	case model.ServiceStartedMsg:
		logging.Debug("TUI", "Service started: %s", msg.Label)
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Service %s started", msg.Label),
			model.StatusBarSuccess,
			3*time.Second,
		))
		// Refresh service data
		cmds = append(cmds, refreshServiceData(m))

	case model.ServiceStoppedMsg:
		logging.Debug("TUI", "Service stopped: %s", msg.Label)
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Service %s stopped", msg.Label),
			model.StatusBarInfo,
			3*time.Second,
		))
		// Refresh service data
		cmds = append(cmds, refreshServiceData(m))

	case model.ServiceRestartedMsg:
		logging.Debug("TUI", "Service restarted: %s", msg.Label)
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Service %s restarted", msg.Label),
			model.StatusBarSuccess,
			3*time.Second,
		))
		// Refresh service data
		cmds = append(cmds, refreshServiceData(m))

	case model.ServiceErrorMsg:
		logging.Debug("TUI", "Service error: %s", msg.Label)
		cmds = append(cmds, m.SetStatusMessage(
			fmt.Sprintf("Error with service %s: %v", msg.Label, msg.Err),
			model.StatusBarError,
			5*time.Second,
		))

	case model.ClearStatusBarMsg:
		m.StatusBarMessage = ""
		m.StatusBarMessageType = model.StatusBarInfo

	case model.NewLogEntryMsg:
		// Filter logs based on debug mode
		if !m.DebugMode && msg.Entry.Level == logging.LevelDebug {
			// Skip debug logs when not in debug mode
			return m, m.ListenForLogs()
		}

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

	case model.MCPToolsLoadedMsg:
		logging.Debug("TUI", "MCP tools loaded: %s", msg.ServerName)
		// Store the loaded tools
		m.MCPTools[msg.ServerName] = msg.Tools

		// Update the viewport content with all loaded tools
		// Make sure viewport dimensions are set first
		if m.McpToolsViewport.Width == 0 {
			// Use a reasonable default width if viewport hasn't been sized yet
			m.McpToolsViewport.Width = 80
		}
		toolsContent := view.GenerateMcpToolsContent(m)
		m.McpToolsViewport.SetContent(toolsContent)

		return m, nil

	case model.MCPToolsErrorMsg:
		logging.Debug("TUI", "MCP tools error: %s", msg.ServerName)
		// Log the error
		logLine := fmt.Sprintf("[%s] [ERROR] Failed to fetch tools for %s: %v",
			time.Now().Format("15:04:05"),
			msg.ServerName,
			msg.Error,
		)
		m.ActivityLog = append(m.ActivityLog, logLine)
		m.ActivityLogDirty = true

		// Store empty tools list for this server
		m.MCPTools[msg.ServerName] = []api.MCPTool{}

		// Update the viewport content
		// Make sure viewport dimensions are set first
		if m.McpToolsViewport.Width == 0 {
			// Use a reasonable default width if viewport hasn't been sized yet
			m.McpToolsViewport.Width = 80
		}
		toolsContent := view.GenerateMcpToolsContent(m)
		m.McpToolsViewport.SetContent(toolsContent)

		return m, nil

	case tea.KeyMsg:
		cmds = append(cmds, handleKeyPress(m, msg))

	case model.KubeContextSwitchedMsg:
		logging.Debug("TUI", "Kube context switched: %s", msg.TargetContext)
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

	case model.AgentCommandResultMsg:
		// Handle agent command result
		if msg.Error != nil {
			m.AgentREPLOutput = append(m.AgentREPLOutput, fmt.Sprintf("Error: %v", msg.Error))
		} else {
			// Split output by lines and add each
			lines := strings.Split(msg.Output, "\n")
			for _, line := range lines {
				if line != "" {
					m.AgentREPLOutput = append(m.AgentREPLOutput, line)
				}
			}
		}

		// Update viewport
		if m.CurrentAppMode == model.ModeAgentREPLOverlay {
			content := view.PrepareAgentREPLContent(m.AgentREPLOutput, m.AgentREPLViewport.Width)
			m.AgentREPLViewport.SetContent(content)
			m.AgentREPLViewport.GotoBottom()
		}

		return m, nil
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
func handleServiceStateChange(m *model.Model, event api.ServiceStateChangedEvent) tea.Cmd {
	// Log the state change to activity log
	var logMsg string
	if event.ServiceType != "" {
		logMsg = fmt.Sprintf("[%s] %s (%s): %s → %s",
			time.Now().Format("15:04:05"),
			event.Label,
			event.ServiceType,
			event.OldState,
			event.NewState,
		)
	} else {
		// Fallback for backward compatibility
		logMsg = fmt.Sprintf("[%s] %s: %s → %s",
			time.Now().Format("15:04:05"),
			event.Label,
			event.OldState,
			event.NewState,
		)
	}

	if event.Error != nil {
		logMsg += fmt.Sprintf(" (error: %v)", event.Error)
	}

	m.ActivityLog = append(m.ActivityLog, logMsg)
	m.ActivityLogDirty = true

	// Limit activity log size
	if len(m.ActivityLog) > model.MaxActivityLogLines {
		m.ActivityLog = m.ActivityLog[len(m.ActivityLog)-model.MaxActivityLogLines:]
	}

	logging.Debug("TUI", "Service state changed: %v", event)

	// Refresh service data to get latest state
	return refreshServiceData(m)
}

// refreshServiceData returns a command to refresh all service data
func refreshServiceData(m *model.Model) tea.Cmd {
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

// handleKeyPress handles keyboard input for the new model
func handleKeyPress(m *model.Model, key tea.KeyMsg) tea.Cmd {
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
			configStr := GenerateMcpConfigJson(m.MCPServerConfig, m.MCPServers, m.AggregatorConfig.Port)
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
			// Unfocus the list
			if m.MCPToolsList != nil {
				listModel := m.MCPToolsList.(*view.ServiceListModel)
				listModel.SetFocused(false)
			}
		default:
			// Pass other keys to the list for navigation
			if m.MCPToolsList != nil {
				listModel := m.MCPToolsList.(*view.ServiceListModel)
				_, cmd := listModel.Update(key)
				return cmd
			}
		}
		return nil

	case model.ModeAgentREPLOverlay:
		return handleAgentREPLKeys(m, key)

	case model.ModeMainDashboard:
		return handleMainDashboardKeys(m, key)
	}

	return nil
}

// handleMainDashboardKeys handles keys in the main dashboard
func handleMainDashboardKeys(m *model.Model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "q", "ctrl+c":
		m.QuitApp = true
		m.CurrentAppMode = model.ModeQuitting
		m.QuittingMessage = "Shutting down services..."

		// Clean up agent client if connected
		if m.AgentClient != nil {
			if adapter, ok := m.AgentClient.(*agent.REPLAdapter); ok {
				adapter.Close()
			}
			m.AgentClient = nil
		}

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
			preparedContent := PrepareLogContent(m.ActivityLog, m.LogViewport.Width)
			m.LogViewport.SetContent(preparedContent)
			m.LogViewport.GotoBottom()
		}

	case "C":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeMcpConfigOverlay
		// Populate the viewport content when entering the mode
		configJSON := GenerateMcpConfigJson(m.MCPServerConfig, m.MCPServers, m.AggregatorConfig.Port)
		m.McpConfigViewport.SetContent(configJSON)
		m.McpConfigViewport.GotoTop()

	case "M":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeMcpToolsOverlay

		// Refresh the tools with status data
		return refreshServiceData(m)

	case "A":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeAgentREPLOverlay
		// Initialize agent REPL if needed
		if m.AgentREPLOutput == nil {
			m.AgentREPLOutput = []string{}
		}
		// Set initial content
		content := view.PrepareAgentREPLContent(m.AgentREPLOutput, m.AgentREPLViewport.Width)
		m.AgentREPLViewport.SetContent(content)
		m.AgentREPLViewport.GotoBottom()
		// Focus the input
		m.AgentREPLInput.Focus()
		m.AgentREPLInput.Reset()
		return nil

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
		serviceLabel := getSelectedServiceLabel(m)
		if serviceLabel != "" {
			return m.RestartService(serviceLabel)
		}

	case "x":
		// Stop focused service
		serviceLabel := getSelectedServiceLabel(m)
		if serviceLabel != "" {
			return m.StopService(serviceLabel)
		}

	case "enter":
		// Start focused service if stopped or handle list selection
		if isListPanel(m.FocusedPanelKey) {
			// For list panels, enter can be used to start stopped services
			serviceLabel := getSelectedServiceLabel(m)
			if serviceLabel != "" {
				// Check if service is stopped
				if svc, exists := m.MCPServers[serviceLabel]; exists && svc.State != "running" {
					return m.StartService(serviceLabel)
				}
				if pf, exists := m.PortForwards[serviceLabel]; exists && pf.State != "running" {
					return m.StartService(serviceLabel)
				}
			}
		}

	case "s":
		// Context switch for K8s connections
		if m.FocusedPanelKey == "clusters" {
			// Get selected cluster from list
			if m.ClustersList != nil {
				listModel := m.ClustersList.(*view.ServiceListModel)
				if item := listModel.GetSelectedItem(); item != nil {
					if clusterItem, ok := item.(view.ClusterListItem); ok {
						// Get the K8s connection for context switch
						if conn, exists := m.K8sConnections[clusterItem.GetID()]; exists {
							return PerformSwitchKubeContextCmd(conn.Context)
						}
					}
				}
			}
		}

	case "tab":
		// Cycle through focusable panels
		cycleFocus(m, 1)

	case "shift+tab":
		// Cycle backwards through focusable panels
		cycleFocus(m, -1)

	case "k", "up":
		// Pass to list if focused panel is a list
		if isListPanel(m.FocusedPanelKey) {
			return handleListNavigation(m, key)
		}
		// Otherwise move focus up
		cycleFocus(m, -1)

	case "j", "down":
		// Pass to list if focused panel is a list
		if isListPanel(m.FocusedPanelKey) {
			return handleListNavigation(m, key)
		}
		// Otherwise move focus down
		cycleFocus(m, 1)
	}

	return nil
}

// handleAgentREPLKeys handles keyboard input in the agent REPL overlay
func handleAgentREPLKeys(m *model.Model, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc", "A":
		// Close the overlay
		m.CurrentAppMode = m.LastAppMode
		// Unfocus the input
		m.AgentREPLInput.Blur()
		return nil

	case "tab":
		// Handle tab completion
		currentInput := m.AgentREPLInput.Value()

		// Get completions from agent client if available
		var completions []string
		if m.AgentClient != nil {
			if executor, ok := m.AgentClient.(agent.CommandExecutor); ok {
				completions = executor.GetCompletions(currentInput)
			}
		}

		if len(completions) == 1 {
			// Single completion - use it
			m.AgentREPLInput.SetValue(completions[0])
		} else if len(completions) > 1 {
			// Multiple completions - show them
			m.AgentREPLOutput = append(m.AgentREPLOutput, fmt.Sprintf("MCP> %s", currentInput))
			m.AgentREPLOutput = append(m.AgentREPLOutput, "Available completions:")
			for _, comp := range completions {
				m.AgentREPLOutput = append(m.AgentREPLOutput, fmt.Sprintf("  %s", comp))
			}

			// Find common prefix
			if len(completions) > 0 {
				commonPrefix := completions[0]
				for _, comp := range completions[1:] {
					// Find common prefix
					for i := 0; i < len(commonPrefix) && i < len(comp); i++ {
						if commonPrefix[i] != comp[i] {
							commonPrefix = commonPrefix[:i]
							break
						}
					}
				}
				if len(commonPrefix) > len(currentInput) {
					m.AgentREPLInput.SetValue(commonPrefix)
				}
			}

			// Update viewport
			content := view.PrepareAgentREPLContent(m.AgentREPLOutput, m.AgentREPLViewport.Width)
			m.AgentREPLViewport.SetContent(content)
			m.AgentREPLViewport.GotoBottom()
		}
		return nil

	case "enter":
		// Execute the command
		cmd := m.AgentREPLInput.Value()
		if cmd != "" {
			// Add command to history
			m.AgentREPLHistory = append(m.AgentREPLHistory, cmd)
			m.AgentREPLHistoryIndex = len(m.AgentREPLHistory)

			// Add command to output
			m.AgentREPLOutput = append(m.AgentREPLOutput, fmt.Sprintf("MCP> %s", cmd))

			// Clear input
			m.AgentREPLInput.SetValue("")

			// Update viewport immediately to show the command
			content := view.PrepareAgentREPLContent(m.AgentREPLOutput, m.AgentREPLViewport.Width)
			m.AgentREPLViewport.SetContent(content)
			m.AgentREPLViewport.GotoBottom()

			// Execute command asynchronously
			return executeAgentCommand(m, cmd)
		}
		return nil

	case "up":
		// Navigate history up
		if m.AgentREPLHistoryIndex > 0 && len(m.AgentREPLHistory) > 0 {
			m.AgentREPLHistoryIndex--
			if m.AgentREPLHistoryIndex < len(m.AgentREPLHistory) {
				m.AgentREPLInput.SetValue(m.AgentREPLHistory[m.AgentREPLHistoryIndex])
			}
		}
		return nil

	case "down":
		// Navigate history down
		if m.AgentREPLHistoryIndex < len(m.AgentREPLHistory)-1 {
			m.AgentREPLHistoryIndex++
			m.AgentREPLInput.SetValue(m.AgentREPLHistory[m.AgentREPLHistoryIndex])
		} else if m.AgentREPLHistoryIndex == len(m.AgentREPLHistory)-1 {
			m.AgentREPLHistoryIndex = len(m.AgentREPLHistory)
			m.AgentREPLInput.SetValue("")
		}
		return nil

	case "pgup":
		// Scroll viewport up
		var vpCmd tea.Cmd
		m.AgentREPLViewport, vpCmd = m.AgentREPLViewport.Update(key)
		return vpCmd

	case "pgdn":
		// Scroll viewport down
		var vpCmd tea.Cmd
		m.AgentREPLViewport, vpCmd = m.AgentREPLViewport.Update(key)
		return vpCmd

	default:
		// Pass to text input for typing
		var inputCmd tea.Cmd
		m.AgentREPLInput, inputCmd = m.AgentREPLInput.Update(key)
		return inputCmd
	}
}

// isListPanel checks if the focused panel is a list panel
func isListPanel(focusedKey string) bool {
	return focusedKey == "clusters" || focusedKey == "mcpservers"
}

// getSelectedServiceLabel returns the label of the currently selected service in a list
func getSelectedServiceLabel(m *model.Model) string {
	switch m.FocusedPanelKey {
	case "clusters":
		if m.ClustersList != nil {
			listModel := m.ClustersList.(*view.ServiceListModel)
			if item := listModel.GetSelectedItem(); item != nil {
				return item.GetID()
			}
		}
	case "mcpservers":
		if m.MCPServersList != nil {
			listModel := m.MCPServersList.(*view.ServiceListModel)
			if item := listModel.GetSelectedItem(); item != nil {
				return item.GetID()
			}
		}
	}
	return ""
}

// updateListItems updates the items in all list models with fresh data
func updateListItems(m *model.Model) {
	// Update clusters list
	if m.ClustersList != nil {
		listModel := m.ClustersList.(*view.ServiceListModel)
		items := []list.Item{}
		for _, label := range m.K8sConnectionOrder {
			if conn, exists := m.K8sConnections[label]; exists {
				items = append(items, view.ConvertK8sConnectionToListItem(conn))
			}
		}
		listModel.List.SetItems(items)
	}

	// Update MCP servers list
	if m.MCPServersList != nil {
		// Temporarily use simple list
		listModel := m.MCPServersList.(*view.ServiceListModel)
		items := []list.Item{}
		for _, config := range m.MCPServerConfig {
			if mcp, exists := m.MCPServers[config.Name]; exists {
				items = append(items, view.ConvertMCPServerToListItem(mcp))
			} else {
				// Create placeholder item for configured but not running MCP server
				items = append(items, view.MCPServerListItem{
					BaseListItem: view.BaseListItem{
						ID:          config.Name,
						Name:        config.Name,
						Status:      view.StatusStopped,
						Health:      view.HealthUnknown,
						Icon:        design.SafeIcon(config.Icon),
						Description: "Not Started",
						Details:     fmt.Sprintf("MCP Server: %s (Not Started)", config.Name),
					},
				})
			}
		}
		listModel.List.SetItems(items)
	}
}

// handleListNavigation handles navigation within list panels
func handleListNavigation(m *model.Model, key tea.KeyMsg) tea.Cmd {
	var listModel *view.ServiceListModel
	switch m.FocusedPanelKey {
	case "clusters":
		if m.ClustersList != nil {
			listModel = m.ClustersList.(*view.ServiceListModel)
		}
	case "mcpservers":
		if m.MCPServersList != nil {
			listModel = m.MCPServersList.(*view.ServiceListModel)
		}
	}

	if listModel != nil {
		_, cmd := listModel.Update(key)
		return cmd
	}
	return nil
}

// cycleFocus moves focus between panels
func cycleFocus(m *model.Model, direction int) {
	// Build list of focusable items in the new layout
	var focusableItems []string

	// Aggregator is always present (primary component)
	focusableItems = append(focusableItems, "mcp-aggregator")

	// Bottom row: Clusters and MCP Servers
	if len(m.K8sConnections) > 0 {
		focusableItems = append(focusableItems, "clusters")
	}
	if len(m.MCPServerConfig) > 0 {
		focusableItems = append(focusableItems, "mcpservers")
	}

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

	// If no current focus, start with first item
	if currentIdx == -1 {
		m.FocusedPanelKey = focusableItems[0]
		return
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

// fetchMCPTools returns a command to fetch tools for an MCP server
func fetchMCPTools(m *model.Model, serverName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		tools, err := m.MCPServiceAPI.GetTools(ctx, serverName)
		if err != nil {
			return model.MCPToolsErrorMsg{
				ServerName: serverName,
				Error:      err,
			}
		}
		return model.MCPToolsLoadedMsg{
			ServerName: serverName,
			Tools:      tools,
		}
	}
}

// executeAgentCommand executes a command asynchronously and handles the result
func executeAgentCommand(m *model.Model, cmd string) tea.Cmd {
	return func() tea.Msg {
		// Check if we have an agent client
		if m.AgentClient == nil {
			// Try to initialize the agent client if we have aggregator info
			if m.AggregatorInfo != nil && m.AggregatorInfo.Port > 0 {
				// Create a logger that writes to the TUI viewport
				writer := NewTUIAgentWriter(m)
				logger := agent.NewLoggerWithWriter(false, false, false, writer)

				// Create adapter with aggregator endpoint
				endpoint := fmt.Sprintf("http://localhost:%d/sse", m.AggregatorInfo.Port)
				adapter, err := agent.NewREPLAdapter(endpoint, logger)
				if err != nil {
					return model.AgentCommandResultMsg{
						Command: cmd,
						Error:   fmt.Errorf("failed to create agent adapter: %w", err),
					}
				}

				// Connect to the aggregator
				ctx := context.Background()
				if err := adapter.Connect(ctx); err != nil {
					return model.AgentCommandResultMsg{
						Command: cmd,
						Error:   fmt.Errorf("failed to connect to aggregator: %w", err),
					}
				}

				// Store the adapter
				m.AgentClient = adapter

				// Return success message
				return model.AgentCommandResultMsg{
					Command: cmd,
					Output:  "Connected to MCP aggregator successfully!",
					Error:   nil,
				}
			} else {
				return model.AgentCommandResultMsg{
					Command: cmd,
					Error:   fmt.Errorf("aggregator not running - start MCP servers first"),
				}
			}
		}

		// Cast to CommandExecutor interface
		var executor agent.CommandExecutor
		if adapter, ok := m.AgentClient.(*agent.REPLAdapter); ok {
			executor = adapter
		} else if simpleExec, ok := m.AgentClient.(agent.CommandExecutor); ok {
			executor = simpleExec
		} else {
			// Fallback to simple executor
			executor = agent.NewSimpleCommandExecutor()
		}

		ctx := context.Background()
		output, err := executor.Execute(ctx, cmd)

		return model.AgentCommandResultMsg{
			Command: cmd,
			Output:  output,
			Error:   err,
		}
	}
}
