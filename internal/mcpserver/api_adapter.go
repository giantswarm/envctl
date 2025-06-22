package mcpserver

import (
	"context"
	"fmt"
	"time"

	"envctl/internal/api"
	"envctl/pkg/logging"
)

// Adapter implements api.MCPServerManagerHandler for the MCP server manager
type Adapter struct {
	manager *MCPServerManager
}

// NewAdapter creates a new MCP server API adapter
func NewAdapter(manager *MCPServerManager) *Adapter {
	return &Adapter{
		manager: manager,
	}
}

// Register registers this adapter with the API layer
func (a *Adapter) Register() {
	api.RegisterMCPServerManager(a)
	logging.Debug("MCPServerAdapter", "Registered MCP server adapter with API layer")
}

// ListMCPServers returns all MCP server definitions with availability status
func (a *Adapter) ListMCPServers() []api.MCPServerInfo {
	if a.manager == nil {
		return []api.MCPServerInfo{}
	}

	definitions := a.manager.ListDefinitions()
	result := make([]api.MCPServerInfo, len(definitions))

	for i, def := range definitions {
		result[i] = api.MCPServerInfo{
			Name:        def.Name,
			Type:        string(def.Type),
			State:       string(def.State),
			Health:      string(def.Health),
			AutoStart:   def.AutoStart,
			Available:   a.manager.IsAvailable(def.Name),
			Description: def.Description,
			Command:     def.Command,
			Image:       def.Image,
			Env:         def.Env,
			Error:       def.Error,
		}
	}

	return result
}

// GetMCPServer returns detailed information about a specific MCP server
func (a *Adapter) GetMCPServer(name string) (*api.MCPServerInfo, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("MCP server manager not available")
	}

	def, exists := a.manager.GetDefinition(name)
	if !exists {
		return nil, api.NewMCPServerNotFoundError(name)
	}

	return &api.MCPServerInfo{
		Name:        def.Name,
		Type:        string(def.Type),
		State:       string(def.State),
		Health:      string(def.Health),
		AutoStart:   def.AutoStart,
		Available:   a.manager.IsAvailable(def.Name),
		Description: def.Description,
		Command:     def.Command,
		Image:       def.Image,
		Env:         def.Env,
		Error:       def.Error,
	}, nil
}

