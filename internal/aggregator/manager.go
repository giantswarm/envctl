package aggregator

import (
	"context"
	"envctl/internal/api"
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

	// External dependencies - now using APIs directly
	orchestratorAPI api.OrchestratorAPI
	serviceRegistry api.ServiceRegistryHandler

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
	orchestratorAPI api.OrchestratorAPI,
	serviceRegistry api.ServiceRegistryHandler,
) *AggregatorManager {
	manager := &AggregatorManager{
		config:          config,
		orchestratorAPI: orchestratorAPI,
		serviceRegistry: serviceRegistry,
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

	// Check if APIs are available
	if am.orchestratorAPI == nil {
		am.aggregatorServer.Stop(am.ctx)
		return fmt.Errorf("required APIs not available")
	}

	// Initial sync: Register all healthy running MCP servers
	if err := am.registerHealthyMCPServers(am.ctx); err != nil {
		logging.Warn("Aggregator-Manager", "Error during initial MCP server registration: %v", err)
		// Continue anyway - the event handler will handle future registrations
	}

	// Create event handler with simple register/deregister callbacks
	am.eventHandler = NewEventHandler(
		am.orchestratorAPI,
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

		// Get total number of MCP servers from the API
		totalServers := 0
		connectedServers := 0

		if am.serviceRegistry != nil {
			// Get all MCP services (running and stopped)
			allServices := am.serviceRegistry.GetByType(api.TypeMCPServer)
			totalServers = len(allServices)

			// Count how many are actually healthy and running
			for _, service := range allServices {
				if service.GetState() == api.StateRunning && service.GetHealth() == api.HealthHealthy {
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
	if am.serviceRegistry == nil {
		return fmt.Errorf("service registry not available")
	}

	// Get all MCP services
	mcpServices := am.serviceRegistry.GetByType(api.TypeMCPServer)

	registeredCount := 0
	for _, service := range mcpServices {
		// Only register servers that are both running AND healthy
		// Use lowercase values to match the current API definitions
		if string(service.GetState()) != "running" || string(service.GetHealth()) != "healthy" {
			continue
		}

		// Register the healthy server
		if err := am.registerSingleServer(ctx, service.GetName()); err != nil {
			logging.Warn("Aggregator-Manager", "Failed to register healthy MCP server %s: %v",
				service.GetName(), err)
			// Continue with other servers
		} else {
			registeredCount++
		}
	}

	if registeredCount > 0 {
		logging.Info("Aggregator-Manager", "Initial sync completed: registered %d healthy MCP servers", registeredCount)
	}

	return nil
}

// registerSingleServer registers a single MCP server with the aggregator
func (am *AggregatorManager) registerSingleServer(ctx context.Context, serverName string) error {
	// Get the service from registry
	service, exists := am.serviceRegistry.Get(serverName)
	if !exists {
		return fmt.Errorf("service %s not found", serverName)
	}

	// Get service data to extract tool prefix
	serviceData := service.GetServiceData()
	if serviceData == nil {
		return fmt.Errorf("no service data available for %s", serverName)
	}

	// Get the tool prefix from service data
	toolPrefix := ""
	if prefix, ok := serviceData["toolPrefix"].(string); ok {
		toolPrefix = prefix
	}

	// Get the actual MCP client from the service
	var mcpClient MCPClient

	// Check if we can get the client directly from service data
	if clientInterface, ok := serviceData["client"]; ok && clientInterface != nil {
		if client, ok := clientInterface.(MCPClient); ok {
			mcpClient = client
		}
	}

	// If we didn't get the client from service data, try to get it through type assertion
	if mcpClient == nil {
		type mcpClientProvider interface {
			GetMCPClient() interface{}
		}

		if provider, ok := service.(mcpClientProvider); ok {
			if clientInterface := provider.GetMCPClient(); clientInterface != nil {
				if client, ok := clientInterface.(MCPClient); ok {
					mcpClient = client
				}
			}
		}
	}

	// If we still don't have a client, we can't proceed
	if mcpClient == nil {
		return fmt.Errorf("no MCP client available for %s", serverName)
	}

	// Register with the aggregator using the actual client
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
