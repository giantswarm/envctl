package managers

import (
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"strings"
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
	initialWg     *sync.WaitGroup
	reporter      reporting.ServiceReporter
	serviceStates map[string]reporting.ServiceState // Added to track current state of services
	// Track the reason for stopping a service
	stopReasons map[string]StopReason // Map label to reason for stopping
}

// StopReason tracks why a service was stopped
type StopReason int

const (
	StopReasonManual     StopReason = iota // User explicitly stopped the service
	StopReasonDependency                   // Service stopped due to dependency failure
)

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
		serviceStates:   make(map[string]reporting.ServiceState), // Initialize the new map
		stopReasons:     make(map[string]StopReason),
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
	var pfConfigs []config.PortForwardDefinition
	var mcpConfigs []config.MCPServerDefinition
	pfOriginalLabels := make(map[string]string)
	mcpOriginalLabels := make(map[string]string)

	for _, cfg := range configs {
		// cfg.Type is now reporting.ServiceType
		switch cfg.Type {
		case reporting.ServiceTypePortForward:
			if pfConfig, ok := cfg.Config.(config.PortForwardDefinition); ok {
				pfConfigs = append(pfConfigs, pfConfig)
				pfOriginalLabels[pfConfig.Name] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid port forward config for label %s: type assertion failed to config.PortForwardDefinition", cfg.Label))
			}
		case reporting.ServiceTypeMCPServer:
			if mcpConfig, ok := cfg.Config.(config.MCPServerDefinition); ok {
				mcpConfigs = append(mcpConfigs, mcpConfig)
				mcpOriginalLabels[mcpConfig.Name] = cfg.Label
			} else {
				startupErrors = append(startupErrors, fmt.Errorf("invalid MCP server config for label %s: type assertion failed to config.MCPServerDefinition", cfg.Label))
			}
		default:
			if sm.reporter != nil {
				sm.reporter.Report(reporting.ManagedServiceUpdate{
					Timestamp:   time.Now(),
					SourceType:  reporting.ServiceTypeSystem,
					SourceLabel: "ServiceManager",
					State:       reporting.StateFailed,
					ErrorDetail: fmt.Errorf("Unknown service type in ManagedServiceConfig for label %s: %s", cfg.Label, cfg.Type),
					IsReady:     false,
				})
			}
			startupErrors = append(startupErrors, fmt.Errorf("unknown service type %q for label %q", string(cfg.Type), cfg.Label))
		}
	}

	sm.mu.Unlock()

	if len(pfConfigs) > 0 {
		logging.Debug("ServiceManager", "Processing %d port forward configs.", len(pfConfigs))
		pfUpdateAdapter := func(serviceLabel string, statusDetail portforwarding.PortForwardStatusDetail, isOpReady bool, operationErr error) {
			sm.mu.Lock() // Lock early to protect serviceStates access

			originalLabel, ok := pfOriginalLabels[serviceLabel]
			if !ok {
				originalLabel = serviceLabel // Fallback
			}

			var state reporting.ServiceState

			if operationErr != nil {
				state = reporting.StateFailed
			} else {
				switch statusDetail {
				case portforwarding.StatusDetailForwardingActive:
					if isOpReady { // Double check with isOpReady
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
						logging.Debug("ServiceManager", "Unmapped PortForwardStatusDetail '%s' for service %s. isOpReady: %t", statusDetail, originalLabel, isOpReady)
					}
				}
			}

			lastReportedState, known := sm.serviceStates[originalLabel]
			updateForReporter := reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypePortForward,
				SourceLabel: originalLabel,
				State:       state,
				IsReady:     (state == reporting.StateRunning),
				ErrorDetail: operationErr,
			}

			if !known || state != lastReportedState {
				sm.serviceStates[originalLabel] = state

				// Clear stop reason when service becomes running
				if state == reporting.StateRunning {
					delete(sm.stopReasons, originalLabel)
				}

				logMessage := fmt.Sprintf("Service %s (PortForward) state changed to: %s", originalLabel, state)
				if state == reporting.StateFailed || operationErr != nil {
					logging.Error("ServiceManager", operationErr, "%s", logMessage)
				} else if state == reporting.StateRunning || state == reporting.StateStopped {
					logging.Info("ServiceManager", "%s", logMessage)
				} else {
					logging.Debug("ServiceManager", "%s", logMessage) // For transient states
				}

				if sm.reporter != nil {
					sm.reporter.Report(updateForReporter)
				}
				sm.mu.Unlock() // Unlock before calling checkAndProcessRestart
				sm.checkAndProcessRestart(updateForReporter)
			} else {
				sm.mu.Unlock() // Unlock if no state change
			}
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
			sm.mu.Lock() // Lock early

			originalLabel, ok := mcpOriginalLabels[mcpStatusUpdate.Label]
			if !ok {
				originalLabel = mcpStatusUpdate.Label // Fallback
			}

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

			lastReportedState, known := sm.serviceStates[originalLabel]

			updateForReporter := reporting.ManagedServiceUpdate{
				Timestamp:   time.Now(),
				SourceType:  reporting.ServiceTypeMCPServer,
				SourceLabel: originalLabel,
				State:       state,
				IsReady:     (state == reporting.StateRunning),
				ErrorDetail: mcpStatusUpdate.ProcessErr,
				ProxyPort:   mcpStatusUpdate.ProxyPort,
				PID:         mcpStatusUpdate.PID,
			}

			if !known || state != lastReportedState {
				sm.serviceStates[originalLabel] = state

				// Clear stop reason when service becomes running
				if state == reporting.StateRunning {
					delete(sm.stopReasons, originalLabel)
				}

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
					logging.Debug("ServiceManager", "%s", logMessage) // For transient states
				}

				if sm.reporter != nil {
					sm.reporter.Report(updateForReporter)
				}
				sm.mu.Unlock() // Unlock before calling checkAndProcessRestart
				sm.checkAndProcessRestart(updateForReporter)
			} else {
				sm.mu.Unlock() // Unlock if no state change
			}
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
	// Default to manual stop when called directly
	return sm.stopServiceWithReason(label, StopReasonManual)
}