// IsMCPServerAvailable checks if an MCP server is available
func (a *Adapter) IsMCPServerAvailable(name string) bool {
	if a.manager == nil {
		return false
	}

	return a.manager.IsAvailable(name)
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
			Name:        "mcpserver_validate",
			Description: "Validate an mcpserver definition",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "MCP server name"},
				{Name: "type", Type: "string", Required: true, Description: "MCP server type (localCommand or container)"},
				{Name: "autoStart", Type: "boolean", Required: false, Description: "Whether server should auto-start"},
				{Name: "command", Type: "array", Required: false, Description: "Command and arguments (for localCommand type)"},
				{Name: "image", Type: "string", Required: false, Description: "Container image (for container type)"},
				{Name: "env", Type: "object", Required: false, Description: "Environment variables"},
				{Name: "containerPorts", Type: "array", Required: false, Description: "Port mappings (for container type)"},
				{Name: "description", Type: "string", Required: false, Description: "MCP server description"},
			},
		},
		{
			Name:        "mcpserver_create",
			Description: "Create a new dynamic MCP server",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "MCP server name"},
				{Name: "type", Type: "string", Required: true, Description: "MCP server type (localCommand or container)"},
				{Name: "autoStart", Type: "boolean", Required: false, Description: "Whether server should auto-start"},
				{Name: "healthCheckInterval", Type: "string", Required: false, Description: "Health check interval duration"},
				{Name: "toolPrefix", Type: "string", Required: false, Description: "Custom tool prefix"},
				{Name: "command", Type: "array", Required: false, Description: "Command and arguments (for localCommand type)"},
				{Name: "env", Type: "object", Required: false, Description: "Environment variables (for localCommand type)"},
				{Name: "image", Type: "string", Required: false, Description: "Container image (for container type)"},
				{Name: "containerPorts", Type: "array", Required: false, Description: "Port mappings (for container type)"},
				{Name: "containerEnv", Type: "object", Required: false, Description: "Container environment variables"},
				{Name: "containerVolumes", Type: "array", Required: false, Description: "Volume mounts"},
				{Name: "healthCheckCmd", Type: "array", Required: false, Description: "Health check command"},
				{Name: "entrypoint", Type: "array", Required: false, Description: "Container entrypoint"},
				{Name: "containerUser", Type: "string", Required: false, Description: "Container user"},
			},
		},
		{
			Name:        "mcpserver_update",
			Description: "Update an existing MCP server",
			Parameters: []api.ParameterMetadata{
				{Name: "name", Type: "string", Required: true, Description: "Name of the MCP server to update"},
				{Name: "type", Type: "string", Required: true, Description: "MCP server type (localCommand or container)"},
				{Name: "autoStart", Type: "boolean", Required: false, Description: "Whether server should auto-start"},
				{Name: "healthCheckInterval", Type: "string", Required: false, Description: "Health check interval duration"},
				{Name: "toolPrefix", Type: "string", Required: false, Description: "Custom tool prefix"},
				{Name: "command", Type: "array", Required: false, Description: "Command and arguments (for localCommand type)"},
				{Name: "env", Type: "object", Required: false, Description: "Environment variables (for localCommand type)"},
				{Name: "image", Type: "string", Required: false, Description: "Container image (for container type)"},
				{Name: "containerPorts", Type: "array", Required: false, Description: "Port mappings (for container type)"},
				{Name: "containerEnv", Type: "object", Required: false, Description: "Container environment variables"},
				{Name: "containerVolumes", Type: "array", Required: false, Description: "Volume mounts"},
				{Name: "healthCheckCmd", Type: "array", Required: false, Description: "Health check command"},
				{Name: "entrypoint", Type: "array", Required: false, Description: "Container entrypoint"},
				{Name: "containerUser", Type: "string", Required: false, Description: "Container user"},
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
	case "mcpserver_validate":
		return a.handleMCPServerValidate(args)
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
		return api.HandleErrorWithPrefix(err, "Failed to get MCP server"), nil
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

// handleMCPServerValidate validates an mcpserver definition
func (a *Adapter) handleMCPServerValidate(args map[string]interface{}) (*api.CallToolResult, error) {
	var req api.MCPServerValidateRequest
	if err := api.ParseRequest(args, &req); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{err.Error()},
			IsError: true,
		}, nil
	}

	// Build internal api.MCPServer from structured parameters
	def := api.MCPServer{
		Name:                req.Name,
		Type:                api.MCPServerType(req.Type),
		AutoStart:           req.AutoStart,
		Image:               req.Image,
		Command:             req.Command,
		Env:                 req.Env,
		ContainerPorts:      req.ContainerPorts,
		HealthCheckInterval: 0, // Default for validation
	}

	// Validate without persisting
	if err := a.manager.ValidateDefinition(&def); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Validation failed: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Validation successful for mcpserver %s", def.Name)},
		IsError: false,
	}, nil
}

func (a *Adapter) handleMCPServerCreate(args map[string]interface{}) (*api.CallToolResult, error) {
	var req api.MCPServerCreateRequest
	if err := api.ParseRequest(args, &req); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{err.Error()},
			IsError: true,
		}, nil
	}

	// Convert typed request to api.MCPServer
	def, err := convertCreateRequestToMCPServer(req)
	if err != nil {
		return simpleError(err.Error())
	}

	// Validate the definition
	if err := a.manager.ValidateDefinition(&def); err != nil {
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
	var req api.MCPServerUpdateRequest
	if err := api.ParseRequest(args, &req); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{err.Error()},
			IsError: true,
		}, nil
	}

	// Convert typed request to api.MCPServer
	def, err := convertRequestToMCPServer(req)
	if err != nil {
		return simpleError(err.Error())
	}

	// Validate the definition
	if err := a.manager.ValidateDefinition(&def); err != nil {
		return simpleError(fmt.Sprintf("Invalid MCP server definition: %v", err))
	}

	// Check if it exists
	if _, exists := a.manager.GetDefinition(req.Name); !exists {
		return api.HandleErrorWithPrefix(api.NewMCPServerNotFoundError(req.Name), "Failed to update MCP server"), nil
	}

	// Update the MCP server
	if err := a.manager.UpdateMCPServer(req.Name, def); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to update MCP server"), nil
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' updated successfully", req.Name))
}

