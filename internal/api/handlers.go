package api

import (
	"context"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler interfaces that services will implement

// ServiceClass and ServiceInstance types moved to serviceclass.go and serviceinstance.go

// ServiceInfo provides information about a service
type ServiceInfo interface {
	GetLabel() string
	GetType() ServiceType
	GetState() ServiceState
	GetHealth() HealthStatus
	GetLastError() error
	GetServiceData() map[string]interface{}
}

// ServiceRegistryHandler provides access to registered services
type ServiceRegistryHandler interface {
	Get(label string) (ServiceInfo, bool)
	GetAll() []ServiceInfo
	GetByType(serviceType ServiceType) []ServiceInfo
}

// ServiceManagerHandler provides unified management for both static and ServiceClass-based services
type ServiceManagerHandler interface {
	// Unified service lifecycle management (works for both static and ServiceClass-based services)
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error

	// Service information and status
	GetServiceStatus(label string) (*ServiceStatus, error)
	GetAllServices() []ServiceStatus
	GetService(labelOrServiceID string) (*ServiceInstance, error) // Get detailed service info by label or serviceID

	// ServiceClass instance creation and deletion (only for ServiceClass-based services)
	CreateService(ctx context.Context, req CreateServiceInstanceRequest) (*ServiceInstance, error)
	DeleteService(ctx context.Context, labelOrServiceID string) error // Delete by label or serviceID

	// Event subscriptions
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
	SubscribeToServiceInstanceEvents() <-chan ServiceInstanceEvent

	ToolProvider
}

// AggregatorHandler provides aggregator-specific functionality
type AggregatorHandler interface {
	GetServiceData() map[string]interface{}
	GetEndpoint() string
	GetPort() int

	// Tool calling methods
	CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*CallToolResult, error)
	CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error)
	IsToolAvailable(toolName string) bool
	GetAvailableTools() []string
}

// ConfigHandler provides configuration management functionality
type ConfigHandler interface {
	// Get configuration
	GetConfig(ctx context.Context) (*config.EnvctlConfig, error)
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

	// Save configuration
	SaveConfig(ctx context.Context) error

	// Reload configuration from disk
	ReloadConfig(ctx context.Context) error

	ToolProvider
}

// CapabilityHandler defines the interface for capability operations
type CapabilityHandler interface {
	// Capability execution
	ExecuteCapability(ctx context.Context, capabilityType, operation string, params map[string]interface{}) (*CallToolResult, error)

	// Capability information and availability
	ListCapabilities() []Capability
	GetCapability(name string) (interface{}, error)
	IsCapabilityAvailable(capabilityType, operation string) bool

	// Embed ToolProvider for tool generation
	ToolProvider
}

// WorkflowHandler defines the interface for workflow operations
type WorkflowHandler interface {
	// ExecuteWorkflow executes a workflow
	ExecuteWorkflow(ctx context.Context, workflowName string, args map[string]interface{}) (*CallToolResult, error)

	// GetWorkflows returns information about all workflows
	GetWorkflows() []Workflow

	// GetWorkflow returns a specific workflow definition
	GetWorkflow(name string) (*Workflow, error)

	// CreateWorkflowFromStructured creates a new workflow from structured parameters
	CreateWorkflowFromStructured(args map[string]interface{}) error

	// UpdateWorkflowFromStructured updates an existing workflow from structured parameters
	UpdateWorkflowFromStructured(name string, args map[string]interface{}) error

	// DeleteWorkflow deletes a workflow
	DeleteWorkflow(name string) error

	// ValidateWorkflowFromStructured validates a workflow definition from structured parameters
	ValidateWorkflowFromStructured(args map[string]interface{}) error

	// Tool availability for workflows
	ToolCaller

	// Embed ToolProvider for tool generation
	ToolProvider
}

