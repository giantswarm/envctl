package capability

import (
	"fmt"
	"sync"

	"envctl/internal/config"
	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// CapabilityManager manages capability definitions and their availability
type CapabilityManager struct {
	mu             sync.RWMutex
	loader         *config.ConfigurationLoader
	definitions    map[string]*CapabilityDefinition // capability name -> definition
	toolChecker    config.ToolAvailabilityChecker
	registry       *Registry
	exposedTools   map[string]bool // Track which capability tools we've exposed
	dynamicStorage *config.DynamicStorage
}

// NewCapabilityManager creates a new capability manager
func NewCapabilityManager(toolChecker config.ToolAvailabilityChecker, registry *Registry, dynamicStorage *config.DynamicStorage) (*CapabilityManager, error) {
	loader, err := config.NewConfigurationLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create configuration loader: %w", err)
	}

	return &CapabilityManager{
		loader:         loader,
		definitions:    make(map[string]*CapabilityDefinition),
		toolChecker:    toolChecker,
		registry:       registry,
		exposedTools:   make(map[string]bool),
		dynamicStorage: dynamicStorage,
	}, nil
}

// LoadDefinitions loads all capability definitions using layered configuration loading
// with enhanced error handling and graceful degradation
func (cm *CapabilityManager) LoadDefinitions() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Clear existing definitions
	cm.definitions = make(map[string]*CapabilityDefinition)
	cm.exposedTools = make(map[string]bool)

	// Load capability definitions using the enhanced configuration loader
	definitions, errorCollection, err := config.LoadAndParseYAMLWithErrors[CapabilityDefinition]("capabilities", func(def CapabilityDefinition) error {
		return cm.validateDefinition(&def)
	})
	if err != nil {
		return fmt.Errorf("failed to load capability definitions: %w", err)
	}

	logging.Info("CapabilityManager", "Loading %d capability definitions from files", len(definitions))

	// Process file-based definitions first
	for _, def := range definitions {
		cm.definitions[def.Name] = &def
		logging.Info("CapabilityManager", "Loaded file-based capability definition: %s (type: %s)", def.Name, def.Type)
	}

	// Load dynamic definitions if DynamicStorage is available
	if cm.dynamicStorage != nil {
		dynamicCapabilities, err := cm.loadDynamicDefinitions()
		if err != nil {
			logging.Error("CapabilityManager", err, "Failed to load dynamic capability definitions, continuing with file-based only")
		} else {
			// Dynamic definitions override file-based ones
			for name, def := range dynamicCapabilities {
				if _, exists := cm.definitions[name]; exists {
					logging.Info("CapabilityManager", "Dynamic definition overriding file-based definition: %s", name)
				} else {
					logging.Info("CapabilityManager", "Loaded dynamic capability definition: %s (type: %s)", def.Name, def.Type)
				}
				cm.definitions[name] = def
			}
			logging.Info("CapabilityManager", "Loaded %d dynamic capability definitions", len(dynamicCapabilities))
		}
	}

	// Check which capabilities can be exposed
	cm.updateAvailableCapabilities()

	// Handle configuration errors with detailed reporting
	if errorCollection.HasErrors() {
		errorCount := errorCollection.Count()
		successCount := len(definitions)

		// Log comprehensive error information
		logging.Warn("CapabilityManager", "Capability loading completed with %d errors (loaded %d successfully)",
			errorCount, successCount)

		// Log detailed error summary for troubleshooting
		logging.Warn("CapabilityManager", "Capability configuration errors:\n%s",
			errorCollection.GetSummary())

		// Log full error details for debugging
		logging.Debug("CapabilityManager", "Detailed error report:\n%s",
			errorCollection.GetDetailedReport())

		// For Capability, we allow graceful degradation - return success with warnings
		// This enables the application to continue with working Capability definitions
		logging.Info("CapabilityManager", "Capability manager initialized with %d valid definitions (graceful degradation enabled)",
			successCount)

		return nil // Return success to allow graceful degradation
	}

	return nil
}

// validateDefinition validates a capability definition
func (cm *CapabilityManager) validateDefinition(def *CapabilityDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("capability name is required")
	}
	if def.Type == "" {
		return fmt.Errorf("capability type is required")
	}
	if len(def.Operations) == 0 {
		return fmt.Errorf("at least one operation is required")
	}

	// Validate the capability type (allow any non-empty string)
	if !IsValidCapabilityType(def.Type) {
		return fmt.Errorf("capability type cannot be empty")
	}

	return nil
}

