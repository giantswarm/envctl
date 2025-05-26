package controller

import (
	"envctl/internal/reporting"
	"envctl/internal/tui/model"
	"envctl/internal/tui/view" // Import for logging.LogEntry
	"envctl/pkg/logging"

	// Added import for logging.LogEntry
	// Already imported, ensure it's used or linter will complain
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	// May be needed for some view-related constants if not moved
	"errors"
)

const controllerDispatchSubsystem = "ControllerDispatch"

// mainControllerDispatch is the central message routing function for the TUI application.
// It receives all Bubble Tea messages and directs them to the appropriate handler functions
// based on the message type and current application mode.
// It's responsible for updating the model and queuing up any necessary commands.
func mainControllerDispatch(m *model.Model, msg tea.Msg) (*model.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Logging for received messages (from original model.Update)
	switch msg.(type) {
	case spinner.TickMsg, tea.MouseMsg, model.NewLogEntryMsg: // Exclude NewLogEntryMsg from this verbose debug log
		// No log for these frequent or self-referential messages
	default:
		if m.DebugMode {
			LogDebug(m, controllerDispatchSubsystem, "Received msg: %T -- Value: %v", msg, msg)
		}
	}

	// Global quit shortcuts (from original model.Update)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.CurrentAppMode = model.ModeQuitting
			m.QuittingMessage = "Shutting down services..."

			// Signal the orchestrator to stop all services
			if m.Orchestrator != nil {
				m.Orchestrator.Stop()
			}

			// Optionally, provide immediate visual feedback if desired, though Orchestrator handles actual stopping.
			// The old logic for direct stopChan closure is now handled by Orchestrator.
			// for _, pf := range m.PortForwards { // This part is now managed by Orchestrator
			// 	if pf.StopChan != nil { ... }
			// }
			// if m.McpServers != nil { // This part is now managed by Orchestrator
			// 	for name, proc := range m.McpServers { ... }
			// }

			model.FinalizeMsgSampling()
			cmds = append(cmds, tea.Quit) // tea.Quit is the primary command to exit Bubble Tea
			// We might want a small delay or a message to confirm shutdown before Quit,
			// but Stop is asynchronous in terms of when goroutines actually end.
			// The WaitGroup in cmd/connect.go handles waiting for actual process termination for CLI mode.
			// For TUI, tea.Quit will terminate the UI loop.
			return m, tea.Batch(cmds...)
		case "ctrl+c":
			// Consider if Stop should also be called here for a cleaner exit,
			// though ctrl+c is often more abrupt.
			if m.Orchestrator != nil {
				m.Orchestrator.Stop()
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
		LogDebug(m, controllerDispatchSubsystem, "Matched KubeLoginResultMsg. Routing to handler...")
		return handleKubeLoginResultMsg(m, msg, cmds) // Pass msg directly
	case model.ContextSwitchAndReinitializeResultMsg:
		LogDebug(m, controllerDispatchSubsystem, "Matched ContextSwitchAndReinitializeResultMsg. Routing to handler...")
		return handleContextSwitchAndReinitializeResultMsg(m, msg, cmds) // Pass msg directly

	case model.KubeContextResultMsg:
		LogDebug(m, controllerDispatchSubsystem, "Matched KubeContextResultMsg. Routing to handler...")
		return handleKubeContextResultMsg(m, msg) // Pass msg directly
	case model.KubeContextSwitchedMsg:
		LogDebug(m, controllerDispatchSubsystem, "Matched KubeContextSwitchedMsg. Routing to handler...")
		return handleKubeContextSwitchedMsg(m, msg) // Pass msg directly
	case model.NodeStatusMsg:
		LogDebug(m, controllerDispatchSubsystem, "Matched NodeStatusMsg. Routing to handler...")
		return handleNodeStatusMsg(m, msg) // Pass msg directly
	case model.ClusterListResultMsg:
		LogDebug(m, controllerDispatchSubsystem, "Matched ClusterListResultMsg. Routing to handler...")
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
		return m, tea.Batch(cmds...)

	case reporting.ReporterUpdateMsg:
		m, cmd = handleReporterUpdate(m, msg.Update)
		// We must batch its command with ChannelReaderCmd if we intend to keep listening.
		cmds = append(cmds, cmd, model.ChannelReaderCmd(m.TUIChannel))
		// No early return here, let it fall through to viewport updates if ActivityLogDirty was set indirectly
		// (though handleReporterUpdate doesn't directly modify ActivityLog anymore)

	case reporting.BackpressureNotificationMsg:
		m, cmd = handleBackpressureNotification(m, msg)
		cmds = append(cmds, cmd, model.ChannelReaderCmd(m.TUIChannel))

	case model.AllServicesStartedMsg:
		return handleAllServicesStartedMsg(m, msg)
	case model.ServiceStopResultMsg:
		return handleServiceStopResultMsg(m, msg)

	case model.NewLogEntryMsg:
		m = handleNewLogEntry(m, msg) // handleNewLogEntry modifies m (sets ActivityLogDirty)
		// The command to re-listen for log entries should be added to the batch.
		cmds = append(cmds, model.ListenForLogEntriesCmd(m.LogChannel))
		// DO NOT return here. Allow fall-through to viewport refresh logic.

	default:
		if m.DebugMode {
			LogDebug(m, controllerDispatchSubsystem, "Unhandled msg type in default case: %T -- Value: %v", msg, msg)
		}
		// Logic for unhandled messages (from original model.Update)
		var unhandledCmd tea.Cmd
		if m.CurrentAppMode == model.ModeNewConnectionInput && m.NewConnectionInput.Focused() {
			m.NewConnectionInput, unhandledCmd = m.NewConnectionInput.Update(msg)
		} else if m.CurrentAppMode == model.ModeLogOverlay {
			m.LogViewport, unhandledCmd = m.LogViewport.Update(msg)
		} else if m.CurrentAppMode == model.ModeMcpConfigOverlay {
			m.McpConfigViewport, unhandledCmd = m.McpConfigViewport.Update(msg)
		}
		// Removed the 'else { m.Spinner, unhandledCmd = m.Spinner.Update(msg) }' from original, as spinner update is handled via TickMsg.
		cmds = append(cmds, unhandledCmd)
	}

	// Consolidate viewport refresh logic here, to be run after all message handling (unless a handler returned early).
	logOverlayWidthChanged := m.LogViewportLastWidth != m.LogViewport.Width
	mainLogPanelWidthChanged := m.MainLogViewportLastWidth != m.MainLogViewport.Width

	if m.ActivityLogDirty || logOverlayWidthChanged {
		preparedLogOverlay := view.PrepareLogContent(m.ActivityLog, m.LogViewport.Width)
		m.LogViewport.SetContent(preparedLogOverlay)
		if m.CurrentAppMode == model.ModeLogOverlay && m.LogViewport.YOffset == 0 && m.LogViewport.AtBottom() {
			// Only autoscroll if already at bottom or just activated, to avoid jumping while user is scrolling.
			m.LogViewport.GotoBottom()
		}
		m.LogViewportLastWidth = m.LogViewport.Width
	}

	if m.ActivityLogDirty || mainLogPanelWidthChanged {
		preparedMainLog := view.PrepareLogContent(m.ActivityLog, m.MainLogViewport.Width)
		m.MainLogViewport.SetContent(preparedMainLog)
		m.MainLogViewport.GotoBottom() // Main log always scrolls to bottom
		m.MainLogViewportLastWidth = m.MainLogViewport.Width
	}

	if m.ActivityLogDirty { // This is key: reset *after* viewports have used it.
		m.ActivityLogDirty = false
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
			State:       reporting.StateRunning, // Assuming 'all services started' means the system is running
			IsReady:     true,
		})
	} else {
		// Fallback if reporter is somehow nil. This should use pkg/logging now.
		// logging.Info("ServiceManager", "All service startup commands processed by ServiceManager (no reporter).")
		// For now, let's keep it simple and assume reporter is available or this path is unlikely.
		// If we need a direct log here, it should be: model.AddRawLineToActivityLog(m, FormatSomehow("[INFO] [System - ServiceManager] All services started"))
		// But the goal is to use pkg/logging for everything.
		// For now, let's assume pkg/logging is used by ServiceManager itself to log this.
		// The fallback to AppendActivityLog is being removed as per plan.
	}
	m.IsLoading = false // Signifies initial batch dispatch of services is done.

	if len(msg.InitialStartupErrors) > 0 {
		for _, err := range msg.InitialStartupErrors {
			if m.Reporter != nil {
				m.Reporter.Report(reporting.ManagedServiceUpdate{
					Timestamp:   time.Now(),
					SourceType:  reporting.ServiceTypeSystem,
					SourceLabel: "ServiceManagerInit",
					State:       reporting.StateFailed, // Or a specific 'PartialFailure' state if we define one
					ErrorDetail: err,
					IsReady:     false, // Or true if some services are up despite errors
				})
			} else {
				// Fallback logging, same note as above, should ideally not be needed.
				// logging.Error("ServiceManagerInit", err, "Initial service startup error (no reporter).")
			}
		}
	}
	return m, nil
}

