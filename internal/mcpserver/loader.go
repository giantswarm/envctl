package mcpserver

import (
	"fmt"

	"envctl/internal/config"
	"envctl/pkg/logging"
)

// LoadMCPServerDefinitions loads MCP server definitions from YAML files in the mcpservers/ directory
// with layered user/project override support. It utilizes the ConfigurationLoader to ensure
// consistency with other configuration types (ServiceClass, Capability, Workflow).
//
// The function loads all YAML files from:
// - User directory: ~/.config/envctl/mcpservers/
// - Project directory: ./.envctl/mcpservers/
//
// Project files override user files with the same base name, ensuring proper precedence.
//
// Returns:
// - []MCPServerDefinition: Successfully loaded and validated definitions
// - *config.ConfigurationErrorCollection: Detailed error information for troubleshooting
// - error: Critical errors that prevent loading
func LoadMCPServerDefinitions() ([]MCPServerDefinition, *config.ConfigurationErrorCollection, error) {
	logging.Info("MCPServerLoader", "Loading MCP server definitions from mcpservers/ directory")

	// Load and parse YAML files with enhanced error handling
	definitions, errorCollection, err := config.LoadAndParseYAML[MCPServerDefinition]("mcpservers", validateMCPServerDefinition)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load MCP server definitions: %w", err)
	}

	// Log loading summary
	successCount := len(definitions)
	errorCount := errorCollection.Count()

	if errorCount > 0 {
		logging.Warn("MCPServerLoader", "Loaded %d MCP server definitions with %d errors", successCount, errorCount)

		// Log detailed error summary for troubleshooting
		logging.Warn("MCPServerLoader", "MCP server configuration errors:\n%s", errorCollection.GetSummary())

		// Log full error details for debugging
		logging.Debug("MCPServerLoader", "Detailed error report:\n%s", errorCollection.GetDetailedReport())
	} else {
		logging.Info("MCPServerLoader", "Successfully loaded %d MCP server definitions", successCount)
	}

	return definitions, errorCollection, nil
}

// LoadMCPServerDefinitionsLegacy provides backward compatibility by returning only the first error
// for systems that expect the old error handling behavior.
//
// Deprecated: Use LoadMCPServerDefinitions for enhanced error handling and graceful degradation.
func LoadMCPServerDefinitionsLegacy() ([]MCPServerDefinition, error) {
	definitions, errorCollection, err := LoadMCPServerDefinitions()
	if err != nil {
		return nil, err
	}

	// For backward compatibility, return the first error if any exist
	if errorCollection.HasErrors() {
		logging.Warn("MCPServerLoader", "MCP server loading completed with errors (legacy mode)")
		return definitions, errorCollection.Errors[0]
	}

	return definitions, nil
}

// validateMCPServerDefinition performs validation on an MCP server definition
// This function is used by the configuration loader to ensure definitions are valid
func validateMCPServerDefinition(def MCPServerDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("MCP server name cannot be empty")
	}

	// Validate type
	if def.Type != MCPServerTypeLocalCommand && def.Type != MCPServerTypeContainer && def.Type != MCPServerTypeMock {
		return fmt.Errorf("field 'type': must be one of: %s, %s, %s",
			MCPServerTypeLocalCommand, MCPServerTypeContainer, MCPServerTypeMock)
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

// GetMCPServerConfigurationPaths returns the paths where MCP server definitions are loaded from
// This is a utility function for debugging and configuration management
func GetMCPServerConfigurationPaths() (userPath, projectPath string, err error) {
	userDir, projectDir, err := config.GetConfigurationPaths()
	if err != nil {
		return "", "", fmt.Errorf("failed to get configuration paths: %w", err)
	}

	userPath = fmt.Sprintf("%s/mcpservers", userDir)
	projectPath = fmt.Sprintf("%s/mcpservers", projectDir)

	return userPath, projectPath, nil
}
