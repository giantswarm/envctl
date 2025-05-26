package managers

import (
	"envctl/internal/reporting"
	"sync"
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
}
