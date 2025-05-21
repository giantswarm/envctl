package controller

import (
	"envctl/internal/managers"
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	// May be needed for some view-related constants if not moved
)

// mainControllerDispatch is the central message routing function for the TUI application.
// It receives all Bubble Tea messages and directs them to the appropriate handler functions
// based on the message type and current application mode.
// It's responsible for updating the model and queuing up any necessary commands.
func mainControllerDispatch(m *model.Model, msg tea.Msg) (*model.Model, tea.Cmd) {
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
			m.QuittingMessage = "Shutting down services..."

			// Signal all services to stop via the ServiceManager
			if m.ServiceManager != nil {
				m.ServiceManager.StopAllServices()
			}

			// Optionally, provide immediate visual feedback if desired, though ServiceManager handles actual stopping.
			// The old logic for direct stopChan closure is now handled by ServiceManager.
			// for _, pf := range m.PortForwards { // This part is now managed by ServiceManager
			// 	if pf.StopChan != nil { ... }
			// }
			// if m.McpServers != nil { // This part is now managed by ServiceManager
			// 	for name, proc := range m.McpServers { ... }
			// }

			model.FinalizeMsgSampling()
			cmds = append(cmds, tea.Quit) // tea.Quit is the primary command to exit Bubble Tea
			// We might want a small delay or a message to confirm shutdown before Quit,
			// but StopAllServices is asynchronous in terms of when goroutines actually end.
			// The WaitGroup in cmd/connect.go handles waiting for actual process termination for CLI mode.
			// For TUI, tea.Quit will terminate the UI loop.
			return m, tea.Batch(cmds...)
		case "ctrl+c":
			// Consider if StopAllServices should also be called here for a cleaner exit,
			// though ctrl+c is often more abrupt.
			if m.ServiceManager != nil {
				m.ServiceManager.StopAllServices()
			}
			model.FinalizeMsgSampling()
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

	case model.SubmitNewConnectionMsg:
		return handleSubmitNewConnectionMsg(m, msg, cmds) // Calls controller handler
	case model.KubeLoginResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched KubeLoginResultMsg. Routing to handler...")
		return handleKubeLoginResultMsg(m, msg, cmds) // Pass msg directly
	case model.ContextSwitchAndReinitializeResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched ContextSwitchAndReinitializeResultMsg. Routing to handler...")
		return handleContextSwitchAndReinitializeResultMsg(m, msg, cmds) // Pass msg directly

	case model.KubeContextResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched KubeContextResultMsg. Routing to handler...")
		return handleKubeContextResultMsg(m, msg) // Pass msg directly
	case model.RequestClusterHealthUpdate: // This one might be a simple type, not a struct with fields
		return handleRequestClusterHealthUpdate(m)
	case model.KubeContextSwitchedMsg:
		LogDebug(m, "[Controller Dispatch] Matched KubeContextSwitchedMsg. Routing to handler...")
		return handleKubeContextSwitchedMsg(m, msg) // Pass msg directly
	case model.NodeStatusMsg:
		LogDebug(m, "[Controller Dispatch] Matched NodeStatusMsg. Routing to handler...")
		return handleNodeStatusMsg(m, msg) // Pass msg directly
	case model.ClusterListResultMsg:
		LogDebug(m, "[Controller Dispatch] Matched ClusterListResultMsg. Routing to handler...")
		m = handleClusterListResultMsg(m, msg) // Pass msg directly
		return m, tea.Batch(cmds...)

	case model.ClearStatusBarMsg: // This one might be a simple type
		m.StatusBarMessage = ""
		if m.StatusBarClearCancel != nil {
			close(m.StatusBarClearCancel)
			m.StatusBarClearCancel = nil
		}
		// cmds = append(cmds, channelReaderCmd(m.TUIChannel))
		return m, tea.Batch(cmds...)

	case model.RestartMcpServerMsg:
		return handleRestartMcpServerMsg(m, msg)

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

	case model.ServiceUpdateMsg:
		return handleServiceUpdateMsg(m, msg)
	case model.AllServicesStartedMsg:
		return handleAllServicesStartedMsg(m, msg)
	case model.ServiceStopResultMsg:
		return handleServiceStopResultMsg(m, msg)

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
		prepared := view.PrepareLogContent(m.ActivityLog, m.LogViewport.Width)
		m.LogViewport.SetContent(prepared)
		if m.CurrentAppMode == model.ModeLogOverlay && m.LogViewport.YOffset == 0 {
			m.LogViewport.GotoBottom()
		}
		m.ActivityLogDirty = false
		m.LogViewportLastWidth = m.LogViewport.Width
	}

	return m, tea.Batch(cmds...)
}

// NEW HANDLER FUNCTIONS

