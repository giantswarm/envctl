package controller

import (
	"envctl/internal/reporting"
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	// May be needed for some view-related constants if not moved
	"errors"
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

	case reporting.ReporterUpdateMsg:
		return handleReporterUpdate(m, msg.Update)

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

func handleAllServicesStartedMsg(m *model.Model, msg model.AllServicesStartedMsg) (*model.Model, tea.Cmd) {
	if m.Reporter != nil {
		m.Reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeSystem,
			SourceLabel: "ServiceManager",
			Level:       reporting.LogLevelInfo,
			Message:     "All service startup commands processed by ServiceManager.",
		})
	} else {
		// Fallback if reporter is somehow nil (should not happen in TUI mode if initialized correctly)
		model.AppendActivityLog(m, "[INFO] [System - ServiceManager] All service startup commands processed by ServiceManager.")
	}
	m.IsLoading = false // Signifies initial batch dispatch of services is done.

	if len(msg.InitialStartupErrors) > 0 {
		for _, err := range msg.InitialStartupErrors {
			if m.Reporter != nil {
				m.Reporter.Report(reporting.ManagedServiceUpdate{
					Timestamp:   time.Now(),
					SourceType:  reporting.ServiceTypeSystem,
					SourceLabel: "ServiceManagerInit", // Or a more specific label if available from err context
					Level:       reporting.LogLevelError,
					Message:     fmt.Sprintf("Initial service startup error: %v", err),
					ErrorDetail: err,
					IsError:     true,
				})
			} else {
				model.AppendActivityLog(m, fmt.Sprintf("[ERROR] [System - ServiceManagerInit] Initial service startup error: %v", err))
			}
		}
	}
	return m, nil
}

func handleServiceStopResultMsg(m *model.Model, msg model.ServiceStopResultMsg) (*model.Model, tea.Cmd) {
	level := reporting.LogLevelInfo
	message := fmt.Sprintf("Service '%s' stop signal processed.", msg.Label)
	isError := false
	var errDetail error = nil // Initialize explicitly

	if msg.Err != nil {
		level = reporting.LogLevelError
		message = fmt.Sprintf("Error processing stop for service '%s': %v", msg.Label, msg.Err)
		isError = true
		errDetail = msg.Err
	}

	if m.Reporter != nil {
		m.Reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeSystem, // Or determine actual type if service is known
			SourceLabel: msg.Label,                   // Label of the service being stopped
			Level:       level,
			Message:     message,
			IsError:     isError,
			ErrorDetail: errDetail,
		})
	} else {
		// Fallback if reporter is somehow nil
		logKey := "INFO"
		if isError {
			logKey = "ERROR"
		}
		model.AppendActivityLog(m, fmt.Sprintf("[%s] [System - %s] %s", logKey, msg.Label, message))
	}
	return m, nil
}

func handleRestartMcpServerMsg(m *model.Model, msg model.RestartMcpServerMsg) (*model.Model, tea.Cmd) {
	// Log initial request using the new reporter system via LogInfo (once refactored)
	// For now, direct report or old LogInfo:
	if m.Reporter != nil {
		m.Reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeSystem,
			SourceLabel: msg.Label, // Service being restarted
			Level:       reporting.LogLevelInfo,
			Message:     fmt.Sprintf("User requested restart for MCP server: %s", msg.Label),
		})
	} else {
		LogInfo(m, "[Controller] User requested restart for MCP server: %s", msg.Label) // Fallback
	}

	if m.ServiceManager == nil {
		errMsg := fmt.Sprintf("ServiceManager not available to restart service: %s", msg.Label)
		if m.Reporter != nil {
			m.Reporter.Report(reporting.ManagedServiceUpdate{Timestamp: time.Now(), SourceType: reporting.ServiceTypeSystem, SourceLabel: "RestartService", Level: reporting.LogLevelError, Message: errMsg, IsError: true, ErrorDetail: errors.New(errMsg)})
		} else {
			model.AppendActivityLog(m, errMsg)
		}
		return m, m.SetStatusMessage(errMsg, model.StatusBarError, 5*time.Second)
	}

	err := m.ServiceManager.RestartService(msg.Label) // This will trigger its own updates via the reporter for "Stopping" and "Restarting..."

	statusBarMsg := fmt.Sprintf("Restart initiated for %s...", msg.Label)
	statusBarMsgType := model.StatusBarInfo

	// Report the immediate outcome of *initiating* the restart
	level := reporting.LogLevelInfo
	message := statusBarMsg
	isError := false
	var errDetail error = nil

	if err != nil {
		message = fmt.Sprintf("Error initiating restart for %s: %v", msg.Label, err)
		statusBarMsg = message // Update status bar message as well
		statusBarMsgType = model.StatusBarError
		level = reporting.LogLevelError
		isError = true
		errDetail = err
	}

	if m.Reporter != nil {
		m.Reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeSystem,
			SourceLabel: msg.Label,
			Level:       level,
			Message:     message,
			IsError:     isError,
			ErrorDetail: errDetail,
		})
	} else {
		model.AppendActivityLog(m, message) // Fallback if reporter is nil
	}

	return m, m.SetStatusMessage(statusBarMsg, statusBarMsgType, 5*time.Second)
}

