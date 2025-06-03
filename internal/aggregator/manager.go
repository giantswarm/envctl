package aggregator

import (
	"context"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health of a component
type HealthStatus int

const (
	HealthUnknown HealthStatus = iota
	HealthHealthy
	HealthUnhealthy
)

// String returns the string representation of health status
func (h HealthStatus) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// This file contains aggregator manager logic.
// Currently, the manager approach is disabled to avoid complexity.
// This can be implemented in a future iteration when event handling is needed.

// TODO: Implement aggregator manager that combines server and event handling
// The manager should coordinate between the aggregator server and event handling
// to provide a unified interface for the aggregator functionality.

// AggregatorManager combines the aggregator server with event handling
// to provide automatic MCP server registration updates when services change state
type AggregatorManager struct {
	mu     sync.RWMutex
	config AggregatorConfig

	// External dependencies
	orchestratorProvider OrchestratorEventProvider
	mcpServiceProvider   MCPServiceProvider

	// Components
	aggregatorServer *AggregatorServer
	eventHandler     *EventHandler

	// Lifecycle
	ctx        context.Context
	cancelFunc context.CancelFunc
	health     HealthStatus
}

// NewAggregatorManager creates a new aggregator manager with event handling
func NewAggregatorManager(
	config AggregatorConfig,
	orchestratorProvider OrchestratorEventProvider,
	mcpServiceProvider MCPServiceProvider,
) *AggregatorManager {
	manager := &AggregatorManager{
		config:               config,
		orchestratorProvider: orchestratorProvider,
		mcpServiceProvider:   mcpServiceProvider,
		health:               HealthUnknown,
	}

	// Create the aggregator server
	manager.aggregatorServer = NewAggregatorServer(config)

	return manager
}

// Start starts the aggregator manager
func (am *AggregatorManager) Start(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Create cancellable context
	am.ctx, am.cancelFunc = context.WithCancel(ctx)

	// Start the aggregator server first
	if err := am.aggregatorServer.Start(am.ctx); err != nil {
		am.health = HealthUnhealthy
		return fmt.Errorf("failed to start aggregator server: %w", err)
	}

	// Check if providers are available
	if am.orchestratorProvider == nil || am.mcpServiceProvider == nil {
		am.aggregatorServer.Stop(am.ctx)
		am.health = HealthUnhealthy
		return fmt.Errorf("required providers not available")
	}

	// Register all MCP servers with the aggregator
	if err := am.registerMCPServersFromProvider(am.ctx); err != nil {
		am.aggregatorServer.Stop(am.ctx)
		am.health = HealthUnhealthy
		return fmt.Errorf("failed to register MCP servers: %w", err)
	}

	// Create event handler adapter
	adapter := &eventProviderAdapter{
		orchestratorProvider: am.orchestratorProvider,
	}
	am.eventHandler = NewEventHandler(adapter, am.refreshMCPServers)

	// Start the event handler for automatic updates
	if err := am.eventHandler.Start(am.ctx); err != nil {
		// Stop the aggregator server if event handler fails
		am.aggregatorServer.Stop(am.ctx)
		am.health = HealthUnhealthy
		return fmt.Errorf("failed to start event handler: %w", err)
	}

	am.health = HealthHealthy

	logging.Info("Aggregator-Manager", "Started aggregator manager with automatic MCP server updates on %s", am.aggregatorServer.GetEndpoint())
	return nil
}

// Stop stops the aggregator manager
func (am *AggregatorManager) Stop(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Cancel context to signal shutdown
	if am.cancelFunc != nil {
		am.cancelFunc()
	}

	// Stop event handler first
	if am.eventHandler != nil {
		if err := am.eventHandler.Stop(); err != nil {
			logging.Error("Aggregator-Manager", err, "Error stopping event handler")
		}
	}

	// Stop aggregator server
	if am.aggregatorServer != nil {
		if err := am.aggregatorServer.Stop(ctx); err != nil {
			logging.Error("Aggregator-Manager", err, "Error stopping aggregator server")
		}
	}

	am.health = HealthUnknown

	logging.Info("Aggregator-Manager", "Stopped aggregator manager")
	return nil
}

// Restart restarts the aggregator manager
func (am *AggregatorManager) Restart(ctx context.Context) error {
	if err := am.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop aggregator manager: %w", err)
	}

	// Small delay before restarting
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	return am.Start(ctx)
}

