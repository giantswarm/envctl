package api

import (
	"context"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"sync"
)

// Handler interfaces that services will implement

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

// OrchestratorHandler manages service lifecycle and clusters
type OrchestratorHandler interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
	GetAvailableClusters(role ClusterRole) []ClusterDefinition
	GetActiveCluster(role ClusterRole) (string, bool)
	SwitchCluster(role ClusterRole, clusterName string) error
	GetServiceStatus(label string) (*ServiceStatus, error)
	GetAllServices() []ServiceStatus
	ToolProvider
}

// AggregatorHandler provides aggregator-specific functionality
type AggregatorHandler interface {
	GetServiceData() map[string]interface{}
	GetEndpoint() string
	GetPort() int
}

// MCPServiceHandler provides MCP service-specific functionality
type MCPServiceHandler interface {
	GetModelID() string
	GetProvider() string
	GetURL() string
	GetClusterLabel() string
	GetMCPTools() []MCPTool
	GetResources() []MCPResource
	ListServers(ctx context.Context) ([]*MCPServerInfo, error)
	GetServerInfo(ctx context.Context, label string) (*MCPServerInfo, error)
	GetServerTools(ctx context.Context, serverName string) ([]MCPTool, error)
	ToolProvider
}

// K8sServiceHandler provides Kubernetes service-specific functionality
type K8sServiceHandler interface {
	GetClusterLabel() string
	GetMetadata() map[string]interface{}
	ListConnections(ctx context.Context) ([]*K8sConnectionInfo, error)
	GetConnectionInfo(ctx context.Context, label string) (*K8sConnectionInfo, error)
	GetConnectionByContext(ctx context.Context, contextName string) (*K8sConnectionInfo, error)
	ToolProvider
}

// ConfigHandler provides configuration management functionality
type ConfigHandler interface {
	// Get configuration
	GetConfig(ctx context.Context) (*config.EnvctlConfig, error)
	GetClusters(ctx context.Context) ([]config.ClusterDefinition, error)
	GetActiveClusters(ctx context.Context) (map[config.ClusterRole]string, error)
	GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error)
	GetPortForwards(ctx context.Context) ([]config.PortForwardDefinition, error)
	GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error)
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration
	UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error
	UpdatePortForward(ctx context.Context, portForward config.PortForwardDefinition) error
	UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

	// Delete configuration
	DeleteMCPServer(ctx context.Context, name string) error
	DeletePortForward(ctx context.Context, name string) error
	DeleteWorkflow(ctx context.Context, name string) error
	DeleteCluster(ctx context.Context, name string) error

	// Save configuration
	SaveConfig(ctx context.Context) error
	ToolProvider
}

// CapabilityHandler defines the interface for capability operations
type CapabilityHandler interface {
	// ExecuteCapability executes a capability operation
	ExecuteCapability(ctx context.Context, capabilityType, operation string, params map[string]interface{}) (*CallToolResult, error)

	// IsCapabilityAvailable checks if a capability operation is available
	IsCapabilityAvailable(capabilityType, operation string) bool

	// ListCapabilities returns information about all available capabilities
	ListCapabilities() []CapabilityInfo

	// Embed ToolProvider for tool generation
	ToolProvider
}

// WorkflowHandler defines the interface for workflow operations
type WorkflowHandler interface {
	// ExecuteWorkflow executes a workflow
	ExecuteWorkflow(ctx context.Context, workflowName string, args map[string]interface{}) (*CallToolResult, error)

	// GetWorkflows returns information about all workflows
	GetWorkflows() []WorkflowInfo

	// GetWorkflow returns a specific workflow definition
	GetWorkflow(name string) (*WorkflowDefinition, error)

	// CreateWorkflow creates a new workflow from YAML
	CreateWorkflow(yamlStr string) error

	// UpdateWorkflow updates an existing workflow
	UpdateWorkflow(name, yamlStr string) error

	// DeleteWorkflow deletes a workflow
	DeleteWorkflow(name string) error

	// ValidateWorkflow validates a workflow YAML
	ValidateWorkflow(yamlStr string) error

	// Tool availability for workflows
	ToolCaller

	// Embed ToolProvider for tool generation
	ToolProvider
}

// PortForwardServiceHandler defines the interface for port forward operations
type PortForwardServiceHandler interface {
	// Service-specific functionality
	GetClusterLabel() string
	GetNamespace() string
	GetServiceName() string
	GetLocalPort() int
	GetRemotePort() int
	
	// List all port forwards
	ListForwards(ctx context.Context) ([]*PortForwardInfo, error)

	// Get info about a specific port forward
	GetForwardInfo(ctx context.Context, label string) (*PortForwardInfo, error)

	// Embed ToolProvider for tool generation
	ToolProvider
}

