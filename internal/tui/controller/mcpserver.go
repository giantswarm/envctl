package controller

import (
	"encoding/json"
	"envctl/internal/mcpserver"
	"fmt"
)

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
