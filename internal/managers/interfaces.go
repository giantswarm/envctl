package managers

import (
	"envctl/internal/reporting"
	"sync"
)

// ServiceManagerAPI defines a unified interface for managing various background services
// like port forwarding and MCP servers.
// Renamed from ServiceManager.
type ServiceManagerAPI interface { // Renamed from ServiceManager
	// StartServices starts multiple services based on the provided configurations.
	// - configs: A slice of ManagedServiceConfig, each defining a service to start.
	// - wg: A WaitGroup to synchronize goroutine completion.
	// It uses the ServiceReporter instance provided at ServiceManager creation for updates.
	// Returns a map of service labels to their individual stop channels, and a slice of startup errors.
	StartServices(
		configs []ManagedServiceConfig, // Defined in types.go in the same package
		// updateCb ServiceUpdateFunc, // REMOVED: Replaced by injected ServiceReporter
		wg *sync.WaitGroup,
	) (map[string]chan struct{}, []error)

	// StopService signals a specific service (by label) to stop.
	StopService(label string) error

	// StopAllServices signals all managed services to stop.
	// It might use a global stop channel mechanism internally or iterate.
	StopAllServices()

	// RestartService signals a specific service to stop and then start again.
	// This is an asynchronous operation. The service will go through stopping/starting states.
	RestartService(label string) error

	// SetReporter allows changing the reporter after initialization (e.g., for testing or mode switches if ever needed).
	// Typically, the reporter is set at construction.
	SetReporter(reporter reporting.ServiceReporter)
}
