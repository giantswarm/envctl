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

// ServiceClassInfo provides information about a registered service class
type ServiceClassInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	Description string `json:"description"`
	ServiceType string `json:"serviceType"`
	Available   bool   `json:"available"`

	// Lifecycle tool availability
	CreateToolAvailable      bool `json:"createToolAvailable"`
	DeleteToolAvailable      bool `json:"deleteToolAvailable"`
	HealthCheckToolAvailable bool `json:"healthCheckToolAvailable"`
	StatusToolAvailable      bool `json:"statusToolAvailable"`

	// Required tools
	RequiredTools []string `json:"requiredTools"`
	MissingTools  []string `json:"missingTools"`
}

// ServiceClassDefinition represents a service class definition (lightweight version for API)
type ServiceClassDefinition struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
}

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

// OrchestratorHandler manages service lifecycle
type OrchestratorHandler interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
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

// ConfigHandler provides configuration management functionality
type ConfigHandler interface {
	// Get configuration
	GetConfig(ctx context.Context) (*config.EnvctlConfig, error)
	GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error)
	GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error)
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration
	UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error
	UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

	// Delete configuration
	DeleteMCPServer(ctx context.Context, name string) error
	DeleteWorkflow(ctx context.Context, name string) error

	// Save configuration
	SaveConfig(ctx context.Context) error

	// Reload configuration from disk
	ReloadConfig(ctx context.Context) error

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

// ServiceClassManagerHandler defines the interface for service class management operations
type ServiceClassManagerHandler interface {
	// Service class definition management
	ListServiceClasses() []ServiceClassInfo
	GetServiceClass(name string) (*ServiceClassDefinition, error)
	IsServiceClassAvailable(name string) bool
	LoadServiceDefinitions() error
	RefreshAvailability()

	// Service class registration (for programmatic definitions)
	RegisterDefinition(def *ServiceClassDefinition) error
	UnregisterDefinition(name string) error

	// Utility methods
	GetDefinitionsPath() string
}

// ServiceOrchestratorHandler defines the interface for service orchestrator operations
type ServiceOrchestratorHandler interface {
	// Service capability management
	ListServiceCapabilities() []ServiceCapabilityInfo
	GetServiceCapability(name string) (*ServiceCapabilityInfo, error)
	IsServiceCapabilityAvailable(name string) bool

	// Service instance management
	CreateService(ctx context.Context, req CreateServiceRequest) (*ServiceInstanceInfo, error)
	DeleteService(ctx context.Context, serviceID string) error
	GetService(serviceID string) (*ServiceInstanceInfo, error)
	GetServiceByLabel(label string) (*ServiceInstanceInfo, error)
	ListServices() []ServiceInstanceInfo

	// Service events
	SubscribeToServiceEvents() <-chan ServiceInstanceEvent

	// Tool provider interface
	ToolProvider
}

// Handler registry
var (
	registryHandler            ServiceRegistryHandler
	orchestratorHandler        OrchestratorHandler
	serviceOrchestratorHandler ServiceOrchestratorHandler
	serviceClassManagerHandler ServiceClassManagerHandler
	aggregatorHandler          AggregatorHandler
	configHandler              ConfigHandler
	capabilityHandler          CapabilityHandler
	workflowHandler            WorkflowHandler

	// Maps for service-specific handlers
	mcpHandlers = make(map[string]MCPServiceHandler)

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

// GetMCPService returns a registered MCP service handler
func GetMCPService(label string) (MCPServiceHandler, bool) {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, ok := mcpHandlers[label]
	return h, ok
}

// UnregisterMCPService removes an MCP service handler
func UnregisterMCPService(label string) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	delete(mcpHandlers, label)
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

// RegisterServiceOrchestrator registers the service orchestrator handler
func RegisterServiceOrchestrator(h ServiceOrchestratorHandler) {
	handlerMutex.Lock()
	defer handlerMutex.Unlock()
	logging.Debug("API", "Registering service orchestrator handler: %v", h != nil)
	serviceOrchestratorHandler = h
}

// GetServiceOrchestrator returns the registered service orchestrator handler
func GetServiceOrchestrator() ServiceOrchestratorHandler {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	return serviceOrchestratorHandler
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