// GetDefinition returns a capability definition by name
func (cm *CapabilityManager) GetDefinition(name string) (CapabilityDefinition, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	def, exists := cm.definitions[name]
	if !exists {
		return CapabilityDefinition{}, false
	}
	return *def, true
}

// ListDefinitions returns all capability definitions
func (cm *CapabilityManager) ListDefinitions() []CapabilityDefinition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]CapabilityDefinition, 0, len(cm.definitions))
	for _, def := range cm.definitions {
		result = append(result, *def)
	}
	return result
}

// ListAvailableDefinitions returns only capability definitions that have all required tools available
func (cm *CapabilityManager) ListAvailableDefinitions() []CapabilityDefinition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]CapabilityDefinition, 0)
	for _, def := range cm.definitions {
		if cm.isDefinitionAvailable(def) {
			result = append(result, *def)
		}
	}
	return result
}

// IsAvailable checks if a capability is available (has all required tools)
func (cm *CapabilityManager) IsAvailable(name string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	def, exists := cm.definitions[name]
	if !exists {
		return false
	}
	return cm.isDefinitionAvailable(def)
}

// isDefinitionAvailable checks if a capability definition has all required tools available
func (cm *CapabilityManager) isDefinitionAvailable(def *CapabilityDefinition) bool {
	for _, op := range def.Operations {
		if !cm.areRequiredToolsAvailable(op.Requires) {
			return false
		}
	}
	return true
}

// RefreshAvailability refreshes the availability status of all capabilities
func (cm *CapabilityManager) RefreshAvailability() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.updateAvailableCapabilities()
}

// GetDefinitionsPath returns the paths where capability definitions are loaded from
func (cm *CapabilityManager) GetDefinitionsPath() string {
	userDir, projectDir, err := config.GetConfigurationPaths()
	if err != nil {
		logging.Error("CapabilityManager", err, "Failed to get configuration paths")
		return "error determining paths"
	}

	userPath := fmt.Sprintf("%s/capabilities", userDir)
	projectPath := fmt.Sprintf("%s/capabilities", projectDir)

	return fmt.Sprintf("User: %s, Project: %s", userPath, projectPath)
}

// updateAvailableCapabilities checks tool availability and updates exposed capabilities
func (cm *CapabilityManager) updateAvailableCapabilities() {
	for _, def := range cm.definitions {
		// Check each operation
		for opName, op := range def.Operations {
			if cm.areRequiredToolsAvailable(op.Requires) {
				// Mark this capability operation as available with api_ format
				toolName := fmt.Sprintf("api_%s_%s", def.Type, opName)
				if !cm.exposedTools[toolName] {
					cm.exposedTools[toolName] = true
					logging.Info("CapabilityManager", "Capability operation available: %s", toolName)
				}
			} else {
				// Mark as unavailable
				toolName := fmt.Sprintf("api_%s_%s", def.Type, opName)
				if cm.exposedTools[toolName] {
					delete(cm.exposedTools, toolName)
					logging.Info("CapabilityManager", "Capability operation unavailable: %s (missing tools)", toolName)
				}
			}
		}
	}
}

// areRequiredToolsAvailable checks if all required tools are available
func (cm *CapabilityManager) areRequiredToolsAvailable(requiredTools []string) bool {
	if cm.toolChecker == nil {
		return false
	}

	for _, tool := range requiredTools {
		if !cm.toolChecker.IsToolAvailable(tool) {
			return false
		}
	}
	return true
}

// GetAvailableCapabilityTools returns all capability tools that can be exposed
func (cm *CapabilityManager) GetAvailableCapabilityTools() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	tools := make([]string, 0, len(cm.exposedTools))
	for tool := range cm.exposedTools {
		tools = append(tools, tool)
	}
	return tools
}

