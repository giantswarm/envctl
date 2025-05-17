package mcpserver

// PredefinedMcpServers defines the static configuration for all known MCP servers.
var PredefinedMcpServers = []PredefinedMcpServer{
	{
		Name:      "kubernetes",
		ProxyPort: 8001,
		Command:   "npx",
		Args:      []string{"mcp-server-kubernetes"},
		Env:       map[string]string{},
	},
	{
		Name:      "prometheus",
		ProxyPort: 8002,
		Command:   "uvx",
		Args:      []string{"mcp-server-prometheus"},
		Env: map[string]string{
			"PROMETHEUS_URL": "http://localhost:8080/prometheus",
			"ORG_ID":         "giantswarm",
		},
	},
	{
		Name:      "grafana",
		ProxyPort: 8003,
		Command:   "uvx",
		Args:      []string{"mcp-server-grafana"},
		Env:       map[string]string{"GRAFANA_URL": "http://localhost:3000"},
	},
}