func (a *Adapter) handleMCPServerDelete(args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return simpleError("name parameter is required")
	}

	// Check if it exists
	if _, exists := a.manager.GetDefinition(name); !exists {
		return api.HandleErrorWithPrefix(api.NewMCPServerNotFoundError(name), "Failed to delete MCP server"), nil
	}

	// Delete the MCP server
	if err := a.manager.DeleteMCPServer(name); err != nil {
		return api.HandleErrorWithPrefix(err, "Failed to delete MCP server"), nil
	}

	return simpleOK(fmt.Sprintf("MCP server '%s' deleted successfully", name))
}

// helper to create simple error CallToolResult
func simpleError(msg string) (*api.CallToolResult, error) {
	return &api.CallToolResult{Content: []interface{}{msg}, IsError: true}, nil
}

func simpleOK(msg string) (*api.CallToolResult, error) {
	return &api.CallToolResult{Content: []interface{}{msg}, IsError: false}, nil
}

// convertToMCPServer converts structured parameters to api.MCPServer
func convertToMCPServer(args map[string]interface{}) (api.MCPServer, error) {
	var def api.MCPServer

	// Required fields
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return def, fmt.Errorf("name parameter is required")
	}
	def.Name = name

	serverType, ok := args["type"].(string)
	if !ok || serverType == "" {
		return def, fmt.Errorf("type parameter is required")
	}
	def.Type = api.MCPServerType(serverType)

	// Optional fields
	if autoStart, ok := args["autoStart"].(bool); ok {
		def.AutoStart = autoStart
	}
	// Note: icon and category parameters are no longer supported in Phase 3
	if toolPrefix, ok := args["toolPrefix"].(string); ok {
		def.ToolPrefix = toolPrefix
	}
	if image, ok := args["image"].(string); ok {
		def.Image = image
	}
	if containerUser, ok := args["containerUser"].(string); ok {
		def.ContainerUser = containerUser
	}

	// Convert healthCheckInterval string to time.Duration
	if healthCheckInterval, ok := args["healthCheckInterval"].(string); ok && healthCheckInterval != "" {
		duration, err := time.ParseDuration(healthCheckInterval)
		if err != nil {
			return def, fmt.Errorf("invalid healthCheckInterval: %v", err)
		}
		def.HealthCheckInterval = duration
	}

	// Convert command array
	if command, ok := args["command"].([]interface{}); ok {
		def.Command = make([]string, len(command))
		for i, cmd := range command {
			if cmdStr, ok := cmd.(string); ok {
				def.Command[i] = cmdStr
			} else {
				return def, fmt.Errorf("command element at index %d must be a string", i)
			}
		}
	}

	// Convert env map
	if env, ok := args["env"].(map[string]interface{}); ok {
		def.Env = make(map[string]string)
		for key, value := range env {
			if strValue, ok := value.(string); ok {
				def.Env[key] = strValue
			} else {
				return def, fmt.Errorf("env value for key '%s' must be a string", key)
			}
		}
	}

	// Convert containerPorts array
	if containerPorts, ok := args["containerPorts"].([]interface{}); ok {
		def.ContainerPorts = make([]string, len(containerPorts))
		for i, port := range containerPorts {
			if portStr, ok := port.(string); ok {
				def.ContainerPorts[i] = portStr
			} else {
				return def, fmt.Errorf("containerPorts element at index %d must be a string", i)
			}
		}
	}

	// Convert containerEnv map
	if containerEnv, ok := args["containerEnv"].(map[string]interface{}); ok {
		def.ContainerEnv = make(map[string]string)
		for key, value := range containerEnv {
			if strValue, ok := value.(string); ok {
				def.ContainerEnv[key] = strValue
			} else {
				return def, fmt.Errorf("containerEnv value for key '%s' must be a string", key)
			}
		}
	}

	// Convert containerVolumes array
	if containerVolumes, ok := args["containerVolumes"].([]interface{}); ok {
		def.ContainerVolumes = make([]string, len(containerVolumes))
		for i, volume := range containerVolumes {
			if volumeStr, ok := volume.(string); ok {
				def.ContainerVolumes[i] = volumeStr
			} else {
				return def, fmt.Errorf("containerVolumes element at index %d must be a string", i)
			}
		}
	}

	// Convert healthCheckCmd array
	if healthCheckCmd, ok := args["healthCheckCmd"].([]interface{}); ok {
		def.HealthCheckCmd = make([]string, len(healthCheckCmd))
		for i, cmd := range healthCheckCmd {
			if cmdStr, ok := cmd.(string); ok {
				def.HealthCheckCmd[i] = cmdStr
			} else {
				return def, fmt.Errorf("healthCheckCmd element at index %d must be a string", i)
			}
		}
	}

	// Convert entrypoint array
	if entrypoint, ok := args["entrypoint"].([]interface{}); ok {
		def.Entrypoint = make([]string, len(entrypoint))
		for i, entry := range entrypoint {
			if entryStr, ok := entry.(string); ok {
				def.Entrypoint[i] = entryStr
			} else {
				return def, fmt.Errorf("entrypoint element at index %d must be a string", i)
			}
		}
	}

	return def, nil
}

