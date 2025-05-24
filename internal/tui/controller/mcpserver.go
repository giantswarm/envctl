package controller

import (
	"encoding/json"
	"envctl/internal/config"
	"fmt"
	"strings"
)

// GenerateMcpConfigJson creates a JSON string with MCP server endpoint configurations.
// It now uses []config.MCPServerDefinition.
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
func GenerateMcpConfigJson(mcpServerConfigs []config.MCPServerDefinition) string {
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
		// The URL construction needs to be determined. The old ProxyPort is gone.
		// If the service is localCommand, its URL might be in cfg.Env (e.g., PROMETHEUS_URL for prometheus server).
		// If it's a container, it's more complex.
		// For now, let's create a placeholder or try to find a URL from Env.
		description := fmt.Sprintf("Locally managed %s MCP server.", cfg.Type)
		url := "local://" + cfg.Name // Placeholder

		if cfg.Type == config.MCPServerTypeLocalCommand {
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
			// It might be one of the mapped ContainerPorts.
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
