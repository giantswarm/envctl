package api

import (
	"time"
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
