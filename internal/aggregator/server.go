package aggregator

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"envctl/internal/workflow"
	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// AggregatorServer implements an MCP server that aggregates multiple backend MCP servers
type AggregatorServer struct {
	config   AggregatorConfig
	registry *ServerRegistry
	server   *server.MCPServer

	// SSE server for HTTP transport
	sseServer *server.SSEServer

	// HTTP server for SSE endpoint
	httpServer *http.Server

	// Lifecycle management
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Handler tracking - tracks which handlers are currently active
	toolManager     *activeItemManager
	promptManager   *activeItemManager
	resourceManager *activeItemManager

	// Workflow manager
	workflowManager *workflow.WorkflowManager
}

// NewAggregatorServer creates a new aggregator server
func NewAggregatorServer(config AggregatorConfig) *AggregatorServer {
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == 0 {
		config.Port = 8080
	}

	return &AggregatorServer{
		config:          config,
		registry:        NewServerRegistry(),
		toolManager:     newActiveItemManager(itemTypeTool),
		promptManager:   newActiveItemManager(itemTypePrompt),
		resourceManager: newActiveItemManager(itemTypeResource),
	}
}

// Start starts the aggregator server
func (a *AggregatorServer) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.server != nil {
		a.mu.Unlock()
		return fmt.Errorf("aggregator server already started")
	}

	// Create cancellable context
	a.ctx, a.cancelFunc = context.WithCancel(ctx)

	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"envctl-aggregator",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true), // subscribe and listChanged
		server.WithPromptCapabilities(true),
	)

	a.server = mcpServer

	// Create SSE server
	baseURL := fmt.Sprintf("http://%s:%d", a.config.Host, a.config.Port)
	a.sseServer = server.NewSSEServer(
		a.server,
		server.WithBaseURL(baseURL),
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
		server.WithKeepAlive(true),
		server.WithKeepAliveInterval(30*time.Second),
	)

	// Initialize workflow manager if config directory is provided
	if a.config.ConfigDir != "" {
		wm, err := workflow.NewWorkflowManager(a.config.ConfigDir, a)
		if err != nil {
			logging.Warn("Aggregator", "Failed to initialize workflow manager: %v", err)
			// Continue without workflows - they're optional
		} else {
			a.workflowManager = wm
			logging.Info("Aggregator", "Initialized workflow manager")
		}
	}

	// Start registry update monitor
	a.wg.Add(1)
	go a.monitorRegistryUpdates()

	// Release the lock before calling updateCapabilities to avoid deadlock
	a.mu.Unlock()

	// Update initial capabilities
	a.updateCapabilities()

	// Start SSE server
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	logging.Info("Aggregator", "Starting MCP aggregator server on %s", addr)

	// Capture sseServer to avoid race condition
	sseServer := a.sseServer
	if sseServer != nil {
		go func() {
			if err := sseServer.Start(addr); err != nil && err != http.ErrServerClosed {
				logging.Error("Aggregator", err, "SSE server error")
			}
		}()
	}

	return nil
}

// Stop stops the aggregator server
func (a *AggregatorServer) Stop(ctx context.Context) error {
	a.mu.Lock()
	if a.server == nil {
		a.mu.Unlock()
		return fmt.Errorf("aggregator server not started")
	}

	logging.Info("Aggregator", "Stopping MCP aggregator server")

	// Cancel context to stop background routines
	cancelFunc := a.cancelFunc
	sseServer := a.sseServer
	a.mu.Unlock()

	if cancelFunc != nil {
		cancelFunc()
	}

	// Shutdown SSE server
	if sseServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := sseServer.Shutdown(shutdownCtx); err != nil {
			logging.Error("Aggregator", err, "Error shutting down SSE server")
		}
	}

	// Wait for background routines
	a.wg.Wait()

	// Stop workflow manager if it exists
	if a.workflowManager != nil {
		a.workflowManager.Stop()
	}

	// Deregister all servers
	for name := range a.registry.GetAllServers() {
		if err := a.registry.Deregister(name); err != nil {
			logging.Warn("Aggregator", "Error deregistering server %s: %v", name, err)
		}
	}

	a.mu.Lock()
	a.server = nil
	a.sseServer = nil
	a.httpServer = nil
	a.workflowManager = nil
	a.mu.Unlock()

	return nil
}

// RegisterServer registers a new backend MCP server
func (a *AggregatorServer) RegisterServer(ctx context.Context, name string, client MCPClient) error {
	logging.Debug("Aggregator", "RegisterServer called for %s at %s", name, time.Now().Format("15:04:05.000"))
	return a.registry.Register(ctx, name, client)
}

