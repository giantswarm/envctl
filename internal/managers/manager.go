package managers

import (
	"envctl/internal/config"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"sync"
)

// ServiceManager implements the ServiceManagerAPI interface.
// It provides simple service lifecycle management without any dependency or restart logic.
type ServiceManager struct {
	activeServices map[string]chan struct{}          // Map of service label to its stop channel
	serviceConfigs map[string]ManagedServiceConfig   // Map label to its original ManagedServiceConfig
	serviceStates  map[string]reporting.ServiceState // Track current state of services
	reporter       reporting.ServiceReporter
	mu             sync.Mutex
}

// NewServiceManager creates a new instance of ServiceManager
func NewServiceManager(reporter reporting.ServiceReporter) ServiceManagerAPI {
	if reporter == nil {
		// Fallback to a console reporter if nil
		fmt.Println("Warning: NewServiceManager called with a nil reporter. Using a new ConsoleReporter as fallback.")
		reporter = reporting.NewConsoleReporter()
	}
	return &ServiceManager{
		activeServices: make(map[string]chan struct{}),
		serviceConfigs: make(map[string]ManagedServiceConfig),
		serviceStates:  make(map[string]reporting.ServiceState),
		reporter:       reporter,
	}
}

// SetReporter allows changing the reporter after initialization
func (sm *ServiceManager) SetReporter(reporter reporting.ServiceReporter) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if reporter == nil {
		fmt.Println("Warning: ServiceManager.SetReporter called with a nil reporter. Using a new ConsoleReporter as fallback.")
		sm.reporter = reporting.NewConsoleReporter()
	} else {
		sm.reporter = reporter
	}
}

// StartServices starts a batch of services without any dependency ordering
func (sm *ServiceManager) StartServices(
	configs []ManagedServiceConfig,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	sm.mu.Lock()
	// Store configs
	if sm.serviceConfigs == nil {
		sm.serviceConfigs = make(map[string]ManagedServiceConfig)
	}
	for _, cfg := range configs {
		sm.serviceConfigs[cfg.Label] = cfg
	}
	sm.mu.Unlock()

	return sm.startServicesInternal(configs, wg)
}

// startServicesInternal is the internal implementation for starting services
func (sm *ServiceManager) startServicesInternal(
	configs []ManagedServiceConfig,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	var startupErrors []error
	allStopChannels := make(map[string]chan struct{})

	// Separate configs by type
	var pfConfigs []config.PortForwardDefinition
	var mcpConfigs []config.MCPServerDefinition
	pfOriginalLabels := make(map[string]string)
	mcpOriginalLabels := make(map[string]string)

	for _, cfg := range configs {
		switch cfg.Type {
		case reporting.ServiceTypePortForward:
			if pfConfig, ok := cfg.Config.(config.PortForwardDefinition); ok {
				pfConfigs = append(pfConfigs, pfConfig)
				pfOriginalLabels[pfConfig.Name] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid port forward config for label %s", cfg.Label))
			}
		case reporting.ServiceTypeMCPServer:
			if mcpConfig, ok := cfg.Config.(config.MCPServerDefinition); ok {
				mcpConfigs = append(mcpConfigs, mcpConfig)
				mcpOriginalLabels[mcpConfig.Name] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid MCP server config for label %s", cfg.Label))
			}
		default:
			startupErrors = append(startupErrors, fmt.Errorf("unknown service type %q for label %q", string(cfg.Type), cfg.Label))
		}
	}

	// Start port forwards
	if len(pfConfigs) > 0 {
		logging.Debug("ServiceManager", "Processing %d port forward configs.", len(pfConfigs))
		pfUpdateAdapter := sm.createPortForwardUpdateAdapter(pfOriginalLabels)
		pfStopChans := portforwarding.StartPortForwardings(pfConfigs, pfUpdateAdapter, wg)

		sm.mu.Lock()
		for label, ch := range pfStopChans {
			originalLabel := pfOriginalLabels[label]
			if originalLabel == "" {
				originalLabel = label
			}
			allStopChannels[originalLabel] = ch
			sm.activeServices[originalLabel] = ch
		}
		sm.mu.Unlock()
	}

	// Start MCP servers
	if len(mcpConfigs) > 0 {
		logging.Debug("ServiceManager", "Processing %d MCP server configs.", len(mcpConfigs))
		mcpUpdateAdapter := sm.createMCPUpdateAdapter(mcpOriginalLabels)
		mcpStopChans, mcpErrs := mcpserver.StartMCPServers(mcpConfigs, mcpUpdateAdapter, wg)
		startupErrors = append(startupErrors, mcpErrs...)

		sm.mu.Lock()
		for label, ch := range mcpStopChans {
			originalLabel := mcpOriginalLabels[label]
			if originalLabel == "" {
				originalLabel = label
			}
			allStopChannels[originalLabel] = ch
			sm.activeServices[originalLabel] = ch
		}
		sm.mu.Unlock()
	}

	return allStopChannels, startupErrors
}

