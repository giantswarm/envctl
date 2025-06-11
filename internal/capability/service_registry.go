package capability

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// ServiceCapabilityRegistry manages service capability definitions and their availability
type ServiceCapabilityRegistry struct {
	mu              sync.RWMutex
	definitionsPath string
	definitions     map[string]*ServiceCapabilityDefinition // capability name -> definition
	toolChecker     ToolAvailabilityChecker
	exposedServices map[string]bool // Track which service capabilities are available

	// Callbacks for lifecycle events
	onRegister   []func(def *ServiceCapabilityDefinition)
	onUnregister []func(capabilityName string)
	onUpdate     []func(def *ServiceCapabilityDefinition)
}

// ServiceCapabilityInfo provides information about a registered service capability
type ServiceCapabilityInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	Description string `json:"description"`
	ServiceType string `json:"serviceType"`
	Available   bool   `json:"available"`

	// Lifecycle tool availability
	CreateToolAvailable      bool `json:"createToolAvailable"`
	DeleteToolAvailable      bool `json:"deleteToolAvailable"`
	HealthCheckToolAvailable bool `json:"healthCheckToolAvailable"`
	StatusToolAvailable      bool `json:"statusToolAvailable"`

	// Required tools
	RequiredTools []string `json:"requiredTools"`
	MissingTools  []string `json:"missingTools"`
}

// NewServiceCapabilityRegistry creates a new service capability registry
func NewServiceCapabilityRegistry(definitionsPath string, toolChecker ToolAvailabilityChecker) *ServiceCapabilityRegistry {
	return &ServiceCapabilityRegistry{
		definitionsPath: definitionsPath,
		definitions:     make(map[string]*ServiceCapabilityDefinition),
		toolChecker:     toolChecker,
		exposedServices: make(map[string]bool),
		onRegister:      []func(def *ServiceCapabilityDefinition){},
		onUnregister:    []func(capabilityName string){},
		onUpdate:        []func(def *ServiceCapabilityDefinition){},
	}
}

// LoadServiceDefinitions loads all service capability definitions from the configured path
func (scr *ServiceCapabilityRegistry) LoadServiceDefinitions() error {
	scr.mu.Lock()
	defer scr.mu.Unlock()

	// Clear existing definitions
	scr.definitions = make(map[string]*ServiceCapabilityDefinition)
	scr.exposedServices = make(map[string]bool)

	// Create definitions path if it doesn't exist
	if err := os.MkdirAll(scr.definitionsPath, 0755); err != nil {
		return fmt.Errorf("failed to create definitions directory: %w", err)
	}

	// Load all YAML files from the definitions directory
	pattern := filepath.Join(scr.definitionsPath, "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to list service capability files: %w", err)
	}

	// Also check subdirectories (like examples/)
	subdirPattern := filepath.Join(scr.definitionsPath, "*", "*.yaml")
	subdirFiles, err := filepath.Glob(subdirPattern)
	if err == nil {
		files = append(files, subdirFiles...)
	}

	for _, file := range files {
		// Skip non-service capability files based on naming convention
		if !scr.isServiceCapabilityFile(file) {
			continue
		}

		def, err := scr.loadServiceDefinitionFile(file)
		if err != nil {
			logging.Error("ServiceCapabilityRegistry", err, "Failed to load service capability file: %s", file)
			continue
		}

		scr.definitions[def.Name] = def
		logging.Info("ServiceCapabilityRegistry", "Loaded service capability definition: %s (type: %s)", def.Name, def.Type)

		// Notify registration callbacks
		for _, callback := range scr.onRegister {
			callback(def)
		}
	}

	// Check which service capabilities are available based on tool availability
	scr.updateServiceAvailability()

	return nil
}

// isServiceCapabilityFile determines if a file is a service capability definition
func (scr *ServiceCapabilityRegistry) isServiceCapabilityFile(filename string) bool {
	// Service capability files should have "service_" prefix or be in service-related directories
	basename := filepath.Base(filename)
	return strings.HasPrefix(basename, "service_") || strings.Contains(filename, "/service") || strings.Contains(filename, "examples")
}

