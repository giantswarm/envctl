package api

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
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

// ToolUpdateEvent represents a tool availability change event
type ToolUpdateEvent struct {
	Type       string    `json:"type"` // "server_registered", "server_deregistered", "tools_updated"
	ServerName string    `json:"server_name"`
	Tools      []string  `json:"tools"` // List of tool names
	Timestamp  time.Time `json:"timestamp"`
	Error      string    `json:"error,omitempty"`
}

// MCPResource represents an MCP resource
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Service states and health statuses moved to shared.go

// Service types
type ServiceType string

const (
	TypeKubeConnection ServiceType = "KubeConnection"
	TypeMCPServer      ServiceType = "MCPServer"
	TypePortForward    ServiceType = "PortForward"
	TypeAggregator     ServiceType = "Aggregator"
)

// Cluster types
type ClusterRole string

const (
	ClusterRoleTalos         ClusterRole = "talos"
	ClusterRoleManagement    ClusterRole = "management"
	ClusterRoleWorkload      ClusterRole = "workload"
	ClusterRoleObservability ClusterRole = "observability"
)

type ClusterDefinition struct {
	Name        string      `json:"name"`
	Context     string      `json:"context"`
	Role        ClusterRole `json:"role"`
	DisplayName string      `json:"display_name"`
	Icon        string      `json:"icon"`
}

// Event types
type ServiceStateChangedEvent struct {
	Name        string    `json:"name"`
	ServiceType string    `json:"service_type"`
	OldState    string    `json:"old_state"`
	NewState    string    `json:"new_state"`
	Health      string    `json:"health"`
	Error       error     `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// API response types
type ServiceStatus struct {
	Name        string                 `json:"name"`
	ServiceType string                 `json:"service_type"`
	State       ServiceState           `json:"state"`
	Health      HealthStatus           `json:"health"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ServiceListResponse struct {
	Services []ServiceStatus `json:"services"`
}

type MCPServiceInfo struct {
	Name     string `json:"name"`
	ModelID  string `json:"model_id"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
	State    string `json:"state"`
	Health   string `json:"health"`
	Error    string `json:"error,omitempty"`
}

type MCPServiceListResponse struct {
	Services []MCPServiceInfo `json:"services"`
}

type AggregatorData struct {
	Services        map[string]interface{} `json:"services"`
	LastUpdateTime  time.Time              `json:"last_update_time"`
	UpdateFrequency string                 `json:"update_frequency"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// CallToolResult represents the result of a tool/capability call
type CallToolResult struct {
	Content []interface{} `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Legacy types - moved to consolidated files
// CapabilityInfo, OperationInfo, WorkflowInfo moved to capability.go and workflow.go

// WorkflowDefinition and WorkflowStep moved to workflow.go and shared.go

// ToolCaller defines the interface for calling tools
type ToolCaller interface {
	CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error)
}

// ToolMetadata describes a tool that can be exposed
type ToolMetadata struct {
	Name        string // e.g., "workflow_list", "action_login", "auth_login"
	Description string
	Parameters  []ParameterMetadata
}

// ParameterMetadata describes a tool parameter
type ParameterMetadata struct {
	Name        string
	Type        string // "string", "number", "boolean", "object"
	Required    bool
	Description string
	Default     interface{}
}

// ToolProvider interface - implemented by workflow and capability packages
type ToolProvider interface {
	// Returns all tools this provider offers
	GetTools() []ToolMetadata

	// Executes a tool by name
	ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*CallToolResult, error)
}

// ToolUpdateSubscriber interface for components that want to receive tool update events
type ToolUpdateSubscriber interface {
	OnToolsUpdated(event ToolUpdateEvent)
}