// createPortForwardUpdateAdapter creates the update callback for port forwards
func (sm *ServiceManager) createPortForwardUpdateAdapter(pfOriginalLabels map[string]string) portforwarding.PortForwardUpdateFunc {
	return func(serviceLabel string, statusDetail portforwarding.PortForwardStatusDetail, isOpReady bool, operationErr error) {
		sm.mu.Lock()
		defer sm.mu.Unlock()

		originalLabel := pfOriginalLabels[serviceLabel]
		if originalLabel == "" {
			originalLabel = serviceLabel
		}

		// Map status to state
		var state reporting.ServiceState
		if operationErr != nil {
			state = reporting.StateFailed
		} else {
			switch statusDetail {
			case portforwarding.StatusDetailForwardingActive:
				if isOpReady {
					state = reporting.StateRunning
				} else {
					state = reporting.StateStarting
				}
			case portforwarding.StatusDetailInitializing:
				state = reporting.StateStarting
			case portforwarding.StatusDetailStopped:
				state = reporting.StateStopped
			case portforwarding.StatusDetailFailed, portforwarding.StatusDetailError:
				state = reporting.StateFailed
			case portforwarding.StatusDetailUnknown:
				state = reporting.StateUnknown
			default:
				if isOpReady {
					state = reporting.StateRunning
				} else {
					state = reporting.StateUnknown
				}
			}
		}

		// Only report if state changed
		lastReportedState, known := sm.serviceStates[originalLabel]
		if !known || state != lastReportedState {
			sm.serviceStates[originalLabel] = state

			// Log state change
			logMessage := fmt.Sprintf("Service %s (PortForward) state changed to: %s", originalLabel, state)
			if state == reporting.StateFailed || operationErr != nil {
				logging.Error("ServiceManager", operationErr, "%s", logMessage)
			} else if state == reporting.StateRunning || state == reporting.StateStopped {
				logging.Info("ServiceManager", "%s", logMessage)
			} else {
				logging.Debug("ServiceManager", "%s", logMessage)
			}

			// Report to observer using new correlation system
			if sm.reporter != nil {
				update := reporting.NewManagedServiceUpdate(
					reporting.ServiceTypePortForward,
					originalLabel,
					state,
				).WithCause("port_forward_status_change")

				if operationErr != nil {
					update = update.WithError(operationErr)
				}

				sm.reporter.Report(update)
			}

			// Clean up if stopped
			if state == reporting.StateStopped || state == reporting.StateFailed {
				delete(sm.activeServices, originalLabel)
				delete(sm.serviceStates, originalLabel)
			}
		}
	}
}