// ServiceClassManagerHandler defines the interface for service class management operations
type ServiceClassManagerHandler interface {
	// Service class definition management
	ListServiceClasses() []ServiceClass
	GetServiceClass(name string) (*ServiceClass, error)
	IsServiceClassAvailable(name string) bool

	// Lifecycle tool access (for service orchestration without direct coupling)
	GetStartTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetStopTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetRestartTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetHealthCheckTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetHealthCheckConfig(name string) (enabled bool, interval time.Duration, failureThreshold, successThreshold int, err error)
	GetServiceDependencies(name string) ([]string, error)

	// Tool provider interface for exposing ServiceClass management tools
	ToolProvider
}

// MCPServerInfo contains consolidated MCP server information (API response)
type MCPServerInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Label       string            `json:"label"`
	State       string            `json:"state"`
	Health      string            `json:"health"`
	Enabled     bool              `json:"enabled"`
	Available   bool              `json:"available"`
	Description string            `json:"description,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Image       string            `json:"image,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// MCPServerManagerHandler defines the interface for MCP server management operations
type MCPServerManagerHandler interface {
	// MCP server definition management
	ListMCPServers() []MCPServerInfo
	GetMCPServer(name string) (*MCPServerInfo, error)
	IsMCPServerAvailable(name string) bool

	// Tool provider interface for exposing MCP server management tools
	ToolProvider
}

// Handler registry
var (
	registryHandler            ServiceRegistryHandler
	serviceManagerHandler      ServiceManagerHandler
	serviceClassManagerHandler ServiceClassManagerHandler
	mcpServerManagerHandler    MCPServerManagerHandler
	aggregatorHandler          AggregatorHandler
	configHandler              ConfigHandler
	capabilityHandler          CapabilityHandler
	workflowHandler            WorkflowHandler

	// Tool update subscribers
	toolUpdateSubscribers []ToolUpdateSubscriber
	toolUpdateMutex       sync.Mutex

	handlerMutex sync.RWMutex
)

// RegisterServiceRegistry registers the service registry handler
func RegisterServiceRegistry(h ServiceRegistryHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	registryHandler = h
}

// RegisterServiceManager registers the service manager handler
func RegisterServiceManager(h ServiceManagerHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering service manager handler: %v", h != nil)
	serviceManagerHandler = h
}

// RegisterAggregator registers the aggregator handler
func RegisterAggregator(h AggregatorHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	aggregatorHandler = h
}

// RegisterConfigHandler registers the configuration handler
func RegisterConfigHandler(h ConfigHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	configHandler = h
}

// RegisterConfig registers a config handler (alias for RegisterConfigHandler)
func RegisterConfig(h ConfigHandler) {
	RegisterConfigHandler(h)
}

// GetServiceRegistry returns the registered service registry handler
func GetServiceRegistry() ServiceRegistryHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return registryHandler
}

// GetServiceManager returns the registered service manager handler
func GetServiceManager() ServiceManagerHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return serviceManagerHandler
}

// GetAggregator returns the registered aggregator handler
func GetAggregator() AggregatorHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return aggregatorHandler
}

// GetConfigHandler returns the registered configuration handler
func GetConfigHandler() ConfigHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return configHandler
}

// GetConfig returns the registered config handler (alias for GetConfigHandler)
func GetConfig() ConfigHandler {
	return GetConfigHandler()
}

// RegisterCapability registers the capability handler
func RegisterCapability(h CapabilityHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering capability handler: %v", h != nil)
	capabilityHandler = h
}

// GetCapability returns the registered capability handler
func GetCapability() CapabilityHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return capabilityHandler
}

// RegisterWorkflow registers the workflow handler
func RegisterWorkflow(h WorkflowHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering workflow handler: %v", h != nil)
	workflowHandler = h
}

// GetWorkflow returns the registered workflow handler
func GetWorkflow() WorkflowHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return workflowHandler
}

// RegisterServiceClassManager registers the service class manager handler
func RegisterServiceClassManager(h ServiceClassManagerHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering service class manager handler: %v", h != nil)
	serviceClassManagerHandler = h
}

// GetServiceClassManager returns the registered service class manager handler
func GetServiceClassManager() ServiceClassManagerHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return serviceClassManagerHandler
}

// SetServiceClassManagerForTesting sets the service class manager handler for testing purposes
func SetServiceClassManagerForTesting(h ServiceClassManagerHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	serviceClassManagerHandler = h
}

