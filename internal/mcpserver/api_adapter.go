package mcpserver

import (
	"context"
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Adapter implements the api.MCPServerManagerHandler interface
// This adapter bridges the MCPServerManager implementation with the central API layer
type Adapter struct {
	manager *MCPServerManager
}

// NewAdapter creates a new API adapter for the MCPServerManager
func NewAdapter(manager *MCPServerManager) *Adapter {
	return &Adapter{
		manager: manager,
	}
}

// Register registers this adapter with the central API layer
// This method follows the project's API Service Locator Pattern
func (a *Adapter) Register() {
	api.RegisterMCPServerManager(a)
	logging.Info("MCPServerAdapter", "Registered MCP server manager with API layer")
}

// ListMCPServers returns information about all registered MCP servers
func (a *Adapter) ListMCPServers() []api.MCPServerConfigInfo {
	if a.manager == nil {
		return []api.MCPServerConfigInfo{}
	}

	// Get MCP server info from the manager
	managerInfo := a.manager.ListDefinitions()

	// Convert to API types
	result := make([]api.MCPServerConfigInfo, len(managerInfo))
	for i, def := range managerInfo {
		available := a.manager.IsAvailable(def.Name)
		result[i] = api.MCPServerConfigInfo{
			Name:        def.Name,
			Type:        string(def.Type),
			Enabled:     def.Enabled,
			Category:    def.Category,
			Icon:        def.Icon,
			Available:   available,
			Description: "", // MCPServerDefinition doesn't have Description field
			Command:     def.Command,
			Image:       def.Image,
		}
	}

	return result
}

// GetMCPServer returns a specific MCP server definition by name
func (a *Adapter) GetMCPServer(name string) (*api.MCPServerDefinition, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("MCP server manager not available")
	}

	def, exists := a.manager.GetDefinition(name)
	if !exists {
		return nil, fmt.Errorf("MCP server %s not found", name)
	}

	// Convert to API type (lightweight version)
	apiDef := &api.MCPServerDefinition{
		Name:        def.Name,
		Type:        string(def.Type),
		Enabled:     def.Enabled,
		Category:    def.Category,
		Icon:        def.Icon,
		Description: "", // MCPServerDefinition doesn't have Description field
		Command:     def.Command,
		Image:       def.Image,
		Env:         def.Env,
	}

	return apiDef, nil
}

// IsMCPServerAvailable checks if an MCP server is available
func (a *Adapter) IsMCPServerAvailable(name string) bool {
	if a.manager == nil {
		return false
	}

	return a.manager.IsAvailable(name)
}

// LoadDefinitions loads all MCP server definitions from the configured path
func (a *Adapter) LoadDefinitions() error {
	if a.manager == nil {
		return fmt.Errorf("MCP server manager not available")
	}

	return a.manager.LoadDefinitions()
}

// RefreshAvailability refreshes the availability status of all MCP servers
func (a *Adapter) RefreshAvailability() {
	if a.manager == nil {
		logging.Warn("MCPServerAdapter", "Cannot refresh availability: manager not available")
		return
	}

	a.manager.RefreshAvailability()
}

// RegisterDefinition registers an MCP server definition programmatically
func (a *Adapter) RegisterDefinition(apiDef *api.MCPServerDefinition) error {
	if a.manager == nil {
		return fmt.Errorf("MCP server manager not available")
	}

	// TODO: Convert from API type to internal type and use RegisterDefinition once it's implemented
	return fmt.Errorf("RegisterDefinition not yet implemented")
}

// UnregisterDefinition unregisters an MCP server definition
func (a *Adapter) UnregisterDefinition(name string) error {
	if a.manager == nil {
		return fmt.Errorf("MCP server manager not available")
	}

	// TODO: Add UnregisterDefinition method to MCPServerManager
	return fmt.Errorf("UnregisterDefinition not yet implemented")
}

// GetDefinitionsPath returns the path where MCP server definitions are loaded from
func (a *Adapter) GetDefinitionsPath() string {
	if a.manager == nil {
		return ""
	}

	return a.manager.GetDefinitionsPath()
}

// GetManager returns the underlying MCPServerManager (for internal use)
// This should only be used by other internal packages that need direct access
func (a *Adapter) GetManager() *MCPServerManager {
	return a.manager
}

