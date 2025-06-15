package api

import (
	"context"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"sync"
	"time"
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

// ServiceClass-based service management types (for unified orchestrator)

// CreateServiceClassRequest represents a request to create a new ServiceClass-based service instance
type CreateServiceClassRequest struct {
	// ServiceClass to use
	ServiceClassName string `json:"serviceClassName"`

	// Label for the service instance (must be unique)
	Label string `json:"label"`

	// Parameters for service creation
	Parameters map[string]interface{} `json:"parameters"`

	// Override default timeouts (future use)
	CreateTimeout *time.Duration `json:"createTimeout,omitempty"`
	DeleteTimeout *time.Duration `json:"deleteTimeout,omitempty"`
}

// ServiceClassInstanceInfo provides information about a ServiceClass-based service instance
type ServiceClassInstanceInfo struct {
	ServiceID          string                 `json:"serviceId"`
	Label              string                 `json:"label"`
	ServiceClassName   string                 `json:"serviceClassName"`
	ServiceClassType   string                 `json:"serviceClassType"`
	State              string                 `json:"state"`
	Health             string                 `json:"health"`
	LastError          string                 `json:"lastError,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
	LastChecked        *time.Time             `json:"lastChecked,omitempty"`
	ServiceData        map[string]interface{} `json:"serviceData,omitempty"`
	CreationParameters map[string]interface{} `json:"creationParameters"`
}

// ServiceClassInstanceEvent represents a ServiceClass-based service instance state change event
type ServiceClassInstanceEvent struct {
	ServiceID   string                 `json:"serviceId"`
	Label       string                 `json:"label"`
	ServiceType string                 `json:"serviceType"`
	OldState    string                 `json:"oldState"`
	NewState    string                 `json:"newState"`
	OldHealth   string                 `json:"oldHealth"`
	NewHealth   string                 `json:"newHealth"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
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

// OrchestratorHandler manages service lifecycle (both static and ServiceClass-based)
type OrchestratorHandler interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
	GetServiceStatus(label string) (*ServiceStatus, error)
	GetAllServices() []ServiceStatus

	// ServiceClass-based dynamic service instance management
	CreateServiceClassInstance(ctx context.Context, req CreateServiceClassRequest) (*ServiceClassInstanceInfo, error)
	DeleteServiceClassInstance(ctx context.Context, serviceID string) error
	GetServiceClassInstance(serviceID string) (*ServiceClassInstanceInfo, error)
	GetServiceClassInstanceByLabel(label string) (*ServiceClassInstanceInfo, error)
	ListServiceClassInstances() []ServiceClassInstanceInfo
	SubscribeToServiceInstanceEvents() <-chan ServiceClassInstanceEvent

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
	GetMCPServers(ctx context.Context) ([]MCPServerDefinition, error)
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration
	UpdateMCPServer(ctx context.Context, server MCPServerDefinition) error
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

	// Delete configuration
	DeleteMCPServer(ctx context.Context, name string) error

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

	// Lifecycle tool access (for service orchestration without direct coupling)
	GetStartTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetStopTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetRestartTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetHealthCheckTool(name string) (toolName string, arguments map[string]interface{}, responseMapping map[string]string, err error)
	GetHealthCheckConfig(name string) (enabled bool, interval time.Duration, failureThreshold, successThreshold int, err error)
	GetServiceDependencies(name string) ([]string, error)

	// Service class registration (for programmatic definitions)
	RegisterDefinition(def *ServiceClassDefinition) error
	UnregisterDefinition(name string) error

	// Utility methods
	GetDefinitionsPath() string

	// Tool provider interface for exposing ServiceClass management tools
	ToolProvider
}

// MCPServerConfigInfo provides information about a registered MCP server configuration
type MCPServerConfigInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Enabled     bool     `json:"enabled"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Available   bool     `json:"available"`
	Description string   `json:"description,omitempty"`
	Command     []string `json:"command,omitempty"`
	Image       string   `json:"image,omitempty"`
}

// MCPServerDefinition represents an MCP server definition (lightweight version for API)
type MCPServerDefinition struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Enabled     bool              `json:"enabled"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Description string            `json:"description,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Image       string            `json:"image,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// MCPServerManagerHandler defines the interface for MCP server management operations
type MCPServerManagerHandler interface {
	// MCP server definition management
	ListMCPServers() []MCPServerConfigInfo
	GetMCPServer(name string) (*MCPServerDefinition, error)
	IsMCPServerAvailable(name string) bool
	LoadDefinitions() error
	RefreshAvailability()

	// MCP server registration (for programmatic definitions)
	RegisterDefinition(def *MCPServerDefinition) error
	UnregisterDefinition(name string) error

	// Utility methods
	GetDefinitionsPath() string

	// Tool provider interface for exposing MCP server management tools
	ToolProvider
}

// Handler registry
var (
	registryHandler            ServiceRegistryHandler
	orchestratorHandler        OrchestratorHandler
	serviceClassManagerHandler ServiceClassManagerHandler
	mcpServerManagerHandler    MCPServerManagerHandler
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