// DeregisterServer removes a backend MCP server
func (a *AggregatorServer) DeregisterServer(name string) error {
	logging.Debug("Aggregator", "DeregisterServer called for %s at %s", name, time.Now().Format("15:04:05.000"))
	return a.registry.Deregister(name)
}

// GetRegistry returns the server registry
func (a *AggregatorServer) GetRegistry() *ServerRegistry {
	return a.registry
}

// monitorRegistryUpdates monitors for changes in the registry and updates capabilities
func (a *AggregatorServer) monitorRegistryUpdates() {
	defer a.wg.Done()

	updateChan := a.registry.GetUpdateChannel()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-updateChan:
			// Update server capabilities based on registered servers
			a.updateCapabilities()
		}
	}
}

// updateCapabilities updates the aggregator's advertised capabilities
func (a *AggregatorServer) updateCapabilities() {
	a.mu.RLock()
	if a.server == nil {
		a.mu.RUnlock()
		return
	}
	a.mu.RUnlock()

	logging.Debug("Aggregator", "Updating capabilities dynamically")

	// Get all servers
	servers := a.registry.GetAllServers()

	// Collect all items from connected servers
	collected := collectItemsFromServers(servers, a.registry)

	// Remove obsolete items
	a.removeObsoleteItems(collected)

	// Add new items
	a.addNewItems(servers)

	// Log summary
	a.logCapabilitiesSummary(servers)
}

// removeObsoleteItems removes items that are no longer available
func (a *AggregatorServer) removeObsoleteItems(collected *collectResult) {
	// Remove obsolete tools
	removeObsoleteItems(
		a.toolManager,
		collected.newTools,
		func(items []string) {
			a.server.DeleteTools(items...)
		},
	)

	// Remove obsolete prompts
	removeObsoleteItems(
		a.promptManager,
		collected.newPrompts,
		func(items []string) {
			a.server.DeletePrompts(items...)
		},
	)

	// Remove obsolete resources
	removeObsoleteItems(
		a.resourceManager,
		collected.newResources,
		func(items []string) {
			// Note: The MCP server API doesn't provide a batch removal method for resources
			// (unlike DeleteTools and DeletePrompts), so we have to remove them one by one.
			// This will cause multiple notifications to the client.
			// TODO: Consider requesting a RemoveResources/DeleteResources method in the MCP library
			for _, uri := range items {
				a.server.RemoveResource(uri)
			}
		},
	)
}

// addNewItems adds new handlers for items that don't exist yet
func (a *AggregatorServer) addNewItems(servers map[string]*ServerInfo) {
	var toolsToAdd []server.ServerTool
	var promptsToAdd []server.ServerPrompt
	var resourcesToAdd []server.ServerResource

	// Process each server
	for serverName, info := range servers {
		if !info.IsConnected() {
			continue
		}

		// Process tools for this server
		toolsToAdd = append(toolsToAdd, processToolsForServer(a, serverName, info)...)

		// Process prompts for this server
		promptsToAdd = append(promptsToAdd, processPromptsForServer(a, serverName, info)...)

		// Process resources for this server
		resourcesToAdd = append(resourcesToAdd, processResourcesForServer(a, serverName, info)...)
	}

	// Add workflow tools if workflow manager exists
	if a.workflowManager != nil {
		workflowTools := a.workflowManager.GetWorkflows()
		for _, tool := range workflowTools {
			toolsToAdd = append(toolsToAdd, a.createWorkflowServerTool(tool))
		}

		// Add workflow management tools
		managementTools := workflow.NewManagementTools(a.workflowManager.GetStorage())
		for _, tool := range managementTools.GetManagementTools() {
			toolsToAdd = append(toolsToAdd, a.createManagementServerTool(tool, managementTools))
		}
	}

	// Add all items in batches
	if len(toolsToAdd) > 0 {
		logging.Debug("Aggregator", "Adding %d tools in batch", len(toolsToAdd))
		a.server.AddTools(toolsToAdd...)
	}

	if len(promptsToAdd) > 0 {
		logging.Debug("Aggregator", "Adding %d prompts in batch", len(promptsToAdd))
		a.server.AddPrompts(promptsToAdd...)
	}

	if len(resourcesToAdd) > 0 {
		logging.Debug("Aggregator", "Adding %d resources in batch", len(resourcesToAdd))
		a.server.AddResources(resourcesToAdd...)
	}
}

// logCapabilitiesSummary logs a summary of current capabilities
func (a *AggregatorServer) logCapabilitiesSummary(servers map[string]*ServerInfo) {
	toolCount := 0
	resourceCount := 0
	promptCount := 0

	for _, info := range servers {
		if info.IsConnected() {
			info.mu.RLock()
			toolCount += len(info.Tools)
			resourceCount += len(info.Resources)
			promptCount += len(info.Prompts)
			info.mu.RUnlock()
		}
	}

	logging.Debug("Aggregator", "Updated capabilities: %d tools, %d resources, %d prompts",
		toolCount, resourceCount, promptCount)
}