// createMCPUpdateAdapter creates the update callback for MCP servers
func (sm *ServiceManager) createMCPUpdateAdapter(mcpOriginalLabels map[string]string) mcpserver.McpUpdateFunc {
	return func(mcpStatusUpdate mcpserver.McpDiscreteStatusUpdate) {
		sm.mu.Lock()
		defer sm.mu.Unlock()

		originalLabel := mcpOriginalLabels[mcpStatusUpdate.Label]
		if originalLabel == "" {
			originalLabel = mcpStatusUpdate.Label
		}

		// Map status to state
		var state reporting.ServiceState
		switch mcpStatusUpdate.ProcessStatus {
		case "NpxStarting", "Initializing", "NpxInitializing":
			state = reporting.StateStarting
		case "NpxRunning":
			state = reporting.StateRunning
		case "NpxStoppedByUser", "NpxExitedGracefully":
			state = reporting.StateStopped
		case "NpxStartFailed", "NpxExitedWithError":
			state = reporting.StateFailed
		default:
			state = reporting.StateUnknown
		}

		if mcpStatusUpdate.ProcessErr != nil {
			state = reporting.StateFailed
		}

		// Only report if state changed
		lastReportedState, known := sm.serviceStates[originalLabel]
		if !known || state != lastReportedState {
			sm.serviceStates[originalLabel] = state

			// Log state change
			logMessage := fmt.Sprintf("Service %s (MCPServer) state changed to: %s", originalLabel, state)
			if mcpStatusUpdate.ProxyPort > 0 {
				logMessage += fmt.Sprintf(" (port: %d)", mcpStatusUpdate.ProxyPort)
			}
			if mcpStatusUpdate.PID > 0 {
				logMessage += fmt.Sprintf(" [PID: %d]", mcpStatusUpdate.PID)
			}

			if state == reporting.StateFailed || mcpStatusUpdate.ProcessErr != nil {
				logging.Error("ServiceManager", mcpStatusUpdate.ProcessErr, "%s", logMessage)
			} else if state == reporting.StateRunning || state == reporting.StateStopped {
				logging.Info("ServiceManager", "%s", logMessage)
			} else {
				logging.Debug("ServiceManager", "%s", logMessage)
			}

			// Report to observer using new correlation system
			if sm.reporter != nil {
				update := reporting.NewManagedServiceUpdate(
					reporting.ServiceTypeMCPServer,
					originalLabel,
					state,
				).WithCause("mcp_server_status_change").
					WithServiceData(mcpStatusUpdate.ProxyPort, mcpStatusUpdate.PID)

				if mcpStatusUpdate.ProcessErr != nil {
					update = update.WithError(mcpStatusUpdate.ProcessErr)
				}

				sm.reporter.Report(update)
			}

			// Clean up if stopped
			if state == reporting.StateStopped || state == reporting.StateFailed {
				delete(sm.activeServices, originalLabel)
				delete(sm.serviceStates, originalLabel)
			}
		}
	}
}

// StopService signals a specific service to stop
func (sm *ServiceManager) StopService(label string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	stopChan, exists := sm.activeServices[label]
	if !exists {
		return fmt.Errorf("service with label '%s' not found or not active", label)
	}

	// Check if already closed
	select {
	case <-stopChan:
		// Already closed
		delete(sm.activeServices, label)
		return nil
	default:
		// Not closed yet
	}

	// Close the stop channel
	close(stopChan)
	delete(sm.activeServices, label)

	logging.Info("ServiceManager", "Stopped service %s", label)
	return nil
}

// StopAllServices signals all managed services to stop
func (sm *ServiceManager) StopAllServices() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logging.Debug("ServiceManager", "StopAllServices: Attempting to stop %d active services.", len(sm.activeServices))
	for label, stopChan := range sm.activeServices {
		select {
		case <-stopChan:
			// Already closed
		default:
			close(stopChan)
			logging.Debug("ServiceManager", "Stopped service: %s", label)
		}
	}

	// Clear all maps
	sm.activeServices = make(map[string]chan struct{})
	sm.serviceStates = make(map[string]reporting.ServiceState)
}

// GetServiceConfig returns the configuration for a service
func (sm *ServiceManager) GetServiceConfig(label string) (ManagedServiceConfig, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cfg, exists := sm.serviceConfigs[label]
	return cfg, exists
}

// IsServiceActive checks if a service is currently active
func (sm *ServiceManager) IsServiceActive(label string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, exists := sm.activeServices[label]
	return exists
}

// GetActiveServices returns a list of all active service labels
func (sm *ServiceManager) GetActiveServices() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	labels := make([]string, 0, len(sm.activeServices))
	for label := range sm.activeServices {
		labels = append(labels, label)
	}
	return labels
}

// Methods that were previously complex but are now removed or delegated to orchestrator:
// - RestartService: Removed (orchestrator handles this)
// - StopServiceWithDependents: Removed (orchestrator handles this)
// - StartServicesDependingOn: Removed (orchestrator handles this)
// - StartServicesWithDependencyOrder: Removed (orchestrator handles this)
