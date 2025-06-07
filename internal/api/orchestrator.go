package api

import (
	"context"
	"fmt"
)

// ServiceOrchestrator is the interface that the orchestrator must implement
type ServiceOrchestrator interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
}

// OrchestratorAPI defines the interface for orchestrating services
type OrchestratorAPI interface {
	// Service lifecycle management
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error

	// Service status
	GetServiceStatus(label string) (*ServiceStatus, error)
	GetAllServices() []ServiceStatus

	// State change events
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent

	// Cluster management
	GetAvailableClusters(role ClusterRole) []ClusterDefinition
	GetActiveCluster(role ClusterRole) (string, bool)
	SwitchCluster(role ClusterRole, clusterName string) error
}

// orchestratorAPI wraps the orchestrator to implement OrchestratorAPI
type orchestratorAPI struct {
	// No fields - uses handlers from registry
}

// NewOrchestratorAPI creates a new API wrapper for the orchestrator
func NewOrchestratorAPI() OrchestratorAPI {
	return &orchestratorAPI{}
}

// StartService starts a specific service
func (a *orchestratorAPI) StartService(label string) error {
	handler := GetOrchestrator()
	if handler == nil {
		return fmt.Errorf("orchestrator not registered")
	}
	return handler.StartService(label)
}

// StopService stops a specific service
func (a *orchestratorAPI) StopService(label string) error {
	handler := GetOrchestrator()
	if handler == nil {
		return fmt.Errorf("orchestrator not registered")
	}
	return handler.StopService(label)
}

// RestartService restarts a specific service
func (a *orchestratorAPI) RestartService(label string) error {
	handler := GetOrchestrator()
	if handler == nil {
		return fmt.Errorf("orchestrator not registered")
	}
	return handler.RestartService(label)
}

// SubscribeToStateChanges returns a channel for receiving service state change events
func (a *orchestratorAPI) SubscribeToStateChanges() <-chan ServiceStateChangedEvent {
	handler := GetOrchestrator()
	if handler == nil {
		// Return a closed channel if no handler is registered
		ch := make(chan ServiceStateChangedEvent)
		close(ch)
		return ch
	}
	return handler.SubscribeToStateChanges()
}

// GetAvailableClusters returns all clusters configured for a specific role
func (a *orchestratorAPI) GetAvailableClusters(role ClusterRole) []ClusterDefinition {
	handler := GetOrchestrator()
	if handler == nil {
		return nil
	}
	return handler.GetAvailableClusters(role)
}

// GetActiveCluster returns the currently active cluster for a role
func (a *orchestratorAPI) GetActiveCluster(role ClusterRole) (string, bool) {
	handler := GetOrchestrator()
	if handler == nil {
		return "", false
	}
	return handler.GetActiveCluster(role)
}

// SwitchCluster changes the active cluster for a role and restarts affected services
func (a *orchestratorAPI) SwitchCluster(role ClusterRole, clusterName string) error {
	handler := GetOrchestrator()
	if handler == nil {
		return fmt.Errorf("orchestrator not registered")
	}
	return handler.SwitchCluster(role, clusterName)
}

// GetServiceStatus returns the status of a specific service
func (a *orchestratorAPI) GetServiceStatus(label string) (*ServiceStatus, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	service, exists := registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("service %s not found", label)
	}

	status := &ServiceStatus{
		Label:       service.GetLabel(),
		ServiceType: string(service.GetType()),
		State:       service.GetState(),
		Health:      service.GetHealth(),
	}

	// Add error if present
	if err := service.GetLastError(); err != nil {
		status.Error = err.Error()
	}

	// Add metadata if available
	if data := service.GetServiceData(); data != nil {
		status.Metadata = data
	}

	return status, nil
}

// ListServices returns the status of all services
func (a *orchestratorAPI) ListServices(ctx context.Context) ([]*ServiceStatus, error) {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("service registry not registered")
	}

	allServices := registry.GetAll()
	statuses := make([]*ServiceStatus, 0, len(allServices))

	for _, service := range allServices {
		status := &ServiceStatus{
			Label:       service.GetLabel(),
			ServiceType: string(service.GetType()),
			State:       service.GetState(),
			Health:      service.GetHealth(),
		}

		// Add error if present
		if err := service.GetLastError(); err != nil {
			status.Error = err.Error()
		}

		// Add metadata if available
		if data := service.GetServiceData(); data != nil {
			status.Metadata = data
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetAllServices returns the status of all services
func (a *orchestratorAPI) GetAllServices() []ServiceStatus {
	registry := GetServiceRegistry()
	if registry == nil {
		return nil
	}

	allServices := registry.GetAll()
	statuses := make([]ServiceStatus, 0, len(allServices))

	for _, service := range allServices {
		status := ServiceStatus{
			Label:       service.GetLabel(),
			ServiceType: string(service.GetType()),
			State:       service.GetState(),
			Health:      service.GetHealth(),
		}

		// Add error if present
		if err := service.GetLastError(); err != nil {
			status.Error = err.Error()
		}

		// Add metadata if available
		if data := service.GetServiceData(); data != nil {
			status.Metadata = data
		}

		statuses = append(statuses, status)
	}

	return statuses
}
