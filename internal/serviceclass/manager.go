package serviceclass

import (
	"fmt"
	"path/filepath"
	"sync"

	"envctl/internal/config"
	"envctl/pkg/logging"
)

// ServiceClassManager manages service class definitions and their availability
type ServiceClassManager struct {
	mu              sync.RWMutex
	loader          *config.ConfigurationLoader
	definitions     map[string]*ServiceClassDefinition // service class name -> definition
	toolChecker     config.ToolAvailabilityChecker
	exposedServices map[string]bool // Track which service classes are available

	// Callbacks for lifecycle events
	onRegister   []func(def *ServiceClassDefinition)
	onUnregister []func(serviceClassName string)
	onUpdate     []func(def *ServiceClassDefinition)
}

// NewServiceClassManager creates a new service class manager
func NewServiceClassManager(toolChecker config.ToolAvailabilityChecker) (*ServiceClassManager, error) {
	loader, err := config.NewConfigurationLoader()
	if err != nil {
		return nil, fmt.Errorf("failed to create configuration loader: %w", err)
	}

	return &ServiceClassManager{
		loader:          loader,
		definitions:     make(map[string]*ServiceClassDefinition),
		toolChecker:     toolChecker,
		exposedServices: make(map[string]bool),
		onRegister:      []func(def *ServiceClassDefinition){},
		onUnregister:    []func(serviceClassName string){},
		onUpdate:        []func(def *ServiceClassDefinition){},
	}, nil
}

// LoadServiceDefinitions loads all service class definitions using layered configuration loading
// with enhanced error handling and graceful degradation
func (scm *ServiceClassManager) LoadServiceDefinitions() error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	// Clear existing definitions
	scm.definitions = make(map[string]*ServiceClassDefinition)
	scm.exposedServices = make(map[string]bool)

	// Load service class definitions using the enhanced configuration loader
	definitions, errorCollection, err := config.LoadAndParseYAMLWithErrors[ServiceClassDefinition]("serviceclasses", func(def ServiceClassDefinition) error {
		return scm.validateServiceClassDefinition(&def)
	})
	if err != nil {
		return fmt.Errorf("failed to load service class definitions: %w", err)
	}

	logging.Info("ServiceClassManager", "Loading %d service class definitions", len(definitions))

	// Process each definition
	for _, def := range definitions {
		scm.definitions[def.Name] = &def
		logging.Info("ServiceClassManager", "Loaded service class definition: %s (type: %s)", def.Name, def.Type)

		// Notify registration callbacks
		for _, callback := range scm.onRegister {
			callback(&def)
		}
	}

	// Check which service classes are available based on tool availability
	scm.updateServiceAvailability()

	// Handle configuration errors with detailed reporting
	if errorCollection.HasErrors() {
		errorCount := errorCollection.Count()
		successCount := len(definitions)

		// Log comprehensive error information
		logging.Warn("ServiceClassManager", "ServiceClass loading completed with %d errors (loaded %d successfully)",
			errorCount, successCount)

		// Log detailed error summary for troubleshooting
		logging.Warn("ServiceClassManager", "ServiceClass configuration errors:\n%s",
			errorCollection.GetSummary())

		// Log full error details for debugging
		logging.Debug("ServiceClassManager", "Detailed error report:\n%s",
			errorCollection.GetDetailedReport())

		// For ServiceClass, we allow graceful degradation - return success with warnings
		// This enables the application to continue with working ServiceClass definitions
		logging.Info("ServiceClassManager", "ServiceClass manager initialized with %d valid definitions (graceful degradation enabled)",
			successCount)

		return nil // Return success to allow graceful degradation
	}

	return nil
}

// validateServiceClassDefinition performs basic validation on a service class definition
func (scm *ServiceClassManager) validateServiceClassDefinition(def *ServiceClassDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("service class name cannot be empty")
	}
	if def.Type == "" {
		return fmt.Errorf("service class type cannot be empty")
	}
	if def.Version == "" {
		return fmt.Errorf("service class version cannot be empty")
	}

	// Validate lifecycle tools if present
	if def.ServiceConfig.LifecycleTools.Start.Tool == "" {
		return fmt.Errorf("start tool is required in lifecycle tools")
	}
	if def.ServiceConfig.LifecycleTools.Stop.Tool == "" {
		return fmt.Errorf("stop tool is required in lifecycle tools")
	}

	return nil
}