// convertRequestToMCPServer converts a typed request to api.MCPServer
func convertRequestToMCPServer(req api.MCPServerUpdateRequest) (api.MCPServer, error) {
	def := api.MCPServer{
		Name:             req.Name,
		Type:             api.MCPServerType(req.Type),
		AutoStart:        req.AutoStart,
		ToolPrefix:       req.ToolPrefix,
		Image:            req.Image,
		Command:          req.Command,
		Env:              req.Env,
		ContainerPorts:   req.ContainerPorts,
		ContainerEnv:     req.ContainerEnv,
		ContainerVolumes: req.ContainerVolumes,
		HealthCheckCmd:   req.HealthCheckCmd,
		Entrypoint:       req.Entrypoint,
		ContainerUser:    req.ContainerUser,
	}

	// Convert healthCheckInterval string to time.Duration
	if req.HealthCheckInterval != "" {
		duration, err := time.ParseDuration(req.HealthCheckInterval)
		if err != nil {
			return def, fmt.Errorf("invalid healthCheckInterval: %v", err)
		}
		def.HealthCheckInterval = duration
	}

	return def, nil
}

// convertCreateRequestToMCPServer converts a typed request to api.MCPServer
func convertCreateRequestToMCPServer(req api.MCPServerCreateRequest) (api.MCPServer, error) {
	def := api.MCPServer{
		Name:             req.Name,
		Type:             api.MCPServerType(req.Type),
		AutoStart:        req.AutoStart,
		ToolPrefix:       req.ToolPrefix,
		Image:            req.Image,
		Command:          req.Command,
		Env:              req.Env,
		ContainerPorts:   req.ContainerPorts,
		ContainerEnv:     req.ContainerEnv,
		ContainerVolumes: req.ContainerVolumes,
		HealthCheckCmd:   req.HealthCheckCmd,
		Entrypoint:       req.Entrypoint,
		ContainerUser:    req.ContainerUser,
	}

	// Convert healthCheckInterval string to time.Duration
	if req.HealthCheckInterval != "" {
		duration, err := time.ParseDuration(req.HealthCheckInterval)
		if err != nil {
			return def, fmt.Errorf("invalid healthCheckInterval: %v", err)
		}
		def.HealthCheckInterval = duration
	}

	return def, nil
}