// loadServiceDefinitionFile loads a single service capability definition file
func (scr *ServiceCapabilityRegistry) loadServiceDefinitionFile(filename string) (*ServiceCapabilityDefinition, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var def ServiceCapabilityDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate the service capability definition using the validation system from 43.1
	if err := ValidateServiceCapabilityDefinition(&def); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &def, nil
}

// updateServiceAvailability checks tool availability and updates service capability availability
func (scr *ServiceCapabilityRegistry) updateServiceAvailability() {
	for name, def := range scr.definitions {
		requiredTools := scr.getRequiredTools(def)
		available := scr.areAllToolsAvailable(requiredTools)

		oldAvailable := scr.exposedServices[name]
		scr.exposedServices[name] = available

		if available && !oldAvailable {
			logging.Info("ServiceCapabilityRegistry", "Service capability available: %s", name)
		} else if !available && oldAvailable {
			logging.Warn("ServiceCapabilityRegistry", "Service capability unavailable: %s (missing tools)", name)
		}
	}
}

// getRequiredTools extracts all required tools from a service capability definition
func (scr *ServiceCapabilityRegistry) getRequiredTools(def *ServiceCapabilityDefinition) []string {
	tools := make(map[string]bool)

	// Add lifecycle tools
	if def.ServiceConfig.LifecycleTools.Create.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Create.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.Delete.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Delete.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.HealthCheck != nil && def.ServiceConfig.LifecycleTools.HealthCheck.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.HealthCheck.Tool] = true
	}
	if def.ServiceConfig.LifecycleTools.Status != nil && def.ServiceConfig.LifecycleTools.Status.Tool != "" {
		tools[def.ServiceConfig.LifecycleTools.Status.Tool] = true
	}

	// Add tools from operations (existing capability system)
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
func (scr *ServiceCapabilityRegistry) areAllToolsAvailable(requiredTools []string) bool {
	for _, tool := range requiredTools {
		if !scr.toolChecker.IsToolAvailable(tool) {
			return false
		}
	}
	return true
}

// GetServiceCapabilityDefinition returns a service capability definition by name
func (scr *ServiceCapabilityRegistry) GetServiceCapabilityDefinition(name string) (*ServiceCapabilityDefinition, bool) {
	scr.mu.RLock()
	defer scr.mu.RUnlock()

	def, exists := scr.definitions[name]
	return def, exists
}

// ListServiceCapabilities returns information about all registered service capabilities
func (scr *ServiceCapabilityRegistry) ListServiceCapabilities() []ServiceCapabilityInfo {
	scr.mu.RLock()
	defer scr.mu.RUnlock()

	result := make([]ServiceCapabilityInfo, 0, len(scr.definitions))

	for _, def := range scr.definitions {
		requiredTools := scr.getRequiredTools(def)
		missingTools := scr.getMissingTools(requiredTools)
		available := len(missingTools) == 0

		info := ServiceCapabilityInfo{
			Name:                     def.Name,
			Type:                     def.Type,
			Version:                  def.Version,
			Description:              def.Description,
			ServiceType:              def.ServiceConfig.ServiceType,
			Available:                available,
			CreateToolAvailable:      scr.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.Create.Tool),
			DeleteToolAvailable:      scr.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.Delete.Tool),
			HealthCheckToolAvailable: def.ServiceConfig.LifecycleTools.HealthCheck != nil && scr.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.HealthCheck.Tool),
			StatusToolAvailable:      def.ServiceConfig.LifecycleTools.Status != nil && scr.toolChecker.IsToolAvailable(def.ServiceConfig.LifecycleTools.Status.Tool),
			RequiredTools:            requiredTools,
			MissingTools:             missingTools,
		}

		result = append(result, info)
	}

	return result
}

