package serviceclass

import (
	"sync"
	"time"

	"envctl/internal/api"
)

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
