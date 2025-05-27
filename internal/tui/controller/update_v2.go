package controller

import (
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	case tea.KeyMsg:
		cmds = append(cmds, handleKeyPressV2(m, msg))
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
	switch m.CurrentAppMode {
	case model.ModeMainDashboard:
		return handleMainDashboardKeysV2(m, key)
	case model.ModeHelpOverlay:
		if key.String() == "esc" || key.String() == "?" {
			m.CurrentAppMode = m.LastAppMode
		}
	case model.ModeLogOverlay:
		if key.String() == "esc" || key.String() == "l" {
			m.CurrentAppMode = m.LastAppMode
		}
	}

	return nil
}

// handleMainDashboardKeysV2 handles keys in the main dashboard
func handleMainDashboardKeysV2(m *model.ModelV2, key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "q", "ctrl+c":
		m.QuitApp = true
		return tea.Quit

	case "?":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeHelpOverlay

	case "l":
		m.LastAppMode = m.CurrentAppMode
		m.CurrentAppMode = model.ModeLogOverlay

	case "r":
		// Restart focused service
		if m.FocusedPanelKey != "" {
			return m.RestartService(m.FocusedPanelKey)
		}

	case "s":
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

	case "tab":
		// Cycle through focusable panels
		cycleFocus(m, 1)

	case "shift+tab":
		// Cycle backwards through focusable panels
		cycleFocus(m, -1)
	}

	return nil
}

// cycleFocus moves focus between panels
func cycleFocus(m *model.ModelV2, direction int) {
	// Build list of focusable items
	var focusableItems []string

	// Add K8s connections
	focusableItems = append(focusableItems, m.K8sConnectionOrder...)

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
