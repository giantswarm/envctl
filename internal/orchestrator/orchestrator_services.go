package orchestrator

import (
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"time"
)

// StopService stops a specific service through the orchestrator
// This handles cascade stops and dependency tracking
func (o *Orchestrator) StopService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Generate correlation ID for this user action
	correlationID := reporting.GenerateCorrelationID()
	logging.Info("Orchestrator", "User requested stop for service: %s (correlationID: %s)", label, correlationID)

	// Get the node ID for this service
	o.mu.Lock()
	cfg, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}

	nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)

	// Mark as manual stop
	o.stopReasons[label] = StopReasonManual
	o.mu.Unlock()

	// Use cascading stop to properly handle dependencies
	return o.stopServiceWithDependentsCorrelated(nodeID, "user_action", correlationID)
}

// RestartService restarts a specific service through the orchestrator
func (o *Orchestrator) RestartService(label string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Generate correlation ID for this user action
	correlationID := reporting.GenerateCorrelationID()

	o.mu.Lock()
	_, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.Unlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}

	// Clear manual stop reason if it exists
	delete(o.stopReasons, label)

	// Mark for restart
	o.pendingRestarts[label] = true
	o.mu.Unlock()

	logging.Info("Orchestrator", "User requested restart for service: %s (correlationID: %s)", label, correlationID)

	// Check if service is active
	if !o.serviceMgr.IsServiceActive(label) {
		// Service not active, start it directly with its dependencies
		o.mu.Lock()
		delete(o.pendingRestarts, label) // Clear pending restart
		o.mu.Unlock()

		// Start the service and any dependencies that were stopped due to cascade
		return o.startServiceWithDependencies(label)
	}

	// Stop the service - restart will be triggered by state update handler
	return o.serviceMgr.StopService(label)
}

// startServiceWithDependencies starts a service and any of its dependencies that were stopped due to cascade
func (o *Orchestrator) startServiceWithDependencies(label string) error {
	o.mu.RLock()
	cfg, exists := o.serviceConfigs[label]
	if !exists {
		o.mu.RUnlock()
		return fmt.Errorf("no configuration found for service %s", label)
	}

	// Get the node ID for this service
	nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)
	o.mu.RUnlock()

	logging.Debug("Orchestrator", "startServiceWithDependencies for %s (nodeID: %s)", label, nodeID)

	// Collect all services to start: the requested service plus its dependencies
	var configsToStart []managers.ManagedServiceConfig

	// Add the requested service
	configsToStart = append(configsToStart, cfg)

	// Find and add dependencies that should be restarted
	if o.depGraph != nil {
		node := o.depGraph.Get(dependency.NodeID(nodeID))
		if node != nil {
			logging.Debug("Orchestrator", "Service %s depends on: %v", label, node.DependsOn)
			// Check each dependency
			for _, depNodeID := range node.DependsOn {
				depLabel := o.getLabelFromNodeID(string(depNodeID))

				// Skip k8s nodes
				if strings.HasPrefix(string(depNodeID), "k8s-") {
					continue
				}

				o.mu.RLock()
				// Check if this dependency was stopped due to cascade (not manual)
				reason, hasReason := o.stopReasons[depLabel]
				depCfg, hasConfig := o.serviceConfigs[depLabel]
				isActive := o.serviceMgr.IsServiceActive(depLabel)
				o.mu.RUnlock()

				logging.Debug("Orchestrator", "Checking dependency %s: hasReason=%v, reason=%v, hasConfig=%v, isActive=%v",
					depLabel, hasReason, reason, hasConfig, isActive)

				if hasConfig && !isActive {
					// If the dependency is not active, we should start it regardless of stop reason
					// This ensures dependencies are satisfied
					configsToStart = append(configsToStart, depCfg)
					logging.Info("Orchestrator", "Including dependency %s for restart", depLabel)
					// Clear the stop reason since we're restarting it
					o.mu.Lock()
					delete(o.stopReasons, depLabel)
					o.mu.Unlock()
				}
			}
		}
	}

	logging.Debug("Orchestrator", "Starting %d services in dependency order", len(configsToStart))

	// Start all services in dependency order
	return o.startServicesInDependencyOrder(configsToStart)
}