func handleServiceStopResultMsg(m *model.Model, msg model.ServiceStopResultMsg) (*model.Model, tea.Cmd) {
	var state reporting.ServiceState

	if msg.Err != nil {
		state = reporting.StateFailed // Or a more specific "StopFailed" state
	} else {
		state = reporting.StateStopped
	}

	if m.Reporter != nil {
		m.Reporter.Report(reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  reporting.ServiceTypeSystem, // Or determine actual type if service is known
			SourceLabel: msg.Label,                   // Label of the service being stopped
			State:       state,
			IsReady:     false, // A stopped/failed service is not ready
			ErrorDetail: msg.Err,
		})
	} else {
		// Fallback logging, should ideally not be needed.
		// if msg.Err != nil {
		// 	logging.Error("ServiceManager", msg.Err, "Error processing stop for service '%s' (no reporter)", msg.Label)
		// } else {
		// 	logging.Info("ServiceManager", "Service '%s' stop signal processed (no reporter).", msg.Label)
		// }
	}
	return m, nil
}

func handleRestartMcpServerMsg(m *model.Model, msg model.RestartMcpServerMsg) (*model.Model, tea.Cmd) {
	// Log initial request using the new pkg/logging system
	LogInfo(controllerSubsystem, "User requested restart for MCP server: %s", msg.Label)

	if m.Orchestrator == nil {
		errMsg := fmt.Sprintf("Orchestrator not available to restart service: %s", msg.Label)
		// Log this error using pkg/logging
		LogError(controllerSubsystem, errors.New(errMsg), "Attempted to restart service without Orchestrator")
		return m, m.SetStatusMessage(errMsg, model.StatusBarError, 5*time.Second)
	}

	// Orchestrator.RestartService will handle the restart with proper dependency management
	err := m.Orchestrator.RestartService(msg.Label)

	statusBarMsg := fmt.Sprintf("Restart initiated for %s...", msg.Label)
	statusBarMsgType := model.StatusBarInfo
	var statusCmd tea.Cmd

	if err != nil {
		statusBarMsg = fmt.Sprintf("Error initiating restart for %s: %v", msg.Label, err)
		statusBarMsgType = model.StatusBarError
		// Log the error of *initiating* the restart
		LogError(controllerSubsystem, err, "Error initiating restart for service %s", msg.Label)
	}
	statusCmd = m.SetStatusMessage(statusBarMsg, statusBarMsgType, 5*time.Second)

	return m, statusCmd
}

