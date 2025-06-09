package api

import (
	"envctl/internal/config"
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
	GetTools() []MCPTool
	GetResources() []MCPResource
}

// PortForwardHandler provides port forward-specific functionality
type PortForwardHandler interface {
	GetClusterLabel() string
	GetNamespace() string
	GetServiceName() string
	GetLocalPort() int
	GetRemotePort() int
}

// K8sServiceHandler provides Kubernetes service-specific functionality
type K8sServiceHandler interface {
	GetClusterLabel() string
	GetMetadata() map[string]interface{}
}

// ConfigHandler provides configuration management functionality
type ConfigHandler interface {
	// Get configuration
	GetConfig() (*config.EnvctlConfig, error)

	// Update configuration sections
	UpdateClusters(clusters []config.ClusterDefinition) error
	UpdateActiveClusters(activeClusters map[config.ClusterRole]string) error
	UpdateMCPServer(server config.MCPServerDefinition) error
	UpdatePortForward(portForward config.PortForwardDefinition) error
	UpdateWorkflow(workflow config.WorkflowDefinition) error
	UpdateAggregatorConfig(aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(settings config.GlobalSettings) error

	// Delete configuration items
	DeleteMCPServer(name string) error
	DeletePortForward(name string) error
	DeleteWorkflow(name string) error
	DeleteCluster(name string) error

	// Save configuration
	SaveConfig() error
}

// Handler registry
var (
	registryHandler     ServiceRegistryHandler
	orchestratorHandler OrchestratorHandler
	aggregatorHandler   AggregatorHandler
	configHandler       ConfigHandler

	// Maps for service-specific handlers
	mcpHandlers         = make(map[string]MCPServiceHandler)
	portForwardHandlers = make(map[string]PortForwardHandler)
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
func RegisterPortForward(label string, h PortForwardHandler) {
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

// GetMCPService returns a registered MCP service handler
func GetMCPService(label string) (MCPServiceHandler, bool) {
	handlerMutex.RLock()
	defer handlerMutex.RUnlock()
	h, ok := mcpHandlers[label]
	return h, ok
}

// GetPortForward returns a registered port forward handler
func GetPortForward(label string) (PortForwardHandler, bool) {
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
