package controller

import (
	"encoding/json"
	"envctl/internal/mcpserver"
	"envctl/internal/tui/model"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleMcpServerSetupCompletedMsg records the outcome of launching an MCP proxy.
func handleMcpServerSetupCompletedMsg(m *model.Model, msg model.McpServerSetupCompletedMsg) (*model.Model, tea.Cmd) {
	if m.McpServers == nil {
		m.McpServers = make(map[string]*model.McpServerProcess)
	}
	proc := &model.McpServerProcess{
		Label:     msg.Label,
		Pid:       msg.PID,
		StopChan:  msg.StopChan,
		Active:    msg.Err == nil && msg.StopChan != nil,
		StatusMsg: msg.Status,
		Err:       msg.Err,
	}
	m.McpServers[msg.Label] = proc

	if msg.Err != nil {
		LogError(m, "[MCP %s] %s – error: %v", msg.Label, msg.Status, msg.Err)
	} else {
		LogInfo(m, "[MCP %s] %s", msg.Label, msg.Status)
	}

	var statusCmd tea.Cmd
	if msg.Err != nil {
		statusCmd = m.SetStatusMessage(fmt.Sprintf("[%s] MCP Setup Failed", msg.Label), model.StatusBarError, 5*time.Second)
	} else {
		statusCmd = m.SetStatusMessage(fmt.Sprintf("[%s] MCP Proxy running (PID %d)", msg.Label, msg.PID), model.StatusBarSuccess, 3*time.Second)
	}
	m.IsLoading = false
	return m, statusCmd
}

// handleMcpServerStatusUpdateMsg updates runtime status/logs for an MCP proxy.
func handleMcpServerStatusUpdateMsg(m *model.Model, msg model.McpServerStatusUpdateMsg) (*model.Model, tea.Cmd) {
	proc, ok := m.McpServers[msg.Label]
	if !ok {
		proc = &model.McpServerProcess{Label: msg.Label}
		m.McpServers[msg.Label] = proc
	}

	if msg.PID != 0 {
		proc.Pid = msg.PID
	}
	if msg.Status != "" {
		proc.StatusMsg = msg.Status
	}
	if msg.Err != nil {
		proc.Err = msg.Err
		proc.Active = false
	}
	if msg.OutputLog != "" {
		proc.Output = append(proc.Output, msg.OutputLog)
		if len(proc.Output) > model.MaxPanelLogLines {
			proc.Output = proc.Output[len(proc.Output)-model.MaxPanelLogLines:]
		}

		LogInfo(m, "[%s MCP] %s", msg.Label, msg.OutputLog)
	}
	return m, nil
}

// handleRestartMcpServerMsg stops a running MCP proxy (if any) and starts a new one.
func handleRestartMcpServerMsg(m *model.Model, msg model.RestartMcpServerMsg) (*model.Model, tea.Cmd) {
	serverName := msg.Label
	proc, ok := m.McpServers[serverName]
	if ok && proc.StopChan != nil {
		safeCloseChan(proc.StopChan)
		proc.StopChan = nil
		proc.StatusMsg = "Stopping..."
		proc.Active = false
	}

	m.IsLoading = true
	LogInfo(m, "[%s MCP Proxy] Restart requested at %s", serverName, time.Now().Format("15:04:05"))

	const restartDelay = 2 * time.Second

	startCmd := func() tea.Msg {
		time.Sleep(restartDelay)

		var cfgFound *mcpserver.MCPServerConfig
		for i := range m.MCPServerConfig {
			if m.MCPServerConfig[i].Name == serverName {
				cfgFound = &m.MCPServerConfig[i]
				break
			}
		}

		if cfgFound == nil {
			return model.McpServerStatusUpdateMsg{Label: serverName, Status: "Error", OutputLog: "Unknown MCP proxy"}
		}

		cfgToStart := *cfgFound

		tuiUpdateFn := func(update mcpserver.McpProcessUpdate) {
			if m.TUIChannel != nil {
				m.TUIChannel <- model.McpServerStatusUpdateMsg{
					Label:     update.Label,
					PID:       update.PID,
					Status:    update.Status,
					OutputLog: update.OutputLog,
					Err:       update.Err,
				}
			}
		}

		stopChan, pid, startErr := m.Services.Proxy.Start(cfgToStart, tuiUpdateFn)

		initialStatusMsg := fmt.Sprintf("Initializing proxy for %s...", cfgToStart.Name)
		if startErr != nil {
			initialStatusMsg = fmt.Sprintf("Failed to start %s: %s", cfgToStart.Name, startErr.Error())
		}

		return model.McpServerSetupCompletedMsg{
			Label:    cfgToStart.Name,
			StopChan: stopChan,
			PID:      pid,
			Status:   initialStatusMsg,
			Err:      startErr,
		}
	}

	return m, startCmd
}

// GenerateMcpConfigJson creates a JSON string with MCP server endpoint configurations.
// It now takes the list of MCP server configurations as an argument.
func GenerateMcpConfigJson(mcpServerConfig []mcpserver.MCPServerConfig) string {
	type entry struct {
		URL string `json:"url"`
	}
	servers := make(map[string]entry)
	for _, cfg := range mcpServerConfig {
		key := fmt.Sprintf("%s-mcp", cfg.Name)
		servers[key] = entry{URL: fmt.Sprintf("http://localhost:%d/sse", cfg.ProxyPort)}
	}
	root := map[string]interface{}{"mcpServers": servers}
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "{}" // Return empty JSON object on error
	}
	return string(b)
}