// stopServiceWithReason is the internal method that tracks why a service was stopped
func (sm *ServiceManager) stopServiceWithReason(label string, reason StopReason) error {
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

	// Track the stop reason
	sm.stopReasons[label] = reason

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
			logging.Debug("ServiceManager", "StopAllServices: Closing channel for service '%s'.", label)
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

		updateForRestart := reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  originalCfg.Type,
			SourceLabel: label,
			State:       reporting.StateStopped,
			IsReady:     false,
		}
		if sm.reporter != nil {
			sm.reporter.Report(updateForRestart)
		}
		// Explicitly call checkAndProcessRestart here to trigger the pending restart logic
		sm.checkAndProcessRestart(updateForRestart)
		return nil
	}
	return sm.StopService(label) // This will eventually lead to a "Stopped" state update, triggering restart via checkAndProcessRestart
}

// checkAndProcessRestart is called by the update adapters and now also by RestartService.
// It now uses reporting.ManagedServiceUpdate.State to determine if a service is stopped.
func (sm *ServiceManager) checkAndProcessRestart(update reporting.ManagedServiceUpdate) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	isConsideredStopped := update.State == reporting.StateStopped || update.State == reporting.StateFailed

	if isConsideredStopped {
		delete(sm.activeServices, update.SourceLabel) // Remove from active services

		if sm.pendingRestarts[update.SourceLabel] {
			delete(sm.pendingRestarts, update.SourceLabel) // Consume the pending restart flag
			// Clear stop reason when restarting
			delete(sm.stopReasons, update.SourceLabel)

			cfg, exists := sm.serviceConfigs[update.SourceLabel]
			if exists {
				logging.Info("ServiceManager", "Pending restart: Preparing to start service %s again.", cfg.Label)

				// Report "StateStarting" if it's a new state for this service instance before actually starting
				if sm.serviceStates[cfg.Label] != reporting.StateStarting {
					sm.serviceStates[cfg.Label] = reporting.StateStarting
					if sm.reporter != nil {
						sm.reporter.Report(reporting.ManagedServiceUpdate{
							Timestamp:   time.Now(),
							SourceType:  cfg.Type,
							SourceLabel: cfg.Label,
							State:       reporting.StateStarting,
							IsReady:     false,
						})
					}
					logging.Info("ServiceManager", "Service %s (restarting) state changed to: %s", cfg.Label, reporting.StateStarting)
				}

				// var restartWg sync.WaitGroup // Removed as it's unused
				// Consider if initialWg needs incrementing if it's for overall app completion.
				// For now, restartWg manages this specific goroutine.
				// If sm.initialWg is used, it should be sm.initialWg.Add(1) before goroutine
				// and sm.initialWg.Done() in the goroutine.
				// For simplicity, this example uses a local wg for the restart goroutine.
				if sm.initialWg != nil { // Assuming sm.initialWg is the global WaitGroup
					sm.initialWg.Add(1)
				}
				go func(localWg *sync.WaitGroup) {
					if localWg != nil { // Check if using a local waitgroup (restartWg in this case, but passed as initialWg to startSpecificServicesLogic)
						// if sm.initialWg was used above, then this should be sm.initialWg.Done()
						defer localWg.Done() // If this is the global initialWg
					}
					sm.startSpecificServicesLogic([]ManagedServiceConfig{cfg}, localWg) // Pass the original config and appropriate wg
				}(sm.initialWg) // Pass the global WaitGroup, or manage a separate one for restarts
			} else {
				logging.Warn("ServiceManager", "Pending restart for %s, but no config found. Removing from tracked states.", update.SourceLabel)
				delete(sm.serviceStates, update.SourceLabel) // Clean up state map
			}
		} else { // Service stopped/failed and no restart is pending for it
			logging.Debug("ServiceManager", "Service %s stopped/failed and no restart pending. Removing from tracked states.", update.SourceLabel)
			delete(sm.serviceStates, update.SourceLabel) // Clean up state map
			// Don't clear stop reasons here - we need to keep them to know if service was manually stopped
		}
	}
}

