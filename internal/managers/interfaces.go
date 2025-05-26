package managers

import (
	"envctl/internal/dependency"
	"envctl/internal/reporting"
	"sync"
)

// ServiceManagerAPI defines the interface for managing services (port forwards and MCP servers).
type ServiceManagerAPI interface {
	// StartServices starts a batch of services and returns their stop channels and any startup errors.
	StartServices(configs []ManagedServiceConfig, wg *sync.WaitGroup) (map[string]chan struct{}, []error)

	// StartServicesWithDependencyOrder starts services in the correct order based on dependencies
	StartServicesWithDependencyOrder(configs []ManagedServiceConfig, depGraph *dependency.Graph, wg *sync.WaitGroup) (map[string]chan struct{}, []error)

	// StopService signals a specific service (by label) to stop.
	StopService(label string) error

	// StopServiceWithDependents stops a service and all services that depend on it.
	StopServiceWithDependents(label string, depGraph *dependency.Graph) error

	// StopAllServices signals all managed services to stop.
	StopAllServices()

	// RestartService signals a specific service to stop and then start again.
	RestartService(label string) error

	// SetReporter allows changing the reporter after initialization.
	SetReporter(reporter reporting.ServiceReporter)

	// StartServicesDependingOn starts all services that depend on the given node ID
	StartServicesDependingOn(nodeID string, depGraph *dependency.Graph) error
}