// GetOperationForTool returns the operation definition for a capability tool
func (cm *CapabilityManager) GetOperationForTool(toolName string) (*OperationDefinition, *CapabilityDefinition, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Tool names are in format api_<type>_<operation>
	// e.g., api_auth_login -> auth type, login operation

	for _, def := range cm.definitions {
		for opName, op := range def.Operations {
			// Check api_ format: api_<type>_<operation>
			expectedTool := fmt.Sprintf("api_%s_%s", def.Type, opName)
			if expectedTool == toolName {
				return &op, def, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no operation found for tool %s", toolName)
}

// loadDynamicDefinitions loads capability definitions from dynamic storage
func (cm *CapabilityManager) loadDynamicDefinitions() (map[string]*CapabilityDefinition, error) {
	definitions := make(map[string]*CapabilityDefinition)

	// List all capability files in dynamic storage
	files, err := cm.dynamicStorage.List("capabilities")
	if err != nil {
		return nil, fmt.Errorf("failed to list dynamic capability files: %w", err)
	}

	for _, filename := range files {
		// Read the capability file
		data, err := cm.dynamicStorage.Load("capabilities", filename)
		if err != nil {
			logging.Error("CapabilityManager", err, "Failed to load dynamic capability file: %s", filename)
			continue
		}

		// Parse the YAML content
		var def CapabilityDefinition
		if err := yaml.Unmarshal(data, &def); err != nil {
			logging.Error("CapabilityManager", err, "Failed to parse dynamic capability file: %s", filename)
			continue
		}

		// Validate the definition
		if err := cm.validateDefinition(&def); err != nil {
			logging.Error("CapabilityManager", err, "Invalid dynamic capability definition: %s", filename)
			continue
		}

		definitions[def.Name] = &def
	}

	return definitions, nil
}

// CreateCapability creates a new capability definition in dynamic storage
func (cm *CapabilityManager) CreateCapability(def *CapabilityDefinition) error {
	if cm.dynamicStorage == nil {
		return fmt.Errorf("dynamic storage not available")
	}

	// Validate the definition
	if err := cm.validateDefinition(def); err != nil {
		return fmt.Errorf("invalid capability definition: %w", err)
	}

	// Serialize to YAML
	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal capability definition: %w", err)
	}

	// Save to dynamic storage
	filename := fmt.Sprintf("%s.yaml", def.Name)
	if err := cm.dynamicStorage.Save("capabilities", filename, data); err != nil {
		return fmt.Errorf("failed to save capability definition: %w", err)
	}

	// Register in memory
	cm.RegisterDefinition(def)

	logging.Info("CapabilityManager", "Created dynamic capability definition: %s", def.Name)
	return nil
}

// UpdateCapability updates an existing capability definition in dynamic storage
func (cm *CapabilityManager) UpdateCapability(def *CapabilityDefinition) error {
	if cm.dynamicStorage == nil {
		return fmt.Errorf("dynamic storage not available")
	}

	// Check if capability exists
	if _, exists := cm.GetDefinition(def.Name); !exists {
		return fmt.Errorf("capability '%s' not found", def.Name)
	}

	// Validate the definition
	if err := cm.validateDefinition(def); err != nil {
		return fmt.Errorf("invalid capability definition: %w", err)
	}

	// Serialize to YAML
	data, err := yaml.Marshal(def)
	if err != nil {
		return fmt.Errorf("failed to marshal capability definition: %w", err)
	}

	// Save to dynamic storage
	filename := fmt.Sprintf("%s.yaml", def.Name)
	if err := cm.dynamicStorage.Save("capabilities", filename, data); err != nil {
		return fmt.Errorf("failed to save capability definition: %w", err)
	}

	// Update in memory
	cm.UpdateDefinition(def)

	logging.Info("CapabilityManager", "Updated dynamic capability definition: %s", def.Name)
	return nil
}

// DeleteCapability deletes a capability definition from dynamic storage
func (cm *CapabilityManager) DeleteCapability(name string) error {
	if cm.dynamicStorage == nil {
		return fmt.Errorf("dynamic storage not available")
	}

	// Check if capability exists
	if _, exists := cm.GetDefinition(name); !exists {
		return fmt.Errorf("capability '%s' not found", name)
	}

	// Delete from dynamic storage
	filename := fmt.Sprintf("%s.yaml", name)
	if err := cm.dynamicStorage.Delete("capabilities", filename); err != nil {
		return fmt.Errorf("failed to delete capability definition: %w", err)
	}

	// Remove from memory
	cm.UnregisterDefinition(name)

	logging.Info("CapabilityManager", "Deleted dynamic capability definition: %s", name)
	return nil
}

// RegisterDefinition adds a capability definition to the in-memory registry
func (cm *CapabilityManager) RegisterDefinition(def *CapabilityDefinition) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.definitions[def.Name] = def
	cm.updateAvailableCapabilities()
}

// UpdateDefinition updates a capability definition in the in-memory registry
func (cm *CapabilityManager) UpdateDefinition(def *CapabilityDefinition) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.definitions[def.Name] = def
	cm.updateAvailableCapabilities()
}

// UnregisterDefinition removes a capability definition from the in-memory registry
func (cm *CapabilityManager) UnregisterDefinition(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.definitions, name)
	cm.updateAvailableCapabilities()
}
