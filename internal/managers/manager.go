package managers

import (
	// No longer importing "envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// ServiceManager implements the ServiceManagerAPI interface.
// Struct name changed to ServiceManager.
type ServiceManager struct { // Renamed from DefaultServiceManager
	activeServices map[string]chan struct{} // Map of service label to its stop channel
	mu             sync.Mutex
	// Store original configs to allow for restarts
	serviceConfigs map[string]ManagedServiceConfig // Map label to its original ManagedServiceConfig
	// Track services pending restart after they stop
	pendingRestarts map[string]bool // Map label to true if restart is pending
	// Store the original update callback and WaitGroup from the initial StartServices call
	// This assumes one main StartServices call, or needs more complex management if multiple overlapping calls.
	initialWg *sync.WaitGroup
	reporter  reporting.ServiceReporter
}

// NewServiceManager creates a new instance of ServiceManager and returns it as a ServiceManagerAPI interface.
func NewServiceManager(reporter reporting.ServiceReporter) ServiceManagerAPI {
	if reporter == nil {
		// Fallback to a NoOpReporter or a ConsoleReporter if nil is provided,
		// to prevent panics if a reporter is absolutely necessary for operation.
		// For now, let's assume a reporter is essential and this is an error if nil.
		// Or, create a default console reporter.
		fmt.Println("Warning: NewServiceManager called with a nil reporter. Using a new ConsoleReporter as fallback.")
		reporter = reporting.NewConsoleReporter() // Fallback
	}
	return &ServiceManager{
		activeServices:  make(map[string]chan struct{}),
		serviceConfigs:  make(map[string]ManagedServiceConfig),
		pendingRestarts: make(map[string]bool),
		initialWg:       &sync.WaitGroup{},
		reporter:        reporter,
	}
}

// SetReporter allows changing the reporter after initialization.
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

// StartServices is the main entry point to start a batch of services.
// It stores the initial callback, waitgroup, and configurations,
// then calls the internal worker to start the services.
func (sm *ServiceManager) StartServices(
	configs []ManagedServiceConfig,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	sm.mu.Lock()
	sm.initialWg = wg

	if sm.serviceConfigs == nil {
		sm.serviceConfigs = make(map[string]ManagedServiceConfig)
	}
	for _, cfg := range configs {
		sm.serviceConfigs[cfg.Label] = cfg // Store/update config
	}
	sm.mu.Unlock()

	return sm.startSpecificServicesLogic(configs, wg)
}

// startSpecificServicesLogic is the internal worker for starting services.
// It's called by StartServices and by the restart mechanism.
func (sm *ServiceManager) startSpecificServicesLogic(
	configs []ManagedServiceConfig,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	sm.mu.Lock()

	var startupErrors []error
	allStopChannels := make(map[string]chan struct{})
	var pfConfigs []portforwarding.PortForwardingConfig
	var mcpConfigs []mcpserver.MCPServerConfig
	pfOriginalLabels := make(map[string]string)
	mcpOriginalLabels := make(map[string]string)

	for _, cfg := range configs {
		// cfg.Type is now reporting.ServiceType
		switch cfg.Type {
		case reporting.ServiceTypePortForward:
			if pfConfig, ok := cfg.Config.(portforwarding.PortForwardingConfig); ok {
				actualPfConfig := pfConfig
				actualPfConfig.Label = cfg.Label
				pfConfigs = append(pfConfigs, actualPfConfig)
				pfOriginalLabels[actualPfConfig.Label] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid port forward config for label %s: type assertion failed", cfg.Label))
			}
		case reporting.ServiceTypeMCPServer:
			if mcpConfig, ok := cfg.Config.(mcpserver.MCPServerConfig); ok {
				actualMcpConfig := mcpConfig
				actualMcpConfig.Name = cfg.Label
				mcpConfigs = append(mcpConfigs, actualMcpConfig)
				mcpOriginalLabels[actualMcpConfig.Name] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid MCP server config for label %s: type assertion failed", cfg.Label))
			}
		default:
			if sm.reporter != nil {
				sm.reporter.Report(reporting.ManagedServiceUpdate{
					Timestamp:    time.Now(),
					SourceType:   reporting.ServiceTypeSystem,
					SourceLabel:  "ServiceManager",
					State:        reporting.StateFailed,
					ServiceLevel: reporting.LogLevelError,
					ErrorDetail:  fmt.Errorf("Unknown service type in ManagedServiceConfig for label %s: %s", cfg.Label, cfg.Type),
					IsReady:      false,
				})
			}
			startupErrors = append(startupErrors, fmt.Errorf("unknown service type %q for label %q", string(cfg.Type), cfg.Label))
		}
	}

	sm.mu.Unlock()

	if len(pfConfigs) > 0 {
		logging.Debug("ServiceManager", "Processing %d port forward configs.", len(pfConfigs))
		pfUpdateAdapter := func(serviceLabel string, statusDetail portforwarding.PortForwardStatusDetail, isOpReady bool, operationErr error) {
			originalLabel, ok := pfOriginalLabels[serviceLabel]
			if !ok {
				originalLabel = serviceLabel // Fallback
			}

			var state reporting.ServiceState
			var level reporting.LogLevel

			if operationErr != nil {
				state = reporting.StateFailed
				level = reporting.LogLevelError
			} else {
				switch statusDetail {
				case portforwarding.StatusDetailForwardingActive:
					if isOpReady { // Double check with isOpReady
						state = reporting.StateRunning
						level = reporting.LogLevelInfo
					} else {
						// This case might indicate a brief period where status is active but not fully ready, treat as Starting.
						state = reporting.StateStarting
						level = reporting.LogLevelInfo
					}
				case portforwarding.StatusDetailInitializing:
					state = reporting.StateStarting
					level = reporting.LogLevelInfo
				case portforwarding.StatusDetailStopped:
					state = reporting.StateStopped
					level = reporting.LogLevelInfo
				case portforwarding.StatusDetailFailed, portforwarding.StatusDetailError:
					state = reporting.StateFailed
					level = reporting.LogLevelError
				case portforwarding.StatusDetailUnknown:
					state = reporting.StateUnknown
					level = reporting.LogLevelWarn
				default:
					// If isOpReady is true, default to Running for any other positive-like status.
					if isOpReady {
						state = reporting.StateRunning
						level = reporting.LogLevelInfo
					} else {
						state = reporting.StateUnknown // Or StateStarting if that feels more appropriate for unmapped non-error states
						level = reporting.LogLevelDebug
						logging.Debug("ServiceManager", "Unmapped PortForwardStatusDetail '%s' for service %s. isOpReady: %t", statusDetail, originalLabel, isOpReady)
					}
				}
			}

			// Log the state change
			logMessage := fmt.Sprintf("Service %s (PortForward) state: %s", originalLabel, state)

			if state == reporting.StateFailed || operationErr != nil {
				logging.Error("ServiceManager", operationErr, "%s", logMessage)
			} else {
				logging.Info("ServiceManager", "%s", logMessage)
			}

			updateForReporter := reporting.ManagedServiceUpdate{
				Timestamp:    time.Now(),
				SourceType:   reporting.ServiceTypePortForward,
				SourceLabel:  originalLabel,
				State:        state,
				IsReady:      (state == reporting.StateRunning),
				ErrorDetail:  operationErr,
				ServiceLevel: level,
			}

			if sm.reporter != nil {
				sm.reporter.Report(updateForReporter)
			}
			sm.checkAndProcessRestart(updateForReporter)
		}

		pfStopChans := portforwarding.StartPortForwardings(pfConfigs, pfUpdateAdapter, wg)

		sm.mu.Lock()
		for label, ch := range pfStopChans {
			originalLabel, ok := pfOriginalLabels[label]
			if !ok {
				originalLabel = label
			}
			allStopChannels[originalLabel] = ch
			sm.activeServices[originalLabel] = ch
		}
		sm.mu.Unlock()
	}

	if len(mcpConfigs) > 0 {
		logging.Debug("ServiceManager", "Processing %d MCP server configs.", len(mcpConfigs))
		mcpUpdateAdapter := func(mcpStatusUpdate mcpserver.McpDiscreteStatusUpdate) {
			originalLabel, ok := mcpOriginalLabels[mcpStatusUpdate.Label]
			if !ok {
				originalLabel = mcpStatusUpdate.Label // Fallback
			}

			var state reporting.ServiceState
			var level reporting.LogLevel

			// Example mapping from mcpStatusUpdate.ProcessStatus string to reporting.ServiceState
			switch mcpStatusUpdate.ProcessStatus { // ProcessStatus is a string like "NpxStarting", "NpxRunning"
			case "NpxStarting", "Initializing": // Define these constants in mcpserver if not already
				state = reporting.StateStarting
				level = reporting.LogLevelInfo
			case "NpxRunning":
				state = reporting.StateRunning
				level = reporting.LogLevelInfo
			case "NpxStoppedByUser", "NpxExitedGracefully":
				state = reporting.StateStopped
				level = reporting.LogLevelInfo
			case "NpxStartFailed", "NpxExitedWithError":
				state = reporting.StateFailed
				level = reporting.LogLevelError
			default:
				state = reporting.StateUnknown
				level = reporting.LogLevelWarn // Unknown state might be a warning
			}

			if mcpStatusUpdate.ProcessErr != nil {
				state = reporting.StateFailed // Override if there's an explicit error
				level = reporting.LogLevelError
			}

			// Log the state change
			logMessage := fmt.Sprintf("Service %s (MCPServer) state: %s", originalLabel, state)

			if state == reporting.StateFailed || mcpStatusUpdate.ProcessErr != nil {
				logging.Error("ServiceManager", mcpStatusUpdate.ProcessErr, "%s", logMessage)
			} else {
				logging.Info("ServiceManager", "%s", logMessage)
			}

			updateForReporter := reporting.ManagedServiceUpdate{
				Timestamp:    time.Now(),
				SourceType:   reporting.ServiceTypeMCPServer,
				SourceLabel:  originalLabel,
				State:        state,
				IsReady:      (state == reporting.StateRunning),
				ErrorDetail:  mcpStatusUpdate.ProcessErr,
				ServiceLevel: level,
			}

			if sm.reporter != nil {
				sm.reporter.Report(updateForReporter)
			}
			sm.checkAndProcessRestart(updateForReporter)
		}
		mcpStopChans, mcpErrs := mcpserver.StartMCPServers(mcpConfigs, mcpUpdateAdapter, wg)
		startupErrors = append(startupErrors, mcpErrs...)

		sm.mu.Lock()
		for label, ch := range mcpStopChans {
			originalLabel, ok := mcpOriginalLabels[label]
			if !ok {
				originalLabel = label
			}
			allStopChannels[originalLabel] = ch
			sm.activeServices[originalLabel] = ch
		}
		sm.mu.Unlock()
	}
	return allStopChannels, startupErrors
}

// StopService signals a specific service (by label) to stop.
func (sm *ServiceManager) StopService(label string) error {
	sm.mu.Lock()
	// Ensure unlock happens even if early return

	stopChan, exists := sm.activeServices[label]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("service with label '%s' not found or not active", label)
	}

	// Check if channel is already closed to prevent panic on re-close
	// And to make StopService somewhat idempotent regarding the close action.
	closed := false
	select {
	case <-stopChan:
		closed = true // Already closed
	default:
		// Not closed yet
	}

	if closed {
		// If it was already closed and a restart is NOT pending, it means it was stopped for good.
		// We should ensure it's not in activeServices (checkAndProcessRestart should have done this
		// if it was closed due to an update). If a restart IS pending, leave it for checkAndProcessRestart.
		if !sm.pendingRestarts[label] {
			delete(sm.activeServices, label) // Ensure cleaned up if truly stopped
		}
		sm.mu.Unlock()
		return fmt.Errorf("service with label '%s' already stopped or stopping", label)
	}

	close(stopChan)

	// If a restart is not pending for this specific service, remove it from active services now.
	// If a restart IS pending, checkAndProcessRestart will remove it from activeServices
	// when it sees the "Stopped" update and then trigger the restart.
	if !sm.pendingRestarts[label] {
		delete(sm.activeServices, label)
		// Also remove from serviceConfigs if this is a permanent stop and not a restart cycle?
		// delete(sm.serviceConfigs, label) // Decided earlier to keep configs for potential later manual restart.
	}
	sm.mu.Unlock()
	return nil
}