// StopServiceWithDependents stops a service and all services that depend on it.
// It traverses the dependency graph to find all dependent services and stops them first.
func (sm *ServiceManager) StopServiceWithDependents(label string, depGraph *dependency.Graph) error {
	if depGraph == nil {
		// If no dependency graph provided, fall back to regular stop
		return sm.StopService(label)
	}

	// Build a list of services to stop in the correct order
	toStop := []string{}
	visited := make(map[string]bool)

	// Recursive function to collect dependents
	var collectDependents func(nodeID string)
	collectDependents = func(nodeID string) {
		if visited[nodeID] {
			return
		}
		visited[nodeID] = true

		// Get all services that depend on this one
		dependents := depGraph.Dependents(dependency.NodeID(nodeID))
		for _, dependent := range dependents {
			collectDependents(string(dependent))
		}

		// Add to stop list after processing dependents (post-order traversal)
		toStop = append(toStop, nodeID)
	}

	// Start collection from the target service
	// Convert label to node ID format
	nodeID := label
	if !strings.Contains(label, ":") {
		// Try to determine the type based on active services
		sm.mu.Lock()
		if cfg, exists := sm.serviceConfigs[label]; exists {
			switch cfg.Type {
			case reporting.ServiceTypePortForward:
				nodeID = "pf:" + label
			case reporting.ServiceTypeMCPServer:
				nodeID = "mcp:" + label
			}
		}
		sm.mu.Unlock()
	}

	collectDependents(nodeID)

	// Stop services in reverse order (dependents first)
	var errors []error
	for i := 0; i < len(toStop); i++ {
		serviceToStop := toStop[i]

		// Skip k8s nodes - they're not actual services
		if strings.HasPrefix(serviceToStop, "k8s:") {
			logging.Debug("ServiceManager", "Skipping k8s node %s (not an actual service)", serviceToStop)
			continue
		}

		// Extract the actual service label from node ID
		parts := strings.SplitN(serviceToStop, ":", 2)
		if len(parts) == 2 {
			serviceLabel := parts[1]

			// Determine stop reason:
			// - The originally requested service (matching label) is stopped manually
			// - All dependent services are stopped due to dependency
			stopReason := StopReasonDependency
			if serviceLabel == label {
				// Check if this was called from the orchestrator (for k8s health failures)
				// by checking if the original label starts with "k8s:"
				if strings.HasPrefix(label, "k8s:") {
					stopReason = StopReasonDependency
				} else {
					stopReason = StopReasonManual
				}
			}

			if err := sm.stopServiceWithReason(serviceLabel, stopReason); err != nil {
				// Don't fail completely, collect errors
				errors = append(errors, fmt.Errorf("failed to stop %s: %w", serviceLabel, err))
			} else {
				logging.Info("ServiceManager", "Stopped service %s as part of cascade from %s (reason: %v)", serviceLabel, label, stopReason)
			}
		}
	}

	if len(errors) > 0 {
		// Combine all errors
		var errStrings []string
		for _, e := range errors {
			errStrings = append(errStrings, e.Error())
		}
		return fmt.Errorf("errors during cascade stop: %v", errStrings)
	}

	return nil
}

