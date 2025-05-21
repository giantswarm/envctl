package managers

import (
	"sync"
)

// ServiceManagerAPI defines a unified interface for managing various background services
// like port forwarding and MCP servers.
// Renamed from ServiceManager.
type ServiceManagerAPI interface { // Renamed from ServiceManager
	// StartServices starts multiple services based on the provided configurations.
	// - configs: A slice of ManagedServiceConfig, each defining a service to start.
	// - updateCb: A callback function that will receive ManagedServiceUpdate messages.
	// - wg: A WaitGroup to synchronize goroutine completion.
	// Returns a map of service labels to their individual stop channels, and a slice of startup errors.
	StartServices(
		configs []ManagedServiceConfig, // Defined in types.go in the same package
		updateCb ServiceUpdateFunc,    // Defined in types.go in the same package
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
}