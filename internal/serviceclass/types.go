package serviceclass

import (
	"sync"
	"time"

	"envctl/internal/api"
)

// Type aliases to local serviceclass types for internal usage
// These map the API package types to the expected local types
type ServiceState = api.ServiceState
type HealthStatus = api.HealthStatus

// Local constants that map to API package constants for backward compatibility
const (
	ServiceStateUnknown  = api.StateUnknown
	ServiceStateWaiting  = api.StateWaiting
	ServiceStateStarting = api.StateStarting
	ServiceStateRunning  = api.StateRunning
	ServiceStateStopping = api.StateStopping
	ServiceStateStopped  = api.StateStopped
	ServiceStateFailed   = api.StateFailed
	ServiceStateRetrying = api.StateRetrying
)

const (
	HealthStatusHealthy   = api.HealthHealthy
	HealthStatusUnhealthy = api.HealthUnhealthy
	HealthStatusUnknown   = api.HealthUnknown
)

// Type aliases for the main types - now using API package as source of truth
type ServiceClassDefinition = api.ServiceClass
type ServiceInstance = api.ServiceInstance
type ServiceClassInfo = api.ServiceClassInfo
type OperationDefinition = api.OperationDefinition
type Parameter = api.Parameter
type WorkflowDefinition = api.WorkflowReference
type WorkflowStep = api.WorkflowStep

// ServiceConfig and related types are available in the API package
type ServiceConfig = api.ServiceConfig
type LifecycleTools = api.LifecycleTools
type ToolCall = api.ToolCall
type ResponseMapping = api.ResponseMapping
type ParameterMapping = api.ParameterMapping
type HealthCheckConfig = api.HealthCheckConfig
type TimeoutConfig = api.TimeoutConfig

// ServiceInstanceState provides state management for service instances
// This is the only part that remains local to the serviceclass package
type ServiceInstanceState struct {
	// In-memory state
	instances map[string]*api.ServiceInstance // ID -> instance
	byLabel   map[string]*api.ServiceInstance // label -> instance

	// Synchronization
	mu *sync.RWMutex
}

// NewServiceInstanceState creates a new service instance state manager
func NewServiceInstanceState() *ServiceInstanceState {
	return &ServiceInstanceState{
		instances: make(map[string]*api.ServiceInstance),
		byLabel:   make(map[string]*api.ServiceInstance),
		mu:        &sync.RWMutex{},
	}
}

// CreateInstance creates a new service instance
func (s *ServiceInstanceState) CreateInstance(id, label, serviceClassName, serviceClassType string, parameters map[string]interface{}) *api.ServiceInstance {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance := &api.ServiceInstance{
		ID:                   id,
		Label:                label,
		ServiceClassName:     serviceClassName,
		ServiceClassType:     serviceClassType,
		State:                api.StateUnknown,
		Health:               api.HealthUnknown,
		Parameters:           parameters,
		ServiceData:          make(map[string]interface{}),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		HealthCheckFailures:  0,
		HealthCheckSuccesses: 0,
		Dependencies:         []string{},
		Enabled:              true,
	}

	s.instances[id] = instance
	s.byLabel[label] = instance

	return instance
}

// GetInstance retrieves a service instance by ID
func (s *ServiceInstanceState) GetInstance(id string) (*api.ServiceInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, exists := s.instances[id]
	return instance, exists
}

// GetInstanceByLabel retrieves a service instance by label
func (s *ServiceInstanceState) GetInstanceByLabel(label string) (*api.ServiceInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, exists := s.byLabel[label]
	return instance, exists
}

// UpdateInstanceState updates the state of a service instance
func (s *ServiceInstanceState) UpdateInstanceState(id string, state api.ServiceState, health api.HealthStatus, err error) {
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
func (s *ServiceInstanceState) ListInstances() []*api.ServiceInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := make([]*api.ServiceInstance, 0, len(s.instances))
	for _, instance := range s.instances {
		instances = append(instances, instance)
	}

	return instances
}