// updatePortForwardFromSnapshot updates a PortForwardProcess from a StateStore snapshot
func updatePortForwardFromSnapshot(pf *model.PortForwardProcess, snapshot reporting.ServiceStateSnapshot) {
	pf.StatusMsg = string(snapshot.State)
	pf.Running = snapshot.IsReady
	pf.Active = snapshot.IsReady
	if snapshot.ErrorDetail != nil {
		pf.Err = snapshot.ErrorDetail
	}
}

// updateMcpServerFromSnapshot updates a McpServerProcess from a StateStore snapshot
func updateMcpServerFromSnapshot(mcp *model.McpServerProcess, snapshot reporting.ServiceStateSnapshot) {
	mcp.StatusMsg = string(snapshot.State)
	mcp.Active = snapshot.IsReady
	if snapshot.ProxyPort > 0 {
		mcp.ProxyPort = snapshot.ProxyPort
	}
	if snapshot.PID > 0 {
		mcp.Pid = snapshot.PID
	}
	if snapshot.ErrorDetail != nil {
		mcp.Err = snapshot.ErrorDetail
	}
}

func handleReporterUpdate(m *model.Model, update reporting.ManagedServiceUpdate) (*model.Model, tea.Cmd) {
	// The new ManagedServiceUpdate focuses on State. We no longer log its content directly here.
	// State changes are already logged by ServiceManager via pkg/logging.
	// This handler is now primarily for updating the UI based on the reported service state.

	// The StateStore is already updated by the TUIReporter before this handler is called.
	// We now reconcile the UI state with the StateStore to ensure consistency.

	// Get the latest state from StateStore
	snapshot, exists := m.GetServiceSnapshot(update.SourceLabel)
	if !exists {
		// Service not found in StateStore, this shouldn't happen but handle gracefully
		logging.Warn("TUIController", "Received update for unknown service: %s", update.SourceLabel)
		return m, nil
	}

	// Update specific model state (for PF and MCP panels) from StateStore
	switch update.SourceType {
	case reporting.ServiceTypePortForward:
		if pfProcess, exists := m.PortForwards[update.SourceLabel]; exists {
			updatePortForwardFromSnapshot(pfProcess, snapshot)
		}
	case reporting.ServiceTypeMCPServer:
		if mcpProcess, exists := m.McpServers[update.SourceLabel]; exists {
			updateMcpServerFromSnapshot(mcpProcess, snapshot)
		}
	}

	// Update status bar based on StateStore data
	var statusCmd tea.Cmd
	statusBarMsg := ""
	statusBarMsgType := model.StatusBarInfo // Default

	if snapshot.State != "" { // Only update status bar if there's a meaningful state
		statusPrefix := fmt.Sprintf("[%s - %s]", snapshot.SourceType, snapshot.Label)

		if snapshot.ErrorDetail != nil {
			statusBarMsgType = model.StatusBarError
			statusBarMsg = fmt.Sprintf("%s %s: %s", statusPrefix, snapshot.State, snapshot.ErrorDetail.Error())
		} else {
			statusBarMsg = fmt.Sprintf("%s %s", statusPrefix, snapshot.State)
			// Add port info to status bar for MCP servers
			if snapshot.SourceType == reporting.ServiceTypeMCPServer && snapshot.ProxyPort > 0 {
				statusBarMsg += fmt.Sprintf(" (port: %d)", snapshot.ProxyPort)
			}
			// Add PID info to status bar for MCP servers
			if snapshot.SourceType == reporting.ServiceTypeMCPServer && snapshot.PID > 0 {
				statusBarMsg += fmt.Sprintf(" [PID: %d]", snapshot.PID)
			}
			switch snapshot.State {
			case reporting.StateFailed:
				statusBarMsgType = model.StatusBarError
			case reporting.StateUnknown:
				statusBarMsgType = model.StatusBarWarning
			case reporting.StateRunning:
				statusBarMsgType = model.StatusBarSuccess
			case reporting.StateStarting, reporting.StateStopping, reporting.StateRetrying, reporting.StateStopped:
				statusBarMsgType = model.StatusBarInfo // Default to Info for transient/stopped states
			default:
				statusBarMsgType = model.StatusBarInfo
			}
		}

		// Filter out less important status updates from the status bar to avoid noise
		showInStatusBar := true
		switch snapshot.State {
		case reporting.StateStarting, reporting.StateStopping, reporting.StateRetrying:
			if !m.DebugMode { // Only show these transient states in status bar if TUI debug mode is on
				showInStatusBar = false
			}
		}

		if showInStatusBar && statusBarMsg != "" {
			statusCmd = m.SetStatusMessage(statusBarMsg, statusBarMsgType, 3*time.Second)
		}
	}

	return m, statusCmd
}