// StopAllServices signals all managed services to stop.
func (sm *ServiceManager) StopAllServices() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logging.Debug("ServiceManager", "StopAllServices: Attempting to stop %d active services.", len(sm.activeServices))
	for label, stopChan := range sm.activeServices {
		logging.Debug("ServiceManager", "StopAllServices: Processing service '%s' for stop.", label)
		select {
		case <-stopChan:
			logging.Debug("ServiceManager", "StopAllServices: Channel for '%s' was already closed or signaled.", label)
		default:
			logging.Info("ServiceManager", "StopAllServices: Closing channel for service '%s'.", label)
			close(stopChan)
		}
	}
	logging.Debug("ServiceManager", "StopAllServices: Finished closing channels, clearing activeServices map.")
	sm.activeServices = make(map[string]chan struct{}) // Clear the map
}

// RestartService signals a specific service to stop and then start again.
func (sm *ServiceManager) RestartService(label string) error {
	sm.mu.Lock()
	originalCfg, configExists := sm.serviceConfigs[label]
	if !configExists {
		sm.mu.Unlock()
		return fmt.Errorf("RestartService: no configuration found for service label '%s'", label)
	}
	sm.pendingRestarts[label] = true
	_, serviceIsCurrentlyActive := sm.activeServices[label]
	sm.mu.Unlock()

	// Log initiation of restart
	logging.Info("ServiceManager", "User requested restart for service: %s", label)

	if !serviceIsCurrentlyActive {
		// If not active, it might already be stopped or failed. Report it as 'Stopped' to trigger restart sequence if pending.
		// checkAndProcessRestart will pick this up.
		logging.Info("ServiceManager", "Service %s not active, reporting as stopped to potentially trigger pending restart.", label)
		if sm.reporter != nil {
			sm.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:    time.Now(),
				SourceType:   originalCfg.Type,
				SourceLabel:  label,
				State:        reporting.StateStopped,
				IsReady:      false,
				ServiceLevel: reporting.LogLevelInfo,
			})
		}
		// The checkAndProcessRestart will handle the actual restart if pending.
		return nil
	}
	return sm.StopService(label) // This will eventually lead to a "Stopped" state update, triggering restart via checkAndProcessRestart
}