// ToolProvider implementation

// GetTools returns all tools this provider offers
func (a *Adapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		{
			Name:        "mcpserver_list",
			Description: "List all MCP server definitions with their availability status",
		},
		{
			Name:        "mcpserver_get",
			Description: "Get detailed information about a specific MCP server definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the MCP server to retrieve",
				},
			},
		},
		{
			Name:        "mcpserver_available",
			Description: "Check if an MCP server is available",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Name of the MCP server to check",
				},
			},
		},
		{
			Name:        "mcpserver_refresh",
			Description: "Refresh the availability status of all MCP server definitions",
		},
		{
			Name:        "mcpserver_load",
			Description: "Load MCP server definitions from the configured directory",
		},
		{
			Name:        "mcpserver_definitions_path",
			Description: "Get the path where MCP server definitions are loaded from",
		},
		{
			Name:        "mcpserver_register",
			Description: "Register an MCP server from YAML on the fly",
			Parameters: []api.ParameterMetadata{
				{Name: "yaml", Type: "string", Required: true, Description: "Full MCP server YAML definition"},
				{Name: "merge", Type: "boolean", Required: false, Description: "If true, replace existing MCP server of the same name"},
			},
		},
		{
			Name:        "mcpserver_unregister",
			Description: "Unregister an MCP server by name",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the MCP server to remove"},
			},
		},
		{
			Name:        "mcpserver_create",
			Description: "Create a new dynamic MCP server",
			Parameters: []api.ParameterMetadata{
				{Name: "yaml", Type: "string", Required: true, Description: "Full MCP server YAML definition"},
			},
		},
		{
			Name:        "mcpserver_update",
			Description: "Update an existing MCP server",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the MCP server to update"},
				{Name: "yaml", Type: "string", Required: true, Description: "Updated MCP server YAML definition"},
			},
		},
		{
			Name:        "mcpserver_delete",
			Description: "Delete an MCP server",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the MCP server to delete"},
			},
		},
	}
}

// ExecuteTool executes a tool by name
func (a *Adapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	case "mcpserver_list":
		return a.handleMCPServerList()
	case "mcpserver_get":
		return a.handleMCPServerGet(args)
	case "mcpserver_available":
		return a.handleMCPServerAvailable(args)
	case "mcpserver_refresh":
		return a.handleMCPServerRefresh()
	case "mcpserver_load":
		return a.handleMCPServerLoad()
	case "mcpserver_definitions_path":
		return a.handleMCPServerDefinitionsPath()
	case "mcpserver_register":
		return a.handleMCPServerRegister(args)
	case "mcpserver_unregister":
		return a.handleMCPServerUnregister(args)
	case "mcpserver_create":
		return a.handleMCPServerCreate(args)
	case "mcpserver_update":
		return a.handleMCPServerUpdate(args)
	case "mcpserver_delete":
		return a.handleMCPServerDelete(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Tool handlers

func (a *Adapter) handleMCPServerList() (*api.CallToolResult, error) {
	mcpServers := a.ListMCPServers()

	result := map[string]interface{}{
		"mcpServers": mcpServers,
		"total":      len(mcpServers),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleMCPServerGet(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	mcpServer, err := a.GetMCPServer(name)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get MCP server: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{mcpServer},
		IsError: false,
	}, nil
}

func (a *Adapter) handleMCPServerAvailable(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name parameter is required"},
			IsError: true,
		}, nil
	}

	available := a.IsMCPServerAvailable(name)

	result := map[string]interface{}{
		"name":      name,
		"available": available,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *Adapter) handleMCPServerRefresh() (*api.CallToolResult, error) {
	a.RefreshAvailability()

	return &api.CallToolResult{
		Content: []interface{}{"MCP server availability refreshed successfully"},
		IsError: false,
	}, nil
}

func (a *Adapter) handleMCPServerLoad() (*api.CallToolResult, error) {
	err := a.LoadDefinitions()
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to load MCP server definitions: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"MCP server definitions loaded successfully"},
		IsError: false,
	}, nil
}

