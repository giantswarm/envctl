package serviceclass

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// ServiceClassManager manages service class definitions and their availability
type ServiceClassManager struct {
	mu              sync.RWMutex
	definitionsPath string
	definitions     map[string]*ServiceClassDefinition // service class name -> definition
	toolChecker     ToolAvailabilityChecker
	exposedServices map[string]bool // Track which service classes are available

	// Callbacks for lifecycle events
	onRegister   []func(def *ServiceClassDefinition)
	onUnregister []func(serviceClassName string)
	onUpdate     []func(def *ServiceClassDefinition)
}

// NewServiceClassManager creates a new service class manager
func NewServiceClassManager(definitionsPath string, toolChecker ToolAvailabilityChecker) *ServiceClassManager {
	return &ServiceClassManager{
		definitionsPath: definitionsPath,
		definitions:     make(map[string]*ServiceClassDefinition),
		toolChecker:     toolChecker,
		exposedServices: make(map[string]bool),
		onRegister:      []func(def *ServiceClassDefinition){},
		onUnregister:    []func(serviceClassName string){},
		onUpdate:        []func(def *ServiceClassDefinition){},
	}
}

// LoadServiceDefinitions loads all service class definitions from the configured path
func (scm *ServiceClassManager) LoadServiceDefinitions() error {
	scm.mu.Lock()
	defer scm.mu.Unlock()

	// Clear existing definitions
	scm.definitions = make(map[string]*ServiceClassDefinition)
	scm.exposedServices = make(map[string]bool)

	// Create definitions path if it doesn't exist
	if err := os.MkdirAll(scm.definitionsPath, 0755); err != nil {
		return fmt.Errorf("failed to create definitions directory: %w", err)
	}

	// Load all YAML files from the definitions directory
	pattern := filepath.Join(scm.definitionsPath, "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list service class files: %w", err)
	}

	// Also check subdirectories (like examples/)
	subdirPattern := filepath.Join(scm.definitionsPath, "*", "*.yaml")
	subdirFiles, err := filepath.Glob(subdirPattern)
	if err == nil {
		files = append(files, subdirFiles...)
	}

	for _, file := range files {
		// Skip non-service class files based on naming convention
		if !scm.isServiceClassFile(file) {
			continue
		}

		def, err := scm.loadServiceDefinitionFile(file)
		if err != nil {
			logging.Error("ServiceClassManager", err, "Failed to load service class file: %s", file)
			continue
		}

		scm.definitions[def.Name] = def
		logging.Info("ServiceClassManager", "Loaded service class definition: %s (type: %s)", def.Name, def.Type)

		// Notify registration callbacks
		for _, callback := range scm.onRegister {
			callback(def)
		}
	}

	// Check which service classes are available based on tool availability
	scm.updateServiceAvailability()

	return nil
}

// isServiceClassFile determines if a file is a service class definition
func (scm *ServiceClassManager) isServiceClassFile(filename string) bool {
	// Service class files should have "service_" prefix or be in service-related directories
	basename := filepath.Base(filename)
	return strings.HasPrefix(basename, "service_") ||
		strings.Contains(filename, "/service") ||
		strings.Contains(filename, "examples") ||
		strings.Contains(basename, "provider")
}

// loadServiceDefinitionFile loads a single service class definition file
func (scm *ServiceClassManager) loadServiceDefinitionFile(filename string) (*ServiceClassDefinition, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var def ServiceClassDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate the service class definition
	if err := scm.validateServiceClassDefinition(&def); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &def, nil
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

// GetDefinitionsPath returns the path where service class definitions are loaded from
func (scm *ServiceClassManager) GetDefinitionsPath() string {
	return scm.definitionsPath
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
