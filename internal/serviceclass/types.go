package serviceclass

import (
	"sync"
	"time"
)

// ServiceState represents the current state of a service
// This mirrors services.ServiceState to avoid import cycles
type ServiceState string

const (
	ServiceStateUnknown  ServiceState = "Unknown"
	ServiceStateWaiting  ServiceState = "Waiting"
	ServiceStateStarting ServiceState = "Starting"
	ServiceStateRunning  ServiceState = "Running"
	ServiceStateStopping ServiceState = "Stopping"
	ServiceStateStopped  ServiceState = "Stopped"
	ServiceStateFailed   ServiceState = "Failed"
	ServiceStateRetrying ServiceState = "Retrying"
)

// HealthStatus represents the health of a service
type HealthStatus string

const (
	// HealthStatusHealthy indicates the service is functioning normally
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusUnhealthy indicates the service has issues
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusUnknown indicates the health status is unknown
	HealthStatusUnknown HealthStatus = "unknown"
)

// ServiceClassDefinition defines a service class with lifecycle management capabilities
// This is the main structure that maps to ServiceClass YAML definitions
type ServiceClassDefinition struct {
	// Base metadata fields
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`

	// Service-specific lifecycle configuration
	ServiceConfig ServiceConfig `yaml:"serviceConfig"`

	// Operations for external API (maintains compatibility with existing capability system)
	Operations map[string]OperationDefinition `yaml:"operations"`
}

// ServiceConfig defines the service lifecycle management configuration
type ServiceConfig struct {
	// Service metadata
	ServiceType  string   `yaml:"serviceType"`  // Custom service type for this service class
	DefaultLabel string   `yaml:"defaultLabel"` // Default label pattern (can use templates)
	Dependencies []string `yaml:"dependencies"` // Default service dependencies

	// Lifecycle tool mappings - these tools will be called by the orchestrator
	LifecycleTools LifecycleTools `yaml:"lifecycleTools"`

	// Service behavior configuration
	HealthCheck HealthCheckConfig `yaml:"healthCheck"`
	Timeout     TimeoutConfig     `yaml:"timeout"`

	// Parameter mapping for service creation
	CreateParameters map[string]ParameterMapping `yaml:"createParameters"`
}

// LifecycleTools maps service lifecycle events to aggregator tools
type LifecycleTools struct {
	// Tool to call when starting the service (maps to Service.Start)
	Start ToolCall `yaml:"start"`

	// Tool to call when stopping the service (maps to Service.Stop)
	Stop ToolCall `yaml:"stop"`

	// Tool to call for restarting the service (optional, maps to Service.Restart)
	Restart *ToolCall `yaml:"restart,omitempty"`

	// Tool to call for health checks (optional)
	HealthCheck *ToolCall `yaml:"healthCheck,omitempty"`

	// Tool to call to get service status/info (optional)
	Status *ToolCall `yaml:"status,omitempty"`
}

// ToolCall defines how to call an aggregator tool for a lifecycle event
type ToolCall struct {
	// Tool name in the aggregator (e.g., "api_kubernetes_port_forward")
	Tool string `yaml:"tool"`

	// Parameter mapping from service creation parameters to tool arguments
	Arguments map[string]interface{} `yaml:"arguments"`

	// Expected response handling
	ResponseMapping ResponseMapping `yaml:"responseMapping"`
}

// ResponseMapping defines how to extract information from tool responses
type ResponseMapping struct {
	// JSON path to extract service ID from response (for tracking)
	ServiceID string `yaml:"serviceId,omitempty"`

	// JSON path to extract status information
	Status string `yaml:"status,omitempty"`

	// JSON path to extract health information
	Health string `yaml:"health,omitempty"`

	// JSON path to extract error information
	Error string `yaml:"error,omitempty"`

	// Additional data to store in service metadata
	Metadata map[string]string `yaml:"metadata,omitempty"`
}

// ParameterMapping defines how service creation parameters map to tool arguments
type ParameterMapping struct {
	// The tool parameter name this maps to
	ToolParameter string `yaml:"toolParameter"`

	// Default value if not provided
	Default interface{} `yaml:"default,omitempty"`

	// Whether this parameter is required
	Required bool `yaml:"required"`

	// Transform function name (for complex mappings)
	Transform string `yaml:"transform,omitempty"`
}

// HealthCheckConfig defines health checking behavior
type HealthCheckConfig struct {
	// Whether health checking is enabled
	Enabled bool `yaml:"enabled"`

	// Interval between health checks
	Interval time.Duration `yaml:"interval"`

	// Number of failed checks before marking unhealthy
	FailureThreshold int `yaml:"failureThreshold"`

	// Number of successful checks to mark healthy again
	SuccessThreshold int `yaml:"successThreshold"`
}

// TimeoutConfig defines timeout behavior for operations
type TimeoutConfig struct {
	// Timeout for create operations
	Create time.Duration `yaml:"create"`

	// Timeout for delete operations
	Delete time.Duration `yaml:"delete"`

	// Timeout for health check operations
	HealthCheck time.Duration `yaml:"healthCheck"`
}

// OperationDefinition defines an operation that can be performed with a service class
// This maintains compatibility with the existing capability system
type OperationDefinition struct {
	Description string               `yaml:"description"`
	Parameters  map[string]Parameter `yaml:"parameters"`
	Requires    []string             `yaml:"requires"`
	Workflow    *WorkflowDefinition  `yaml:"workflow,omitempty"`
}

// Parameter defines a parameter for an operation
type Parameter struct {
	Type        string      `yaml:"type"`
	Required    bool        `yaml:"required"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default,omitempty"`
}

