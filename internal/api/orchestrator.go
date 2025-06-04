package api

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"fmt"
)

// ServiceOrchestrator is the interface that the orchestrator must implement
type ServiceOrchestrator interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
	SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Label       string                `json:"label"`
	ServiceType string                `json:"serviceType"`
	State       services.ServiceState `json:"state"`
	Health      services.HealthStatus `json:"health"`
	Error       string                `json:"error,omitempty"`
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
	SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent

	// Cluster management
	GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition
	GetActiveCluster(role config.ClusterRole) (string, bool)
	SwitchCluster(role config.ClusterRole, clusterName string) error
}

// orchestratorAPI wraps the orchestrator to implement OrchestratorAPI
type orchestratorAPI struct {
	orch     *orchestrator.Orchestrator
	registry services.ServiceRegistry
}

// NewOrchestratorAPI creates a new API wrapper for the orchestrator
func NewOrchestratorAPI(orch *orchestrator.Orchestrator, registry services.ServiceRegistry) OrchestratorAPI {
	api := &orchestratorAPI{
		orch:     orch,
		registry: registry,
	}

	// The global registry is used by other packages to access APIs
	return api
}

// StartService starts a specific service
func (a *orchestratorAPI) StartService(label string) error {
	return a.orch.StartService(label)
}

// StopService stops a specific service
func (a *orchestratorAPI) StopService(label string) error {
	return a.orch.StopService(label)
}

// RestartService restarts a specific service
func (a *orchestratorAPI) RestartService(label string) error {
	return a.orch.RestartService(label)
}

// SubscribeToStateChanges returns a channel for receiving service state change events
func (a *orchestratorAPI) SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent {
	return a.orch.SubscribeToStateChanges()
}

// GetAvailableClusters returns all clusters configured for a specific role
func (a *orchestratorAPI) GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition {
	return a.orch.GetAvailableClusters(role)
}

// GetActiveCluster returns the currently active cluster for a role
func (a *orchestratorAPI) GetActiveCluster(role config.ClusterRole) (string, bool) {
	return a.orch.GetActiveCluster(role)
}

// SwitchCluster changes the active cluster for a role and restarts affected services
func (a *orchestratorAPI) SwitchCluster(role config.ClusterRole, clusterName string) error {
	return a.orch.SwitchCluster(role, clusterName)
}

// GetServiceStatus returns the status of a specific service
func (api *orchestratorAPI) GetServiceStatus(label string) (*ServiceStatus, error) {
	service, exists := api.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("service %s not found", label)
	}

	status := ServiceStatus{
		Label:       service.GetLabel(),
		ServiceType: string(service.GetType()),
		State:       service.GetState(),
		Health:      service.GetHealth(),
	}

	// Add error if service is in error state
	if service.GetState() == services.StateFailed || service.GetHealth() == services.HealthUnhealthy {
		if err := service.GetLastError(); err != nil {
			status.Error = err.Error()
		}
	}

	return &status, nil
}

// ListServices returns the status of all services
func (api *orchestratorAPI) ListServices(ctx context.Context) ([]*ServiceStatus, error) {
	allServices := api.registry.GetAll()

	statuses := make([]*ServiceStatus, 0, len(allServices))
	for _, service := range allServices {
		status, err := api.GetServiceStatus(service.GetLabel())
		if err != nil {
			// Skip services that error
			continue
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetAllServices returns the status of all services
func (api *orchestratorAPI) GetAllServices() []ServiceStatus {
	allServices := api.registry.GetAll()

	statuses := make([]ServiceStatus, 0, len(allServices))
	for _, service := range allServices {
		status := ServiceStatus{
			Label:       service.GetLabel(),
			ServiceType: string(service.GetType()),
			State:       service.GetState(),
			Health:      service.GetHealth(),
		}

		// Add error if service is in error state
		if service.GetState() == services.StateFailed || service.GetHealth() == services.HealthUnhealthy {
			if err := service.GetLastError(); err != nil {
				status.Error = err.Error()
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}
