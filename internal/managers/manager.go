package managers

import (
	// No longer importing "envctl/internal/managers"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"fmt"
	"sync"
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
	initialUpdateCb ServiceUpdateFunc
	initialWg       *sync.WaitGroup
}

// NewServiceManager creates a new instance of ServiceManager and returns it as a ServiceManagerAPI interface.
func NewServiceManager() ServiceManagerAPI { // Returns the ServiceManagerAPI INTERFACE
	return &ServiceManager{ // Instantiates the struct ServiceManager
		activeServices:  make(map[string]chan struct{}),
		serviceConfigs:  make(map[string]ManagedServiceConfig),
		pendingRestarts: make(map[string]bool),
	}
}

// StartServices is the main entry point to start a batch of services.
// It stores the initial callback, waitgroup, and configurations,
// then calls the internal worker to start the services.
func (sm *ServiceManager) StartServices(
	configs []ManagedServiceConfig,
	updateCb ServiceUpdateFunc,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	sm.mu.Lock()
	sm.initialUpdateCb = updateCb
	sm.initialWg = wg

	if sm.serviceConfigs == nil {
		sm.serviceConfigs = make(map[string]ManagedServiceConfig)
	}
	for _, cfg := range configs {
		sm.serviceConfigs[cfg.Label] = cfg // Store/update config
	}
	sm.mu.Unlock()

	return sm.startSpecificServicesLogic(configs, updateCb, wg)
}

// startSpecificServicesLogic is the internal worker for starting services.
// It's called by StartServices and by the restart mechanism.
func (sm *ServiceManager) startSpecificServicesLogic(
	configs []ManagedServiceConfig,
	updateCb ServiceUpdateFunc, // This specific call's updateCb
	wg *sync.WaitGroup, // This specific call's wg
) (map[string]chan struct{}, []error) {
	sm.mu.Lock() // Lock for initial checks and setup

	var startupErrors []error
	allStopChannels := make(map[string]chan struct{})
	var pfConfigs []portforwarding.PortForwardingConfig
	var mcpConfigs []mcpserver.MCPServerConfig
	pfOriginalLabels := make(map[string]string)
	mcpOriginalLabels := make(map[string]string)

	hasPortForwards := false
	for _, cfg := range configs {
		// Also ensure this config is in the main serviceConfigs map if it's a fresh individual start
		// This is already handled by StartServices or when RestartService re-fetches from sm.serviceConfigs
		// if _, exists := sm.serviceConfigs[cfg.Label]; !exists {
		// 	sm.serviceConfigs[cfg.Label] = cfg
		// }

		switch cfg.Type {
		case ServiceTypePortForward:
			hasPortForwards = true
			if pfConfig, ok := cfg.Config.(portforwarding.PortForwardingConfig); ok {
				actualPfConfig := pfConfig
				actualPfConfig.Label = cfg.Label
				pfConfigs = append(pfConfigs, actualPfConfig)
				pfOriginalLabels[actualPfConfig.Label] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid port forward config for label %s", cfg.Label))
			}
		case ServiceTypeMCPServer:
			if mcpConfig, ok := cfg.Config.(mcpserver.MCPServerConfig); ok {
				actualMcpConfig := mcpConfig
				actualMcpConfig.Name = cfg.Label
				mcpConfigs = append(mcpConfigs, actualMcpConfig)
				mcpOriginalLabels[actualMcpConfig.Name] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid MCP server config for label %s", cfg.Label))
			}
		default:
			startupErrors = append(startupErrors, fmt.Errorf("unknown service type for label %s: %s", cfg.Label, cfg.Type))
		}
	}

	if hasPortForwards && sm.pfGlobalStopChan == nil {
		sm.pfGlobalStopChan = make(chan struct{})
	}
	sm.mu.Unlock() // Unlock after config processing and pfGlobalStopChan check

	// --- Start Port Forwarding Services ---
	if len(pfConfigs) > 0 {
		pfUpdateAdapter := func(pfLabel, status, outputLog string, isError, isReady bool) {
			originalLabel, ok := pfOriginalLabels[pfLabel]
			if !ok {
				originalLabel = pfLabel
			}

			genericUpdate := ManagedServiceUpdate{
				Type: ServiceTypePortForward, Label: originalLabel, Status: status,
				OutputLog: outputLog, IsError: isError, IsReady: isReady, Error: nil,
			}
			if updateCb != nil { // Use the updateCb passed to this specific call
				updateCb(genericUpdate)
			}
			sm.checkAndProcessRestart(genericUpdate) // checkAndProcessRestart uses sm.initialUpdateCb
		}

		currentPfGlobalStopChan := sm.pfGlobalStopChan
		pfStopChans := portforwarding.StartPortForwards(pfConfigs, pfUpdateAdapter, currentPfGlobalStopChan, wg)

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

	// --- Start MCP Server Services ---
	if len(mcpConfigs) > 0 {
		mcpUpdateAdapter := func(mcpUpdate mcpserver.McpProcessUpdate) {
			originalLabel, ok := mcpOriginalLabels[mcpUpdate.Label]
			if !ok {
				originalLabel = mcpUpdate.Label
			}

			genericUpdate := ManagedServiceUpdate{
				Type: ServiceTypeMCPServer, Label: originalLabel, Status: mcpUpdate.Status,
				OutputLog: mcpUpdate.OutputLog, IsError: mcpUpdate.IsError,
				IsReady: mcpUpdate.Status == "Running", Error: mcpUpdate.Err,
			}
			if updateCb != nil { // Use the updateCb passed to this specific call
				updateCb(genericUpdate)
			}
			sm.checkAndProcessRestart(genericUpdate) // checkAndProcessRestart uses sm.initialUpdateCb
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

	// Mark for restart
	sm.pendingRestarts[label] = true

	_, serviceIsCurrentlyActive := sm.activeServices[label]
	sm.mu.Unlock() // Unlock before calling StopService or synthetic update

	if !serviceIsCurrentlyActive {
		if sm.initialUpdateCb != nil {
			sm.initialUpdateCb(ManagedServiceUpdate{
				Type: originalCfg.Type, Label: label, Status: "Stopped", // Synthetic stop to trigger checkAndProcessRestart
				IsReady: false, IsError: false,
			})
		}
		return nil
	}

	return sm.StopService(label)
}

// checkAndProcessRestart is called by the update adapters after an update is sent.
// If a service has stopped and was pending restart, it triggers the restart.
func (sm *ServiceManager) checkAndProcessRestart(update ManagedServiceUpdate) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	isConsideredStopped := update.Status == "Stopped" || update.Status == "Error" || (update.IsError && !update.IsReady)

	if isConsideredStopped {
		delete(sm.activeServices, update.Label)

		if sm.pendingRestarts[update.Label] {
			delete(sm.pendingRestarts, update.Label)

			cfg, exists := sm.serviceConfigs[update.Label]
			if exists {
				if sm.initialUpdateCb != nil {
					sm.initialUpdateCb(ManagedServiceUpdate{
						Type:   cfg.Type,
						Label:  cfg.Label,
						Status: "Restarting...",
					})
				}
				var restartWg sync.WaitGroup
				// Pass sm.initialUpdateCb and the new restartWg
				// The original sm.initialWg is for the whole batch, not individual restarts.
				go sm.startSpecificServicesLogic([]ManagedServiceConfig{cfg}, sm.initialUpdateCb, &restartWg)
			}
		}
	}
}
