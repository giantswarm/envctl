package controller

import (
	"encoding/json"
	"envctl/internal/config"
	"envctl/internal/tui/model"
	"fmt"
	"strings"
)

// GenerateMcpConfigJson creates a JSON string with MCP server endpoint configurations.
// It now uses both []config.MCPServerDefinition and the runtime McpServerProcess map to get actual proxy ports.
// The concept of a ProxyPort for mcp-proxy itself is removed from MCPServerDefinition.
// The URL for the MCP server will depend on how it exposes itself (e.g., via an env var, or a fixed port).
// This function might need to be re-thought if the goal is to provide SSE endpoint URLs managed by an mcp-proxy.
// For now, if an MCP server is a localCommand, its URL is not directly known from MCPServerDefinition in a generic way
// unless specified in its Env vars or Command itself.
// If it's a container, ContainerPorts might be relevant but not for an SSE endpoint managed by a separate mcp-proxy.
// This function previously assumed an mcp-proxy per service on a known ProxyPort.
// Given the refactor, this JSON generation needs to adapt or be re-evaluated for its purpose.
// Let's assume for now, we can't reliably generate these URLs without more info on how mcp-proxy works with the new config.
// For a placeholder, we can construct a dummy URL or indicate it's locally managed.
func GenerateMcpConfigJson(mcpServerConfigs []config.MCPServerDefinition, mcpProcesses map[string]*model.McpServerProcess) string {
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

		// Check if we have runtime process information with actual proxy port
		if proc, exists := mcpProcesses[cfg.Name]; exists && proc.ProxyPort > 0 {
			// We have an actual port from mcp-proxy
			url = fmt.Sprintf("http://localhost:%d/sse", proc.ProxyPort)
			description = fmt.Sprintf("%s MCP server via mcp-proxy on port %d", cfg.Type, proc.ProxyPort)
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