// GetEndpoint returns the aggregator's SSE endpoint URL
func (a *AggregatorServer) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d/sse", a.config.Host, a.config.Port)
}

// GetTools returns all available tools with smart prefixing (only prefixed when conflicts exist)
func (a *AggregatorServer) GetTools() []mcp.Tool {
	return a.registry.GetAllTools()
}

// GetToolsWithStatus returns all available tools with their blocked status
func (a *AggregatorServer) GetToolsWithStatus() []ToolWithStatus {
	a.mu.RLock()
	yolo := a.config.Yolo
	a.mu.RUnlock()

	tools := a.registry.GetAllTools()
	result := make([]ToolWithStatus, 0, len(tools))

	for _, tool := range tools {
		// Resolve the tool to get the original name
		var originalName string
		if serverName, origName, err := a.registry.ResolveToolName(tool.Name); err == nil {
			originalName = origName
			_ = serverName // unused
		} else {
			// If we can't resolve, use the exposed name
			originalName = tool.Name
		}

		result = append(result, ToolWithStatus{
			Tool:    tool,
			Blocked: !yolo && isDestructiveTool(originalName),
		})
	}

	return result
}

// GetResources returns all available resources
func (a *AggregatorServer) GetResources() []mcp.Resource {
	return a.registry.GetAllResources()
}

// GetPrompts returns all available prompts with smart prefixing (only prefixed when conflicts exist)
func (a *AggregatorServer) GetPrompts() []mcp.Prompt {
	return a.registry.GetAllPrompts()
}

// ToggleToolBlock toggles the blocked status of a specific tool
// This allows runtime changes to the denylist behavior for individual tools
func (a *AggregatorServer) ToggleToolBlock(toolName string) error {
	// For now, we can only toggle between fully enabled (yolo) or default denylist
	// In a future enhancement, we could maintain a runtime override list
	// For now, we just return an error indicating this needs more work
	return fmt.Errorf("individual tool blocking toggle not yet implemented")
}

// IsYoloMode returns whether yolo mode is enabled
func (a *AggregatorServer) IsYoloMode() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.Yolo
}

// CallToolInternal allows internal components to call tools directly
func (a *AggregatorServer) CallToolInternal(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// Resolve the tool name to find which server provides it
	serverName, originalName, err := a.registry.ResolveToolName(toolName)
	if err != nil {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Get the server info
	serverInfo, exists := a.registry.GetServerInfo(serverName)
	if !exists || serverInfo == nil {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	// Call the tool through the client using the original name
	return serverInfo.Client.CallTool(ctx, originalName, args)
}

// createWorkflowServerTool creates a server tool handler for a workflow
func (a *AggregatorServer) createWorkflowServerTool(tool mcp.Tool) server.ServerTool {
	return server.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract workflow name from tool name (remove "workflow_" prefix)
			workflowName := tool.Name[9:] // len("workflow_") = 9

			// Extract arguments
			args := make(map[string]interface{})
			if req.Params.Arguments != nil {
				if argsMap, ok := req.Params.Arguments.(map[string]interface{}); ok {
					args = argsMap
				}
			}

			// Execute workflow
			result, err := a.workflowManager.ExecuteWorkflow(ctx, workflowName, args)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Workflow execution failed: %v", err)), nil
			}

			return result, nil
		},
	}
}

// createManagementServerTool creates a server tool handler for workflow management
func (a *AggregatorServer) createManagementServerTool(tool mcp.Tool, mt *workflow.ManagementTools) server.ServerTool {
	return server.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Route to appropriate handler based on tool name
			var result *mcp.CallToolResult
			var err error

			switch tool.Name {
			case "workflow_list":
				result, err = mt.HandleListWorkflows(ctx, req)
			case "workflow_get":
				result, err = mt.HandleGetWorkflow(ctx, req)
			case "workflow_create":
				result, err = mt.HandleCreateWorkflow(ctx, req)
			case "workflow_update":
				result, err = mt.HandleUpdateWorkflow(ctx, req)
			case "workflow_delete":
				result, err = mt.HandleDeleteWorkflow(ctx, req)
			case "workflow_validate":
				result, err = mt.HandleValidateWorkflow(ctx, req)
			case "workflow_spec":
				result, err = mt.HandleGetWorkflowSpec(ctx, req)
			default:
				err = fmt.Errorf("unknown workflow management tool: %s", tool.Name)
			}

			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %v", err)), nil
			}

			return result, nil
		},
	}
}