// ExecuteCapability is a convenience function for executing capabilities
func ExecuteCapability(ctx context.Context, capabilityType, operation string, params map[string]interface{}) (*CallToolResult, error) {
	handler := GetCapability()
	if handler == nil {
		return nil, fmt.Errorf("capability handler not registered")
	}
	return handler.ExecuteCapability(ctx, capabilityType, operation, params)
}

// ExecuteWorkflow is a convenience function for executing workflows
func ExecuteWorkflow(ctx context.Context, workflowName string, args map[string]interface{}) (*CallToolResult, error) {
	handler := GetWorkflow()
	if handler == nil {
		return nil, fmt.Errorf("workflow handler not registered")
	}
	return handler.ExecuteWorkflow(ctx, workflowName, args)
}

// IsCapabilityAvailable checks if a capability operation is available
func IsCapabilityAvailable(capabilityType, operation string) bool {
	handler := GetCapability()
	if handler == nil {
		return false
	}
	return handler.IsCapabilityAvailable(capabilityType, operation)
}

// ListCapabilities returns information about all available capabilities
func ListCapabilities() []Capability {
	handler := GetCapability()
	if handler == nil {
		return nil
	}
	return handler.ListCapabilities()
}

// GetWorkflowInfo returns information about all workflows
func GetWorkflowInfo() []Workflow {
	handler := GetWorkflow()
	if handler == nil {
		return nil
	}
	return handler.GetWorkflows()
}

// ToolNameToCapability converts a tool name to capability type and operation
func ToolNameToCapability(toolName string) (capabilityType, operation string, isCapability bool) {
	// Remove prefix if present
	toolName = strings.TrimPrefix(toolName, "x_")

	// Check if it's a capability tool (format: type_operation)
	parts := strings.SplitN(toolName, "_", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	// Check if this capability exists
	capabilities := ListCapabilities()
	for _, cap := range capabilities {
		if cap.Type == parts[0] {
			// Check if the operation exists in the operations map
			if _, exists := cap.Operations[parts[1]]; exists {
				return parts[0], parts[1], true
			}
		}
	}

	return "", "", false
}

// RegisterMCPServerManager registers the MCP server manager handler
func RegisterMCPServerManager(h MCPServerManagerHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering MCP server manager handler: %v", h != nil)
	mcpServerManagerHandler = h
}

// GetMCPServerManager returns the registered MCP server manager handler
func GetMCPServerManager() MCPServerManagerHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return mcpServerManagerHandler
}

// SetMCPServerManagerForTesting sets the MCP server manager handler for testing purposes
func SetMCPServerManagerForTesting(h MCPServerManagerHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	mcpServerManagerHandler = h
}

// SubscribeToToolUpdates allows managers to subscribe to tool update events
func SubscribeToToolUpdates(subscriber ToolUpdateSubscriber) {
	toolUpdateMutex.Lock()
	defer toolUpdateMutex.Unlock()
	toolUpdateSubscribers = append(toolUpdateSubscribers, subscriber)
	logging.Debug("API", "Added tool update subscriber, total subscribers: %d", len(toolUpdateSubscribers))
}

// PublishToolUpdateEvent publishes a tool update event to all subscribers
func PublishToolUpdateEvent(event ToolUpdateEvent) {
	toolUpdateMutex.Lock()
	subscribers := make([]ToolUpdateSubscriber, len(toolUpdateSubscribers))
	copy(subscribers, toolUpdateSubscribers)
	toolUpdateMutex.Unlock()

	logging.Debug("API", "Publishing tool update event: type=%s, server=%s, tools=%d, subscribers=%d",
		event.Type, event.ServerName, len(event.Tools), len(subscribers))

	for _, subscriber := range subscribers {
		// Call subscriber in goroutine to avoid blocking
		go func(s ToolUpdateSubscriber) {
			defer func() {
				if r := recover(); r != nil {
					logging.Error("API", fmt.Errorf("panic in tool update subscriber: %v", r), "Tool update subscriber panicked")
				}
			}()
			s.OnToolsUpdated(event)
		}(subscriber)
	}
}
