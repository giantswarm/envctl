package api

// MCPTool represents an MCP tool
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

// MCPResource represents an MCP resource
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ServiceStateChangedEvent represents a service state change event from the orchestrator
// This is the standard event type used throughout the system for state changes
type ServiceStateChangedEvent struct {
	Label    string
	OldState string
	NewState string
	Health   string
	Error    error
}
