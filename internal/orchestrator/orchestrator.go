package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// StopReason tracks why a service was stopped.
type StopReason int

const (
	StopReasonManual StopReason = iota
	StopReasonDependency
)

// Orchestrator manages services using the new service registry architecture.
type Orchestrator struct {
	registry services.ServiceRegistry

	// Configuration
	mcpServers []config.MCPServerDefinition
	aggregator config.AggregatorConfig
	yolo       bool

	// Service tracking
	stopReasons map[string]StopReason

	// State change event subscribers
	stateChangeSubscribers []chan<- ServiceStateChangedEvent

	// Context for cancellation
	ctx        context.Context
	cancelFunc context.CancelFunc

	mu sync.RWMutex
}

// Config holds the configuration for the orchestrator.
type Config struct {
	MCPServers []config.MCPServerDefinition
	Aggregator config.AggregatorConfig
	Yolo       bool
}

// New creates a new orchestrator.
func New(cfg Config) *Orchestrator {
	registry := services.NewRegistry()

	return &Orchestrator{
		registry:               registry,
		mcpServers:             cfg.MCPServers,
		aggregator:             cfg.Aggregator,
		yolo:                   cfg.Yolo,
		stopReasons:            make(map[string]StopReason),
		stateChangeSubscribers: make([]chan<- ServiceStateChangedEvent, 0),
	}
}

// Start initializes and starts all services.
func (o *Orchestrator) Start(ctx context.Context) error {
	o.ctx, o.cancelFunc = context.WithCancel(ctx)

	services := o.registry.GetAll()
	if len(services) == 0 {
		logging.Info("Orchestrator", "Started orchestrator with core resource provider architecture (no services)")
		return nil
	}

	// Set up state change callbacks on all services so we can publish events
	o.setupStateChangeNotifications(services)

	// Start all registered services asynchronously
	// This prevents individual service failures from blocking other services
	for _, service := range services {
		go func(svc interface{}) {
			// Use direct interface methods without type assertion
			if service, ok := svc.(interface {
				Start(context.Context) error
				GetLabel() string
			}); ok {
				if err := service.Start(o.ctx); err != nil {
					logging.Error("Orchestrator", err, "Failed to start service: %s", service.GetLabel())
					// Individual service failures don't stop the orchestrator
				} else {
					logging.Info("Orchestrator", "Started service: %s", service.GetLabel())
				}
			}
		}(service)
	}

	logging.Info("Orchestrator", "Started orchestrator with core resource provider architecture")
	return nil
}

// setupStateChangeNotifications configures services to notify the orchestrator of state changes
func (o *Orchestrator) setupStateChangeNotifications(services []services.Service) {
	for _, service := range services {
		service.SetStateChangeCallback(o.createStateChangeCallback())
		logging.Debug("Orchestrator", "Set up state change notifications for service: %s", service.GetLabel())
	}
}

// createStateChangeCallback creates a state change callback that publishes events
func (o *Orchestrator) createStateChangeCallback() services.StateChangeCallback {
	return func(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
		o.publishStateChangeEvent(label, oldState, newState, health, err)
	}
}

// publishStateChangeEvent publishes a state change event to all subscribers
func (o *Orchestrator) publishStateChangeEvent(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
	// Get service to determine its type
	service, exists := o.registry.Get(label)
	if !exists {
		return
	}

	logging.Debug("Orchestrator", "Service %s state changed: %s -> %s (health: %s)", label, oldState, newState, health)

	// Create the event
	event := ServiceStateChangedEvent{
		Label:       label,
		ServiceType: string(service.GetType()),
		OldState:    string(oldState),
		NewState:    string(newState),
		Health:      string(health),
		Error:       err,
		Timestamp:   time.Now().Unix(),
	}

	// Publish to all subscribers
	o.mu.RLock()
	subscribers := make([]chan<- ServiceStateChangedEvent, len(o.stateChangeSubscribers))
	copy(subscribers, o.stateChangeSubscribers)
	o.mu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		default:
			// Don't block if subscriber can't receive immediately
			logging.Debug("Orchestrator", "Subscriber blocked, skipping event for service %s", label)
		}
	}
}

// Stop gracefully stops all services.
func (o *Orchestrator) Stop() error {
	if o.cancelFunc != nil {
		o.cancelFunc()
	}
	return nil
}

// StartService starts a specific service by label.
func (o *Orchestrator) StartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	if err := service.Start(o.ctx); err != nil {
		return fmt.Errorf("failed to start service %s: %w", label, err)
	}

	logging.Info("Orchestrator", "Started service: %s", label)
	return nil
}

// StopService stops a specific service by label.
func (o *Orchestrator) StopService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	if err := service.Stop(o.ctx); err != nil {
		return fmt.Errorf("failed to stop service %s: %w", label, err)
	}

	logging.Info("Orchestrator", "Stopped service: %s", label)
	return nil
}

// RestartService restarts a specific service by label.
func (o *Orchestrator) RestartService(label string) error {
	service, exists := o.registry.Get(label)
	if !exists {
		return fmt.Errorf("service %s not found", label)
	}

	if err := service.Restart(o.ctx); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", label, err)
	}

	logging.Info("Orchestrator", "Restarted service: %s", label)
	return nil
}

// GetServiceRegistry returns the service registry.
func (o *Orchestrator) GetServiceRegistry() services.ServiceRegistry {
	return o.registry
}

// SubscribeToStateChanges returns a channel for state change events.
func (o *Orchestrator) SubscribeToStateChanges() <-chan ServiceStateChangedEvent {
	eventChan := make(chan ServiceStateChangedEvent, 100)
	o.mu.Lock()
	o.stateChangeSubscribers = append(o.stateChangeSubscribers, eventChan)
	o.mu.Unlock()
	return eventChan
}

// ServiceStateChangedEvent represents a service state change event.
type ServiceStateChangedEvent struct {
	Label       string
	ServiceType string
	OldState    string
	NewState    string
	Health      string
	Error       error
	Timestamp   int64
}

// GetServiceStatus returns the status of a specific service.
func (o *Orchestrator) GetServiceStatus(label string) (*ServiceStatus, error) {
	service, exists := o.registry.Get(label)
	if !exists {
		return nil, fmt.Errorf("service %s not found", label)
	}

	return &ServiceStatus{
		Label:  label,
		Type:   string(service.GetType()),
		State:  string(service.GetState()),
		Health: string(service.GetHealth()),
		Error:  service.GetLastError(),
	}, nil
}

// GetAllServices returns status for all services.
func (o *Orchestrator) GetAllServices() []ServiceStatus {
	services := o.registry.GetAll()
	statuses := make([]ServiceStatus, len(services))

	for i, service := range services {
		statuses[i] = ServiceStatus{
			Label:  service.GetLabel(),
			Type:   string(service.GetType()),
			State:  string(service.GetState()),
			Health: string(service.GetHealth()),
			Error:  service.GetLastError(),
		}
	}

	return statuses
}

// ServiceStatus represents the status of a service.
type ServiceStatus struct {
	Label  string
	Type   string
	State  string
	Health string
	Error  error
}