// ListAvailableServiceCapabilities returns only service capabilities that have all required tools available
func (scr *ServiceCapabilityRegistry) ListAvailableServiceCapabilities() []ServiceCapabilityInfo {
	all := scr.ListServiceCapabilities()
	result := make([]ServiceCapabilityInfo, 0, len(all))

	for _, info := range all {
		if info.Available {
			result = append(result, info)
		}
	}

	return result
}

// getMissingTools returns tools that are required but not available
func (scr *ServiceCapabilityRegistry) getMissingTools(requiredTools []string) []string {
	var missing []string
	for _, tool := range requiredTools {
		if !scr.toolChecker.IsToolAvailable(tool) {
			missing = append(missing, tool)
		}
	}
	return missing
}

// IsServiceCapabilityAvailable checks if a service capability is available
func (scr *ServiceCapabilityRegistry) IsServiceCapabilityAvailable(name string) bool {
	scr.mu.RLock()
	defer scr.mu.RUnlock()

	return scr.exposedServices[name]
}

// RefreshAvailability refreshes the availability status of all service capabilities
func (scr *ServiceCapabilityRegistry) RefreshAvailability() {
	scr.mu.Lock()
	defer scr.mu.Unlock()

	scr.updateServiceAvailability()
}

// RegisterDefinition registers a service capability definition programmatically
func (scr *ServiceCapabilityRegistry) RegisterDefinition(def *ServiceCapabilityDefinition) error {
	scr.mu.Lock()
	defer scr.mu.Unlock()

	// Validate the definition
	if err := ValidateServiceCapabilityDefinition(def); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if already registered
	if _, exists := scr.definitions[def.Name]; exists {
		return fmt.Errorf("service capability %s already registered", def.Name)
	}

	// Register the definition
	scr.definitions[def.Name] = def

	// Update availability
	requiredTools := scr.getRequiredTools(def)
	scr.exposedServices[def.Name] = scr.areAllToolsAvailable(requiredTools)

	logging.Info("ServiceCapabilityRegistry", "Registered service capability: %s (type: %s)", def.Name, def.Type)

	// Notify registration callbacks
	for _, callback := range scr.onRegister {
		callback(def)
	}

	return nil
}

// UnregisterDefinition unregisters a service capability definition
func (scr *ServiceCapabilityRegistry) UnregisterDefinition(name string) error {
	scr.mu.Lock()
	defer scr.mu.Unlock()

	if _, exists := scr.definitions[name]; !exists {
		return fmt.Errorf("service capability %s not found", name)
	}

	delete(scr.definitions, name)
	delete(scr.exposedServices, name)

	logging.Info("ServiceCapabilityRegistry", "Unregistered service capability: %s", name)

	// Notify unregistration callbacks
	for _, callback := range scr.onUnregister {
		callback(name)
	}

	return nil
}

// OnRegister adds a callback for service capability registration
func (scr *ServiceCapabilityRegistry) OnRegister(callback func(def *ServiceCapabilityDefinition)) {
	scr.mu.Lock()
	defer scr.mu.Unlock()
	scr.onRegister = append(scr.onRegister, callback)
}

// OnUnregister adds a callback for service capability removal
func (scr *ServiceCapabilityRegistry) OnUnregister(callback func(capabilityName string)) {
	scr.mu.Lock()
	defer scr.mu.Unlock()
	scr.onUnregister = append(scr.onUnregister, callback)
}

// OnUpdate adds a callback for service capability updates
func (scr *ServiceCapabilityRegistry) OnUpdate(callback func(def *ServiceCapabilityDefinition)) {
	scr.mu.Lock()
	defer scr.mu.Unlock()
	scr.onUpdate = append(scr.onUpdate, callback)
}

// GetDefinitionsPath returns the path where service capability definitions are loaded from
func (scr *ServiceCapabilityRegistry) GetDefinitionsPath() string {
	return scr.definitionsPath
}
