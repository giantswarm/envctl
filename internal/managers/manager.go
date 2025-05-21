package managers

import (
	// No longer importing "envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ServiceManager implements the ServiceManagerAPI interface.
// Struct name changed to ServiceManager.
type ServiceManager struct { // Renamed from DefaultServiceManager
	activeServices   map[string]chan struct{} // Map of service label to its stop channel
	mu               sync.Mutex
	pfGlobalStopChan chan struct{} // Global stop channel for port forwarding services
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

	hasPortForwards := false
	for _, cfg := range configs {
		// cfg.Type is now reporting.ServiceType
		switch cfg.Type {
		case reporting.ServiceTypePortForward:
			hasPortForwards = true
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
					Timestamp:   time.Now(),
					SourceType:  reporting.ServiceTypeSystem,
					SourceLabel: "ServiceManager",
					Level:       reporting.LogLevelError,
					Message:     fmt.Sprintf("Unknown service type in ManagedServiceConfig for label %s: %s", cfg.Label, cfg.Type),
				})
			}
			startupErrors = append(startupErrors, fmt.Errorf("unknown service type %q for label %q", string(cfg.Type), cfg.Label))
		}
	}

	if hasPortForwards && sm.pfGlobalStopChan == nil {
		sm.pfGlobalStopChan = make(chan struct{})
	}
	sm.mu.Unlock()

	if len(pfConfigs) > 0 {
		if sm.reporter != nil {
			sm.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeSystem,
				SourceLabel: "ServiceManager",
				Level:       reporting.LogLevelDebug,
				Message:     fmt.Sprintf("Processing %d port forward configs. About to call portforwarding.DefaultStartPortForwards.", len(pfConfigs)),
			})
		}

		pfUpdateAdapter := func(serviceLabel, statusMsg, detailsMsg string, isErrorFlag, isReadyFlag bool) {
			originalLabel, ok := pfOriginalLabels[serviceLabel]
			if !ok {
				originalLabel = serviceLabel
			}
			level := reporting.LogLevelInfo
			if isErrorFlag {
				level = reporting.LogLevelError
			} else if isReadyFlag {
				level = reporting.LogLevelInfo
			} else if detailsMsg != "" && statusMsg == "" {
				level = reporting.LogLevelDebug
			}

			updateForReporter := reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypePortForward,
				SourceLabel: originalLabel,
				Message:     statusMsg,
				Details:     detailsMsg,
				IsError:     isErrorFlag,
				IsReady:     isReadyFlag,
				Level:       level,
				// ErrorDetail will be set below if isErrorFlag is true
			}
			if isErrorFlag {
				// If detailsMsg contains the actual error text from the port forward attempt
				if detailsMsg != "" {
					updateForReporter.ErrorDetail = errors.New(detailsMsg)
				} else if statusMsg != "" && statusMsg != "Error" { // If statusMsg has specific error but detailsMsg is empty
					updateForReporter.ErrorDetail = errors.New(statusMsg)
				} else {
					updateForReporter.ErrorDetail = errors.New("port-forward operation failed") // Generic fallback
				}
			}

			if sm.reporter != nil {
				sm.reporter.Report(updateForReporter)
			}
			sm.checkAndProcessRestart(updateForReporter)
		}

		currentPfGlobalStopChan := sm.pfGlobalStopChan
		pfStopChans := portforwarding.DefaultStartPortForwards(pfConfigs, pfUpdateAdapter, currentPfGlobalStopChan, wg)
		if sm.reporter != nil {
			sm.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeSystem,
				SourceLabel: "ServiceManager",
				Level:       reporting.LogLevelDebug,
				Message:     fmt.Sprintf("Returned from portforwarding.DefaultStartPortForwards. Stop chans count: %d", len(pfStopChans)),
			})
		}

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
		mcpUpdateAdapter := func(mcpProcUpdate mcpserver.McpProcessUpdate) {
			originalLabel, ok := mcpOriginalLabels[mcpProcUpdate.Label]
			if !ok {
				originalLabel = mcpProcUpdate.Label
			}

			level := reporting.LogLevelInfo
			if mcpProcUpdate.IsError {
				level = reporting.LogLevelError
			}

			// Construct reporting.ManagedServiceUpdate from mcpserver.McpProcessUpdate
			updateForReporter := reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeMCPServer,
				SourceLabel: originalLabel,
				Message:     mcpProcUpdate.Status,
				Details:     mcpProcUpdate.OutputLog,
				IsError:     mcpProcUpdate.IsError,
				IsReady:     mcpProcUpdate.Status == "Running", // Example: derive IsReady
				ErrorDetail: mcpProcUpdate.Err,
				Level:       level,
			}
			if sm.reporter != nil {
				sm.reporter.Report(updateForReporter)
			}
			sm.checkAndProcessRestart(updateForReporter)
		}
		mcpStopChans, mcpErrs := mcpserver.StartAndManageMCPServers(mcpConfigs, mcpUpdateAdapter, wg)
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

	for _, stopChan := range sm.activeServices {
		select {
		case <-stopChan:
		default:
			close(stopChan)
		}
	}
	sm.activeServices = make(map[string]chan struct{}) // Clear the map

	// Close the global port forward stop channel if it was initialized
	if sm.pfGlobalStopChan != nil {
		select {
		case <-sm.pfGlobalStopChan:
		default:
			close(sm.pfGlobalStopChan)
		}
		sm.pfGlobalStopChan = nil // Reset for potential future StartServices calls
	}
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

	if !serviceIsCurrentlyActive {
		if sm.reporter != nil {
			// originalCfg.Type is now reporting.ServiceType
			sm.reporter.Report(reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  originalCfg.Type, // Directly use if it's reporting.ServiceType
				SourceLabel: label,
				Level:       reporting.LogLevelInfo,
				Message:     "Stopped",
				IsReady:     false,
				IsError:     false,
			})
		}
		return nil
	}
	return sm.StopService(label)
}

// checkAndProcessRestart is called by the update adapters after an update is sent.
// It now accepts reporting.ManagedServiceUpdate.
func (sm *ServiceManager) checkAndProcessRestart(update reporting.ManagedServiceUpdate) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Use fields from reporting.ManagedServiceUpdate
	isConsideredStopped := update.Message == "Stopped" || update.Message == "Error" || (update.IsError && !update.IsReady)

	if isConsideredStopped {
		delete(sm.activeServices, update.SourceLabel)

		if sm.pendingRestarts[update.SourceLabel] {
			delete(sm.pendingRestarts, update.SourceLabel)

			cfg, exists := sm.serviceConfigs[update.SourceLabel]
			if exists {
				if sm.reporter != nil {
					// cfg.Type is now reporting.ServiceType
					sm.reporter.Report(reporting.ManagedServiceUpdate{
						Timestamp:   time.Now(),
						SourceType:  cfg.Type, // Directly use if it's reporting.ServiceType
						SourceLabel: cfg.Label,
						Level:       reporting.LogLevelInfo,
						Message:     "Restarting...",
					})
				}
				var restartWg sync.WaitGroup
				go sm.startSpecificServicesLogic([]ManagedServiceConfig{cfg}, &restartWg)
			}
		}
	}
}
