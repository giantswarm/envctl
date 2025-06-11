package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"sync"
)

// StopReason tracks why a service was stopped.
type StopReason int

const (
	StopReasonManual     StopReason = iota
	StopReasonDependency
)

// Orchestrator manages services using the new service registry architecture.
type Orchestrator struct {
	registry services.ServiceRegistry

	// Configuration
	mcName     string
	wcName     string
	mcpServers []config.MCPServerDefinition
	aggregator config.AggregatorConfig
	yolo       bool

	// Cluster management
	clusterState *ClusterState
	clusters     []config.ClusterDefinition

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
	MCName         string
	WCName         string
	Clusters       []config.ClusterDefinition
	ActiveClusters map[config.ClusterRole]string
	MCPServers     []config.MCPServerDefinition
	Aggregator     config.AggregatorConfig
	Yolo           bool
}

// New creates a new orchestrator.
func New(cfg Config) *Orchestrator {
	registry := services.NewRegistry()

	clusters := cfg.Clusters
	activeClusters := cfg.ActiveClusters
	if len(clusters) == 0 && (cfg.MCName != "" || cfg.WCName != "") {
		clusters = config.GenerateGiantSwarmClusters(cfg.MCName, cfg.WCName)
		activeClusters = make(map[config.ClusterRole]string)
		for _, c := range clusters {
			if _, exists := activeClusters[c.Role]; !exists {
				activeClusters[c.Role] = c.Name
			}
		}
	}

	clusterState := NewClusterState(clusters, activeClusters)

	return &Orchestrator{
		registry:               registry,
		mcName:                 cfg.MCName,
		wcName:                 cfg.WCName,
		clusters:               clusters,
		clusterState:           clusterState,
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
	logging.Info("Orchestrator", "Started orchestrator (K8s/PortForward services removed)")
	return nil
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

// GetAvailableClusters returns available clusters for a role.
func (o *Orchestrator) GetAvailableClusters(role config.ClusterRole) []config.ClusterDefinition {
	return o.clusterState.GetAvailableClusters(role)
}

// GetActiveCluster returns the active cluster for a role.
func (o *Orchestrator) GetActiveCluster(role config.ClusterRole) (string, bool) {
	return o.clusterState.GetActiveCluster(role)
}

// SwitchCluster switches the active cluster for a role.
func (o *Orchestrator) SwitchCluster(role config.ClusterRole, clusterName string) error {
	return o.clusterState.SetActiveCluster(role, clusterName)
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