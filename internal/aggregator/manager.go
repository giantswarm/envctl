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
		data["tools"] = len(am.aggregatorServer.GetTools())
		data["resources"] = len(am.aggregatorServer.GetResources())
		data["prompts"] = len(am.aggregatorServer.GetPrompts())

		// Get connected servers count
		registry := am.aggregatorServer.GetRegistry()
		if registry != nil {
			servers := registry.GetAllServers()
			connected := 0
			for _, info := range servers {
				if info.IsConnected() {
					connected++
				}
			}
			data["servers_total"] = len(servers)
			data["servers_connected"] = connected
		}
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

	// Build map of running services
	currentClients := make(map[string]MCPClient)
	for _, service := range mcpServices {
		if service.State == "Running" {
			client := mcpProvider.GetMCPClient(service.Name)
			if client != nil {
				if mcpClient, ok := client.(MCPClient); ok {
					currentClients[service.Name] = mcpClient
				}
			}
		}
	}

	logging.Info("Aggregator-Manager", "Current MCP clients: %d, Registered servers: %d",
		len(currentClients), len(registry.GetAllServers()))

	// Register new clients
	for label, client := range currentClients {
		if _, exists := registry.GetServerInfo(label); !exists {
			if err := server.RegisterServer(ctx, label, client); err != nil {
				logging.Warn("Aggregator-Manager", "Failed to register new MCP server %s: %v", label, err)
			} else {
				logging.Info("Aggregator-Manager", "Registered new MCP server %s with aggregator", label)
			}
		}
	}

	// Deregister removed clients
	for name := range registry.GetAllServers() {
		if _, exists := currentClients[name]; !exists {
			if err := server.DeregisterServer(name); err != nil {
				logging.Warn("Aggregator-Manager", "Failed to deregister MCP server %s: %v", name, err)
			} else {
				logging.Info("Aggregator-Manager", "Deregistered MCP server %s from aggregator", name)
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
				Label:    orchEvent.Label,
				OldState: orchEvent.OldState,
				NewState: orchEvent.NewState,
				Health:   orchEvent.Health,
				Error:    orchEvent.Error,
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
