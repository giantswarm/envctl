package mcpserver

import (
	"fmt"
	"path/filepath"
	"sync"

	"envctl/internal/config"
	"envctl/pkg/logging"
)

// MCPServerManager manages MCP server definitions
type MCPServerManager struct {
	mu          sync.RWMutex
	loader      *config.ConfigurationLoader
	definitions map[string]*MCPServerDefinition // server name -> definition
}

// NewMCPServerManager creates a new MCP server manager
func NewMCPServerManager() (*MCPServerManager, error) {
	loader, err := config.NewConfigurationLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create configuration loader: %w", err)
	}

	return &MCPServerManager{
		loader:      loader,
		definitions: make(map[string]*MCPServerDefinition),
	}, nil
}

// LoadDefinitions loads all MCP server definitions using the dedicated loader
// with enhanced error handling and graceful degradation
func (msm *MCPServerManager) LoadDefinitions() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	// Clear existing definitions
	msm.definitions = make(map[string]*MCPServerDefinition)

	// Use the dedicated MCP server loader
	definitions, errorCollection, err := LoadMCPServerDefinitions()
	if err != nil {
		return fmt.Errorf("failed to load MCP server definitions: %w", err)
	}

	logging.Info("MCPServerManager", "Loading %d MCP server definitions", len(definitions))

	// Process each definition
	for _, def := range definitions {
		msm.definitions[def.Name] = &def
		logging.Info("MCPServerManager", "Loaded MCP server definition: %s (type: %s)", def.Name, def.Type)
	}

	// Handle configuration errors with detailed reporting
	if errorCollection.HasErrors() {
		errorCount := errorCollection.Count()
		successCount := len(definitions)

		// Log comprehensive error information
		logging.Warn("MCPServerManager", "MCP server loading completed with %d errors (loaded %d successfully)",
			errorCount, successCount)

		// Log detailed error summary for troubleshooting
		logging.Warn("MCPServerManager", "MCP server configuration errors:\n%s",
			errorCollection.GetSummary())

		// Log full error details for debugging
		logging.Debug("MCPServerManager", "Detailed error report:\n%s",
			errorCollection.GetDetailedReport())

		// For MCP servers, we allow graceful degradation - return success with warnings
		// This enables the application to continue with working MCP server definitions
		logging.Info("MCPServerManager", "MCP server manager initialized with %d valid definitions (graceful degradation enabled)",
			successCount)

		return nil // Return success to allow graceful degradation
	}

	return nil
}

// validateDefinition performs basic validation on an MCP server definition
func (msm *MCPServerManager) validateDefinition(def *MCPServerDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("MCP server name cannot be empty")
	}

	// Validate type
	if def.Type != MCPServerTypeLocalCommand && def.Type != MCPServerTypeContainer {
		return fmt.Errorf("invalid MCP server type: %s", def.Type)
	}

	// Validate type-specific requirements
	switch def.Type {
	case MCPServerTypeLocalCommand:
		if len(def.Command) == 0 {
			return fmt.Errorf("command is required for local command MCP servers")
		}
	case MCPServerTypeContainer:
		if def.Image == "" {
			return fmt.Errorf("image is required for container MCP servers")
		}
	}

	return nil
}

// GetDefinition returns an MCP server definition by name
func (msm *MCPServerManager) GetDefinition(name string) (MCPServerDefinition, bool) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	def, exists := msm.definitions[name]
	if !exists {
		return MCPServerDefinition{}, false
	}
	return *def, true
}

// ListDefinitions returns all MCP server definitions
func (msm *MCPServerManager) ListDefinitions() []MCPServerDefinition {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	result := make([]MCPServerDefinition, 0, len(msm.definitions))
	for _, def := range msm.definitions {
		result = append(result, *def)
	}
	return result
}

// ListAvailableDefinitions returns all MCP server definitions (since no tool checking is done)
func (msm *MCPServerManager) ListAvailableDefinitions() []MCPServerDefinition {
	// For MCP servers, all definitions are considered available since we don't check tool availability
	return msm.ListDefinitions()
}

// IsAvailable checks if an MCP server definition is available
func (msm *MCPServerManager) IsAvailable(name string) bool {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	_, exists := msm.definitions[name]
	// For MCP servers, availability is simply whether the definition exists
	return exists
}

// RefreshAvailability refreshes the availability status of all MCP servers
func (msm *MCPServerManager) RefreshAvailability() {
	// For MCP servers, availability is static (no tool checking), so no refresh needed
	logging.Debug("MCPServerManager", "Refreshed MCP server availability (no tool checking required)")
}

// GetDefinitionsPath returns the paths where MCP server definitions are loaded from
func (msm *MCPServerManager) GetDefinitionsPath() string {
	userDir, projectDir, err := config.GetConfigurationPaths()
	if err != nil {
		logging.Error("MCPServerManager", err, "Failed to get configuration paths")
		return "error determining paths"
	}

	userPath := filepath.Join(userDir, "mcpservers")
	projectPath := filepath.Join(projectDir, "mcpservers")

	return fmt.Sprintf("User: %s, Project: %s", userPath, projectPath)
}

// GetAllDefinitions returns all MCP server definitions (for internal use)
func (msm *MCPServerManager) GetAllDefinitions() map[string]*MCPServerDefinition {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*MCPServerDefinition)
	for name, def := range msm.definitions {
		result[name] = def
	}
	return result
} 