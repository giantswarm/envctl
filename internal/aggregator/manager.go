package aggregator

import (
	"context"
	"envctl/pkg/logging"
	"fmt"
	"sync"
)

// This file contains aggregator manager logic that coordinates between
// the aggregator server and event handling to provide automatic MCP
// server registration based on health status.

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
		return fmt.Errorf("failed to start aggregator server: %w", err)
	}

	// Check if providers are available
	if am.orchestratorProvider == nil || am.mcpServiceProvider == nil {
		am.aggregatorServer.Stop(am.ctx)
		return fmt.Errorf("required providers not available")
	}

	// Initial sync: Register all healthy running MCP servers
	if err := am.registerHealthyMCPServers(am.ctx); err != nil {
		logging.Warn("Aggregator-Manager", "Error during initial MCP server registration: %v", err)
		// Continue anyway - the event handler will handle future registrations
	}

	// Create event handler adapter
	adapter := &eventProviderAdapter{
		orchestratorProvider: am.orchestratorProvider,
	}

	// Create event handler with simple register/deregister callbacks
	am.eventHandler = NewEventHandler(
		adapter,
		am.registerSingleServer,
		am.deregisterSingleServer,
	)

	// Start the event handler for automatic updates
	if err := am.eventHandler.Start(am.ctx); err != nil {
		// Stop the aggregator server if event handler fails
		am.aggregatorServer.Stop(am.ctx)
		return fmt.Errorf("failed to start event handler: %w", err)
	}

	logging.Info("Aggregator-Manager", "Started aggregator manager on %s", am.aggregatorServer.GetEndpoint())
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

	logging.Info("Aggregator-Manager", "Stopped aggregator manager")
	return nil
}

// GetServiceData returns service data for monitoring
func (am *AggregatorManager) GetServiceData() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	data := map[string]interface{}{
		"port": am.config.Port,
		"host": am.config.Host,
		"yolo": am.config.Yolo,
	}

	// Add aggregator server data
	if am.aggregatorServer != nil {
		data["endpoint"] = am.aggregatorServer.GetEndpoint()

		// Get tool/resource/prompt counts
		tools := am.aggregatorServer.GetTools()
		resources := am.aggregatorServer.GetResources()
		prompts := am.aggregatorServer.GetPrompts()

		data["tools"] = len(tools)
		data["resources"] = len(resources)
		data["prompts"] = len(prompts)

		// Get tools with status
		toolsWithStatus := am.aggregatorServer.GetToolsWithStatus()
		data["tools_with_status"] = toolsWithStatus

		// Count blocked tools
		blockedCount := 0
		for _, t := range toolsWithStatus {
			if t.Blocked {
				blockedCount++
			}
		}
		data["blocked_tools"] = blockedCount

		// Debug logging for tool counts
		logging.Debug("Aggregator-Manager", "GetServiceData: %d tools (%d blocked), %d resources, %d prompts",
			len(tools), blockedCount, len(resources), len(prompts))

		// Get total number of MCP servers from the provider
		totalServers := 0
		connectedServers := 0

		if am.mcpServiceProvider != nil {
			// Get all MCP services (running and stopped)
			allMCPServices := am.mcpServiceProvider.GetAllMCPServices()
			totalServers = len(allMCPServices)

			// Count how many are actually healthy and running
			for _, service := range allMCPServices {
				if service.State == "Running" && service.Health == "Healthy" {
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

// registerHealthyMCPServers registers all healthy running MCP servers during initial sync
func (am *AggregatorManager) registerHealthyMCPServers(ctx context.Context) error {
	// Get all MCP services
	mcpServices := am.mcpServiceProvider.GetAllMCPServices()

	logging.Debug("Aggregator-Manager", "Initial sync: found %d MCP services", len(mcpServices))

	registeredCount := 0
	for _, service := range mcpServices {
		// Only register servers that are both running AND healthy
		if service.State != "Running" || service.Health != "Healthy" {
			logging.Debug("Aggregator-Manager", "Skipping MCP service %s (state=%s, health=%s)",
				service.Name, service.State, service.Health)
			continue
		}

		// Register the healthy server
		if err := am.registerSingleServer(ctx, service.Name); err != nil {
			logging.Warn("Aggregator-Manager", "Failed to register healthy MCP server %s: %v",
				service.Name, err)
			// Continue with other servers
		} else {
			registeredCount++
		}
	}

	if registeredCount == 0 {
		logging.Info("Aggregator-Manager", "No healthy MCP servers found during initial sync")
	} else {
		logging.Info("Aggregator-Manager", "Initial sync completed: registered %d healthy MCP servers", registeredCount)
	}

	return nil
}

// registerSingleServer registers a single MCP server with the aggregator
func (am *AggregatorManager) registerSingleServer(ctx context.Context, serverName string) error {
	// Get the MCP client for this specific server
	client := am.mcpServiceProvider.GetMCPClient(serverName)
	if client == nil {
		return fmt.Errorf("no MCP client available for %s", serverName)
	}

	// Type assert to our MCPClient interface
	mcpClient, ok := client.(MCPClient)
	if !ok {
		return fmt.Errorf("invalid MCP client type for %s (got %T)", serverName, client)
	}

	// Get the service info to find the tool prefix
	var toolPrefix string
	mcpServices := am.mcpServiceProvider.GetAllMCPServices()
	for _, service := range mcpServices {
		if service.Name == serverName {
			toolPrefix = service.ToolPrefix
			break
		}
	}

	// Register with the aggregator
	if err := am.aggregatorServer.RegisterServer(ctx, serverName, mcpClient, toolPrefix); err != nil {
		return fmt.Errorf("failed to register server: %w", err)
	}

	logging.Info("Aggregator-Manager", "Successfully registered MCP server %s with prefix %s", serverName, toolPrefix)
	return nil
}

// deregisterSingleServer deregisters a single MCP server from the aggregator
func (am *AggregatorManager) deregisterSingleServer(serverName string) error {
	// Deregister from the aggregator
	if err := am.aggregatorServer.DeregisterServer(serverName); err != nil {
		return fmt.Errorf("failed to deregister server: %w", err)
	}

	logging.Info("Aggregator-Manager", "Successfully deregistered MCP server %s", serverName)
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

// GetEventHandler returns the event handler (mainly for testing)
func (am *AggregatorManager) GetEventHandler() *EventHandler {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.eventHandler
}

// ManualRefresh manually triggers a re-sync of all healthy MCP servers
// This can be useful for debugging or forced updates
func (am *AggregatorManager) ManualRefresh(ctx context.Context) error {
	return am.registerHealthyMCPServers(ctx)
}

// eventProviderAdapter adapts the OrchestratorEventProvider to StateEventProvider
type eventProviderAdapter struct {
	orchestratorProvider OrchestratorEventProvider
}

// SubscribeToStateChanges adapts the orchestrator event channel
func (a *eventProviderAdapter) SubscribeToStateChanges() <-chan ServiceStateEvent {
	orchestratorChan := a.orchestratorProvider.SubscribeToStateChanges()
	adaptedChan := make(chan ServiceStateEvent)

	go func() {
		for event := range orchestratorChan {
			// Convert ServiceStateChangedEvent to ServiceStateEvent
			adaptedChan <- ServiceStateEvent{
				Label:       event.Label,
				ServiceType: event.ServiceType,
				OldState:    event.OldState,
				NewState:    event.NewState,
				Health:      event.Health,
				Error:       event.Error,
			}
		}
		close(adaptedChan)
	}()

	return adaptedChan
}
