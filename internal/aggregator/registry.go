package aggregator

import (
	"context"
	"fmt"
	"sync"

	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
)

// NameTracker tracks tool and prompt name conflicts across servers
type NameTracker struct {
	// Map of tool name -> list of servers that have this tool
	toolToServers map[string][]string
	// Map of prompt name -> list of servers that have this prompt
	promptToServers map[string][]string
	// Map of exposed name -> (server, original name)
	nameMapping map[string]struct {
		serverName   string
		originalName string
		isPrompt     bool // false for tool, true for prompt
	}
	mu sync.RWMutex
}

// NewNameTracker creates a new name tracker
func NewNameTracker() *NameTracker {
	return &NameTracker{
		toolToServers:   make(map[string][]string),
		promptToServers: make(map[string][]string),
		nameMapping: make(map[string]struct {
			serverName   string
			originalName string
			isPrompt     bool
		}),
	}
}

// ServerRegistry manages registered MCP servers
type ServerRegistry struct {
	servers map[string]*ServerInfo
	mu      sync.RWMutex

	// Channel for notifying about changes
	updateChan chan struct{}

	// Name conflict tracking
	nameTracker *NameTracker
}

// NewServerRegistry creates a new server registry
func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{
		servers:     make(map[string]*ServerInfo),
		updateChan:  make(chan struct{}, 1),
		nameTracker: NewNameTracker(),
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
	}

	// Get initial capabilities
	if err := r.refreshServerCapabilities(ctx, info); err != nil {
		logging.Warn("Aggregator", "Failed to get initial capabilities for %s: %v", name, err)
		// Log more details about what we did get
		info.mu.RLock()
		logging.Debug("Aggregator", "Server %s registered with %d tools, %d resources, %d prompts", 
			name, len(info.Tools), len(info.Resources), len(info.Prompts))
		info.mu.RUnlock()
	} else {
		info.mu.RLock()
		logging.Info("Aggregator", "Server %s registered successfully with %d tools, %d resources, %d prompts", 
			name, len(info.Tools), len(info.Resources), len(info.Prompts))
		info.mu.RUnlock()
	}

	r.servers[name] = info
	r.notifyUpdate()

	// Rebuild name mappings with the new server
	r.nameTracker.RebuildMappings(r.servers)

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

	// Rebuild name mappings without the removed server
	r.nameTracker.RebuildMappings(r.servers)

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

// GetAllTools returns all tools from all registered servers with smart prefixing
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
			// Use smart prefixing - only prefix if there are conflicts
			exposedTool := tool
			exposedTool.Name = r.nameTracker.GetExposedToolName(serverName, tool.Name)
			allTools = append(allTools, exposedTool)
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

// GetAllPrompts returns all prompts from all registered servers with smart prefixing
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
			// Use smart prefixing - only prefix if there are conflicts
			exposedPrompt := prompt
			exposedPrompt.Name = r.nameTracker.GetExposedPromptName(serverName, prompt.Name)
			allPrompts = append(allPrompts, exposedPrompt)
		}
		info.mu.RUnlock()
	}

	return allPrompts
}

// ResolveToolName resolves an exposed tool name to server and original name
func (r *ServerRegistry) ResolveToolName(exposedName string) (serverName, originalName string, err error) {
	serverName, originalName, isPrompt, err := r.nameTracker.ResolveName(exposedName)
	if err != nil {
		return "", "", err
	}
	if isPrompt {
		return "", "", fmt.Errorf("name %s is a prompt, not a tool", exposedName)
	}
	return serverName, originalName, nil
}

// ResolvePromptName resolves an exposed prompt name to server and original name
func (r *ServerRegistry) ResolvePromptName(exposedName string) (serverName, originalName string, err error) {
	serverName, originalName, isPrompt, err := r.nameTracker.ResolveName(exposedName)
	if err != nil {
		return "", "", err
	}
	if !isPrompt {
		return "", "", fmt.Errorf("name %s is a tool, not a prompt", exposedName)
	}
	return serverName, originalName, nil
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

// RebuildMappings rebuilds the name mappings based on current server capabilities
func (nt *NameTracker) RebuildMappings(servers map[string]*ServerInfo) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	// Clear existing mappings
	nt.toolToServers = make(map[string][]string)
	nt.promptToServers = make(map[string][]string)
	nt.nameMapping = make(map[string]struct {
		serverName   string
		originalName string
		isPrompt     bool
	})

	// Build tool mappings
	for serverName, info := range servers {
		if !info.IsConnected() {
			continue
		}

		info.mu.RLock()
		for _, tool := range info.Tools {
			nt.toolToServers[tool.Name] = append(nt.toolToServers[tool.Name], serverName)
		}
		for _, prompt := range info.Prompts {
			nt.promptToServers[prompt.Name] = append(nt.promptToServers[prompt.Name], serverName)
		}
		info.mu.RUnlock()
	}

	// Build exposed name mappings
	// Tools
	for toolName, serverList := range nt.toolToServers {
		if len(serverList) == 1 {
			// No conflict - use original name
			nt.nameMapping[toolName] = struct {
				serverName   string
				originalName string
				isPrompt     bool
			}{
				serverName:   serverList[0],
				originalName: toolName,
				isPrompt:     false,
			}
		} else {
			// Conflict - prefix with server name
			for _, serverName := range serverList {
				prefixedName := fmt.Sprintf("%s.%s", serverName, toolName)
				nt.nameMapping[prefixedName] = struct {
					serverName   string
					originalName string
					isPrompt     bool
				}{
					serverName:   serverName,
					originalName: toolName,
					isPrompt:     false,
				}
			}
		}
	}

	// Prompts
	for promptName, serverList := range nt.promptToServers {
		if len(serverList) == 1 {
			// No conflict - use original name
			nt.nameMapping[promptName] = struct {
				serverName   string
				originalName string
				isPrompt     bool
			}{
				serverName:   serverList[0],
				originalName: promptName,
				isPrompt:     true,
			}
		} else {
			// Conflict - prefix with server name
			for _, serverName := range serverList {
				prefixedName := fmt.Sprintf("%s.%s", serverName, promptName)
				nt.nameMapping[prefixedName] = struct {
					serverName   string
					originalName string
					isPrompt     bool
				}{
					serverName:   serverName,
					originalName: promptName,
					isPrompt:     true,
				}
			}
		}
	}
}

// GetExposedToolName returns the name to expose for a tool (with or without prefix)
func (nt *NameTracker) GetExposedToolName(serverName, toolName string) string {
	nt.mu.RLock()
	defer nt.mu.RUnlock()

	servers := nt.toolToServers[toolName]
	if len(servers) <= 1 {
		return toolName
	}
	return fmt.Sprintf("%s.%s", serverName, toolName)
}

// GetExposedPromptName returns the name to expose for a prompt (with or without prefix)
func (nt *NameTracker) GetExposedPromptName(serverName, promptName string) string {
	nt.mu.RLock()
	defer nt.mu.RUnlock()

	servers := nt.promptToServers[promptName]
	if len(servers) <= 1 {
		return promptName
	}
	return fmt.Sprintf("%s.%s", serverName, promptName)
}

// ResolveName resolves an exposed name to server and original name
func (nt *NameTracker) ResolveName(exposedName string) (serverName, originalName string, isPrompt bool, err error) {
	nt.mu.RLock()
	defer nt.mu.RUnlock()

	mapping, exists := nt.nameMapping[exposedName]
	if !exists {
		return "", "", false, fmt.Errorf("unknown name: %s", exposedName)
	}

	return mapping.serverName, mapping.originalName, mapping.isPrompt, nil
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
