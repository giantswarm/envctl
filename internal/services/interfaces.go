package services

import (
	"context"
	"time"
)

// ServiceState represents the current state of a service
type ServiceState string

const (
	StateUnknown  ServiceState = "Unknown"
	StateWaiting  ServiceState = "Waiting"
	StateStarting ServiceState = "Starting"
	StateRunning  ServiceState = "Running"
	StateStopping ServiceState = "Stopping"
	StateStopped  ServiceState = "Stopped"
	StateFailed   ServiceState = "Failed"
	StateRetrying ServiceState = "Retrying"
)

// HealthStatus represents the health status of a service
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "Unknown"
	HealthHealthy   HealthStatus = "Healthy"
	HealthUnhealthy HealthStatus = "Unhealthy"
	HealthChecking  HealthStatus = "Checking"
)

// ServiceType represents the type of service
type ServiceType string

const (
	TypeKubeConnection ServiceType = "KubeConnection"
	TypePortForward    ServiceType = "PortForward"
	TypeMCPServer      ServiceType = "MCPServer"
)

// Service is the core interface that all services must implement
type Service interface {
	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error

	// State management
	GetState() ServiceState
	GetHealth() HealthStatus
	GetLastError() error

	// Service metadata
	GetLabel() string
	GetType() ServiceType
	GetDependencies() []string

	// State change notifications
	// The service should call this callback when its state changes
	SetStateChangeCallback(callback StateChangeCallback)
}

// StateChangeCallback is called when a service's state changes
type StateChangeCallback func(label string, oldState, newState ServiceState, health HealthStatus, err error)

// StateUpdater is an optional interface for services that allow external state updates
// This is used by the orchestrator to set services to StateWaiting when dependencies fail
type StateUpdater interface {
	UpdateState(state ServiceState, health HealthStatus, err error)
}

// ServiceDataProvider is an optional interface for services that expose additional data
type ServiceDataProvider interface {
	// GetServiceData returns service-specific data that can be accessed via the API layer
	// This data should not be stored in the state store
	GetServiceData() map[string]interface{}
}

// HealthChecker is an optional interface for services that support health checking
type HealthChecker interface {
	// CheckHealth performs a health check and returns the current health status
	CheckHealth(ctx context.Context) (HealthStatus, error)

	// GetHealthCheckInterval returns the interval at which health checks should be performed
	GetHealthCheckInterval() time.Duration
}

// ServiceRegistry manages all registered services
type ServiceRegistry interface {
	// Register adds a service to the registry
	Register(service Service) error

	// Unregister removes a service from the registry
	Unregister(label string) error

	// Get returns a service by label
	Get(label string) (Service, bool)

	// GetAll returns all registered services
	GetAll() []Service

	// GetByType returns all services of a specific type
	GetByType(serviceType ServiceType) []Service
}

// ServiceManager orchestrates service lifecycle
type ServiceManager interface {
	// Start starts a service by label
	StartService(ctx context.Context, label string) error

	// Stop stops a service by label
	StopService(ctx context.Context, label string) error

	// Restart restarts a service by label
	RestartService(ctx context.Context, label string) error

	// StartAll starts all registered services respecting dependencies
	StartAll(ctx context.Context) error

	// StopAll stops all registered services
	StopAll(ctx context.Context) error

	// GetServiceState returns the current state of a service
	GetServiceState(label string) (ServiceState, HealthStatus, error)
}
