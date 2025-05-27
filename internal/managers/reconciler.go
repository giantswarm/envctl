package managers

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// serviceReconciler implements the ServiceReconciler interface
type serviceReconciler struct {
	manager             *ServiceManager
	healthCheckInterval time.Duration
	healthCheckers      map[string]ServiceHealthChecker
	healthStatus        map[string]*healthStatusEntry
	serviceIntervals    map[string]time.Duration // Per-service custom intervals
	mu                  sync.RWMutex
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
}

// healthStatusEntry tracks the health status of a service
type healthStatusEntry struct {
	isHealthy bool
	lastCheck time.Time
	lastError error
}

// newServiceReconciler creates a new service reconciler
func newServiceReconciler(manager *ServiceManager) *serviceReconciler {
	return &serviceReconciler{
		manager:             manager,
		healthCheckInterval: 30 * time.Second, // Default interval
		healthCheckers:      make(map[string]ServiceHealthChecker),
		healthStatus:        make(map[string]*healthStatusEntry),
		serviceIntervals:    make(map[string]time.Duration),
	}
}

// StartHealthMonitoring starts health monitoring for all services
func (r *serviceReconciler) StartHealthMonitoring(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx != nil {
		return fmt.Errorf("health monitoring already started")
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	// Start the monitoring goroutine
	r.wg.Add(1)
	go r.monitorHealth()

	logging.Info("ServiceReconciler", "Started health monitoring with interval %v", r.healthCheckInterval)
	return nil
}

// StopHealthMonitoring stops all health monitoring
func (r *serviceReconciler) StopHealthMonitoring() {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
		r.ctx = nil
	}
	r.mu.Unlock()

	// Wait for monitoring goroutine to finish
	r.wg.Wait()

	logging.Info("ServiceReconciler", "Stopped health monitoring")
}

// CheckServiceHealth performs an immediate health check on a specific service
func (r *serviceReconciler) CheckServiceHealth(ctx context.Context, label string) error {
	r.mu.RLock()
	checker, exists := r.healthCheckers[label]
	r.mu.RUnlock()

	if !exists {
		// Try to create a health checker for this service
		if err := r.createHealthChecker(label); err != nil {
			// For MCP servers, this is expected until the port is available
			if r.manager != nil {
				r.manager.mu.Lock()
				cfg, hasCfg := r.manager.serviceConfigs[label]
				r.manager.mu.Unlock()

				if hasCfg && cfg.Type == reporting.ServiceTypeMCPServer {
					// Don't treat this as an error for MCP servers, just debug log
					logging.Debug("ServiceReconciler", "MCP server %s health checker not ready: %v", label, err)
					// Update health status to indicate we're still checking
					r.updateHealthStatus(label, false, fmt.Errorf("waiting for server to be ready"))
					return nil
				}
			}
			return fmt.Errorf("no health checker available for service %s: %w", label, err)
		}

		// Try again after creating
		r.mu.RLock()
		checker, exists = r.healthCheckers[label]
		r.mu.RUnlock()

		if !exists {
			return fmt.Errorf("failed to create health checker for service %s", label)
		}
	}

	// Perform the health check
	err := checker.CheckHealth(ctx)

	// Update health status
	r.updateHealthStatus(label, err == nil, err)

	// Report health status change if needed
	r.reportHealthChange(label, err == nil, err)

	return err
}

// SetHealthCheckInterval sets the interval for periodic health checks
func (r *serviceReconciler) SetHealthCheckInterval(interval time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.healthCheckInterval = interval
	logging.Info("ServiceReconciler", "Health check interval set to %v", interval)
}

// GetHealthStatus returns the current health status of a service
func (r *serviceReconciler) GetHealthStatus(label string) (isHealthy bool, lastCheck time.Time, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, exists := r.healthStatus[label]
	if !exists {
		return false, time.Time{}, fmt.Errorf("no health status available for service %s", label)
	}

	return status.isHealthy, status.lastCheck, status.lastError
}

