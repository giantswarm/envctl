package aggregator

import (
	"fmt"
	"sync"
)

// NameTracker tracks tool and prompt name conflicts across servers
type NameTracker struct {
	// Map of tool name -> list of servers that have this tool
	toolToServers map[string][]string
	// Map of prompt name -> list of servers that have this prompt
	promptToServers map[string][]string
	// Map of resource URI -> list of servers that have this resource
	resourceToServers map[string][]string
	// Map of exposed name -> (server, original name)
	nameMapping map[string]struct {
		serverName   string
		originalName string
		itemType     string // "tool", "prompt", or "resource"
	}
	mu sync.RWMutex
}

// NewNameTracker creates a new name tracker
func NewNameTracker() *NameTracker {
	return &NameTracker{
		toolToServers:     make(map[string][]string),
		promptToServers:   make(map[string][]string),
		resourceToServers: make(map[string][]string),
		nameMapping: make(map[string]struct {
			serverName   string
			originalName string
			itemType     string
		}),
	}
}

// RebuildMappings rebuilds the name mappings based on current server capabilities
func (nt *NameTracker) RebuildMappings(servers map[string]*ServerInfo) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	// Clear existing mappings
	nt.toolToServers = make(map[string][]string)
	nt.promptToServers = make(map[string][]string)
	nt.resourceToServers = make(map[string][]string)
	nt.nameMapping = make(map[string]struct {
		serverName   string
		originalName string
		itemType     string
	})

	// Build item mappings
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
		for _, resource := range info.Resources {
			nt.resourceToServers[resource.URI] = append(nt.resourceToServers[resource.URI], serverName)
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
				itemType     string
			}{
				serverName:   serverList[0],
				originalName: toolName,
				itemType:     "tool",
			}
		} else {
			// Conflict - prefix with server name
			for _, serverName := range serverList {
				prefixedName := fmt.Sprintf("%s.%s", serverName, toolName)
				nt.nameMapping[prefixedName] = struct {
					serverName   string
					originalName string
					itemType     string
				}{
					serverName:   serverName,
					originalName: toolName,
					itemType:     "tool",
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
				itemType     string
			}{
				serverName:   serverList[0],
				originalName: promptName,
				itemType:     "prompt",
			}
		} else {
			// Conflict - prefix with server name
			for _, serverName := range serverList {
				prefixedName := fmt.Sprintf("%s.%s", serverName, promptName)
				nt.nameMapping[prefixedName] = struct {
					serverName   string
					originalName string
					itemType     string
				}{
					serverName:   serverName,
					originalName: promptName,
					itemType:     "prompt",
				}
			}
		}
	}

	// Resources
	for resourceURI, serverList := range nt.resourceToServers {
		if len(serverList) == 1 {
			// No conflict - use original URI
			nt.nameMapping[resourceURI] = struct {
				serverName   string
				originalName string
				itemType     string
			}{
				serverName:   serverList[0],
				originalName: resourceURI,
				itemType:     "resource",
			}
		} else {
			// Conflict - prefix with server name
			for _, serverName := range serverList {
				prefixedURI := fmt.Sprintf("%s.%s", serverName, resourceURI)
				nt.nameMapping[prefixedURI] = struct {
					serverName   string
					originalName string
					itemType     string
				}{
					serverName:   serverName,
					originalName: resourceURI,
					itemType:     "resource",
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

// GetExposedResourceURI returns the URI to expose for a resource (with or without prefix)
func (nt *NameTracker) GetExposedResourceURI(serverName, resourceURI string) string {
	nt.mu.RLock()
	defer nt.mu.RUnlock()

	servers := nt.resourceToServers[resourceURI]
	if len(servers) <= 1 {
		return resourceURI
	}
	return fmt.Sprintf("%s.%s", serverName, resourceURI)
}

// ResolveName resolves an exposed name to server and original name
func (nt *NameTracker) ResolveName(exposedName string) (serverName, originalName string, itemType string, err error) {
	nt.mu.RLock()
	defer nt.mu.RUnlock()

	mapping, exists := nt.nameMapping[exposedName]
	if !exists {
		return "", "", "", fmt.Errorf("unknown name: %s", exposedName)
	}

	return mapping.serverName, mapping.originalName, mapping.itemType, nil
}
