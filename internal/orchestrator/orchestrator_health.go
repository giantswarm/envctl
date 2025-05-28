package orchestrator

import (
	"context"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"time"
)

// startHealthCheckers starts health check goroutines for services that support health checking.
// Each service that implements the HealthChecker interface gets its own goroutine
// that periodically calls CheckHealth. This allows services to update their internal
// health status based on actual connectivity/functionality checks.
func (o *Orchestrator) startHealthCheckers() {
	allServices := o.registry.GetAll()

	logging.Debug("Orchestrator", "Starting health checkers for %d services", len(allServices))

	for _, service := range allServices {
		label := service.GetLabel()

		// Skip if health checker already running for this service
		o.mu.RLock()
		if o.healthCheckers[label] {
			o.mu.RUnlock()
			continue
		}
		o.mu.RUnlock()

		// Check if service implements HealthChecker interface
		if healthChecker, ok := service.(services.HealthChecker); ok {
			logging.Debug("Orchestrator", "Starting health checker for service: %s", label)

			// Mark health checker as running to prevent duplicates
			o.mu.Lock()
			o.healthCheckers[label] = true
			o.mu.Unlock()

			// Start a goroutine for this service's health checks
			go o.runHealthChecksForService(service, healthChecker)
		} else {
			logging.Debug("Orchestrator", "Service %s does not implement HealthChecker", label)
		}
	}
}

// runHealthChecksForService runs periodic health checks for a single service.
// This method:
// 1. Performs health checks at the interval specified by the service
// 2. Only checks health when the service is in running state
// 3. Stops checking when the service stops or the orchestrator shuts down
//
// The service is responsible for updating its own health status based on
// the CheckHealth results. The orchestrator just triggers the checks.
func (o *Orchestrator) runHealthChecksForService(service services.Service, healthChecker services.HealthChecker) {
	label := service.GetLabel()
	defer func() {
		// Clean up health checker tracking when goroutine exits
		o.mu.Lock()
		delete(o.healthCheckers, label)
		o.mu.Unlock()
		logging.Debug("Orchestrator", "Health check goroutine stopped for %s", label)
	}()

	// Get health check interval from service, use default if not specified
	interval := healthChecker.GetHealthCheckInterval()
	if interval <= 0 {
		interval = 30 * time.Second // Default interval
	}

	logging.Debug("Orchestrator", "Health check goroutine started for %s with interval %v", label, interval)

	// Perform initial health check immediately
	if service.GetState() == services.StateRunning {
		ctx, cancel := context.WithTimeout(o.ctx, 5*time.Second)
		health, err := healthChecker.CheckHealth(ctx)
		cancel()

		if err != nil {
			logging.Debug("Orchestrator", "Initial health check failed for %s: %v", label, err)
		} else {
			logging.Debug("Orchestrator", "Initial health check for %s: %s", label, health)
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			// Only check health if service is running
			if service.GetState() == services.StateRunning {
				ctx, cancel := context.WithTimeout(o.ctx, 5*time.Second)
				health, err := healthChecker.CheckHealth(ctx)
				cancel()

				if err != nil {
					logging.Debug("Orchestrator", "Health check failed for %s: %v", service.GetLabel(), err)
				} else {
					logging.Debug("Orchestrator", "Health check for %s: %s", service.GetLabel(), health)
				}

				// The service should update its own health status internally
				// based on the CheckHealth result
			} else {
				// Service is not running, stop health checks
				logging.Debug("Orchestrator", "Service %s is not running, stopping health checks", label)
				return
			}
		}
	}
}

// checkAndRestartFailedServices checks for failed services and restarts them if needed.
// This is part of the auto-recovery mechanism that attempts to restore services
// that have failed, as long as they weren't manually stopped by the user.
func (o *Orchestrator) checkAndRestartFailedServices() {
	allServices := o.registry.GetAll()

	for _, service := range allServices {
		label := service.GetLabel()

		// Skip manually stopped services to respect user intent
		o.mu.RLock()
		if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
			o.mu.RUnlock()
			continue
		}
		o.mu.RUnlock()

		// Check if service has failed
		if service.GetState() == services.StateFailed {
			// Check if dependencies are still satisfied
			if err := o.checkDependencies(label); err != nil {
				logging.Debug("Orchestrator", "Skipping restart of %s: %v", label, err)
				continue
			}

			// Attempt to restart
			logging.Info("Orchestrator", "Attempting to restart failed service: %s", label)
			if err := service.Restart(o.ctx); err != nil {
				logging.Error("Orchestrator", err, "Failed to restart service %s", label)
			}
		}
	}
}
