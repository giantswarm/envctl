package capability

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

// ServiceCapabilityDefinition extends CapabilityDefinition for service lifecycle management
// This defines how to create and manage dynamic service instances
type ServiceCapabilityDefinition struct {
	// Base capability fields
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`

	// Service-specific lifecycle configuration
	ServiceConfig ServiceConfig `yaml:"serviceConfig"`

	// Operations for external API (same as existing capabilities)
	Operations map[string]OperationDefinition `yaml:"operations"`

	// Metadata
	Metadata map[string]string `yaml:"metadata"`
}

// ServiceConfig defines the service lifecycle management
type ServiceConfig struct {
	// Service metadata
	ServiceType  string   `yaml:"serviceType"`  // Custom service type for this capability
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
	// Tool to call when creating/starting the service
	Create ToolCall `yaml:"create"`

	// Tool to call when stopping/deleting the service
	Delete ToolCall `yaml:"delete"`

	// Tool to call for health checks (optional)
	HealthCheck *ToolCall `yaml:"healthCheck,omitempty"`

	// Tool to call to get service status/info (optional)
	Status *ToolCall `yaml:"status,omitempty"`
}

// ToolCall defines how to call an aggregator tool for a lifecycle event
type ToolCall struct {
	// Tool name in the aggregator (e.g., "x_kubernetes_port_forward")
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

// ServiceInstance represents a runtime instance of a service created from a capability
type ServiceInstance struct {
	// Instance identification
	ID    string `json:"id"`    // Unique instance ID
	Label string `json:"label"` // Service label (from orchestrator)

	// Capability reference
	CapabilityName string `json:"capabilityName"` // Which capability created this
	CapabilityType string `json:"capabilityType"` // Type of capability

	// Service state (using local types to avoid import cycles)
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

	// Dependencies (from capability definition or runtime)
	Dependencies []string `json:"dependencies"`
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
func (s *ServiceInstanceState) CreateInstance(id, label, capabilityName, capabilityType string, parameters map[string]interface{}) *ServiceInstance {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance := &ServiceInstance{
		ID:                   id,
		Label:                label,
		CapabilityName:       capabilityName,
		CapabilityType:       capabilityType,
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