// checkAndProcessRestart is called by the update adapters.
// It now uses reporting.ManagedServiceUpdate.State to determine if a service is stopped.
func (sm *ServiceManager) checkAndProcessRestart(update reporting.ManagedServiceUpdate) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// A service is considered stopped if its state is Stopped or Failed.
	isConsideredStopped := update.State == reporting.StateStopped || update.State == reporting.StateFailed

	if isConsideredStopped {
		delete(sm.activeServices, update.SourceLabel) // Remove from active services

		if sm.pendingRestarts[update.SourceLabel] {
			delete(sm.pendingRestarts, update.SourceLabel) // Consume the pending restart flag

			cfg, exists := sm.serviceConfigs[update.SourceLabel]
			if exists {
				logging.Info("ServiceManager", "Pending restart: Starting service %s again.", cfg.Label)
				// Report "Restarting..." state before actually starting
				if sm.reporter != nil {
					sm.reporter.Report(reporting.ManagedServiceUpdate{
						Timestamp:    time.Now(),
						SourceType:   cfg.Type,
						SourceLabel:  cfg.Label,
						State:        reporting.StateStarting, // Changed from "Restarting..." message to StateStarting
						IsReady:      false,
						ServiceLevel: reporting.LogLevelInfo,
					})
				}
				var restartWg sync.WaitGroup
				// Increment initialWg if it's used globally for all service goroutines.
				// Or, manage wg per restart. For now, using a local wg for the goroutine itself.
				go sm.startSpecificServicesLogic([]ManagedServiceConfig{cfg}, &restartWg) // Pass the original config
			} else {
				logging.Warn("ServiceManager", "Pending restart for %s, but no config found.", update.SourceLabel)
			}
		}
	}
}