// monitorHealth runs periodic health checks on all active services
func (r *serviceReconciler) monitorHealth() {
	defer r.wg.Done()

	// Wait for context to be set (defensive programming)
	r.mu.RLock()
	ctx := r.ctx
	r.mu.RUnlock()

	if ctx == nil {
		logging.Error("ServiceReconciler", nil, "Context is nil in monitorHealth")
		return
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		logging.Debug("ServiceReconciler", "Context cancelled before monitoring started")
		return
	default:
		// Continue with monitoring
	}

	// Check if manager is nil
	if r.manager == nil {
		logging.Error("ServiceReconciler", nil, "Manager is nil in monitorHealth")
		return
	}

	// Use a shorter base ticker that can handle different intervals
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Track last check time for each service
	lastChecks := make(map[string]time.Time)

	// Perform initial health check
	logging.Debug("ServiceReconciler", "About to call checkAllServices")
	r.checkAllServices()
	logging.Debug("ServiceReconciler", "About to call GetActiveServices")
	activeServices := r.manager.GetActiveServices()
	logging.Debug("ServiceReconciler", "Got %d active services", len(activeServices))
	for _, label := range activeServices {
		lastChecks[label] = time.Now()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check each service based on its individual interval
			now := time.Now()
			activeServices := r.manager.GetActiveServices()

			for _, label := range activeServices {
				// Get the interval for this service
				interval := r.getServiceInterval(label)

				// Check if enough time has passed since last check
				if lastCheck, exists := lastChecks[label]; !exists || now.Sub(lastCheck) >= interval {
					// Skip if context is cancelled
					if ctx.Err() != nil {
						return
					}

					// Create health check context with timeout
					checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

					// Perform health check in goroutine to avoid blocking
					go func(serviceLabel string) {
						defer cancel()

						if err := r.CheckServiceHealth(checkCtx, serviceLabel); err != nil {
							logging.Debug("ServiceReconciler", "Health check failed for %s: %v", serviceLabel, err)
						}
					}(label)

					// Update last check time
					lastChecks[label] = now
				}
			}

			// Clean up lastChecks for services that are no longer active
			for label := range lastChecks {
				found := false
				for _, activeLabel := range activeServices {
					if label == activeLabel {
						found = true
						break
					}
				}
				if !found {
					delete(lastChecks, label)
				}
			}
		}
	}
}

// getServiceInterval returns the health check interval for a specific service
func (r *serviceReconciler) getServiceInterval(label string) time.Duration {
	r.mu.RLock()
	if interval, exists := r.serviceIntervals[label]; exists {
		r.mu.RUnlock()
		return interval
	}
	r.mu.RUnlock()

	// Return default interval
	return r.healthCheckInterval
}

// checkAllServices performs health checks on all active services
func (r *serviceReconciler) checkAllServices() {
	// Check if manager is nil
	if r.manager == nil {
		logging.Error("ServiceReconciler", nil, "Manager is nil in checkAllServices")
		return
	}

	// Check if context is nil
	if r.ctx == nil {
		logging.Error("ServiceReconciler", nil, "Context is nil in checkAllServices")
		return
	}

	// Get all active services
	activeServices := r.manager.GetActiveServices()

	for _, label := range activeServices {
		// Skip if context is cancelled
		if r.ctx.Err() != nil {
			return
		}

		// Create health check context with timeout
		ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)

		// Perform health check in goroutine to avoid blocking
		go func(serviceLabel string) {
			defer cancel()

			if err := r.CheckServiceHealth(ctx, serviceLabel); err != nil {
				logging.Debug("ServiceReconciler", "Health check failed for %s: %v", serviceLabel, err)
			}
		}(label)
	}
}

