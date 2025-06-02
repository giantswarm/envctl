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
		config:   config,
		registry: NewServerRegistry(),
	}
}

// Start starts the aggregator server
func (a *AggregatorServer) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server != nil {
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

	// Start health check routine
	a.wg.Add(1)
	go a.healthCheckRoutine()

	// Update initial capabilities
	a.updateCapabilities()

	// Start SSE server
	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	logging.Info("Aggregator", "Starting MCP aggregator server on %s", addr)

	go func() {
		if err := a.sseServer.Start(addr); err != nil && err != http.ErrServerClosed {
			logging.Error("Aggregator", err, "SSE server error")
		}
	}()

	return nil
}

// Stop stops the aggregator server
func (a *AggregatorServer) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server == nil {
		return fmt.Errorf("aggregator server not started")
	}

	logging.Info("Aggregator", "Stopping MCP aggregator server")

	// Cancel context to stop background routines
	if a.cancelFunc != nil {
		a.cancelFunc()
	}

	// Shutdown SSE server
	if a.sseServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := a.sseServer.Shutdown(shutdownCtx); err != nil {
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

	a.server = nil
	a.sseServer = nil
	a.httpServer = nil

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

	// Add tool handlers for each backend server
	for serverName, info := range servers {
		if !info.IsConnected() {
			continue
		}

		// Add handlers for each tool with prefixed names
		info.mu.RLock()
		for _, tool := range info.Tools {
			prefixedTool := tool
			prefixedTool.Name = fmt.Sprintf("%s.%s", serverName, tool.Name)

			// Capture serverName and original tool name in the closure
			sName := serverName
			originalName := tool.Name

			a.server.AddTool(prefixedTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			// Capture resource URI in the closure
			resURI := resource.URI

			a.server.AddResource(resource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
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

		// Add prompt handlers with prefixed names
		info.mu.RLock()
		for _, prompt := range info.Prompts {
			prefixedPrompt := prompt
			prefixedPrompt.Name = fmt.Sprintf("%s.%s", serverName, prompt.Name)

			// Capture serverName and original prompt name in the closure
			sName := serverName
			originalName := prompt.Name

			a.server.AddPrompt(prefixedPrompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
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

// healthCheckRoutine periodically checks the health of registered servers
func (a *AggregatorServer) healthCheckRoutine() {
	defer a.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
			if err := a.registry.RefreshAll(ctx); err != nil {
				logging.Warn("Aggregator", "Health check failed: %v", err)
			}
			cancel()
		}
	}
}

// GetEndpoint returns the aggregator's SSE endpoint URL
func (a *AggregatorServer) GetEndpoint() string {
	return fmt.Sprintf("http://%s:%d/sse", a.config.Host, a.config.Port)
}

// GetTools returns all available tools with prefixed names
func (a *AggregatorServer) GetTools() []mcp.Tool {
	return a.registry.GetAllTools()
}

// GetResources returns all available resources
func (a *AggregatorServer) GetResources() []mcp.Resource {
	return a.registry.GetAllResources()
}

// GetPrompts returns all available prompts with prefixed names
func (a *AggregatorServer) GetPrompts() []mcp.Prompt {
	return a.registry.GetAllPrompts()
}