// StartServicesDependingOn starts all services that depend on the given node ID
func (sm *ServiceManager) StartServicesDependingOn(nodeID string, depGraph *dependency.Graph) error {
	if depGraph == nil {
		return fmt.Errorf("dependency graph is required")
	}

	// Find all services that depend on the given node
	dependents := depGraph.Dependents(dependency.NodeID(nodeID))
	if len(dependents) == 0 {
		logging.Debug("ServiceManager", "No services depend on %s", nodeID)
		return nil
	}

	// Build list of service configs to start
	var configsToStart []ManagedServiceConfig

	sm.mu.Lock()
	for _, dependentNode := range dependents {
		// Extract service label from node ID
		parts := strings.SplitN(string(dependentNode), ":", 2)
		if len(parts) != 2 {
			continue
		}
		serviceLabel := parts[1]

		// Check if service is already active
		if _, isActive := sm.activeServices[serviceLabel]; isActive {
			logging.Debug("ServiceManager", "Service %s is already active, skipping", serviceLabel)
			continue
		}

		// Check if service was stopped due to dependency failure
		stopReason, hasStopReason := sm.stopReasons[serviceLabel]
		if hasStopReason && stopReason == StopReasonManual {
			logging.Info("ServiceManager", "Service %s was manually stopped, not restarting", serviceLabel)
			continue
		}

		// Get the service config
		if cfg, exists := sm.serviceConfigs[serviceLabel]; exists {
			configsToStart = append(configsToStart, cfg)
			logging.Info("ServiceManager", "Preparing to start service %s as it depends on %s", serviceLabel, nodeID)
		} else {
			logging.Warn("ServiceManager", "No config found for dependent service %s", serviceLabel)
		}
	}
	sm.mu.Unlock()

	if len(configsToStart) == 0 {
		logging.Debug("ServiceManager", "No services to start that depend on %s", nodeID)
		return nil
	}

	// Start the services in dependency order
	logging.Info("ServiceManager", "Starting %d services that depend on %s", len(configsToStart), nodeID)
	_, errs := sm.StartServicesWithDependencyOrder(configsToStart, depGraph, sm.initialWg)

	if len(errs) > 0 {
		var errStrings []string
		for _, e := range errs {
			errStrings = append(errStrings, e.Error())
		}
		return fmt.Errorf("errors starting services: %v", errStrings)
	}

	return nil
}