func handleNewLogEntry(m *model.Model, msg model.NewLogEntryMsg) *model.Model {
	entry := msg.Entry

	// Only add to TUI activity log if the level is INFO or above,
	// OR if TUI debug mode is enabled (m.DebugMode is true).
	// Assumes LogLevel enum order: Debug < Info < Warn < Error.
	if entry.Level >= logging.LevelInfo || m.DebugMode {
		logLine := fmt.Sprintf("%s [%s] [%s] %s",
			entry.Timestamp.Format("15:04:05.000"),
			entry.Level.String(),
			entry.Subsystem,
			entry.Message)

		if entry.Err != nil {
			logLine = fmt.Sprintf("%s -- Error: %v", logLine, entry.Err)
		}
		model.AddRawLineToActivityLog(m, logLine)
	}
	return m
}

func handleBackpressureNotification(m *model.Model, notification reporting.BackpressureNotificationMsg) (*model.Model, tea.Cmd) {
	// Set timestamp if not provided
	if notification.Timestamp.IsZero() {
		notification.Timestamp = time.Now()
	}

	// Create a warning message for the user
	warningMsg := fmt.Sprintf("⚠️  Critical update dropped for %s (state: %s) - %s",
		notification.ServiceLabel, notification.DroppedState, notification.Reason)

	// Show in status bar with warning type
	statusCmd := m.SetStatusMessage(warningMsg, model.StatusBarWarning, 10*time.Second)

	// Log the notification
	logging.Warn("TUIController", "Backpressure notification: %s", warningMsg)

	return m, statusCmd
}