// createHealthChecker creates a health checker for a service based on its type
func (r *serviceReconciler) createHealthChecker(label string) error {
	r.manager.mu.Lock()
	cfg, exists := r.manager.serviceConfigs[label]
	r.manager.mu.Unlock()

	if !exists {
		return fmt.Errorf("service configuration not found")
	}

	var checker ServiceHealthChecker
	var customInterval time.Duration

	switch cfg.Type {
	case reporting.ServiceTypeKube:
		if k8sCfg, ok := cfg.Config.(K8sConnectionConfig); ok {
			checker = &k8sConnectionHealthChecker{
				config: k8sCfg,
			}
			// K8s connections already have their interval in the config
			if k8sCfg.HealthCheckInterval > 0 {
				customInterval = k8sCfg.HealthCheckInterval
			}
		}
	case reporting.ServiceTypePortForward:
		if pfCfg, ok := cfg.Config.(config.PortForwardDefinition); ok {
			checker = &portForwardHealthChecker{
				config: pfCfg,
			}
			// Use custom interval if specified
			if pfCfg.HealthCheckInterval > 0 {
				customInterval = pfCfg.HealthCheckInterval
			}
		}
	case reporting.ServiceTypeMCPServer:
		if mcpCfg, ok := cfg.Config.(config.MCPServerDefinition); ok {
			// Get the current port from the state store
			if r.manager.reporter != nil && r.manager.reporter.GetStateStore() != nil {
				snapshot, exists := r.manager.reporter.GetStateStore().GetServiceState(label)
				if exists && snapshot.ProxyPort > 0 {
					checker = &mcpServerHealthChecker{
						config: mcpCfg,
						port:   snapshot.ProxyPort,
					}
					// Use custom interval if specified
					if mcpCfg.HealthCheckInterval > 0 {
						customInterval = mcpCfg.HealthCheckInterval
					}
					logging.Debug("ServiceReconciler", "Created MCP health checker for %s on port %d", label, snapshot.ProxyPort)
				} else {
					// Port not available yet, don't create checker
					// It will be created on next health check attempt when port is available
					logging.Debug("ServiceReconciler", "MCP server %s port not available yet (port: %d), will retry later", label, snapshot.ProxyPort)
					return fmt.Errorf("MCP server port not available yet")
				}
			} else {
				logging.Debug("ServiceReconciler", "No state store available for MCP server %s", label)
				return fmt.Errorf("state store not available")
			}
		}
	}

	if checker == nil {
		return fmt.Errorf("unable to create health checker for service type %s", cfg.Type)
	}

	r.mu.Lock()
	r.healthCheckers[label] = checker
	// Store custom interval if specified
	if customInterval > 0 {
		r.serviceIntervals[label] = customInterval
		logging.Debug("ServiceReconciler", "Service %s using custom health check interval: %v", label, customInterval)
	}
	r.mu.Unlock()

	return nil
}

// updateHealthStatus updates the health status for a service
func (r *serviceReconciler) updateHealthStatus(label string, isHealthy bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.healthStatus[label]; !exists {
		r.healthStatus[label] = &healthStatusEntry{}
	}

	r.healthStatus[label].isHealthy = isHealthy
	r.healthStatus[label].lastCheck = time.Now()
	r.healthStatus[label].lastError = err
}

// reportHealthChange reports health status changes through the reporter
func (r *serviceReconciler) reportHealthChange(label string, isHealthy bool, err error) {
	if r.manager.reporter == nil {
		return
	}

	// Get current service state from state store
	stateStore := r.manager.reporter.GetStateStore()
	if stateStore == nil {
		return
	}

	snapshot, exists := stateStore.GetServiceState(label)
	if !exists {
		return
	}

	// Only report if IsReady state changed
	if snapshot.IsReady != isHealthy {
		update := reporting.NewManagedServiceUpdate(
			snapshot.SourceType,
			label,
			snapshot.State, // Keep current state
		).WithCause("health_check")

		// Set IsReady based on health check
		update.IsReady = isHealthy

		// Preserve other fields
		update.ProxyPort = snapshot.ProxyPort
		update.PID = snapshot.PID

		// Only add error details if the service is actually in a failed state
		// Don't use WithError for health check failures as it changes state to Failed
		if err != nil && snapshot.State == reporting.StateFailed {
			update = update.WithError(err)
		} else if err != nil {
			// Just set the error detail without changing the state
			update.ErrorDetail = err
		}

		r.manager.reporter.Report(update)

		logging.Debug("ServiceReconciler", "Reported health change for %s: IsReady=%v", label, isHealthy)
	}
}

// cleanupHealthChecker removes the health checker for a stopped service
func (r *serviceReconciler) cleanupHealthChecker(label string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.healthCheckers, label)
	delete(r.healthStatus, label)
	delete(r.serviceIntervals, label)
}
