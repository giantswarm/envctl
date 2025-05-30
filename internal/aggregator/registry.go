package aggregator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
)

// ServerRegistry manages registered MCP servers
type ServerRegistry struct {
	servers map[string]*ServerInfo
	mu      sync.RWMutex

	// Channel for notifying about changes
	updateChan chan struct{}
}

// NewServerRegistry creates a new server registry
func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{
		servers:    make(map[string]*ServerInfo),
		updateChan: make(chan struct{}, 1),
	}
}

// Register adds a new MCP server to the registry
func (r *ServerRegistry) Register(ctx context.Context, name string, client MCPClient) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[name]; exists {
		return fmt.Errorf("server %s already registered", name)
	}

	// Initialize the client
	if err := client.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize client for %s: %w", name, err)
	}

	// Create server info
	info := &ServerInfo{
		Name:      name,
		Client:    client,
		Connected: true,
		LastPing:  time.Now().Unix(),
	}

	// Get initial capabilities
	if err := r.refreshServerCapabilities(ctx, info); err != nil {
		logging.Warn("Aggregator", "Failed to get initial capabilities for %s: %v", name, err)
	}

	r.servers[name] = info
	r.notifyUpdate()

	logging.Info("Aggregator", "Registered MCP server: %s", name)
	return nil
}

// Deregister removes an MCP server from the registry
func (r *ServerRegistry) Deregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.servers[name]
	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	// Close the client connection
	if err := info.Client.Close(); err != nil {
		logging.Warn("Aggregator", "Error closing client for %s: %v", name, err)
	}

	delete(r.servers, name)
	r.notifyUpdate()

	logging.Info("Aggregator", "Deregistered MCP server: %s", name)
	return nil
}

// GetClient returns the client for a specific server
func (r *ServerRegistry) GetClient(name string) (MCPClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.servers[name]
	if !exists {
		return nil, fmt.Errorf("server %s not found", name)
	}

	if !info.IsConnected() {
		return nil, fmt.Errorf("server %s is not connected", name)
	}

	return info.Client, nil
}

// GetAllTools returns all tools from all registered servers with prefixed names
func (r *ServerRegistry) GetAllTools() []mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allTools []mcp.Tool

	for serverName, info := range r.servers {
		if !info.IsConnected() {
			continue
		}

		info.mu.RLock()
		for _, tool := range info.Tools {
			// Prefix tool name with server name
			prefixedTool := tool
			prefixedTool.Name = fmt.Sprintf("%s.%s", serverName, tool.Name)
			allTools = append(allTools, prefixedTool)
		}
		info.mu.RUnlock()
	}

	return allTools
}

// GetAllResources returns all resources from all registered servers
func (r *ServerRegistry) GetAllResources() []mcp.Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allResources []mcp.Resource

	for _, info := range r.servers {
		if !info.IsConnected() {
			continue
		}

		info.mu.RLock()
		allResources = append(allResources, info.Resources...)
		info.mu.RUnlock()
	}

	return allResources
}

// GetAllPrompts returns all prompts from all registered servers with prefixed names
func (r *ServerRegistry) GetAllPrompts() []mcp.Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allPrompts []mcp.Prompt

	for serverName, info := range r.servers {
		if !info.IsConnected() {
			continue
		}

		info.mu.RLock()
		for _, prompt := range info.Prompts {
			// Prefix prompt name with server name
			prefixedPrompt := prompt
			prefixedPrompt.Name = fmt.Sprintf("%s.%s", serverName, prompt.Name)
			allPrompts = append(allPrompts, prefixedPrompt)
		}
		info.mu.RUnlock()
	}

	return allPrompts
}

// SplitPrefixedName splits a prefixed name into server and original name
func (r *ServerRegistry) SplitPrefixedName(prefixedName string) (serverName, originalName string, err error) {
	parts := strings.SplitN(prefixedName, ".", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid prefixed name format: %s", prefixedName)
	}
	return parts[0], parts[1], nil
}

// RefreshAll refreshes capabilities for all connected servers
func (r *ServerRegistry) RefreshAll(ctx context.Context) error {
	r.mu.RLock()
	servers := make([]*ServerInfo, 0, len(r.servers))
	for _, info := range r.servers {
		servers = append(servers, info)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, info := range servers {
		if err := r.refreshServerCapabilities(ctx, info); err != nil {
			logging.Error("Aggregator", err, "Failed to refresh capabilities for %s", info.Name)
			lastErr = err
			info.SetConnected(false)
		} else {
			info.SetConnected(true)
			info.mu.Lock()
			info.LastPing = time.Now().Unix()
			info.mu.Unlock()
		}
	}

	if lastErr == nil {
		r.notifyUpdate()
	}

	return lastErr
}

// refreshServerCapabilities updates the tools, resources, and prompts for a server
func (r *ServerRegistry) refreshServerCapabilities(ctx context.Context, info *ServerInfo) error {
	// Get tools
	tools, err := info.Client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	info.UpdateTools(tools)

	// Get resources
	resources, err := info.Client.ListResources(ctx)
	if err != nil {
		// Resources might not be supported
		logging.Debug("Aggregator", "Failed to list resources for %s: %v", info.Name, err)
		info.UpdateResources([]mcp.Resource{})
	} else {
		info.UpdateResources(resources)
	}

	// Get prompts
	prompts, err := info.Client.ListPrompts(ctx)
	if err != nil {
		// Prompts might not be supported
		logging.Debug("Aggregator", "Failed to list prompts for %s: %v", info.Name, err)
		info.UpdatePrompts([]mcp.Prompt{})
	} else {
		info.UpdatePrompts(prompts)
	}

	return nil
}

// notifyUpdate signals that the registry has been updated
func (r *ServerRegistry) notifyUpdate() {
	select {
	case r.updateChan <- struct{}{}:
	default:
		// Channel already has a notification
	}
}

// GetUpdateChannel returns a channel that receives notifications on registry updates
func (r *ServerRegistry) GetUpdateChannel() <-chan struct{} {
	return r.updateChan
}

// GetServerInfo returns information about a specific server
func (r *ServerRegistry) GetServerInfo(name string) (*ServerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.servers[name]
	return info, exists
}

// GetAllServers returns information about all registered servers
func (r *ServerRegistry) GetAllServers() map[string]*ServerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid external modifications
	result := make(map[string]*ServerInfo, len(r.servers))
	for k, v := range r.servers {
		result[k] = v
	}
	return result
}
