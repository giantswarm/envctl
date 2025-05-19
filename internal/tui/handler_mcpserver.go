package tui

import (
	"fmt"
	"time"

	"envctl/internal/mcpserver"

	tea "github.com/charmbracelet/bubbletea"
)

// handleMcpServerSetupCompletedMsg records the outcome of launching an MCP proxy.
func handleMcpServerSetupCompletedMsg(m model, msg mcpServerSetupCompletedMsg) (model, tea.Cmd) {
    if m.mcpServers == nil {
        m.mcpServers = make(map[string]*mcpServerProcess)
    }
    proc := &mcpServerProcess{
        label:     msg.Label,
        pid:       msg.pid,
        stopChan:  msg.stopChan,
        active:    msg.err == nil && msg.stopChan != nil,
        statusMsg: msg.status,
        err:       msg.err,
    }
    m.mcpServers[msg.Label] = proc

    // Append to combined output for diagnostics
    logLine := fmt.Sprintf("[MCP %s] %s", msg.Label, msg.status)
    if msg.err != nil {
        logLine += fmt.Sprintf(" – error: %v", msg.err)
    }
    m.appendLogLine(logLine)

    var status tea.Cmd
    if msg.err != nil {
        status = m.setStatusMessage(fmt.Sprintf("[%s] MCP Setup Failed", msg.Label), StatusBarError, 5*time.Second)
    } else {
        status = m.setStatusMessage(fmt.Sprintf("[%s] MCP Proxy running (PID %d)", msg.Label, msg.pid), StatusBarSuccess, 3*time.Second)
    }
    m.isLoading = false
    return m, status
}

// handleMcpServerStatusUpdateMsg updates runtime status/logs for an MCP proxy.
func handleMcpServerStatusUpdateMsg(m model, msg mcpServerStatusUpdateMsg) (model, tea.Cmd) {
    proc, ok := m.mcpServers[msg.Label]
    if !ok {
        // First time we hear about it – create entry.
        proc = &mcpServerProcess{label: msg.Label}
        m.mcpServers[msg.Label] = proc
    }

    if msg.pid != 0 {
        proc.pid = msg.pid
    }
    if msg.status != "" {
        proc.statusMsg = msg.status
    }
    if msg.err != nil {
        proc.err = msg.err
        proc.active = false
    }
    if msg.outputLog != "" {
        proc.output = append(proc.output, msg.outputLog)
        if len(proc.output) > maxPanelLogLines {
            proc.output = proc.output[len(proc.output)-maxPanelLogLines:]
        }
        m.appendLogLine(fmt.Sprintf("[%s MCP] %s", msg.Label, msg.outputLog))
    }
    return m, nil
}

// handleRestartMcpServerMsg stops a running MCP proxy (if any) and starts a new one.
func handleRestartMcpServerMsg(m model, msg restartMcpServerMsg) (model, tea.Cmd) {
    serverName := msg.Label
    proc, ok := m.mcpServers[serverName]
    if ok && proc.stopChan != nil {
        safeCloseChan(proc.stopChan)
        proc.stopChan = nil
        proc.statusMsg = "Stopping..."
        proc.active = false
    }

    m.isLoading = true
    m.LogInfo("[%s MCP Proxy] Restart requested at %s", serverName, time.Now().Format("15:04:05"))

    // Introduce a small delay to give the underlying process time to
    // shut down and release its listening socket (e.g. :8001) before we
    // attempt to start the replacement. This avoids the race where the
    // new instance fails with "address already in use".
    const restartDelay = 2 * time.Second

    startCmd := func() tea.Msg {
        // Allow the previous process some time to shut down.
        time.Sleep(restartDelay)

        // Lookup the configuration for the requested MCP proxy.
        var cfg *mcpserver.PredefinedMcpServer
        for i := range mcpserver.PredefinedMcpServers {
            if mcpserver.PredefinedMcpServers[i].Name == serverName {
                cfg = &mcpserver.PredefinedMcpServers[i]
                break
            }
        }

        if cfg == nil {
            return mcpServerStatusUpdateMsg{Label: serverName, status: "Error", outputLog: "Unknown MCP proxy"}
        }

        // Bridge updates back into the TUI via the existing channel.
        tuiUpdateFn := func(update mcpserver.McpProcessUpdate) {
            if m.TUIChannel != nil {
                m.TUIChannel <- mcpServerStatusUpdateMsg{
                    Label:     update.Label,
                    pid:       update.PID,
                    status:    update.Status,
                    outputLog: update.OutputLog,
                    err:       update.Err,
                }
            }
        }

        pid, stopChan, startErr := mcpserver.StartAndManageIndividualMcpServer(*cfg, tuiUpdateFn, nil)

        initialStatusMsg := fmt.Sprintf("Initializing proxy for %s...", cfg.Name)
        if startErr != nil {
            initialStatusMsg = fmt.Sprintf("Failed to start %s: %s", cfg.Name, startErr.Error())
        }

        return mcpServerSetupCompletedMsg{
            Label:    cfg.Name,
            stopChan: stopChan,
            pid:      pid,
            status:   initialStatusMsg,
            err:      startErr,
        }
    }

    return m, startCmd
} 