func handleReporterUpdate(m *model.Model, update reporting.ManagedServiceUpdate) (*model.Model, tea.Cmd) {
	// Extremely simplified version for testing channel throughput
	fmt.Printf("DEBUG_TUI_HANDLER_SIMPLIFIED: Received: %s - %s: %s\n", update.SourceType, update.SourceLabel, update.Message)

	// Comment out all previous work:
	/*
		// 1. Format and append to ActivityLog
		var logParts []string
		logParts = append(logParts, fmt.Sprintf("[%s]", update.Timestamp.Format("15:04:05.000")))
		if update.Level != "" {
			logParts = append(logParts, fmt.Sprintf("[%s]", strings.ToUpper(string(update.Level))))
		}

		var sourceDisplay strings.Builder
		if update.SourceType != "" {
			sourceDisplay.WriteString(string(update.SourceType))
		}
		if update.SourceLabel != "" {
			if sourceDisplay.Len() > 0 {
				sourceDisplay.WriteString(" - ")
			}
			sourceDisplay.WriteString(update.SourceLabel)
		}
		if sourceDisplay.Len() > 0 {
			logParts = append(logParts, fmt.Sprintf("[%s]", sourceDisplay.String()))
		}

		logParts = append(logParts, update.Message)

		if strings.TrimSpace(update.Details) != "" && update.Details != update.Message {
			detailLines := strings.Split(strings.TrimSuffix(update.Details, "\n"), "\n")
			for i, line := range detailLines {
				if i == 0 {
					logParts = append(logParts, fmt.Sprintf("\n  Details: %s", line))
				} else {
					logParts = append(logParts, fmt.Sprintf("\n           %s", line))
				}
			}
		}

		if update.ErrorDetail != nil {
			if update.Message != update.ErrorDetail.Error() {
				logParts = append(logParts, fmt.Sprintf("\n  ErrorDetail: %v", update.ErrorDetail))
			}
		}
		model.AppendActivityLog(m, strings.Join(logParts, " "))
		m.ActivityLogDirty = true

		// 2. Update specific model state
		switch update.SourceType {
		case reporting.ServiceTypePortForward:
			if pfProcess, exists := m.PortForwards[update.SourceLabel]; exists {
				pfProcess.StatusMsg = update.Message
				pfProcess.Running = update.IsReady
				pfProcess.Err = update.ErrorDetail
				if (update.Level == reporting.LogLevelStdout || update.Level == reporting.LogLevelStderr) && update.Details != "" {
					pfProcess.Log = append(pfProcess.Log, strings.Split(update.Details, "\n")...)
					if len(pfProcess.Log) > model.MaxPanelLogLines {
						pfProcess.Log = pfProcess.Log[len(pfProcess.Log)-model.MaxPanelLogLines:]
					}
				}
			}
		case reporting.ServiceTypeMCPServer:
			if mcpProcess, exists := m.McpServers[update.SourceLabel]; exists {
				mcpProcess.StatusMsg = update.Message
				mcpProcess.Active = update.IsReady
				mcpProcess.Err = update.ErrorDetail
				if (update.Level == reporting.LogLevelStdout || update.Level == reporting.LogLevelStderr) && update.Details != "" {
					mcpProcess.Output = append(mcpProcess.Output, strings.Split(update.Details, "\n")...)
					if len(mcpProcess.Output) > model.MaxPanelLogLines {
						mcpProcess.Output = mcpProcess.Output[len(mcpProcess.Output)-model.MaxPanelLogLines:]
					}
				}
			}
		}
	*/

	return m, nil
}
