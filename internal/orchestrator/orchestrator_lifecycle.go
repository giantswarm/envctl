package orchestrator

import (
	"context"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"time"
)

// Start begins orchestration - builds dependency graph, starts services, and monitors health
func (o *Orchestrator) Start(ctx context.Context) error {
	// Build dependency graph
	o.depGraph = o.buildDependencyGraph()

	// Initialize service configs
	o.initializeServiceConfigs()

	// Set up service state monitoring
	o.setupServiceStateMonitoring()

	// Create K8s connection services
	k8sServices := o.createK8sConnectionServices()

	// Add K8s services to the service configs map
	o.mu.Lock()
	for _, svc := range k8sServices {
		o.serviceConfigs[svc.Label] = svc
	}
	o.mu.Unlock()

	// Start service health monitoring
	healthCtx, cancel := context.WithCancel(ctx)
	o.cancelHealthChecks = cancel
	go o.StartServiceHealthMonitoring(healthCtx)

	// Get all enabled services
	var allServices []managers.ManagedServiceConfig
	o.mu.RLock()
	for _, cfg := range o.serviceConfigs {
		// Skip manually stopped services
		if reason, exists := o.stopReasons[cfg.Label]; exists && reason == StopReasonManual {
			continue
		}
		allServices = append(allServices, cfg)
	}
	o.mu.RUnlock()

	// Start all services in dependency order
	if err := o.startServicesInDependencyOrder(allServices); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Start monitoring for services that need to be restarted
	go o.monitorAndStartServices(ctx)

	return nil
}

// Stop gracefully stops all services and health monitoring
func (o *Orchestrator) Stop() {
	if o.cancelHealthChecks != nil {
		o.cancelHealthChecks()
	}

	if o.serviceMgr != nil {
		o.serviceMgr.StopAllServices()
	}
}

// initializeServiceConfigs builds the initial service configuration map
func (o *Orchestrator) initializeServiceConfigs() {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Add port forward configs
	for _, pf := range o.portForwards {
		if !pf.Enabled {
			continue
		}
		o.serviceConfigs[pf.Name] = managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypePortForward,
			Label:  pf.Name,
			Config: pf,
		}
	}

	// Add MCP server configs
	for _, mcp := range o.mcpServers {
		if !mcp.Enabled {
			continue
		}
		o.serviceConfigs[mcp.Name] = managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeMCPServer,
			Label:  mcp.Name,
			Config: mcp,
		}
	}
}

// setupServiceStateMonitoring sets up monitoring for service state changes
func (o *Orchestrator) setupServiceStateMonitoring() {
	// Create a custom reporter that intercepts updates for restart logic
	interceptor := &serviceStateInterceptor{
		orchestrator:     o,
		originalReporter: o.reporter,
	}
	o.serviceMgr.SetReporter(interceptor)
}

// createK8sConnectionServices creates K8s connection services for MC and WC
func (o *Orchestrator) createK8sConnectionServices() []managers.ManagedServiceConfig {
	var services []managers.ManagedServiceConfig

	// Create MC service if configured
	if o.mcName != "" {
		mcContext := o.kubeMgr.BuildMcContextName(o.mcName)
		mcConfig := managers.K8sConnectionConfig{
			Name:                fmt.Sprintf("k8s-mc-%s", o.mcName),
			ContextName:         mcContext,
			IsMC:                true,
			HealthCheckInterval: 15 * time.Second,
		}

		services = append(services, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeKube,
			Label:  mcConfig.Name,
			Config: mcConfig,
		})
	}

	// Create WC service if configured
	if o.wcName != "" && o.mcName != "" {
		wcContext := o.kubeMgr.BuildWcContextName(o.mcName, o.wcName)
		wcConfig := managers.K8sConnectionConfig{
			Name:                fmt.Sprintf("k8s-wc-%s", o.wcName),
			ContextName:         wcContext,
			IsMC:                false,
			HealthCheckInterval: 15 * time.Second,
		}

		services = append(services, managers.ManagedServiceConfig{
			Type:   reporting.ServiceTypeKube,
			Label:  wcConfig.Name,
			Config: wcConfig,
		})
	}

	return services
}

// monitorAndStartServices monitors for services that need to be restarted after failures
func (o *Orchestrator) monitorAndStartServices(ctx context.Context) {
	// This goroutine is now primarily for handling services that fail and need to be restarted
	// Initial startup is handled by Start() method directly

	// Check every 5 seconds for services that might need restarting
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if any services that should be running are not active
			var servicesToRestart []managers.ManagedServiceConfig

			o.mu.RLock()
			for label, cfg := range o.serviceConfigs {
				// Skip manually stopped services
				if reason, exists := o.stopReasons[label]; exists && reason == StopReasonManual {
					continue
				}

				// Skip services stopped due to dependency failure
				// These should only restart when their dependencies are restored
				if reason, exists := o.stopReasons[label]; exists && reason == StopReasonDependency {
					continue
				}

				// Skip already active services
				if o.serviceMgr.IsServiceActive(label) {
					continue
				}

				// Skip if pending restart (will be handled by state update handler)
				if o.pendingRestarts[label] {
					continue
				}

				// This service should be running but isn't - add to restart list
				servicesToRestart = append(servicesToRestart, cfg)
			}
			o.mu.RUnlock()

			// If we found services that need restarting, attempt to restart them
			if len(servicesToRestart) > 0 {
				logging.Info("Orchestrator", "Found %d services that need restarting", len(servicesToRestart))

				// Start services in dependency order
				if err := o.startServicesInDependencyOrder(servicesToRestart); err != nil {
					logging.Error("Orchestrator", err, "Failed to restart services")
				}
			}
		}
	}
}

// waitForServicesToBeRunning waits for services to reach running state
func (o *Orchestrator) waitForServicesToBeRunning(configs []managers.ManagedServiceConfig, timeout time.Duration) {
	// For now, use a simple time-based wait
	// In a more sophisticated implementation, we could monitor service states
	time.Sleep(500 * time.Millisecond)
}
