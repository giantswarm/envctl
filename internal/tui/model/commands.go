package model

import (
	"context"
	"time"

	"envctl/internal/api"

	tea "github.com/charmbracelet/bubbletea"
)

// FetchMCPToolsCmd creates a command to fetch tools for an MCP server
func FetchMCPToolsCmd(apis *api.Provider, serverName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		tools, err := apis.MCP.GetTools(ctx, serverName)
		if err != nil {
			return MCPToolsErrorMsg{
				ServerName: serverName,
				Error:      err,
			}
		}

		return MCPToolsLoadedMsg{
			ServerName: serverName,
			Tools:      tools,
		}
	}
}
