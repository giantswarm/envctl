package aggregator

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

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
	activeTools     map[string]bool // exposed tool name -> active
	activePrompts   map[string]bool // exposed prompt name -> active
	activeResources map[string]bool // resource URI -> active
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
		activeTools:     make(map[string]bool),
		activePrompts:   make(map[string]bool),
		activeResources: make(map[string]bool),
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
	a.mu.Unlock()

	return nil
}

// RegisterServer registers a new backend MCP server
func (a *AggregatorServer) RegisterServer(ctx context.Context, name string, client MCPClient) error {
	return a.registry.Register(ctx, name, client)
}

// DeregisterServer removes a backend MCP server
func (a *AggregatorServer) DeregisterServer(name string) error {
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
	if a.server == nil {
		return
	}

	// Clear existing handlers
	a.clearHandlers()

	// Get all servers
	servers := a.registry.GetAllServers()

	// Rebuild active handler sets
	a.mu.Lock()
	a.activeTools = make(map[string]bool)
	a.activePrompts = make(map[string]bool)
	a.activeResources = make(map[string]bool)
	a.mu.Unlock()

	// Add tool handlers for each backend server
	for serverName, info := range servers {
		if !info.IsConnected() {
			continue
		}

		// Add handlers for each tool with smart prefixing
		info.mu.RLock()
		for _, tool := range info.Tools {
			exposedTool := tool
			exposedTool.Name = a.registry.nameTracker.GetExposedToolName(serverName, tool.Name)

			// Mark this tool as active
			a.mu.Lock()
			a.activeTools[exposedTool.Name] = true
			a.mu.Unlock()

			// Capture the exposed name in the closure
			exposedName := exposedTool.Name

			a.server.AddTool(exposedTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// Check if this tool is still active
				a.mu.RLock()
				isActive := a.activeTools[exposedName]
				a.mu.RUnlock()

				if !isActive {
					return nil, fmt.Errorf("tool %s is no longer available", exposedName)
				}

				// Resolve the exposed name back to server and original tool name
				sName, originalName, err := a.registry.ResolveToolName(exposedName)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve tool name: %w", err)
				}

				// Get the backend client
				client, err := a.registry.GetClient(sName)
				if err != nil {
					return nil, fmt.Errorf("server not available: %w", err)
				}

				// Forward the request with the original tool name
				args := make(map[string]interface{})
				if req.Params.Arguments != nil {
					// Type assert to map[string]interface{}
					if argsMap, ok := req.Params.Arguments.(map[string]interface{}); ok {
						args = argsMap
					}
				}

				result, err := client.CallTool(ctx, originalName, args)
				if err != nil {
					return nil, fmt.Errorf("tool execution failed: %w", err)
				}

				return result, nil
			})
		}
		info.mu.RUnlock()

		// Add resource handlers
		info.mu.RLock()
		for _, resource := range info.Resources {
			// Mark this resource as active
			a.mu.Lock()
			a.activeResources[resource.URI] = true
			a.mu.Unlock()

			// Capture resource URI in the closure
			resURI := resource.URI

			a.server.AddResource(resource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				// Check if this resource is still active
				a.mu.RLock()
				isActive := a.activeResources[resURI]
				a.mu.RUnlock()

				if !isActive {
					return nil, fmt.Errorf("resource %s is no longer available", resURI)
				}

				// Find which server can handle this resource
				for _, srvInfo := range a.registry.GetAllServers() {
					if !srvInfo.IsConnected() {
						continue
					}

					// Check if this server has the resource
					hasResource := false
					srvInfo.mu.RLock()
					for _, res := range srvInfo.Resources {
						if res.URI == resURI {
							hasResource = true
							break
						}
					}
					srvInfo.mu.RUnlock()

					if hasResource {
						result, err := srvInfo.Client.ReadResource(ctx, resURI)
						if err == nil {
							// Return the resource contents from the result
							var contents []mcp.ResourceContents
							if result != nil && len(result.Contents) > 0 {
								// The contents are already of the correct type
								contents = result.Contents
							}
							return contents, nil
						}
					}
				}

				return nil, fmt.Errorf("no server can handle resource: %s", resURI)
			})
		}
		info.mu.RUnlock()

		// Add prompt handlers with smart prefixing
		info.mu.RLock()
		for _, prompt := range info.Prompts {
			exposedPrompt := prompt
			exposedPrompt.Name = a.registry.nameTracker.GetExposedPromptName(serverName, prompt.Name)

			// Mark this prompt as active
			a.mu.Lock()
			a.activePrompts[exposedPrompt.Name] = true
			a.mu.Unlock()

			// Capture the exposed name in the closure
			exposedName := exposedPrompt.Name

			a.server.AddPrompt(exposedPrompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				// Check if this prompt is still active
				a.mu.RLock()
				isActive := a.activePrompts[exposedName]
				a.mu.RUnlock()

				if !isActive {
					return nil, fmt.Errorf("prompt %s is no longer available", exposedName)
				}

				// Resolve the exposed name back to server and original prompt name
				sName, originalName, err := a.registry.ResolvePromptName(exposedName)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve prompt name: %w", err)
				}

				// Get the backend client
				client, err := a.registry.GetClient(sName)
				if err != nil {
					return nil, fmt.Errorf("server not available: %w", err)
				}

				// Forward the request with the original prompt name
				args := make(map[string]interface{})
				if req.Params.Arguments != nil {
					// req.Params.Arguments is already map[string]string
					for k, v := range req.Params.Arguments {
						args[k] = v
					}
				}

				result, err := client.GetPrompt(ctx, originalName, args)
				if err != nil {
					return nil, fmt.Errorf("prompt retrieval failed: %w", err)
				}

				return result, nil
			})
		}
		info.mu.RUnlock()
	}

	// Count tools, resources, and prompts manually since there are no getter methods
	toolCount := 0
	resourceCount := 0
	promptCount := 0

	// Count from registry instead
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

// clearHandlers removes all existing handlers
func (a *AggregatorServer) clearHandlers() {
	// The mcp-go library doesn't provide a clear method, so we'll need to track and manage this
	// For now, we'll rely on the fact that adding a handler with the same name overwrites the previous one
}

// GetEndpoint returns the aggregator's SSE endpoint URL
func (a *AggregatorServer) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d/sse", a.config.Host, a.config.Port)
}

// GetTools returns all available tools with smart prefixing (only prefixed when conflicts exist)
func (a *AggregatorServer) GetTools() []mcp.Tool {
	return a.registry.GetAllTools()
}

// GetResources returns all available resources
func (a *AggregatorServer) GetResources() []mcp.Resource {
	return a.registry.GetAllResources()
}

// GetPrompts returns all available prompts with smart prefixing (only prefixed when conflicts exist)
func (a *AggregatorServer) GetPrompts() []mcp.Prompt {
	return a.registry.GetAllPrompts()
}
