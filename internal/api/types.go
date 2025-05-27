package api

// MCPTool represents a tool exposed by an MCP server
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// MCPToolUpdateEvent represents an update to MCP tools
type MCPToolUpdateEvent struct {
	ServerName string
	Tools      []MCPTool
	Error      error
}