// Handler registry
var (
	registryHandler     ServiceRegistryHandler
	orchestratorHandler OrchestratorHandler
	aggregatorHandler   AggregatorHandler
	configHandler       ConfigHandler
	capabilityHandler   CapabilityHandler
	workflowHandler     WorkflowHandler

	// Maps for service-specific handlers
	mcpHandlers         = make(map[string]MCPServiceHandler)
	portForwardHandlers = make(map[string]PortForwardServiceHandler)
	k8sHandlers         = make(map[string]K8sServiceHandler)

	handlerMutex sync.RWMutex
)

// RegisterServiceRegistry registers the service registry handler
func RegisterServiceRegistry(h ServiceRegistryHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	registryHandler = h
}

// RegisterOrchestrator registers the orchestrator handler
func RegisterOrchestrator(h OrchestratorHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering orchestrator handler: %v", h != nil)
	orchestratorHandler = h
}

// RegisterAggregator registers the aggregator handler
func RegisterAggregator(h AggregatorHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	aggregatorHandler = h
}

// RegisterMCPService registers an MCP service handler
func RegisterMCPService(label string, h MCPServiceHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	mcpHandlers[label] = h
}

// RegisterPortForward registers a port forward handler
func RegisterPortForward(label string, h PortForwardServiceHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	portForwardHandlers[label] = h
}

// RegisterK8sService registers a K8s service handler
func RegisterK8sService(label string, h K8sServiceHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	k8sHandlers[label] = h
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

// RegisterMCPServiceHandler registers a global MCP service handler
func RegisterMCPServiceHandler(h MCPServiceHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering MCP service handler: %v", h != nil)
	// Store it as a special global handler
	mcpHandlers["__global__"] = h
}

// RegisterK8sServiceHandler registers a global K8s service handler
func RegisterK8sServiceHandler(h K8sServiceHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering K8s service handler: %v", h != nil)
	// Store it as a special global handler
	k8sHandlers["__global__"] = h
}

// RegisterPortForwardServiceHandler registers a global port forward service handler
func RegisterPortForwardServiceHandler(h PortForwardServiceHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering port forward service handler: %v", h != nil)
	// Store it as a special global handler
	portForwardHandlers["__global__"] = h
}

// GetServiceRegistry returns the registered service registry handler
func GetServiceRegistry() ServiceRegistryHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return registryHandler
}

// GetOrchestrator returns the registered orchestrator handler
func GetOrchestrator() OrchestratorHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return orchestratorHandler
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

// GetMCPServiceHandler returns the global MCP service handler
func GetMCPServiceHandler() MCPServiceHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, _ := mcpHandlers["__global__"]
	return h
}

// GetK8sServiceHandler returns the global K8s service handler
func GetK8sServiceHandler() K8sServiceHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, _ := k8sHandlers["__global__"]
	return h
}

// GetPortForwardServiceHandler returns the global port forward service handler
func GetPortForwardServiceHandler() PortForwardServiceHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	// First check for global handler
	if h, ok := portForwardHandlers["__global__"]; ok {
		return h
	}
	return nil
}

// GetMCPService returns a registered MCP service handler
func GetMCPService(label string) (MCPServiceHandler, bool) {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, ok := mcpHandlers[label]
	return h, ok
}

// GetPortForward returns a registered port forward handler
func GetPortForward(label string) (PortForwardServiceHandler, bool) {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, ok := portForwardHandlers[label]
	return h, ok
}

// GetK8sService returns a registered K8s service handler
func GetK8sService(label string) (K8sServiceHandler, bool) {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, ok := k8sHandlers[label]
	return h, ok
}

// UnregisterMCPService removes an MCP service handler
func UnregisterMCPService(label string) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	delete(mcpHandlers, label)
}

// UnregisterPortForward removes a port forward handler
func UnregisterPortForward(label string) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	delete(portForwardHandlers, label)
}

// UnregisterK8sService removes a K8s service handler
func UnregisterK8sService(label string) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	delete(k8sHandlers, label)
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

// CreateWorkflow is a convenience function for creating workflows
func CreateWorkflow(yamlDefinition string) error {
	handler := GetWorkflow()
	if handler == nil {
		return fmt.Errorf("workflow handler not registered")
	}

	// Validate the workflow
	if err := handler.ValidateWorkflow(yamlDefinition); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	// Create the workflow
	return handler.CreateWorkflow(yamlDefinition)
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
func ListCapabilities() []CapabilityInfo {
	handler := GetCapability()
	if handler == nil {
		return nil
	}
	return handler.ListCapabilities()
}

// GetWorkflowInfo returns information about all workflows
func GetWorkflowInfo() []WorkflowInfo {
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
			for _, op := range cap.Operations {
				if op.Name == parts[1] {
					return parts[0], parts[1], true
				}
			}
		}
	}

	return "", "", false
}