// CheckHealth checks the health of the aggregator manager
func (am *AggregatorManager) CheckHealth(ctx context.Context) (HealthStatus, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// Check aggregator server is running
	if am.aggregatorServer == nil {
		am.health = HealthUnhealthy
		return HealthUnhealthy, fmt.Errorf("aggregator server not initialized")
	}

	// Check event handler
	if am.eventHandler != nil && !am.eventHandler.IsRunning() {
		am.health = HealthUnhealthy
		return HealthUnhealthy, fmt.Errorf("event handler not running")
	}

	am.health = HealthHealthy
	return HealthHealthy, nil
}

// GetHealthCheckInterval returns the health check interval
func (am *AggregatorManager) GetHealthCheckInterval() time.Duration {
	return 30 * time.Second
}

// GetServiceData returns service data for monitoring
func (am *AggregatorManager) GetServiceData() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	data := map[string]interface{}{
		"port":   am.config.Port,
		"host":   am.config.Host,
		"health": am.health.String(),
	}

	// Add aggregator server data
	if am.aggregatorServer != nil {
		data["endpoint"] = am.aggregatorServer.GetEndpoint()

		// Get tool/resource/prompt counts with logging
		tools := am.aggregatorServer.GetTools()
		resources := am.aggregatorServer.GetResources()
		prompts := am.aggregatorServer.GetPrompts()

		data["tools"] = len(tools)
		data["resources"] = len(resources)
		data["prompts"] = len(prompts)

		// Debug logging for tool counts with timestamp
		logging.Debug("Aggregator-Manager", "GetServiceData called at %s: %d tools, %d resources, %d prompts from aggregator",
			time.Now().Format("15:04:05.000"), len(tools), len(resources), len(prompts))

		// Get total number of MCP servers from the provider
		totalServers := 0
		connectedServers := 0

		if am.mcpServiceProvider != nil {
			// Get all MCP services (running and stopped)
			allMCPServices := am.mcpServiceProvider.GetAllMCPServices()
			totalServers = len(allMCPServices)

			// Count how many are actually running/connected
			for _, service := range allMCPServices {
				if service.State == "Running" {
					connectedServers++
				}
			}
		}

		data["servers_total"] = totalServers
		data["servers_connected"] = connectedServers
	}

	// Add event handler status
	if am.eventHandler != nil {
		data["event_handler_running"] = am.eventHandler.IsRunning()
	}

	return data
}

// registerMCPServersFromProvider registers all running MCP servers using the provider
func (am *AggregatorManager) registerMCPServersFromProvider(ctx context.Context) error {
	// Get all MCP services
	mcpServices := am.mcpServiceProvider.GetAllMCPServices()

	logging.Debug("Aggregator-Manager", "Registering MCP servers: found %d MCP services", len(mcpServices))

	registeredCount := 0
	for _, service := range mcpServices {
		// Only register running services
		if service.State != "Running" {
			logging.Debug("Aggregator-Manager", "Skipping MCP service %s (state: %s)", service.Name, service.State)
			continue
		}

		// Get the MCP client from the service
		client := am.mcpServiceProvider.GetMCPClient(service.Name)
		if client == nil {
			logging.Warn("Aggregator-Manager", "Failed to get MCP client for %s", service.Name)
			continue
		}

		logging.Debug("Aggregator-Manager", "Got MCP client for %s (type: %T)", service.Name, client)

		// Type assert to our MCPClient interface
		mcpClient, ok := client.(MCPClient)
		if !ok {
			logging.Warn("Aggregator-Manager", "Invalid MCP client type for %s (got %T)", service.Name, client)
			continue
		}

		// Register with the aggregator
		if err := am.aggregatorServer.RegisterServer(ctx, service.Name, mcpClient); err != nil {
			logging.Warn("Aggregator-Manager", "Failed to register MCP server %s: %v", service.Name, err)
			// Continue with other servers
		} else {
			logging.Info("Aggregator-Manager", "Registered MCP server %s with aggregator", service.Name)
			registeredCount++
		}
	}

	if registeredCount == 0 {
		logging.Info("Aggregator-Manager", "No MCP servers are currently running to register")
	} else {
		logging.Info("Aggregator-Manager", "Registered %d MCP servers with aggregator", registeredCount)
	}

	return nil
}

