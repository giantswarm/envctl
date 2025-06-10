package capability

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// CapabilityDefinition represents a capability definition from YAML
type CapabilityDefinition struct {
	Name        string                         `yaml:"name"`
	Type        string                         `yaml:"type"`
	Version     string                         `yaml:"version"`
	Description string                         `yaml:"description"`
	Operations  map[string]OperationDefinition `yaml:"operations"`
	Metadata    map[string]string              `yaml:"metadata"`
}

// OperationDefinition defines a single capability operation
type OperationDefinition struct {
	Description string                 `yaml:"description"`
	Workflow    interface{}            `yaml:"workflow"` // Can be string (name) or WorkflowDefinition (embedded)
	Parameters  map[string]ParamSchema `yaml:"parameters"`
	Requires    []string               `yaml:"requires"` // Required tool names
}

// WorkflowDefinition represents an embedded workflow in a capability
type WorkflowDefinition struct {
	Name            string                 `yaml:"name"`
	Description     string                 `yaml:"description"`
	AgentModifiable bool                   `yaml:"agentModifiable"`
	InputSchema     map[string]interface{} `yaml:"inputSchema"`
	Steps           []WorkflowStep         `yaml:"steps"`
}

// WorkflowStep represents a step in an embedded workflow
type WorkflowStep struct {
	ID    string                 `yaml:"id"`
	Tool  string                 `yaml:"tool"`
	Args  map[string]interface{} `yaml:"args"`
	Store string                 `yaml:"store"`
}

// ParamSchema defines a parameter schema
type ParamSchema struct {
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

// CapabilityLoader loads capability definitions from YAML files
type CapabilityLoader struct {
	mu              sync.RWMutex
	definitionsPath string
	definitions     map[string]*CapabilityDefinition // capability name -> definition
	toolChecker     ToolAvailabilityChecker
	registry        *Registry
	exposedTools    map[string]bool // Track which capability tools we've exposed
}

// ToolAvailabilityChecker checks if tools are available
type ToolAvailabilityChecker interface {
	IsToolAvailable(toolName string) bool
	GetAvailableTools() []string
}

// NewCapabilityLoader creates a new capability loader
func NewCapabilityLoader(definitionsPath string, toolChecker ToolAvailabilityChecker, registry *Registry) *CapabilityLoader {
	return &CapabilityLoader{
		definitionsPath: definitionsPath,
		definitions:     make(map[string]*CapabilityDefinition),
		toolChecker:     toolChecker,
		registry:        registry,
		exposedTools:    make(map[string]bool),
	}
}

// LoadDefinitions loads all capability definitions from the configured path
func (cl *CapabilityLoader) LoadDefinitions() error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Clear existing definitions
	cl.definitions = make(map[string]*CapabilityDefinition)

	// Load all YAML files from the definitions directory
	files, err := filepath.Glob(filepath.Join(cl.definitionsPath, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to list capability files: %w", err)
	}

	for _, file := range files {
		def, err := cl.loadDefinitionFile(file)
		if err != nil {
			logging.Error("CapabilityLoader", err, "Failed to load capability file: %s", file)
			continue
		}

		cl.definitions[def.Name] = def
		logging.Info("CapabilityLoader", "Loaded capability definition: %s (type: %s)", def.Name, def.Type)
	}

	// Check which capabilities can be exposed
	cl.updateAvailableCapabilities()

	return nil
}

// loadDefinitionFile loads a single capability definition file
func (cl *CapabilityLoader) loadDefinitionFile(filename string) (*CapabilityDefinition, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var def CapabilityDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate the definition
	if def.Name == "" {
		return nil, fmt.Errorf("capability name is required")
	}
	if def.Type == "" {
		return nil, fmt.Errorf("capability type is required")
	}
	if len(def.Operations) == 0 {
		return nil, fmt.Errorf("at least one operation is required")
	}

	return &def, nil
}

// updateAvailableCapabilities checks tool availability and updates exposed capabilities
func (cl *CapabilityLoader) updateAvailableCapabilities() {
	for _, def := range cl.definitions {
		// Check each operation
		for opName, op := range def.Operations {
			if cl.areRequiredToolsAvailable(op.Requires) {
				// Mark this capability operation as available
				toolName := fmt.Sprintf("x_%s_%s", def.Type, opName)
				if !cl.exposedTools[toolName] {
					cl.exposedTools[toolName] = true
					logging.Info("CapabilityLoader", "Capability operation available: %s", toolName)
				}
			} else {
				// Mark as unavailable
				toolName := fmt.Sprintf("x_%s_%s", def.Type, opName)
				if cl.exposedTools[toolName] {
					delete(cl.exposedTools, toolName)
					logging.Info("CapabilityLoader", "Capability operation unavailable: %s (missing tools)", toolName)
				}
			}
		}
	}
}

// areRequiredToolsAvailable checks if all required tools are available
func (cl *CapabilityLoader) areRequiredToolsAvailable(requiredTools []string) bool {
	for _, tool := range requiredTools {
		if !cl.toolChecker.IsToolAvailable(tool) {
			return false
		}
	}
	return true
}

// GetAvailableCapabilityTools returns all capability tools that can be exposed
func (cl *CapabilityLoader) GetAvailableCapabilityTools() []string {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	tools := make([]string, 0, len(cl.exposedTools))
	for tool := range cl.exposedTools {
		tools = append(tools, tool)
	}
	return tools
}

// GetCapabilityDefinition returns a capability definition by name
func (cl *CapabilityLoader) GetCapabilityDefinition(name string) (*CapabilityDefinition, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	def, exists := cl.definitions[name]
	return def, exists
}

// GetOperationForTool returns the operation definition for a capability tool
func (cl *CapabilityLoader) GetOperationForTool(toolName string) (*OperationDefinition, *CapabilityDefinition, error) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	// Tool names are in format x_<type>_<operation>
	// e.g., x_auth_provider_login -> auth_provider type, login operation

	for _, def := range cl.definitions {
		for opName, op := range def.Operations {
			expectedTool := fmt.Sprintf("x_%s_%s", def.Type, opName)
			if expectedTool == toolName {
				return &op, def, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no operation found for tool %s", toolName)
}

// RefreshAvailability is called when tool availability might have changed
func (cl *CapabilityLoader) RefreshAvailability() {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	cl.updateAvailableCapabilities()
}