func (a *Adapter) handleMCPServerDefinitionsPath() (*api.CallToolResult, error) {
	path := a.GetDefinitionsPath()

	result := map[string]interface{}{
		"definitionsPath": path,
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

// helper to create simple error CallToolResult
func simpleError(msg string) (*api.CallToolResult, error) {
	return &api.CallToolResult{Content: []interface{}{msg}, IsError: true}, nil
}

func simpleOK(msg string) (*api.CallToolResult, error) {
	return &api.CallToolResult{Content: []interface{}{msg}, IsError: false}, nil
}

func (a *Adapter) handleMCPServerRegister(args map[string]interface{}) (*api.CallToolResult, error) {
	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return simpleError("yaml parameter is required")
	}

	merge, _ := args["merge"].(bool)

	// Parse the YAML
	var def MCPServerDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return simpleError(fmt.Sprintf("Invalid YAML: %v", err))
	}

	// Validate the definition
	if err := validateMCPServerDefinition(def); err != nil {
		return simpleError(fmt.Sprintf("Invalid MCP server definition: %v", err))
	}

	// Check if it already exists and merge flag
	if _, exists := a.manager.GetDefinition(def.Name); exists && !merge {
		return simpleError(fmt.Sprintf("MCP server '%s' already exists. Use merge=true to replace.", def.Name))
	} else if exists && merge {
		logging.Info("MCPServerAdapter", "Replacing existing MCP server '%s'", def.Name)
		if err := a.manager.UpdateMCPServer(def.Name, def); err != nil {
			return simpleError(fmt.Sprintf("Failed to update MCP server: %v", err))
		}
	} else {
		if err := a.manager.CreateMCPServer(def); err != nil {
			return simpleError(fmt.Sprintf("Failed to create MCP server: %v", err))
		}
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' registered successfully", def.Name))
}

func (a *Adapter) handleMCPServerUnregister(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}

	// Check if it exists
	if _, exists := a.manager.GetDefinition(name); !exists {
		return simpleError(fmt.Sprintf("MCP server '%s' not found", name))
	}

	if err := a.manager.DeleteMCPServer(name); err != nil {
		return simpleError(fmt.Sprintf("Failed to delete MCP server: %v", err))
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' unregistered successfully", name))
}

func (a *Adapter) handleMCPServerCreate(args map[string]interface{}) (*api.CallToolResult, error) {
	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return simpleError("yaml parameter is required")
	}

	// Parse the YAML
	var def MCPServerDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return simpleError(fmt.Sprintf("Invalid YAML: %v", err))
	}

	// Validate the definition
	if err := validateMCPServerDefinition(def); err != nil {
		return simpleError(fmt.Sprintf("Invalid MCP server definition: %v", err))
	}

	// Check if it already exists
	if _, exists := a.manager.GetDefinition(def.Name); exists {
		return simpleError(fmt.Sprintf("MCP server '%s' already exists", def.Name))
	}

	// Create the new MCP server
	if err := a.manager.CreateMCPServer(def); err != nil {
		return simpleError(fmt.Sprintf("Failed to create MCP server: %v", err))
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' created successfully", def.Name))
}

func (a *Adapter) handleMCPServerUpdate(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}

	yamlStr, ok := args["yaml"].(string)
	if !ok || yamlStr == "" {
		return simpleError("yaml parameter is required")
	}

	// Parse the YAML
	var def MCPServerDefinition
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return simpleError(fmt.Sprintf("Invalid YAML: %v", err))
	}

	// Validate the definition
	if err := validateMCPServerDefinition(def); err != nil {
		return simpleError(fmt.Sprintf("Invalid MCP server definition: %v", err))
	}

	// Check if it exists
	if _, exists := a.manager.GetDefinition(name); !exists {
		return simpleError(fmt.Sprintf("MCP server '%s' not found", name))
	}

	// Update the MCP server
	if err := a.manager.UpdateMCPServer(name, def); err != nil {
		return simpleError(fmt.Sprintf("Failed to update MCP server: %v", err))
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' updated successfully", name))
}

func (a *Adapter) handleMCPServerDelete(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}

	// Check if it exists
	if _, exists := a.manager.GetDefinition(name); !exists {
		return simpleError(fmt.Sprintf("MCP server '%s' not found", name))
	}

	// Delete the MCP server
	if err := a.manager.DeleteMCPServer(name); err != nil {
		return simpleError(fmt.Sprintf("Failed to delete MCP server: %v", err))
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' deleted successfully", name))
}