// updateServiceAvailability checks tool availability and updates service class availability
func (scm *ServiceClassManager) updateServiceAvailability() {
	for name, def := range scm.definitions {
		requiredTools := scm.getRequiredTools(def)
		available := scm.areAllToolsAvailable(requiredTools)

		oldAvailable := scm.exposedServices[name]
		scm.exposedServices[name] = available

		if available && !oldAvailable {
			logging.Info("ServiceClassManager", "Service class available: %s", name)
		} else if !available && oldAvailable {
			logging.Warn("ServiceClassManager", "Service class unavailable: %s (missing tools)", name)
		}
	}
}

// getRequiredTools extracts all required tools from a service class definition
func (scm *ServiceClassManager) getRequiredTools(def *ServiceClassDefinition) []string {
	tools := make(map[string]bool)

	// Add lifecycle tools
	if def.ServiceConfig.LifecycleTools.Start.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Start.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.Stop.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Stop.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.Restart != nil && def.ServiceConfig.LifecycleTools.Restart.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Restart.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.HealthCheck != nil && def.ServiceConfig.LifecycleTools.HealthCheck.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.HealthCheck.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.Status != nil && def.ServiceConfig.LifecycleTools.Status.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Status.Tool] = true
	}

	// Add tools from operations (existing capability system compatibility)
	for _, op := range def.Operations {
		for _, tool := range op.Requires {
			tools[tool] = true
		}
	}

	// Convert to slice
	result := make([]string, 0, len(tools))
	for tool := range tools {
		result = append(result, tool)
	}

	return result
}

// areAllToolsAvailable checks if all required tools are available
func (scm *ServiceClassManager) areAllToolsAvailable(requiredTools []string) bool {
	if scm.toolChecker == nil {
		return false
	}

	for _, tool := range requiredTools {
		if !scm.toolChecker.IsToolAvailable(tool) {
			return false
		}
	}
	return true
}

// GetServiceClassDefinition returns a service class definition by name
func (scm *ServiceClassManager) GetServiceClassDefinition(name string) (*ServiceClassDefinition, bool) {
	scm.mu.RLock()
	defer scm.mu.RUnlock()

	def, exists := scm.definitions[name]
	return def, exists
}

// ListServiceClasses returns information about all registered service classes
func (scm *ServiceClassManager) ListServiceClasses() []ServiceClassInfo {
	scm.mu.RLock()
	defer scm.mu.RUnlock()

	result := make([]ServiceClassInfo, 0, len(scm.definitions))

	for _, def := range scm.definitions {
		requiredTools := scm.getRequiredTools(def)
		missingTools := scm.getMissingTools(requiredTools)
		available := len(missingTools) == 0

		info := ServiceClassInfo{
			Name:                     def.Name,
			Type:                     def.Type,
			Version:                  def.Version,
			Description:              def.Description,
			ServiceType:              def.ServiceConfig.ServiceType,
			Available:                available,
			CreateToolAvailable:      scm.toolChecker != nil && scm.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.Start.Tool),
			DeleteToolAvailable:      scm.toolChecker != nil && scm.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.Stop.Tool),
			HealthCheckToolAvailable: def.ServiceConfig.LifecycleTools.HealthCheck != nil && scm.toolChecker != nil && scm.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.HealthCheck.Tool),
			StatusToolAvailable:      def.ServiceConfig.LifecycleTools.Status != nil && scm.toolChecker != nil && scm.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.Status.Tool),
			RequiredTools:            requiredTools,
			MissingTools:             missingTools,
		}

		result = append(result, info)
	}

	return result
}

