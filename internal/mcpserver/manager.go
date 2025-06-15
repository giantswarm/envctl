package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"envctl/internal/config"
	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// MCPServerManager manages MCP server definitions
type MCPServerManager struct {
	mu          sync.RWMutex
	loader      *config.ConfigurationLoader
	definitions map[string]*MCPServerDefinition // server name -> definition
	storage     *config.DynamicStorage
}

// NewMCPServerManager creates a new MCP server manager
func NewMCPServerManager(storage *config.DynamicStorage) (*MCPServerManager, error) {
	loader, err := config.NewConfigurationLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create configuration loader: %w", err)
	}

	if storage == nil {
		return nil, fmt.Errorf("storage is required")
	}

	return &MCPServerManager{
		loader:      loader,
		definitions: make(map[string]*MCPServerDefinition),
		storage:     storage,
	}, nil
}

// LoadDefinitions loads all MCP server definitions from files and dynamic storage
// Definitions from dynamic storage will override file-based ones with the same name
func (msm *MCPServerManager) LoadDefinitions() error {
	// 1. Load from YAML files and dynamic storage without holding a lock.
	// This prevents deadlocks during I/O.
	validator := func(def MCPServerDefinition) error {
		return msm.validateDefinition(&def)
	}
	fileDefs, errorCollection, err := config.LoadAndParseYAMLWithErrors[MCPServerDefinition]("mcpservers", validator)
	if err != nil {
		// Log as a warning and continue, allowing the app to start with what it can load.
		logging.Warn("MCPServerManager", "Error loading file-based MCP servers: %v", err)
	}

	dynamicDefs := make(map[string]*MCPServerDefinition)
	dynamicNames, err := msm.storage.List("mcpservers")
	if err != nil {
		logging.Warn("MCPServerManager", "Could not list MCP servers from dynamic storage: %v", err)
	} else {
		for _, name := range dynamicNames {
			data, err := msm.storage.Load("mcpservers", name)
			if err != nil {
				logging.Warn("MCPServerManager", "Failed to load MCP server '%s': %v", name, err)
				continue
			}

			var mcpDef MCPServerDefinition
			if err := yaml.Unmarshal(data, &mcpDef); err != nil {
				logging.Warn("MCPServerManager", "Failed to parse MCP server '%s': %v", name, err)
				continue
			}
			mcpDef.Name = name // Name from storage is the source of truth
			dynamicDefs[name] = &mcpDef
		}
	}

	// 2. Now, acquire a single lock to update the in-memory state.
	msm.mu.Lock()
	defer msm.mu.Unlock()

	// Clear the old definitions
	msm.definitions = make(map[string]*MCPServerDefinition)

	// Add file-based definitions first
	for i := range fileDefs {
		def := fileDefs[i] // Important: take a copy
		msm.definitions[def.Name] = &def
	}
	logging.Info("MCPServerManager", "Loaded %d MCP servers from files", len(fileDefs))

	// Add/overwrite with dynamic definitions
	for name, def := range dynamicDefs {
		msm.definitions[name] = def
	}
	logging.Info("MCPServerManager", "Loaded/updated %d MCP servers from DynamicStorage", len(dynamicDefs))

	// Handle configuration errors with detailed reporting
	if errorCollection != nil && errorCollection.HasErrors() {
		errorCount := errorCollection.Count()
		successCount := len(fileDefs)

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

// RegisterDefinition registers a new MCP server definition
func (msm *MCPServerManager) RegisterDefinition(def MCPServerDefinition) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if err := msm.validateDefinition(&def); err != nil {
		return fmt.Errorf("invalid MCP server definition: %w", err)
	}

	msm.definitions[def.Name] = &def
	return nil
}

// UpdateDefinition updates an existing MCP server definition
func (msm *MCPServerManager) UpdateDefinition(name string, def MCPServerDefinition) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if err := msm.validateDefinition(&def); err != nil {
		return fmt.Errorf("invalid MCP server definition: %w", err)
	}

	existingDef, exists := msm.definitions[name]
	if !exists {
		return fmt.Errorf("MCP server definition %s does not exist", name)
	}

	*existingDef = def
	return nil
}

// UnregisterDefinition removes an MCP server definition
func (msm *MCPServerManager) UnregisterDefinition(name string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if _, exists := msm.definitions[name]; !exists {
		return fmt.Errorf("MCP server definition %s does not exist", name)
	}

	delete(msm.definitions, name)
	return nil
}

// CreateMCPServer creates and persists a new MCP server
func (msm *MCPServerManager) CreateMCPServer(def MCPServerDefinition) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if _, exists := msm.definitions[def.Name]; exists {
		return fmt.Errorf("MCP server '%s' already exists", def.Name)
	}

	if err := msm.validateDefinition(&def); err != nil {
		return fmt.Errorf("invalid MCP server definition: %w", err)
	}

	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal MCP server %s: %w", def.Name, err)
	}

	if err := msm.storage.Save("mcpservers", def.Name, data); err != nil {
		return fmt.Errorf("failed to save MCP server %s: %w", def.Name, err)
	}

	msm.definitions[def.Name] = &def
	logging.Info("MCPServerManager", "Created MCP server %s", def.Name)
	return nil
}

// UpdateMCPServer updates and persists an existing MCP server
func (msm *MCPServerManager) UpdateMCPServer(name string, def MCPServerDefinition) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if _, exists := msm.definitions[name]; !exists {
		return fmt.Errorf("MCP server '%s' not found", name)
	}
	def.Name = name

	if err := msm.validateDefinition(&def); err != nil {
		return fmt.Errorf("invalid MCP server definition: %w", err)
	}

	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal MCP server %s: %w", name, err)
	}

	if err := msm.storage.Save("mcpservers", name, data); err != nil {
		return fmt.Errorf("failed to save MCP server %s: %w", name, err)
	}

	msm.definitions[name] = &def
	logging.Info("MCPServerManager", "Updated MCP server %s", name)
	return nil
}

// DeleteMCPServer deletes an MCP server from storage and memory
func (msm *MCPServerManager) DeleteMCPServer(name string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if _, exists := msm.definitions[name]; !exists {
		return fmt.Errorf("MCP server '%s' not found", name)
	}

	if err := msm.storage.Delete("mcpservers", name); err != nil {
		// If it doesn't exist in storage, but exists in memory (from file), that's ok.
		// We just need to remove it from memory.
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete MCP server %s from storage: %w", name, err)
		}
	}

	delete(msm.definitions, name)
	logging.Info("MCPServerManager", "Deleted MCP server %s", name)
	return nil
}