// WorkflowDefinition defines a workflow for an operation
type WorkflowDefinition struct {
	Name            string                 `yaml:"name"`
	Description     string                 `yaml:"description"`
	AgentModifiable bool                   `yaml:"agentModifiable"`
	InputSchema     map[string]interface{} `yaml:"inputSchema"`
	Steps           []WorkflowStep         `yaml:"steps"`
}

// WorkflowStep defines a step in a workflow
type WorkflowStep struct {
	ID    string                 `yaml:"id"`
	Tool  string                 `yaml:"tool"`
	Args  map[string]interface{} `yaml:"args"`
	Store string                 `yaml:"store"`
}

// ServiceInstance represents a runtime instance of a service created from a service class
type ServiceInstance struct {
	// Instance identification
	ID    string `json:"id"`    // Unique instance ID
	Label string `json:"label"` // Service label (from orchestrator)

	// Service class reference
	ServiceClassName string `json:"serviceClassName"` // Which service class created this
	ServiceClassType string `json:"serviceClassType"` // Type of service class

	// Service state
	State     ServiceState `json:"state"`
	Health    HealthStatus `json:"health"`
	LastError string       `json:"lastError,omitempty"`

	// Creation and runtime data
	CreationParameters map[string]interface{} `json:"creationParameters"` // Parameters used to create
	ServiceData        map[string]interface{} `json:"serviceData"`        // Runtime data from tools

	// Lifecycle tracking
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	LastChecked *time.Time `json:"lastChecked,omitempty"`

	// Health tracking
	HealthCheckFailures  int `json:"healthCheckFailures"`
	HealthCheckSuccesses int `json:"healthCheckSuccesses"`

	// Dependencies (from service class definition or runtime)
	Dependencies []string `json:"dependencies"`
}

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

// ServiceInstanceState provides state management for service instances
type ServiceInstanceState struct {
	// In-memory state
	instances map[string]*ServiceInstance // ID -> instance
	byLabel   map[string]*ServiceInstance // label -> instance

	// Synchronization
	mu *sync.RWMutex
}

// NewServiceInstanceState creates a new service instance state manager
func NewServiceInstanceState() *ServiceInstanceState {
	return &ServiceInstanceState{
		instances: make(map[string]*ServiceInstance),
		byLabel:   make(map[string]*ServiceInstance),
		mu:        &sync.RWMutex{},
	}
}

// CreateInstance creates a new service instance
func (s *ServiceInstanceState) CreateInstance(id, label, serviceClassName, serviceClassType string, parameters map[string]interface{}) *ServiceInstance {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance := &ServiceInstance{
		ID:                   id,
		Label:                label,
		ServiceClassName:     serviceClassName,
		ServiceClassType:     serviceClassType,
		State:                ServiceStateUnknown,
		Health:               HealthStatusUnknown,
		CreationParameters:   parameters,
		ServiceData:          make(map[string]interface{}),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		HealthCheckFailures:  0,
		HealthCheckSuccesses: 0,
		Dependencies:         []string{},
	}

	s.instances[id] = instance
	s.byLabel[label] = instance

	return instance
}

// GetInstance retrieves a service instance by ID
func (s *ServiceInstanceState) GetInstance(id string) (*ServiceInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, exists := s.instances[id]
	return instance, exists
}

// GetInstanceByLabel retrieves a service instance by label
func (s *ServiceInstanceState) GetInstanceByLabel(label string) (*ServiceInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, exists := s.byLabel[label]
	return instance, exists
}

// UpdateInstanceState updates the state of a service instance
func (s *ServiceInstanceState) UpdateInstanceState(id string, state ServiceState, health HealthStatus, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if instance, exists := s.instances[id]; exists {
		instance.State = state
		instance.Health = health
		if err != nil {
			instance.LastError = err.Error()
		} else {
			instance.LastError = ""
		}
		instance.UpdatedAt = time.Now()
	}
}

// DeleteInstance removes a service instance
func (s *ServiceInstanceState) DeleteInstance(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if instance, exists := s.instances[id]; exists {
		delete(s.instances, id)
		delete(s.byLabel, instance.Label)
	}
}

// ListInstances returns all service instances
func (s *ServiceInstanceState) ListInstances() []*ServiceInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := make([]*ServiceInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		instances = append(instances, instance)
	}

	return instances
}
