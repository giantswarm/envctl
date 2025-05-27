package managers

import (
	"context"
	"envctl/internal/reporting"
	"sync"
	"time"
)

// ServiceManagerAPI defines the interface for managing services (port forwards and MCP servers).
// This interface provides simple lifecycle management without dependency or restart logic.
type ServiceManagerAPI interface {
	// StartServices starts a batch of services and returns their stop channels and any startup errors.
	StartServices(configs []ManagedServiceConfig, wg *sync.WaitGroup) (map[string]chan struct{}, []error)

	// StopService signals a specific service (by label) to stop.
	StopService(label string) error

	// StopAllServices signals all managed services to stop.
	StopAllServices()

	// SetReporter allows changing the reporter after initialization.
	SetReporter(reporter reporting.ServiceReporter)

	// GetServiceConfig returns the configuration for a service.
	GetServiceConfig(label string) (ManagedServiceConfig, bool)

	// IsServiceActive checks if a service is currently active.
	IsServiceActive(label string) bool

	// GetActiveServices returns a list of all active service labels.
	GetActiveServices() []string

	// GetReconciler returns the service reconciler for health monitoring
	GetReconciler() ServiceReconciler
}

// ServiceReconciler defines the interface for service health monitoring and reconciliation
type ServiceReconciler interface {
	// StartHealthMonitoring starts health monitoring for all services
	StartHealthMonitoring(ctx context.Context) error

	// StopHealthMonitoring stops all health monitoring
	StopHealthMonitoring()

	// CheckServiceHealth performs an immediate health check on a specific service
	CheckServiceHealth(ctx context.Context, label string) error

	// SetHealthCheckInterval sets the interval for periodic health checks
	SetHealthCheckInterval(interval time.Duration)

	// GetHealthStatus returns the current health status of a service
	GetHealthStatus(label string) (isHealthy bool, lastCheck time.Time, err error)
}

// ServiceHealthChecker defines the interface that each service type must implement
type ServiceHealthChecker interface {
	// CheckHealth performs a health check and returns an error if unhealthy
	CheckHealth(ctx context.Context) error
}
