package api

import (
	"context"
	"envctl/internal/services"
	"fmt"
	"time"
)

// ServiceOrchestrator is the interface that the orchestrator must implement
type ServiceOrchestrator interface {
	StartService(label string) error
	StopService(label string) error
	RestartService(label string) error
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

// ServiceStateChangedEvent is emitted when a service state changes
type ServiceStateChangedEvent struct {
	Label    string
	OldState string
	NewState string
	Health   string
	Error    error
}

// OrchestratorAPI provides access to service lifecycle management
type OrchestratorAPI interface {
	// Service lifecycle management
	StartService(ctx context.Context, label string) error
	StopService(ctx context.Context, label string) error
	RestartService(ctx context.Context, label string) error

	// Service status queries
	GetServiceStatus(ctx context.Context, label string) (*ServiceStatus, error)
	ListServices(ctx context.Context) ([]*ServiceStatus, error)

	// Service state monitoring
	SubscribeToStateChanges() <-chan ServiceStateChangedEvent
}

// orchestratorAPI implements OrchestratorAPI
type orchestratorAPI struct {
	orchestrator ServiceOrchestrator
	registry     services.ServiceRegistry
	eventChan    chan ServiceStateChangedEvent
}

// NewOrchestratorAPI creates a new orchestrator API
func NewOrchestratorAPI(orch ServiceOrchestrator, registry services.ServiceRegistry) OrchestratorAPI {
	api := &orchestratorAPI{
		orchestrator: orch,
		registry:     registry,
		eventChan:    make(chan ServiceStateChangedEvent, 100),
	}

	// TODO: Set up event forwarding from orchestrator to API event channel

	return api
}

// StartService starts a service by label
func (api *orchestratorAPI) StartService(ctx context.Context, label string) error {
	service, exists := api.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Check if already running
	if service.GetState() == services.StateRunning {
		return fmt.Errorf("service %s is already running", label)
	}

	// Use orchestrator to start service (handles dependencies)
	return api.orchestrator.StartService(label)
}

// StopService stops a service by label
func (api *orchestratorAPI) StopService(ctx context.Context, label string) error {
	service, exists := api.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Check if already stopped
	if service.GetState() == services.StateStopped {
		return fmt.Errorf("service %s is already stopped", label)
	}

	// Use orchestrator to stop service
	return api.orchestrator.StopService(label)
}

// RestartService restarts a service by label
func (api *orchestratorAPI) RestartService(ctx context.Context, label string) error {
	_, exists := api.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	// Use orchestrator to restart service
	return api.orchestrator.RestartService(label)
}

// GetServiceStatus returns the status of a specific service
func (api *orchestratorAPI) GetServiceStatus(ctx context.Context, label string) (*ServiceStatus, error) {
	service, exists := api.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("service %s not found", label)
	}

	status := &ServiceStatus{
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
		status, err := api.GetServiceStatus(ctx, service.GetLabel())
		if err != nil {
			// Skip services that error
			continue
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// SubscribeToStateChanges returns a channel for service state change events
func (api *orchestratorAPI) SubscribeToStateChanges() <-chan ServiceStateChangedEvent {
	return api.eventChan
}

// forwardStateChange is called by the orchestrator when a service state changes
func (api *orchestratorAPI) forwardStateChange(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
	event := ServiceStateChangedEvent{
		Label:    label,
		OldState: string(oldState),
		NewState: string(newState),
		Health:   string(health),
		Error:    err,
	}

	select {
	case api.eventChan <- event:
	default:
		// Channel full, drop event
		// TODO: Add logging
	}
}
