package aggregator

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
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
	mcpAPI          api.MCPServiceAPI
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
	mcpAPI api.MCPServiceAPI,
	serviceRegistry api.ServiceRegistryHandler,
) *AggregatorManager {
	manager := &AggregatorManager{
		config:          config,
		orchestratorAPI: orchestratorAPI,
		mcpAPI:          mcpAPI,
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
	if am.orchestratorAPI == nil || am.mcpAPI == nil {
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

		// Debug logging for tool counts
		logging.Debug("Aggregator-Manager", "GetServiceData: %d tools (%d blocked), %d resources, %d prompts",
			len(tools), blockedCount, len(resources), len(prompts))

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

	logging.Debug("Aggregator-Manager", "Initial sync: found %d MCP services", len(mcpServices))

	registeredCount := 0
	for _, service := range mcpServices {
		// Only register servers that are both running AND healthy
		if service.GetState() != api.StateRunning || service.GetHealth() != api.HealthHealthy {
			logging.Debug("Aggregator-Manager", "Skipping MCP service %s (state=%s, health=%s)",
				service.GetLabel(), service.GetState(), service.GetHealth())
			continue
		}

		// Register the healthy server
		if err := am.registerSingleServer(ctx, service.GetLabel()); err != nil {
			logging.Warn("Aggregator-Manager", "Failed to register healthy MCP server %s: %v",
				service.GetLabel(), err)
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
	// Get the service from registry
	service, exists := am.serviceRegistry.Get(serverName)
	if !exists {
		return fmt.Errorf("service %s not found", serverName)
	}

	// Get MCP-specific handler for this service
	mcpHandler, ok := api.GetMCPService(serverName)
	if !ok {
		return fmt.Errorf("no MCP handler registered for %s", serverName)
	}

	// Get the MCP client from the service
	// The service should implement a method to get the client
	type mcpClientProvider interface {
		GetMCPClient() interface{}
	}

	// Try to get the client through service data
	serviceData := service.GetServiceData()
	if serviceData == nil {
		return fmt.Errorf("no service data available for %s", serverName)
	}

	// Get the tool prefix from handler
	toolPrefix := ""
	tools := mcpHandler.GetTools()
	if len(tools) > 0 {
		// Extract tool prefix from service data if available
		if prefix, ok := serviceData["toolPrefix"].(string); ok {
			toolPrefix = prefix
		}
	}

	// For the actual MCP client, we need to get it differently
	// The MCP handler should provide the client
	// For now, we'll create a wrapper that uses the handler
	clientWrapper := &mcpClientWrapper{
		handler: mcpHandler,
		label:   serverName,
	}

	// Register with the aggregator
	if err := am.aggregatorServer.RegisterServer(ctx, serverName, clientWrapper, toolPrefix); err != nil {
		return fmt.Errorf("failed to register server: %w", err)
	}

	logging.Info("Aggregator-Manager", "Successfully registered MCP server %s with prefix %s", serverName, toolPrefix)
	return nil
}

// mcpClientWrapper wraps an MCP handler to implement MCPClient interface
type mcpClientWrapper struct {
	handler api.MCPServiceHandler
	label   string
}

func (w *mcpClientWrapper) Initialize(ctx context.Context) error {
	// Already initialized through the service
	return nil
}

func (w *mcpClientWrapper) Close() error {
	// Service manages its own lifecycle
	return nil
}

func (w *mcpClientWrapper) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	tools := w.handler.GetTools()
	// Convert api.MCPTool to mcp.Tool
	result := make([]mcp.Tool, len(tools))
	for i, tool := range tools {
		result[i] = mcp.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		}
	}
	return result, nil
}

func (w *mcpClientWrapper) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// This would need to be implemented by extending the handler interface
	return nil, fmt.Errorf("CallTool not implemented")
}

func (w *mcpClientWrapper) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	resources := w.handler.GetResources()
	// Convert api.MCPResource to mcp.Resource
	result := make([]mcp.Resource, len(resources))
	for i, res := range resources {
		result[i] = mcp.Resource{
			URI:         res.URI,
			Name:        res.Name,
			Description: res.Description,
			MIMEType:    res.MimeType,
		}
	}
	return result, nil
}

func (w *mcpClientWrapper) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	// This would need to be implemented by extending the handler interface
	return nil, fmt.Errorf("ReadResource not implemented")
}

func (w *mcpClientWrapper) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	// This would need to be implemented by extending the handler interface
	return nil, fmt.Errorf("ListPrompts not implemented")
}

func (w *mcpClientWrapper) GetPrompt(ctx context.Context, name string, args map[string]interface{}) (*mcp.GetPromptResult, error) {
	// This would need to be implemented by extending the handler interface
	return nil, fmt.Errorf("GetPrompt not implemented")
}

func (w *mcpClientWrapper) Ping(ctx context.Context) error {
	// The service is running if we have a handler
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
