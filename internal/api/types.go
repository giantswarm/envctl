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
	Type        string    `json:"type"`        // "server_registered", "server_deregistered", "tools_updated"
	ServerName  string    `json:"server_name"`
	Tools       []string  `json:"tools"`       // List of tool names
	Timestamp   time.Time `json:"timestamp"`
	Error       string    `json:"error,omitempty"`
}

// MCPResource represents an MCP resource
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Service states
type ServiceState string

const (
	StateStopped  ServiceState = "stopped"
	StateStarting ServiceState = "starting"
	StateRunning  ServiceState = "running"
	StateStopping ServiceState = "stopping"
	StateError    ServiceState = "error"
	StateFailed   ServiceState = "Failed"
)

// Health statuses
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthChecking  HealthStatus = "Checking"
)

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
	Label       string    `json:"label"`
	ServiceType string    `json:"service_type"`
	OldState    string    `json:"old_state"`
	NewState    string    `json:"new_state"`
	Health      string    `json:"health"`
	Error       error     `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// API response types
type ServiceStatus struct {
	Label       string                 `json:"label"`
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
	Label        string `json:"label"`
	ModelID      string `json:"model_id"`
	Provider     string `json:"provider"`
	URL          string `json:"url"`
	State        string `json:"state"`
	Health       string `json:"health"`
	Error        string `json:"error,omitempty"`
	ClusterLabel string `json:"cluster_label,omitempty"`
}

type MCPServiceListResponse struct {
	Services []MCPServiceInfo `json:"services"`
}

type PortForwardInfo struct {
	Label        string `json:"label"`
	ClusterLabel string `json:"cluster_label"`
	Namespace    string `json:"namespace"`
	ServiceName  string `json:"service_name"`
	LocalPort    int    `json:"local_port"`
	RemotePort   int    `json:"remote_port"`
	State        string `json:"state"`
	Health       string `json:"health"`
	Error        string `json:"error,omitempty"`
}

type PortForwardListResponse struct {
	PortForwards []PortForwardInfo `json:"port_forwards"`
}

type K8sServiceInfo struct {
	Label        string                 `json:"label"`
	ClusterLabel string                 `json:"cluster_label"`
	State        string                 `json:"state"`
	Health       string                 `json:"health"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type K8sServiceListResponse struct {
	Services []K8sServiceInfo `json:"services"`
}

type ClusterSwitchRequest struct {
	ClusterName string `json:"cluster_name"`
}

type ClusterListResponse struct {
	Clusters []ClusterDefinition `json:"clusters"`
	Active   string              `json:"active,omitempty"`
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

// CapabilityInfo provides information about a capability
type CapabilityInfo struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Version     string          `json:"version"`
	Operations  []OperationInfo `json:"operations"`
}

// OperationInfo provides information about a capability operation
type OperationInfo struct {
	Name        string
	Description string
	Available   bool
}

// WorkflowInfo provides information about a workflow
type WorkflowInfo struct {
	Name        string
	Description string
	Version     string
}

// WorkflowDefinition represents a complete workflow definition
type WorkflowDefinition struct {
	Name         string                 `yaml:"name"`
	Description  string                 `yaml:"description"`
	Version      string                 `yaml:"version,omitempty"`
	InputSchema  map[string]interface{} `yaml:"inputSchema,omitempty"`
	Steps        []WorkflowStep         `yaml:"steps"`
	OutputSchema map[string]interface{} `yaml:"outputSchema,omitempty"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID          string                 `yaml:"id"`
	Tool        string                 `yaml:"tool"`
	Args        map[string]interface{} `yaml:"args,omitempty"`
	Store       string                 `yaml:"store,omitempty"`
	Condition   string                 `yaml:"condition,omitempty"`
	Description string                 `yaml:"description,omitempty"`
}

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
