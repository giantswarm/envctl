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
// This now generates a single aggregator endpoint that provides access to all MCP servers
func GenerateMcpConfigJson(mcpServerConfigs []config.MCPServerDefinition, mcpServers map[string]*api.MCPServerInfo, aggregatorPort int) string {
	type entry struct {
		URL         string `json:"url"`
		Description string `json:"description,omitempty"`
	}
	servers := make(map[string]entry)

	// Check if any MCP servers are enabled
	hasEnabledServers := false
	for _, cfg := range mcpServerConfigs {
		if cfg.Enabled {
			hasEnabledServers = true
			break
		}
	}

	if !hasEnabledServers {
		// No MCP servers enabled, return empty config
		root := map[string]interface{}{"mcpServers": servers}
		b, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return "{\"error\": \"failed to marshal mcp server config to json\"}"
		}
		return string(b)
	}

	// Use default aggregator port if not specified
	if aggregatorPort == 0 {
		aggregatorPort = 8080
	}

	// Generate single aggregator endpoint
	servers["envctl-aggregator"] = entry{
		URL:         fmt.Sprintf("http://localhost:%d/sse", aggregatorPort),
		Description: "Aggregated MCP endpoint providing access to all configured MCP servers",
	}

	// Add information about available servers in the description
	var availableServers []string
	for _, cfg := range mcpServerConfigs {
		if !cfg.Enabled {
			continue
		}
		// Check runtime status
		if srv, exists := mcpServers[cfg.Name]; exists {
			if srv.State == "Running" && srv.Health == "Healthy" {
				availableServers = append(availableServers, fmt.Sprintf("%s (✓)", cfg.Name))
			} else {
				availableServers = append(availableServers, fmt.Sprintf("%s (⚠)", cfg.Name))
			}
		} else {
			availableServers = append(availableServers, fmt.Sprintf("%s (?)", cfg.Name))
		}
	}

	if len(availableServers) > 0 {
		servers["envctl-aggregator"] = entry{
			URL:         fmt.Sprintf("http://localhost:%d/sse", aggregatorPort),
			Description: fmt.Sprintf("Aggregated MCP endpoint with servers: %s", strings.Join(availableServers, ", ")),
		}
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
