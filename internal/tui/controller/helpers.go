package controller

import (
	"encoding/json"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/tui/model"
	"envctl/internal/tui/view"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// GenerateMcpConfigJson generates MCP configuration JSON
func GenerateMcpConfigJson(mcpServerConfigs []config.MCPServerDefinition, mcpServers map[string]*api.MCPServerInfo) string {
	type entry struct {
		URL         string `json:"url"`
		Description string `json:"description,omitempty"`
	}
	servers := make(map[string]entry)

	for _, cfg := range mcpServerConfigs {
		if !cfg.Enabled {
			continue
		}
		key := fmt.Sprintf("%s-mcp", cfg.Name)
		description := fmt.Sprintf("Locally managed %s MCP server.", cfg.Type)
		url := "local://" + cfg.Name // Default placeholder

		// Check if we have runtime server information with actual proxy port
		if srv, exists := mcpServers[cfg.Name]; exists && srv.Port > 0 {
			// We have an actual port from mcp-proxy
			url = fmt.Sprintf("http://localhost:%d/sse", srv.Port)
			description = fmt.Sprintf("%s MCP server via mcp-proxy on port %d", cfg.Type, srv.Port)
		} else if cfg.ProxyPort > 0 {
			// Use configured port if no runtime port available
			url = fmt.Sprintf("http://localhost:%d/sse", cfg.ProxyPort)
			description = fmt.Sprintf("%s MCP server configured for port %d", cfg.Type, cfg.ProxyPort)
		} else if cfg.Type == config.MCPServerTypeLocalCommand {
			// Attempt to find a common URL env var, e.g., ending with _URL
			for envKey, envVal := range cfg.Env {
				if strings.HasSuffix(strings.ToUpper(envKey), "_URL") && strings.HasPrefix(envVal, "http") {
					url = envVal
					description = fmt.Sprintf("Local command, URL from env: %s", envKey)
					break
				}
			}
		} else if cfg.Type == config.MCPServerTypeContainer {
			// For containers, the URL is not as straightforward to determine for an external SSE endpoint.
			description = fmt.Sprintf("Containerized MCP server. Ports: %v", cfg.ContainerPorts)
			// Could try to pick a port, e.g., the first one, but it's a guess.
			if len(cfg.ContainerPorts) > 0 {
				parts := strings.Split(cfg.ContainerPorts[0], ":")
				hostPort := parts[0]
				url = fmt.Sprintf("http://localhost:%s", hostPort) // Assuming localhost and http
			}
		}

		servers[key] = entry{URL: url, Description: description}
	}

	root := map[string]interface{}{"mcpServers": servers}
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "{\"error\": \"failed to marshal mcp server config to json\"}"
	}
	return string(b)
}

// PrepareLogContent is a wrapper around the view function
func PrepareLogContent(lines []string, maxWidth int) string {
	return view.PrepareLogContent(lines, maxWidth)
}

// PerformSwitchKubeContextCmd returns a command to switch Kubernetes context
func PerformSwitchKubeContextCmd(targetContext string) tea.Cmd {
	return func() tea.Msg {
		kubeMgr := kube.NewManager(nil)
		// Perform the context switch
		if err := kubeMgr.SwitchContext(targetContext); err != nil {
			return model.KubeContextSwitchedMsg{
				TargetContext: targetContext,
				Err:           err,
			}
		}
		return model.KubeContextSwitchedMsg{
			TargetContext: targetContext,
			Err:           nil,
		}
	}
}