// stopServiceWithDependentsCorrelated stops a service and all its dependents with correlation tracking
func (o *Orchestrator) stopServiceWithDependentsCorrelated(nodeID, causedBy, correlationID string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Find all dependents
	dependents := o.findAllDependents(nodeID)

	// Record cascade operation if there are dependents
	if len(dependents) > 0 && o.reporter != nil && o.reporter.GetStateStore() != nil {
		cascade := reporting.CascadeInfo{
			InitiatingService: nodeID,
			AffectedServices:  dependents,
			Reason:            causedBy,
			CorrelationID:     correlationID,
			Timestamp:         time.Now(),
			CascadeType:       reporting.CascadeTypeStop,
		}
		o.reporter.GetStateStore().RecordCascadeOperation(cascade)
	}

	// Stop dependents first (reverse dependency order)
	for i := len(dependents) - 1; i >= 0; i-- {
		dependentNodeID := dependents[i]
		if strings.HasPrefix(dependentNodeID, "k8s-") {
			// Skip K8s connections - they are managed separately
			continue
		}

		dependentLabel := o.getLabelFromNodeID(dependentNodeID)

		o.mu.Lock()
		o.stopReasons[dependentLabel] = StopReasonDependency
		o.mu.Unlock()

		logging.Info("Orchestrator", "Stopping dependent service %s due to %s (correlationID: %s)", dependentLabel, causedBy, correlationID)
		if err := o.serviceMgr.StopService(dependentLabel); err != nil {
			logging.Error("Orchestrator", err, "Failed to stop dependent service %s", dependentLabel)
		}
	}

	// Stop the main service if it's not a K8s connection
	if !strings.HasPrefix(nodeID, "k8s-") {
		mainLabel := o.getLabelFromNodeID(nodeID)
		logging.Info("Orchestrator", "Stopping service %s due to %s (correlationID: %s)", mainLabel, causedBy, correlationID)
		return o.serviceMgr.StopService(mainLabel)
	}

	return nil
}

// startServicesDependingOnCorrelated starts services that depend on the given node with correlation tracking
func (o *Orchestrator) startServicesDependingOnCorrelated(nodeID, causedBy, correlationID string) error {
	if o.serviceMgr == nil {
		return fmt.Errorf("service manager not initialized")
	}

	// Find all services that were stopped due to dependency failure
	var servicesToRestart []managers.ManagedServiceConfig

	o.mu.RLock()
	for label, reason := range o.stopReasons {
		if reason == StopReasonDependency {
			if cfg, exists := o.serviceConfigs[label]; exists {
				// Check if this service depends on the recovered node
				serviceNodeID := o.getNodeIDForService(cfg.Label, cfg.Type)
				if o.depGraph != nil {
					dependencies := o.depGraph.Dependencies(dependency.NodeID(serviceNodeID))
					for _, dep := range dependencies {
						if string(dep) == nodeID {
							servicesToRestart = append(servicesToRestart, cfg)
							break
						}
					}
				}
			}
		}
	}
	o.mu.RUnlock()

	if len(servicesToRestart) == 0 {
		return nil
	}

	// Record cascade operation
	if o.reporter != nil && o.reporter.GetStateStore() != nil {
		var affectedServices []string
		for _, cfg := range servicesToRestart {
			affectedServices = append(affectedServices, cfg.Label)
		}

		cascade := reporting.CascadeInfo{
			InitiatingService: nodeID,
			AffectedServices:  affectedServices,
			Reason:            causedBy,
			CorrelationID:     correlationID,
			Timestamp:         time.Now(),
			CascadeType:       reporting.CascadeTypeRestart,
		}
		o.reporter.GetStateStore().RecordCascadeOperation(cascade)
	}

	// Clear stop reasons for services being restarted
	o.mu.Lock()
	for _, cfg := range servicesToRestart {
		delete(o.stopReasons, cfg.Label)
	}
	o.mu.Unlock()

	logging.Info("Orchestrator", "Restarting %d services that depend on %s (correlationID: %s)", len(servicesToRestart), nodeID, correlationID)

	// Start services in dependency order
	return o.startServicesInDependencyOrder(servicesToRestart)
}

// Backwards compatibility methods that use the new correlated versions
func (o *Orchestrator) stopServiceWithDependents(label string) error {
	return o.stopServiceWithDependentsCorrelated(label, "unknown", reporting.GenerateCorrelationID())
}

func (o *Orchestrator) startServicesDependingOn(nodeID string) error {
	return o.startServicesDependingOnCorrelated(nodeID, "unknown", reporting.GenerateCorrelationID())
}

// ReconfigureAndRestart stops all services and restarts with new configuration
// This is used when switching clusters
func (o *Orchestrator) ReconfigureAndRestart(mcName, wcName string, portForwards []config.PortForwardDefinition, mcpServers []config.MCPServerDefinition) error {
	// Stop all current services
	if o.serviceMgr != nil {
		o.serviceMgr.StopAllServices()
		// Give services a moment to stop cleanly
		time.Sleep(250 * time.Millisecond)
	}

	// Clear all state
	o.mu.Lock()
	o.stopReasons = make(map[string]StopReason)
	o.pendingRestarts = make(map[string]bool)
	o.serviceConfigs = make(map[string]managers.ManagedServiceConfig)
	o.mu.Unlock()

	// Update configuration
	o.mcName = mcName
	o.wcName = wcName
	o.portForwards = portForwards
	o.mcpServers = mcpServers

	// Rebuild dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Reinitialize service configs
	o.initializeServiceConfigs()

	// Get all enabled services
	var allServices []managers.ManagedServiceConfig
	o.mu.RLock()
	for _, cfg := range o.serviceConfigs {
		allServices = append(allServices, cfg)
	}
	o.mu.RUnlock()

	// Start all services in dependency order
	return o.startServicesInDependencyOrder(allServices)
}