// refreshMCPServers is called by the event handler to refresh the aggregator's MCP server registrations
// This function ensures that:
// 1. Stopped servers are deregistered
// 2. Running servers are registered (or re-registered if they were restarted)
// 3. The aggregator always has fresh MCP clients for all running servers
func (am *AggregatorManager) refreshMCPServers(ctx context.Context) error {
	logging.Info("Aggregator-Manager", "RefreshMCPServers called - updating registered servers")

	am.mu.RLock()
	server := am.aggregatorServer
	mcpProvider := am.mcpServiceProvider
	am.mu.RUnlock()

	if server == nil {
		return fmt.Errorf("aggregator server not running")
	}

	if mcpProvider == nil {
		return fmt.Errorf("MCP service provider not available")
	}

	// Get all current MCP services
	mcpServices := mcpProvider.GetAllMCPServices()
	registry := server.GetRegistry()

	// Debug log all MCP services and their states
	logging.Debug("Aggregator-Manager", "MCP services from provider:")
	for _, service := range mcpServices {
		logging.Debug("Aggregator-Manager", "  - %s: state=%s", service.Name, service.State)
	}

	// Build map of running services
	currentClients := make(map[string]MCPClient)
	for _, service := range mcpServices {
		if service.State == "Running" {
			// Retry logic to handle timing issues where service is Running but client not yet available
			var client interface{}
			var mcpClient MCPClient
			retryCount := 0
			maxRetries := 3

			for retryCount <= maxRetries {
				client = mcpProvider.GetMCPClient(service.Name)
				if client != nil {
					var ok bool
					if mcpClient, ok = client.(MCPClient); ok {
						currentClients[service.Name] = mcpClient
						logging.Debug("Aggregator-Manager", "Got client for running service %s on attempt %d", service.Name, retryCount+1)
						break
					} else {
						logging.Warn("Aggregator-Manager", "Failed to type assert MCP client for %s", service.Name)
						break // Don't retry type assertion failures
					}
				} else {
					if retryCount < maxRetries {
						logging.Debug("Aggregator-Manager", "GetMCPClient returned nil for %s, retrying... (attempt %d/%d)",
							service.Name, retryCount+1, maxRetries+1)
						// Small delay before retry
						select {
						case <-time.After(200 * time.Millisecond):
						case <-ctx.Done():
							return ctx.Err()
						}
					} else {
						logging.Warn("Aggregator-Manager", "GetMCPClient returned nil for %s after %d attempts", service.Name, maxRetries+1)
					}
				}
				retryCount++
			}
		}
	}

	logging.Info("Aggregator-Manager", "Current MCP clients: %d, Registered servers: %d",
		len(currentClients), len(registry.GetAllServers()))

	// First, only deregister servers that are no longer running
	// This minimizes the window where tools might appear as 0
	for name := range registry.GetAllServers() {
		if _, exists := currentClients[name]; !exists {
			// This server is no longer running, deregister it
			if err := server.DeregisterServer(name); err != nil {
				logging.Warn("Aggregator-Manager", "Failed to deregister MCP server %s: %v", name, err)
			} else {
				logging.Info("Aggregator-Manager", "Deregistered MCP server %s from aggregator", name)
			}
		}
	}

	// Then, register new servers or update existing ones
	// For existing servers that need updates, we'll try to update in-place first
	for label, client := range currentClients {
		serverInfo, exists := registry.GetServerInfo(label)

		if !exists {
			// New server, register it
			logging.Debug("Aggregator-Manager", "Registering new MCP server %s", label)

			// Small delay to ensure the MCP server's client is fully ready
			// This helps avoid race conditions where the service reports Running
			// but the client isn't yet available
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}

			retryCount := 0
			maxRetries := 2
			var lastErr error

			for retryCount <= maxRetries {
				if err := server.RegisterServer(ctx, label, client); err != nil {
					lastErr = err
					retryCount++
					if retryCount <= maxRetries {
						logging.Debug("Aggregator-Manager", "Failed to register MCP server %s (attempt %d/%d): %v",
							label, retryCount, maxRetries+1, err)
						// Small delay before retry
						select {
						case <-time.After(500 * time.Millisecond):
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				} else {
					// Success
					logging.Info("Aggregator-Manager", "Registered MCP server %s with aggregator", label)
					break
				}
			}

			if lastErr != nil {
				logging.Error("Aggregator-Manager", lastErr, "Failed to register MCP server %s after %d attempts",
					label, maxRetries+1)
			}
		} else if serverInfo != nil {
			// Server already exists, but we want to ensure it has a fresh client
			// For now, we still need to deregister and re-register to update the client
			// In the future, we could add an UpdateServer method to do this atomically
			logging.Debug("Aggregator-Manager", "Server %s already registered, will re-register with fresh client", label)

			// Deregister the old instance
			if err := server.DeregisterServer(label); err != nil {
				logging.Warn("Aggregator-Manager", "Failed to deregister existing MCP server %s: %v", label, err)
				// Continue anyway - try to register
			}

			// Register with fresh client
			retryCount := 0
			maxRetries := 2
			var lastErr error

			for retryCount <= maxRetries {
				if err := server.RegisterServer(ctx, label, client); err != nil {
					lastErr = err
					retryCount++
					if retryCount <= maxRetries {
						logging.Debug("Aggregator-Manager", "Failed to re-register MCP server %s (attempt %d/%d): %v",
							label, retryCount, maxRetries+1, err)
						// Small delay before retry
						select {
						case <-time.After(500 * time.Millisecond):
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				} else {
					// Success
					logging.Info("Aggregator-Manager", "Re-registered MCP server %s with aggregator", label)
					break
				}
			}

			if lastErr != nil {
				logging.Error("Aggregator-Manager", lastErr, "Failed to re-register MCP server %s after %d attempts",
					label, maxRetries+1)
			}
		}
	}

	return nil
}

// GetEndpoint returns the aggregator's SSE endpoint URL
func (am *AggregatorManager) GetEndpoint() string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if am.aggregatorServer != nil {
		return am.aggregatorServer.GetEndpoint()
	}

	return ""
}

// GetAggregatorServer returns the underlying aggregator server for advanced operations
func (am *AggregatorManager) GetAggregatorServer() *AggregatorServer {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.aggregatorServer
}

// GetEventHandler returns the event handler for testing or advanced operations
func (am *AggregatorManager) GetEventHandler() *EventHandler {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.eventHandler
}

// ManualRefresh manually triggers a refresh of MCP server registrations
// This can be useful for debugging or forced updates
func (am *AggregatorManager) ManualRefresh(ctx context.Context) error {
	return am.refreshMCPServers(ctx)
}

// eventProviderAdapter adapts OrchestratorEventProvider to StateEventProvider
type eventProviderAdapter struct {
	orchestratorProvider OrchestratorEventProvider
}

// SubscribeToStateChanges adapts the orchestrator events to the event handler format
func (a *eventProviderAdapter) SubscribeToStateChanges() <-chan ServiceStateEvent {
	if a.orchestratorProvider == nil {
		// Return a closed channel if no provider
		ch := make(chan ServiceStateEvent)
		close(ch)
		return ch
	}

	// Get events from orchestrator
	orchEvents := a.orchestratorProvider.SubscribeToStateChanges()

	// Create adapter channel
	adapterChan := make(chan ServiceStateEvent, 100)

	// Start conversion goroutine
	go func() {
		defer close(adapterChan)
		for orchEvent := range orchEvents {
			// Convert to event handler format
			event := ServiceStateEvent{
				Label:       orchEvent.Label,
				ServiceType: orchEvent.ServiceType,
				OldState:    orchEvent.OldState,
				NewState:    orchEvent.NewState,
				Health:      orchEvent.Health,
				Error:       orchEvent.Error,
			}

			select {
			case adapterChan <- event:
			default:
				// Drop event if channel is full
				logging.Warn("Aggregator-Manager", "Dropped state change event (channel full)")
			}
		}
	}()

	return adapterChan
}