// ListAvailableServiceClasses returns only service classes that have all required tools available
func (scm *ServiceClassManager) ListAvailableServiceClasses() []ServiceClassInfo {
	all := scm.ListServiceClasses()
	result := make([]ServiceClassInfo, 0, len(all))

	for _, info := range all {
		if info.Available {
			result = append(result, info)
		}
	}

	return result
}

// getMissingTools returns tools that are required but not available
func (scm *ServiceClassManager) getMissingTools(requiredTools []string) []string {
	if scm.toolChecker == nil {
		return requiredTools // All tools are missing if no checker
	}

	var missing []string
	for _, tool := range requiredTools {
		if !scm.toolChecker.IsToolAvailable(tool) {
			missing = append(missing, tool)
		}
	}
	return missing
}

// IsServiceClassAvailable checks if a service class is available
func (scm *ServiceClassManager) IsServiceClassAvailable(name string) bool {
	scm.mu.RLock()
	defer scm.mu.RUnlock()

	return scm.exposedServices[name]
}

// RefreshAvailability refreshes the availability status of all service classes
func (scm *ServiceClassManager) RefreshAvailability() {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	scm.updateServiceAvailability()
}

// RegisterDefinition registers a service class definition programmatically
func (scm *ServiceClassManager) RegisterDefinition(def *ServiceClassDefinition) error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	// Validate the definition
	if err := scm.validateServiceClassDefinition(def); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if already registered
	if _, exists := scm.definitions[def.Name]; exists {
		return fmt.Errorf("service class %s already registered", def.Name)
	}

	// Register the definition
	scm.definitions[def.Name] = def

	// Update availability
	requiredTools := scm.getRequiredTools(def)
	scm.exposedServices[def.Name] = scm.areAllToolsAvailable(requiredTools)

	logging.Info("ServiceClassManager", "Registered service class: %s (type: %s)", def.Name, def.Type)

	// Notify registration callbacks
	for _, callback := range scm.onRegister {
		callback(def)
	}

	return nil
}

// UnregisterDefinition unregisters a service class definition
func (scm *ServiceClassManager) UnregisterDefinition(name string) error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	if _, exists := scm.definitions[name]; !exists {
		return fmt.Errorf("service class %s not found", name)
	}

	delete(scm.definitions, name)
	delete(scm.exposedServices, name)

	logging.Info("ServiceClassManager", "Unregistered service class: %s", name)

	// Notify unregistration callbacks
	for _, callback := range scm.onUnregister {
		callback(name)
	}

	return nil
}

// OnRegister adds a callback for service class registration
func (scm *ServiceClassManager) OnRegister(callback func(def *ServiceClassDefinition)) {
	scm.mu.Lock()
	defer scm.mu.Unlock()
	scm.onRegister = append(scm.onRegister, callback)
}

// OnUnregister adds a callback for service class removal
func (scm *ServiceClassManager) OnUnregister(callback func(serviceClassName string)) {
	scm.mu.Lock()
	defer scm.mu.Unlock()
	scm.onUnregister = append(scm.onUnregister, callback)
}

// OnUpdate adds a callback for service class updates
func (scm *ServiceClassManager) OnUpdate(callback func(def *ServiceClassDefinition)) {
	scm.mu.Lock()
	defer scm.mu.Unlock()
	scm.onUpdate = append(scm.onUpdate, callback)
}

// GetDefinitionsPath returns the paths where service class definitions are loaded from
func (scm *ServiceClassManager) GetDefinitionsPath() string {
	userDir, projectDir, err := config.GetConfigurationPaths()
	if err != nil {
		logging.Error("ServiceClassManager", err, "Failed to get configuration paths")
		return "error determining paths"
	}

	userPath := filepath.Join(userDir, "serviceclasses")
	projectPath := filepath.Join(projectDir, "serviceclasses")

	return fmt.Sprintf("User: %s, Project: %s", userPath, projectPath)
}

// GetAllDefinitions returns all service class definitions (for internal use)
func (scm *ServiceClassManager) GetAllDefinitions() map[string]*ServiceClassDefinition {
	scm.mu.RLock()
	defer scm.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]*ServiceClassDefinition)
	for name, def := range scm.definitions {
		result[name] = def
	}
	return result
}
