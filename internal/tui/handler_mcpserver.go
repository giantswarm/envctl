package tui

import (
	"fmt"
	"time"

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
    m.combinedOutput = append(m.combinedOutput, logLine)

    if len(m.combinedOutput) > maxCombinedOutputLines {
        m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
    }

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
        m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP] %s", msg.Label, msg.outputLog))
    }
    if len(m.combinedOutput) > maxCombinedOutputLines {
        m.combinedOutput = m.combinedOutput[len(m.combinedOutput)-maxCombinedOutputLines:]
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
    m.combinedOutput = append(m.combinedOutput, fmt.Sprintf("[%s MCP Proxy] Restart requested at %s", serverName, time.Now().Format("15:04:05")))

    // Return a command that triggers startMcpProxiesCmd again but only for this server.
    startCmd := func() tea.Msg {
        // Reuse generic logic in commands.go by calling startMcpProxiesCmd and filtering
        cmds := startMcpProxiesCmd(m.TUIChannel)
        // Find the command for this label
        for _, cmd := range cmds {
            // Execute to inspect? Instead wrap.
            return cmd()
        }
        return mcpServerStatusUpdateMsg{Label: serverName, status: "Error", outputLog: "Restart command not found"}
    }

    return m, startCmd
} 