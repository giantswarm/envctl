package controller

import (
	"envctl/internal/tui/model"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	// May be needed for some view-related constants if not moved
)

// mainControllerDispatch is the new home for the primary message handling switch.
// It takes the current model and the message, calls appropriate controller handlers,
// and returns the (potentially updated) model and any commands.
func mainControllerDispatch(m *model.Model, msg tea.Msg) (*model.Model, tea.Cmd) {
	// recordMsgSample(msg) // This was in model.Update, model can do it or controller if msg is passed through
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Logging for received messages (from original model.Update)
	switch msg.(type) {
	case spinner.TickMsg, tea.MouseMsg:
		// No log for these frequent messages
	default:
		if m.DebugMode {
			LogDebug(m, "[Controller Dispatch] Received msg: %T -- Value: %v", msg, msg)
		}
	}

	// Global quit shortcuts (from original model.Update)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.CurrentAppMode = model.ModeQuitting
			m.QuittingMessage = "Shutting down..."
			for _, pf := range m.PortForwards {
				if pf.StopChan != nil {
					close(pf.StopChan)
					pf.StopChan = nil
					pf.StatusMsg = "Stopping..."
				}
			}
			if m.McpServers != nil {
				for name, proc := range m.McpServers {
					if proc.Active && proc.StopChan != nil {
						LogInfo(m, "[%s MCP Proxy] Sending stop signal...", name)
						close(proc.StopChan)
						proc.StopChan = nil
						proc.StatusMsg = "Stopping..."
						proc.Active = false
					}
				}
			}
			model.FinalizeMsgSampling() // Call model.FinalizeMsgSampling
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		case "ctrl+c":
			model.FinalizeMsgSampling() // Call model.FinalizeMsgSampling
			return m, tea.Quit
		}
	}

	// Mode specific handling & message processing (main switch from model.Update)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.CurrentAppMode == model.ModeNewConnectionInput && m.NewConnectionInput.Focused() {
			return handleKeyMsgInputMode(m, msg) // Calls controller handler
		} else {
			// This part of key handling (overlays, debug) might also be in handleKeyMsgGlobal
			// or needs to be carefully merged.
			// For now, let's assume handleKeyMsgGlobal covers non-input mode keys.
			return handleKeyMsgGlobal(m, msg, cmds) // Calls controller handler
		}

	case tea.WindowSizeMsg:
		return handleWindowSizeMsg(m, msg) // Calls controller handler

	case model.PortForwardSetupResultMsg:
		return handlePortForwardSetupResultMsg(m, msg) // Calls controller handler
	case model.PortForwardCoreUpdateMsg:
		return handlePortForwardCoreUpdateMsg(m, msg) // Calls controller handler

	case model.SubmitNewConnectionMsg:
		return handleSubmitNewConnectionMsg(m, msg, cmds) // Calls controller handler
	case model.KubeLoginResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched KubeLoginResultMsg. Routing to handler...")
		return handleKubeLoginResultMsg(m, msg, cmds) // Calls controller handler
	case model.ContextSwitchAndReinitializeResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched ContextSwitchAndReinitializeResultMsg. Routing to handler...")
		return handleContextSwitchAndReinitializeResultMsg(m, msg, cmds) // Calls controller handler

	case model.KubeContextResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched KubeContextResultMsg. Routing to handler...")
		return handleKubeContextResultMsg(m, msg) // Calls controller handler
	case model.RequestClusterHealthUpdate:
		return handleRequestClusterHealthUpdate(m) // Calls controller handler
	case model.KubeContextSwitchedMsg:
		LogDebug(m, "[Controller Dispatch] Matched KubeContextSwitchedMsg. Routing to handler...")
		return handleKubeContextSwitchedMsg(m, msg) // Calls controller handler
	case model.NodeStatusMsg:
		LogDebug(m, "[Controller Dispatch] Matched NodeStatusMsg. Routing to handler...")
		return handleNodeStatusMsg(m, msg) // Calls controller handler
	case model.ClusterListResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched ClusterListResultMsg. Routing to handler...")
		m = handleClusterListResultMsg(m, msg)
		return m, tea.Batch(cmds...)

	case model.ClearStatusBarMsg:
		m.StatusBarMessage = ""
		if m.StatusBarClearCancel != nil {
			close(m.StatusBarClearCancel)
			m.StatusBarClearCancel = nil
		}
		// cmds = append(cmds, channelReaderCmd(m.TUIChannel))
		return m, tea.Batch(cmds...)

	case model.McpServerSetupCompletedMsg:
		return handleMcpServerSetupCompletedMsg(m, msg) // Calls controller handler
	case model.McpServerStatusUpdateMsg:
		return handleMcpServerStatusUpdateMsg(m, msg) // Calls controller handler
	case model.RestartMcpServerMsg:
		return handleRestartMcpServerMsg(m, msg) // Calls controller handler

	case tea.MouseMsg:
		if m.CurrentAppMode == model.ModeLogOverlay {
			m.LogViewport, cmd = m.LogViewport.Update(msg)
		} else if m.CurrentAppMode == model.ModeMcpConfigOverlay {
			m.McpConfigViewport, cmd = m.McpConfigViewport.Update(msg)
		} else {
			m.MainLogViewport, cmd = m.MainLogViewport.Update(msg)
		}
		cmds = append(cmds, cmd)

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.Spinner, spinCmd = m.Spinner.Update(msg)
		cmds = append(cmds, spinCmd)

	default:
		LogDebug(m, "[Controller Dispatch] Unhandled msg type in default case: %T -- Value: %v", msg, msg)
		// Logic for unhandled messages (from original model.Update)
		if m.CurrentAppMode == model.ModeNewConnectionInput && m.NewConnectionInput.Focused() {
			m.NewConnectionInput, cmd = m.NewConnectionInput.Update(msg)
		} else if m.CurrentAppMode == model.ModeLogOverlay {
			m.LogViewport, cmd = m.LogViewport.Update(msg)
		} else if m.CurrentAppMode == model.ModeMcpConfigOverlay {
			m.McpConfigViewport, cmd = m.McpConfigViewport.Update(msg)
		} else {
			// Spinner update for truly unhandled, if loading
			// m.Spinner, cmd = m.Spinner.Update(msg) // This was possibly incorrect in original
		}
		cmds = append(cmds, cmd)
	}

	// Viewport refresh logic (from original model.Update)
	// This is view-related logic, but depends on model state (ActivityLogDirty, LogViewportLastWidth)
	// Ideally, view package handles this if it's told to re-render.
	// For now, keep it, but it might need to move or be triggered differently.
	widthChanged := m.LogViewportLastWidth != m.LogViewport.Width
	if m.ActivityLogDirty || widthChanged {
		// prepared := view.PrepareLogContent(m.ActivityLog, m.LogViewport.Width) // This would be view.PrepareLogContent
		// m.LogViewport.SetContent(prepared)
		if m.CurrentAppMode == model.ModeLogOverlay && m.LogViewport.YOffset == 0 {
			// m.LogViewport.GotoBottom() // This returns a tea.Cmd, should be handled
		}
		m.ActivityLogDirty = false
		m.LogViewportLastWidth = m.LogViewport.Width
	}

	return m, tea.Batch(cmds...)
}