// StartServicesWithDependencyOrder starts services in the correct order based on dependencies
func (sm *ServiceManager) StartServicesWithDependencyOrder(
	configs []ManagedServiceConfig,
	depGraph *dependency.Graph,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	if depGraph == nil {
		// No dependency graph, fall back to regular start
		return sm.StartServices(configs, wg)
	}

	// Build a map of configs by node ID for quick lookup
	configsByNodeID := make(map[string]ManagedServiceConfig)
	nodeIDToLabel := make(map[string]string)
	for _, cfg := range configs {
		nodeID := ""
		switch cfg.Type {
		case reporting.ServiceTypePortForward:
			nodeID = "pf:" + cfg.Label
		case reporting.ServiceTypeMCPServer:
			nodeID = "mcp:" + cfg.Label
		}
		if nodeID != "" {
			configsByNodeID[nodeID] = cfg
			nodeIDToLabel[nodeID] = cfg.Label
		}
	}

	// Group services by dependency levels
	levels := sm.groupServicesByDependencyLevel(configsByNodeID, depGraph)
	
	// Start services level by level
	allStopChannels := make(map[string]chan struct{})
	var allErrors []error
	
	for levelIndex, level := range levels {
		if len(level) == 0 {
			continue
		}
		
		logging.Info("ServiceManager", "Starting dependency level %d with %d services", levelIndex, len(level))
		
		// Start all services in this level
		levelConfigs := make([]ManagedServiceConfig, 0, len(level))
		for _, nodeID := range level {
			if cfg, exists := configsByNodeID[nodeID]; exists {
				levelConfigs = append(levelConfigs, cfg)
			}
		}
		
		stopChans, errs := sm.startSpecificServicesLogic(levelConfigs, wg)
		
		// Collect stop channels and errors
		for label, ch := range stopChans {
			allStopChannels[label] = ch
		}
		allErrors = append(allErrors, errs...)
		
		// Wait for all services in this level to become running before starting next level
		// (except for the last level)
		if levelIndex < len(levels)-1 && len(level) > 0 {
			logging.Info("ServiceManager", "Waiting for level %d services to become running...", levelIndex)
			if err := sm.waitForServicesToBeRunning(level, nodeIDToLabel, 30*time.Second); err != nil {
				logging.Error("ServiceManager", err, "Some services in level %d did not become running", levelIndex)
				// Continue anyway - dependent services might still work
			}
		}
	}
	
	return allStopChannels, allErrors
}

// groupServicesByDependencyLevel groups services into levels based on their dependencies
func (sm *ServiceManager) groupServicesByDependencyLevel(
	configsByNodeID map[string]ManagedServiceConfig,
	depGraph *dependency.Graph,
) [][]string {
	// Calculate dependency depth for each service
	depths := make(map[string]int)
	visited := make(map[string]bool)
	
	var calculateDepth func(nodeID string) int
	calculateDepth = func(nodeID string) int {
		if depth, exists := depths[nodeID]; exists {
			return depth
		}
		
		if visited[nodeID] {
			// Circular dependency, return 0
			return 0
		}
		visited[nodeID] = true
		
		maxDepth := -1
		node := depGraph.Get(dependency.NodeID(nodeID))
		if node != nil {
			for _, dep := range node.DependsOn {
				depStr := string(dep)
				// Only consider dependencies that we're actually starting
				if _, exists := configsByNodeID[depStr]; exists {
					depDepth := calculateDepth(depStr)
					if depDepth > maxDepth {
						maxDepth = depDepth
					}
				}
			}
		}
		
		depth := maxDepth + 1
		depths[nodeID] = depth
		return depth
	}
	
	// Calculate depths for all services
	maxLevel := 0
	for nodeID := range configsByNodeID {
		depth := calculateDepth(nodeID)
		if depth > maxLevel {
			maxLevel = depth
		}
	}
	
	// Group services by level
	levels := make([][]string, maxLevel+1)
	for nodeID := range configsByNodeID {
		level := depths[nodeID]
		levels[level] = append(levels[level], nodeID)
	}
	
	return levels
}

// waitForServicesToBeRunning waits for the specified services to reach running state
func (sm *ServiceManager) waitForServicesToBeRunning(
	nodeIDs []string,
	nodeIDToLabel map[string]string,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 100 * time.Millisecond
	
	for {
		allRunning := true
		notRunning := []string{}
		
		sm.mu.Lock()
		for _, nodeID := range nodeIDs {
			label, exists := nodeIDToLabel[nodeID]
			if !exists {
				continue
			}
			
			state, exists := sm.serviceStates[label]
			if !exists || state != reporting.StateRunning {
				allRunning = false
				notRunning = append(notRunning, label)
			}
		}
		sm.mu.Unlock()
		
		if allRunning {
			logging.Info("ServiceManager", "All services in level are now running")
			return nil
		}
		
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for services to become running: %v", notRunning)
		}
		
		time.Sleep(checkInterval)
	}
}
