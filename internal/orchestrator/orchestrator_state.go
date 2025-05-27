package orchestrator

import (
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"time"
)

// serviceStateInterceptor intercepts service state updates to handle restarts
type serviceStateInterceptor struct {
	orchestrator     *Orchestrator
	originalReporter reporting.ServiceReporter
}

// Report intercepts service state updates
func (s *serviceStateInterceptor) Report(update reporting.ManagedServiceUpdate) {
	// Forward to original reporter
	if s.originalReporter != nil {
		s.originalReporter.Report(update)
	}

	// Check for restart handling
	s.orchestrator.handleServiceStateUpdate(update)
}

// GetStateStore forwards to the original reporter
func (s *serviceStateInterceptor) GetStateStore() reporting.StateStore {
	if s.originalReporter != nil {
		return s.originalReporter.GetStateStore()
	}
	return nil
}

// handleServiceStateUpdate processes service state changes and triggers restarts if needed
func (o *Orchestrator) handleServiceStateUpdate(update reporting.ManagedServiceUpdate) {
	o.mu.Lock()
	defer o.mu.Unlock()

	label := update.SourceLabel

	// Log the state update with correlation info
	logging.Debug("Orchestrator", "Received service state update: %s -> %s (correlationID: %s, causedBy: %s)",
		label, update.State, update.CorrelationID, update.CausedBy)

	// Check if this service was pending restart
	if wasPendingRestart, exists := o.pendingRestarts[label]; exists && wasPendingRestart {
		if update.State == reporting.StateStopped || update.State == reporting.StateFailed {
			// Service has stopped, now restart it
			delete(o.pendingRestarts, label)

			if _, configExists := o.serviceConfigs[label]; configExists {
				logging.Info("Orchestrator", "Restarting service %s after stop (correlationID: %s)", label, update.CorrelationID)

				// Start the service with correlation tracking
				go func() {
					// Add a small delay to ensure the port is released
					time.Sleep(1 * time.Second)

					// Use startServiceWithDependencies to also restart any dependencies
					if err := o.startServiceWithDependencies(label); err != nil {
						logging.Error("Orchestrator", err, "Failed to restart service %s", label)

						// Report restart failure with correlation
						if o.reporter != nil {
							failureUpdate := reporting.NewManagedServiceUpdate(
								update.SourceType,
								label,
								reporting.StateFailed,
							).WithCause("restart_failed", update.CorrelationID).WithError(err)

							o.reporter.Report(failureUpdate)
						}
					}
				}()
			}
		}
	}

	// Check if a service has failed - need to stop dependent services
	if update.State == reporting.StateFailed {
		// Get the node ID for this service
		cfg, exists := o.serviceConfigs[label]
		if exists {
			nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)

			// Mark this service as stopped due to failure
			o.stopReasons[label] = StopReasonDependency

			// Stop all dependent services
			go func() {
				correlationID := reporting.GenerateCorrelationID()
				logging.Info("Orchestrator", "Service %s failed, stopping dependent services (correlationID: %s)", label, correlationID)
				if err := o.stopServiceWithDependentsCorrelated(nodeID, "dependency_failure", correlationID); err != nil {
					logging.Error("Orchestrator", err, "Failed to stop dependent services for failed service %s", label)
				}
			}()
		}
	}

	// Check if a service has become running - might need to restart dependent services
	if update.State == reporting.StateRunning {
		// Get the node ID for this service
		cfg, exists := o.serviceConfigs[label]
		if exists {
			nodeID := o.getNodeIDForService(cfg.Label, cfg.Type)

			// Check if any services were stopped due to this service failing
			go func() {
				// Small delay to ensure state is settled
				time.Sleep(100 * time.Millisecond)

				// Use the existing method to start services depending on this one
				correlationID := reporting.GenerateCorrelationID()
				if err := o.startServicesDependingOnCorrelated(nodeID, "dependency_restored", correlationID); err != nil {
					logging.Error("Orchestrator", err, "Failed to restart dependent services for %s", label)
				}
			}()
		}
	}
}
