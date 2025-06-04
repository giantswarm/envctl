package api

import (
	"envctl/internal/orchestrator"
)

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

// ServiceStateChangedEvent is re-exported from orchestrator for consistency
type ServiceStateChangedEvent = orchestrator.ServiceStateChangedEvent