func handleServiceUpdateMsg(m *model.Model, msg model.ServiceUpdateMsg) (*model.Model, tea.Cmd) {
	update := msg.Update
	LogDebug(m, "[Controller] Handling ServiceUpdateMsg for Label: %s, Status: %s, Type: %s", update.Label, update.Status, update.Type)

	var logMsg string
	if update.OutputLog != "" {
		logMsg = fmt.Sprintf("%s: %s", update.Status, update.OutputLog)
	} else {
		logMsg = update.Status
	}

	if update.Type == managers.ServiceTypePortForward {
		pfProcess, exists := m.PortForwards[update.Label]
		if exists {
			pfProcess.StatusMsg = update.Status
			pfProcess.Running = update.IsReady
			if update.IsError {
				pfProcess.Err = update.Error
			}
			if update.OutputLog != "" {
				pfProcess.Log = append(pfProcess.Log, update.OutputLog)
				if len(pfProcess.Log) > model.MaxPanelLogLines { // Keep log size in check
					pfProcess.Log = pfProcess.Log[len(pfProcess.Log)-model.MaxPanelLogLines:]
				}
			}
			LogDebug(m, "[Controller] Updated PortForward '%s': Status=%s, Running=%t", update.Label, pfProcess.StatusMsg, pfProcess.Running)
		} else {
			LogDebug(m, "[Controller] Received ServiceUpdateMsg for unknown PortForward label: %s", update.Label)
			// Optionally, create a new PortForwardProcess entry if it dynamically appears
		}
	} else if update.Type == managers.ServiceTypeMCPServer {
		mcpProcess, exists := m.McpServers[update.Label]
		if exists {
			mcpProcess.StatusMsg = update.Status
			// mcpProcess.Running = update.IsReady // McpServerProcess doesn't have a 'Running' field, Active is for config
			if update.IsError {
				mcpProcess.Err = update.Error
			}
			if update.OutputLog != "" {
				mcpProcess.Output = append(mcpProcess.Output, update.OutputLog)
				// TODO: Limit McpServerProcess output log size similarly to PortForwards
				if len(mcpProcess.Output) > model.MaxPanelLogLines {
					mcpProcess.Output = mcpProcess.Output[len(mcpProcess.Output)-model.MaxPanelLogLines:]
				}
			}
			LogDebug(m, "[Controller] Updated McpServer '%s': Status=%s", update.Label, mcpProcess.StatusMsg)
		} else {
			LogDebug(m, "[Controller] Received ServiceUpdateMsg for unknown McpServer label: %s", update.Label)
		}
	}

	// Add to global activity log
	logEntry := fmt.Sprintf("[%s - %s] %s", update.Type, update.Label, logMsg)
	if update.IsError && update.Error != nil {
		logEntry = fmt.Sprintf("%s (Error: %v)", logEntry, update.Error)
	}
	model.AppendActivityLog(m, logEntry)

	// Potentially update overall app status if a critical service fails
	// if update.IsError && isCriticalService(update.Label) { m.OverallStatus = model.AppStatusDegraded }

	return m, nil // No further command from this simple update, view will re-render
}

func handleAllServicesStartedMsg(m *model.Model, msg model.AllServicesStartedMsg) (*model.Model, tea.Cmd) {
	LogInfo(m, "[Controller] All services processed by ServiceManager. StartServices completed.")
	m.IsLoading = false // Example: Turn off global loading spinner if it was on for service init

	if len(msg.InitialStartupErrors) > 0 {
		model.AppendActivityLog(m, "--- Initial Service Startup Errors ---")
		for _, err := range msg.InitialStartupErrors {
			model.AppendActivityLog(m, fmt.Sprintf("[ERROR] %v", err))
		}
		model.AppendActivityLog(m, "------------------------------------")
		// Optionally set a status bar message or change app mode if critical errors occurred
		// return m, m.SetStatusMessage("Some services failed to start. Check logs.", model.StatusBarError, 10*time.Second)
	}
	return m, nil
}

func handleServiceStopResultMsg(m *model.Model, msg model.ServiceStopResultMsg) (*model.Model, tea.Cmd) {
	if msg.Err != nil {
		logEntry := fmt.Sprintf("[Controller] Failed to stop service '%s': %v", msg.Label, msg.Err)
		model.AppendActivityLog(m, logEntry)
		LogInfo(m, "%s", logEntry)
	} else {
		logEntry := fmt.Sprintf("[Controller] Service '%s' signalled to stop.", msg.Label)
		model.AppendActivityLog(m, logEntry)
		LogInfo(m, "%s", logEntry)
	}
	return m, nil
}

func handleRestartMcpServerMsg(m *model.Model, msg model.RestartMcpServerMsg) (*model.Model, tea.Cmd) {
	LogInfo(m, "[Controller] Received request to restart service: %s", msg.Label)

	if m.ServiceManager == nil {
		errMsg := fmt.Sprintf("ServiceManager not available to restart service: %s", msg.Label)
		LogInfo(m, "%s", errMsg)
		model.AppendActivityLog(m, errMsg)
		return m, m.SetStatusMessage(errMsg, model.StatusBarError, 5*time.Second)
	}

	err := m.ServiceManager.RestartService(msg.Label)
	var statusMsg string
	var statusMsgType model.MessageType

	if err != nil {
		statusMsg = fmt.Sprintf("Error initiating restart for %s: %v", msg.Label, err)
		statusMsgType = model.StatusBarError
		model.AppendActivityLog(m, statusMsg)
		LogInfo(m, "%s", statusMsg)
	} else {
		statusMsg = fmt.Sprintf("Restart initiated for %s...", msg.Label)
		statusMsgType = model.StatusBarInfo
		model.AppendActivityLog(m, statusMsg)
	}

	return m, m.SetStatusMessage(statusMsg, statusMsgType, 5*time.Second)
}
