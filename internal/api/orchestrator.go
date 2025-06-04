package api

import (
	"context"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"time"
)

// ServiceOrchestrator is the interface that the orchestrator must implement
type ServiceOrchestrator interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
	SubscribeToStateChanges() <-chan orchestrator.ServiceStateChangedEvent
}

// ServiceStatus contains the status information for a service
type ServiceStatus struct {
	Label        string    `json:"label"`
	Type         string    `json:"type"`
	State        string    `json:"state"`
	Health       string    `json:"health"`
	Dependencies []string  `json:"dependencies"`
	Error        string    `json:"error,omitempty"`
	LastUpdated  time.Time `json:"lastUpdated"`
}

// OrchestratorAPI provides access to orchestrator functionality
type OrchestratorAPI interface {
	// Service lifecycle management
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error

	// Service status
	GetServiceStatus(label string) (ServiceStatus, error)
	GetAllServices() []ServiceStatus

	// State change events
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
}

// orchestratorAPI implements OrchestratorAPI
type orchestratorAPI struct {
	orch     ServiceOrchestrator
	registry services.ServiceRegistry
}

// NewOrchestratorAPI creates a new orchestrator API
func NewOrchestratorAPI(orch ServiceOrchestrator, registry services.ServiceRegistry) OrchestratorAPI {
	api := &orchestratorAPI{
		orch:     orch,
		registry: registry,
	}

	return api
}

// StartService starts a service by label
func (api *orchestratorAPI) StartService(label string) error {
	service, exists := api.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Check if already running
	if service.GetState() == services.StateRunning {
		return fmt.Errorf("service %s is already running", label)
	}

	// Use orchestrator to start service (handles dependencies)
	return api.orch.StartService(label)
}

// StopService stops a service by label
func (api *orchestratorAPI) StopService(label string) error {
	service, exists := api.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Check if already stopped
	if service.GetState() == services.StateStopped {
		return fmt.Errorf("service %s is already stopped", label)
	}

	// Use orchestrator to stop service
	return api.orch.StopService(label)
}

// RestartService restarts a service by label
func (api *orchestratorAPI) RestartService(label string) error {
	_, exists := api.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Use orchestrator to restart service
	return api.orch.RestartService(label)
}

// GetServiceStatus returns the status of a specific service
func (api *orchestratorAPI) GetServiceStatus(label string) (ServiceStatus, error) {
	service, exists := api.registry.Get(label)
	if !exists {
		return ServiceStatus{}, fmt.Errorf("service %s not found", label)
	}

	status := ServiceStatus{
		Label:        service.GetLabel(),
		Type:         string(service.GetType()),
		State:        string(service.GetState()),
		Health:       string(service.GetHealth()),
		Dependencies: service.GetDependencies(),
		LastUpdated:  time.Now(), // TODO: Get actual last update time
	}

	if err := service.GetLastError(); err != nil {
		status.Error = err.Error()
	}

	return status, nil
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
		statuses = append(statuses, &status)
	}

	return statuses, nil
}

// GetAllServices returns the status of all services
func (api *orchestratorAPI) GetAllServices() []ServiceStatus {
	allServices := api.registry.GetAll()

	statuses := make([]ServiceStatus, 0, len(allServices))
	for _, service := range allServices {
		status := ServiceStatus{
			Label:        service.GetLabel(),
			Type:         string(service.GetType()),
			State:        string(service.GetState()),
			Health:       string(service.GetHealth()),
			Dependencies: service.GetDependencies(),
			LastUpdated:  time.Now(),
		}

		if err := service.GetLastError(); err != nil {
			status.Error = err.Error()
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// SubscribeToStateChanges returns a channel for service state change events
func (api *orchestratorAPI) SubscribeToStateChanges() <-chan ServiceStateChangedEvent {
	// Create a new subscription to the orchestrator each time
	// This ensures each subscriber gets their own channel
	orchEvents := api.orch.SubscribeToStateChanges()

	// Create a new channel for this subscriber
	apiEvents := make(chan ServiceStateChangedEvent, 100)

	// Convert orchestrator events to API events in a goroutine
	go func() {
		for event := range orchEvents {
			apiEvent := ServiceStateChangedEvent{
				Label:       event.Label,
				ServiceType: event.ServiceType,
				OldState:    event.OldState,
				NewState:    event.NewState,
				Health:      event.Health,
				Error:       event.Error,
			}

			select {
			case apiEvents <- apiEvent:
				// Event forwarded successfully
			default:
				// Channel full, drop event
				logging.Warn("OrchestratorAPI", "Dropped state change event for %s (subscriber channel full)", event.Label)
			}
		}
		close(apiEvents)
	}()

	return apiEvents
}
