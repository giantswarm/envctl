package api

import (
	"context"
	"encoding/json"
	"envctl/internal/reporting"
	"time"
)

// MCPServerAPI provides access to MCP server functionality
type MCPServerAPI interface {
	// GetTools returns the list of tools exposed by an MCP server
	GetTools(ctx context.Context, serverName string) ([]MCPTool, error)

	// GetToolDetails returns detailed information about a specific tool
	GetToolDetails(ctx context.Context, serverName string, toolName string) (*MCPToolDetails, error)

	// ExecuteTool executes a tool and returns the result
	ExecuteTool(ctx context.Context, serverName string, toolName string, params map[string]interface{}) (interface{}, error)

	// GetServerStatus returns the current status of an MCP server
	GetServerStatus(serverName string) (*MCPServerStatus, error)

	// SubscribeToToolUpdates subscribes to tool list changes
	SubscribeToToolUpdates(serverName string) <-chan MCPToolUpdateEvent
}

// MCPTool represents a tool exposed by an MCP server
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPToolDetails includes full details about a tool
type MCPToolDetails struct {
	MCPTool
	Examples    []MCPToolExample `json:"examples,omitempty"`
	LastUpdated time.Time        `json:"lastUpdated"`
}

// MCPToolExample represents an example usage of a tool
type MCPToolExample struct {
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output,omitempty"`
}

// MCPServerStatus represents the status of an MCP server
type MCPServerStatus struct {
	Name      string                 `json:"name"`
	State     reporting.ServiceState `json:"state"`
	ProxyPort int                    `json:"proxyPort"`
	ToolCount int                    `json:"toolCount"`
	LastCheck time.Time              `json:"lastCheck"`
	Error     error                  `json:"error,omitempty"`
}

// MCPToolUpdateEvent represents a change in MCP server tools
type MCPToolUpdateEvent struct {
	ServerName string    `json:"serverName"`
	EventType  string    `json:"eventType"` // "added", "removed", "updated", "refreshed"
	Tools      []MCPTool `json:"tools"`
	Timestamp  time.Time `json:"timestamp"`
}

// KubernetesAPI provides Kubernetes-specific functionality
type KubernetesAPI interface {
	// GetClusterHealth returns health information for a cluster
	GetClusterHealth(ctx context.Context, contextName string) (*ClusterHealth, error)

	// GetNamespaces returns list of namespaces
	GetNamespaces(ctx context.Context, contextName string) ([]string, error)

	// GetResources returns resources in a namespace
	GetResources(ctx context.Context, contextName string, namespace string, resourceType string) ([]Resource, error)

	// SubscribeToHealthUpdates subscribes to cluster health changes
	SubscribeToHealthUpdates(contextName string) <-chan ClusterHealthEvent

	// StartHealthMonitoring starts health monitoring for a specific context
	StartHealthMonitoring(contextName string, interval time.Duration) error

	// StopHealthMonitoring stops health monitoring for a specific context
	StopHealthMonitoring(contextName string) error
}

// ClusterHealth represents the health status of a Kubernetes cluster
type ClusterHealth struct {
	ContextName    string    `json:"contextName"`
	IsHealthy      bool      `json:"isHealthy"`
	NodeCount      int       `json:"nodeCount"`
	ReadyNodeCount int       `json:"readyNodeCount"`
	LastCheck      time.Time `json:"lastCheck"`
	Error          error     `json:"error,omitempty"`
}

// Resource represents a Kubernetes resource
type Resource struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Kind      string                 `json:"kind"`
	Status    string                 `json:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ClusterHealthEvent represents a change in cluster health
type ClusterHealthEvent struct {
	ContextName string        `json:"contextName"`
	OldHealth   ClusterHealth `json:"oldHealth"`
	NewHealth   ClusterHealth `json:"newHealth"`
	Timestamp   time.Time     `json:"timestamp"`
}

// PortForwardAPI provides port forwarding functionality
type PortForwardAPI interface {
	// GetActiveForwards returns all active port forwards
	GetActiveForwards() []PortForwardInfo

	// GetForwardMetrics returns metrics for a port forward
	GetForwardMetrics(name string) (*PortForwardMetrics, error)

	// TestConnection tests if a port forward is working
	TestConnection(ctx context.Context, name string) error
}

// PortForwardInfo represents information about an active port forward
type PortForwardInfo struct {
	Name        string                 `json:"name"`
	LocalPort   int                    `json:"localPort"`
	RemotePort  int                    `json:"remotePort"`
	Namespace   string                 `json:"namespace"`
	TargetType  string                 `json:"targetType"`
	TargetName  string                 `json:"targetName"`
	State       reporting.ServiceState `json:"state"`
	StartedAt   time.Time              `json:"startedAt"`
	LastChecked time.Time              `json:"lastChecked"`
}

// PortForwardMetrics represents metrics for a port forward
type PortForwardMetrics struct {
	Name              string        `json:"name"`
	BytesSent         int64         `json:"bytesSent"`
	BytesReceived     int64         `json:"bytesReceived"`
	ActiveConnections int           `json:"activeConnections"`
	TotalConnections  int64         `json:"totalConnections"`
	LastActivity      time.Time     `json:"lastActivity"`
	Uptime            time.Duration `json:"uptime"`
}
