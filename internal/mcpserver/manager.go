package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"envctl/internal/config"
	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// MCPServerManager manages MCP server definitions
type MCPServerManager struct {
	mu          sync.RWMutex
	loader      *config.ConfigurationLoader
	definitions map[string]*MCPServerDefinition // server name -> definition
	storage     *config.Storage
	configPath  string // Optional custom config path
}

// NewMCPServerManager creates a new MCP server manager
func NewMCPServerManager(storage *config.Storage) (*MCPServerManager, error) {
	loader, err := config.NewConfigurationLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create configuration loader: %w", err)
	}

	if storage == nil {
		return nil, fmt.Errorf("storage is required")
	}

	// Extract config path from storage if it has one
	var configPath string
	if storage != nil {
		// We can't directly access the configPath from storage, so we'll pass it via parameter later
		// For now, leave it empty
	}

	return &MCPServerManager{
		loader:      loader,
		definitions: make(map[string]*MCPServerDefinition),
		storage:     storage,
		configPath:  configPath,
	}, nil
}

// SetConfigPath sets the custom configuration path
func (msm *MCPServerManager) SetConfigPath(configPath string) {
	msm.mu.Lock()
	defer msm.mu.Unlock()
	msm.configPath = configPath
}

// LoadDefinitions loads all MCP server definitions from YAML files.
// All MCP servers are just YAML files, regardless of how they were created.
func (msm *MCPServerManager) LoadDefinitions() error {
	// Load all MCP server YAML files using the config path-aware helper
	validator := func(def MCPServerDefinition) error {
		return msm.validateDefinition(&def)
	}

	definitions, errorCollection, err := config.LoadAndParseYAMLWithConfig[MCPServerDefinition](msm.configPath, "mcpservers", validator)
	if err != nil {
		logging.Warn("MCPServerManager", "Error loading MCP servers: %v", err)
		return err
	}

	// Log any validation errors but continue with valid definitions
	if errorCollection != nil && errorCollection.HasErrors() {
		logging.Warn("MCPServerManager", "Some MCP server files had errors:\n%s", errorCollection.GetSummary())
	}

	// Acquire lock to update in-memory state
	msm.mu.Lock()
	defer msm.mu.Unlock()

	// Clear the old definitions
	msm.definitions = make(map[string]*MCPServerDefinition)

	// Add all valid definitions to in-memory store
	for i := range definitions {
		def := definitions[i] // Important: take a copy
		msm.definitions[def.Name] = &def
	}

	logging.Info("MCPServerManager", "Loaded %d MCP servers from YAML files", len(definitions))
	return nil
}

// validateDefinition performs comprehensive validation on an MCP server definition
func (msm *MCPServerManager) validateDefinition(def *MCPServerDefinition) error {
	var errors config.ValidationErrors

	// Validate entity name using common helper
	if err := config.ValidateEntityName(def.Name, "MCP server"); err != nil {
		errors = append(errors, err.(config.ValidationError))
	}

	// Validate type
	validTypes := []string{string(MCPServerTypeLocalCommand), string(MCPServerTypeContainer), string(MCPServerTypeMock)}
	if err := config.ValidateOneOf("type", string(def.Type), validTypes); err != nil {
		errors = append(errors, err.(config.ValidationError))
	}

	// Validate icon (optional)
	if def.Icon != "" {
		if err := config.ValidateMaxLength("icon", def.Icon, 10); err != nil {
			errors = append(errors, err.(config.ValidationError))
		}
	}

	// Validate type-specific requirements
	switch def.Type {
	case MCPServerTypeLocalCommand:
		if len(def.Command) == 0 {
			errors.Add("command", "is required for local command MCP servers")
		} else {
			// Validate command elements
			for i, cmd := range def.Command {
				if strings.TrimSpace(cmd) == "" {
					errors.Add(fmt.Sprintf("command[%d]", i), "command element cannot be empty")
				}
			}
		}

		// Note: Args are part of Command array, no separate validation needed

	case MCPServerTypeContainer:
		if err := config.ValidateRequired("image", def.Image, "container MCP server"); err != nil {
			errors = append(errors, err.(config.ValidationError))
		}

		// Validate environment variables if present
		for key, value := range def.Env {
			if key == "" {
				errors.Add("env", "environment variable key cannot be empty")
			}
			if value == "" {
				errors.Add(fmt.Sprintf("env.%s", key), "environment variable value cannot be empty")
			}
		}
	}

	// Validate health check interval if specified
	if def.HealthCheckInterval != 0 {
		if def.HealthCheckInterval < time.Second {
			errors.Add("healthCheckInterval", "must be at least 1 second")
		}
	}

	if errors.HasErrors() {
		return config.FormatValidationError("MCP server", def.Name, errors)
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

	// Add to in-memory store after successful save
	msm.definitions[def.Name] = &def

	logging.Info("MCPServerManager", "Created MCP server %s (type: %s)", def.Name, def.Type)
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

	// Update in-memory store after successful save
	msm.definitions[name] = &def

	logging.Info("MCPServerManager", "Updated MCP server %s (type: %s)", name, def.Type)
	return nil
}

// DeleteMCPServer deletes an MCP server from YAML files and memory
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
			return fmt.Errorf("failed to delete MCP server %s from YAML files: %w", name, err)
		}
	}

	// Remove from in-memory store after successful deletion
	delete(msm.definitions, name)

	logging.Info("MCPServerManager", "Deleted MCP server %s", name)
	return nil
}
