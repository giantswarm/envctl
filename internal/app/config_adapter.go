package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"envctl/internal/api"
	"envctl/internal/config"

	"gopkg.in/yaml.v3"
)

// ConfigAdapter adapts the config system to implement api.ConfigHandler
type ConfigAdapter struct {
	config     *config.EnvctlConfig
	configPath string
	mu         sync.RWMutex
}

// NewConfigAdapter creates a new config adapter
func NewConfigAdapter(cfg *config.EnvctlConfig, configPath string) *ConfigAdapter {
	return &ConfigAdapter{
		config:     cfg,
		configPath: configPath,
	}
}

// Register registers the adapter with the API
func (a *ConfigAdapter) Register() {
	api.RegisterConfig(a)
}

// Get configuration
func (a *ConfigAdapter) GetConfig(ctx context.Context) (*config.EnvctlConfig, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.config, nil
}

func (a *ConfigAdapter) GetClusters(ctx context.Context) ([]config.ClusterDefinition, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.config.Clusters, nil
}

func (a *ConfigAdapter) GetActiveClusters(ctx context.Context) (map[config.ClusterRole]string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.config.ActiveClusters, nil
}

func (a *ConfigAdapter) GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.config.MCPServers, nil
}

func (a *ConfigAdapter) GetPortForwards(ctx context.Context) ([]config.PortForwardDefinition, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.config.PortForwards, nil
}

func (a *ConfigAdapter) GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.config.Workflows, nil
}

func (a *ConfigAdapter) GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return &a.config.Aggregator, nil
}

func (a *ConfigAdapter) GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.config == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return &a.config.GlobalSettings, nil
}

// Update configuration
func (a *ConfigAdapter) UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and update existing server or add new one
	found := false
	for i, existing := range a.config.MCPServers {
		if existing.Name == server.Name {
			a.config.MCPServers[i] = server
			found = true
			break
		}
	}
	if !found {
		a.config.MCPServers = append(a.config.MCPServers, server)
	}

	return a.saveConfig()
}

func (a *ConfigAdapter) UpdatePortForward(ctx context.Context, portForward config.PortForwardDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and update existing port forward or add new one
	found := false
	for i, existing := range a.config.PortForwards {
		if existing.Name == portForward.Name {
			a.config.PortForwards[i] = portForward
			found = true
			break
		}
	}
	if !found {
		a.config.PortForwards = append(a.config.PortForwards, portForward)
	}

	return a.saveConfig()
}

func (a *ConfigAdapter) UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and update existing workflow or add new one
	found := false
	for i, existing := range a.config.Workflows {
		if existing.Name == workflow.Name {
			a.config.Workflows[i] = workflow
			found = true
			break
		}
	}
	if !found {
		a.config.Workflows = append(a.config.Workflows, workflow)
	}

	return a.saveConfig()
}

func (a *ConfigAdapter) UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	a.config.Aggregator = aggregator
	return a.saveConfig()
}

func (a *ConfigAdapter) UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	a.config.GlobalSettings = settings
	return a.saveConfig()
}

// Delete configuration
func (a *ConfigAdapter) DeleteMCPServer(ctx context.Context, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and remove the server
	newServers := make([]config.MCPServerDefinition, 0, len(a.config.MCPServers))
	for _, server := range a.config.MCPServers {
		if server.Name != name {
			newServers = append(newServers, server)
		}
	}

	if len(newServers) == len(a.config.MCPServers) {
		return fmt.Errorf("MCP server %s not found", name)
	}

	a.config.MCPServers = newServers
	return a.saveConfig()
}

func (a *ConfigAdapter) DeletePortForward(ctx context.Context, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and remove the port forward
	newPortForwards := make([]config.PortForwardDefinition, 0, len(a.config.PortForwards))
	for _, pf := range a.config.PortForwards {
		if pf.Name != name {
			newPortForwards = append(newPortForwards, pf)
		}
	}

	if len(newPortForwards) == len(a.config.PortForwards) {
		return fmt.Errorf("port forward %s not found", name)
	}

	a.config.PortForwards = newPortForwards
	return a.saveConfig()
}

func (a *ConfigAdapter) DeleteWorkflow(ctx context.Context, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and remove the workflow
	newWorkflows := make([]config.WorkflowDefinition, 0, len(a.config.Workflows))
	for _, wf := range a.config.Workflows {
		if wf.Name != name {
			newWorkflows = append(newWorkflows, wf)
		}
	}

	if len(newWorkflows) == len(a.config.Workflows) {
		return fmt.Errorf("workflow %s not found", name)
	}

	a.config.Workflows = newWorkflows
	return a.saveConfig()
}

func (a *ConfigAdapter) DeleteCluster(ctx context.Context, name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Find and remove the cluster
	newClusters := make([]config.ClusterDefinition, 0, len(a.config.Clusters))
	for _, cluster := range a.config.Clusters {
		if cluster.Name != name {
			newClusters = append(newClusters, cluster)
		}
	}

	if len(newClusters) == len(a.config.Clusters) {
		return fmt.Errorf("cluster %s not found", name)
	}

	a.config.Clusters = newClusters

	// Also remove from active clusters if it was active
	for role, clusterName := range a.config.ActiveClusters {
		if clusterName == name {
			delete(a.config.ActiveClusters, role)
		}
	}

	return a.saveConfig()
}

// Save configuration
func (a *ConfigAdapter) SaveConfig(ctx context.Context) error {
	return a.saveConfig()
}

// ReloadConfig reloads the configuration from disk
func (a *ConfigAdapter) ReloadConfig(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Reload configuration using the existing loader
	// We'll get mcName and wcName from the current config if available
	mcName := ""
	wcName := ""

	// Try to preserve the current MC/WC names if available
	if a.config != nil && len(a.config.Clusters) > 0 {
		for _, cluster := range a.config.Clusters {
			if cluster.Role == config.ClusterRoleObservability {
				mcName = cluster.Name
			} else if cluster.Role == config.ClusterRoleTarget {
				wcName = cluster.Name
			}
		}
	}

	newConfig, err := config.LoadConfig(mcName, wcName)
	if err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	a.config = &newConfig
	return nil
}

// GetTools returns all tools this provider offers
func (a *ConfigAdapter) GetTools() []api.ToolMetadata {
	return []api.ToolMetadata{
		// Get configuration tools
		{
			Name:        "config_get",
			Description: "Get the entire envctl configuration",
		},
		{
			Name:        "config_get_clusters",
			Description: "Get all configured clusters",
		},
		{
			Name:        "config_get_active_clusters",
			Description: "Get active clusters mapping",
		},
		{
			Name:        "config_get_mcp_servers",
			Description: "Get all MCP server definitions",
		},
		{
			Name:        "config_get_port_forwards",
			Description: "Get all port forward definitions",
		},
		{
			Name:        "config_get_workflows",
			Description: "Get all workflow definitions",
		},
		{
			Name:        "config_get_aggregator",
			Description: "Get aggregator configuration",
		},
		{
			Name:        "config_get_global_settings",
			Description: "Get global settings",
		},

		// Update configuration tools
		{
			Name:        "config_update_mcp_server",
			Description: "Update or add an MCP server definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "server",
					Type:        "object",
					Required:    true,
					Description: "MCP server definition object",
				},
			},
		},
		{
			Name:        "config_update_port_forward",
			Description: "Update or add a port forward definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "port_forward",
					Type:        "object",
					Required:    true,
					Description: "Port forward definition object",
				},
			},
		},
		{
			Name:        "config_update_workflow",
			Description: "Update or add a workflow definition",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "workflow",
					Type:        "object",
					Required:    true,
					Description: "Workflow definition object",
				},
			},
		},
		{
			Name:        "config_update_aggregator",
			Description: "Update aggregator configuration",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "aggregator",
					Type:        "object",
					Required:    true,
					Description: "Aggregator configuration object",
				},
			},
		},
		{
			Name:        "config_update_global_settings",
			Description: "Update global settings",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "settings",
					Type:        "object",
					Required:    true,
					Description: "Global settings object",
				},
			},
		},

		// Delete configuration tools
		{
			Name:        "config_delete_mcp_server",
			Description: "Delete an MCP server by name",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "MCP server name to delete",
				},
			},
		},
		{
			Name:        "config_delete_port_forward",
			Description: "Delete a port forward by name",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Port forward name to delete",
				},
			},
		},
		{
			Name:        "config_delete_workflow",
			Description: "Delete a workflow by name",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Workflow name to delete",
				},
			},
		},
		{
			Name:        "config_delete_cluster",
			Description: "Delete a cluster by name",
			Parameters: []api.ParameterMetadata{
				{
					Name:        "name",
					Type:        "string",
					Required:    true,
					Description: "Cluster name to delete",
				},
			},
		},

		// Save configuration
		{
			Name:        "config_save",
			Description: "Save the current configuration to file",
		},

		// Reload configuration
		{
			Name:        "config_reload",
			Description: "Reload configuration from disk including capability definitions",
		},
	}
}

// ExecuteTool executes a tool by name
func (a *ConfigAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	switch toolName {
	// Get operations
	case "config_get":
		return a.handleConfigGet(ctx)
	case "config_get_clusters":
		return a.handleConfigGetClusters(ctx)
	case "config_get_active_clusters":
		return a.handleConfigGetActiveClusters(ctx)
	case "config_get_mcp_servers":
		return a.handleConfigGetMCPServers(ctx)
	case "config_get_port_forwards":
		return a.handleConfigGetPortForwards(ctx)
	case "config_get_workflows":
		return a.handleConfigGetWorkflows(ctx)
	case "config_get_aggregator":
		return a.handleConfigGetAggregator(ctx)
	case "config_get_global_settings":
		return a.handleConfigGetGlobalSettings(ctx)

	// Update operations
	case "config_update_mcp_server":
		return a.handleConfigUpdateMCPServer(ctx, args)
	case "config_update_port_forward":
		return a.handleConfigUpdatePortForward(ctx, args)
	case "config_update_workflow":
		return a.handleConfigUpdateWorkflow(ctx, args)
	case "config_update_aggregator":
		return a.handleConfigUpdateAggregator(ctx, args)
	case "config_update_global_settings":
		return a.handleConfigUpdateGlobalSettings(ctx, args)

	// Delete operations
	case "config_delete_mcp_server":
		return a.handleConfigDeleteMCPServer(ctx, args)
	case "config_delete_port_forward":
		return a.handleConfigDeletePortForward(ctx, args)
	case "config_delete_workflow":
		return a.handleConfigDeleteWorkflow(ctx, args)
	case "config_delete_cluster":
		return a.handleConfigDeleteCluster(ctx, args)

	// Save operation
	case "config_save":
		return a.handleConfigSave(ctx)

	// Reload operation
	case "config_reload":
		return a.handleConfigReload(ctx)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Helper to save configuration
func (a *ConfigAdapter) saveConfig() error {
	if a.configPath == "" {
		// Try to determine the config path - check project dir first, then user dir
		projectPath, err := getProjectConfigPath()
		if err == nil {
			// Create directory if it doesn't exist
			dir := filepath.Dir(projectPath)
			if err := os.MkdirAll(dir, 0755); err == nil {
				a.configPath = projectPath
			}
		}

		// If we still don't have a path, try user config
		if a.configPath == "" {
			userPath, err := getUserConfigPath()
			if err == nil {
				// Create directory if it doesn't exist
				dir := filepath.Dir(userPath)
				if err := os.MkdirAll(dir, 0755); err == nil {
					a.configPath = userPath
				}
			}
		}

		if a.configPath == "" {
			return fmt.Errorf("unable to determine config file path")
		}
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(a.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(a.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Helper functions to get config paths
func getUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "envctl", "config.yaml"), nil
}

func getProjectConfigPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, ".envctl", "config.yaml"), nil
}

// Handler implementations
func (a *ConfigAdapter) handleConfigGet(ctx context.Context) (*api.CallToolResult, error) {
	cfg, err := a.GetConfig(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get configuration: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{cfg},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetClusters(ctx context.Context) (*api.CallToolResult, error) {
	clusters, err := a.GetClusters(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get clusters: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"clusters": clusters,
		"total":    len(clusters),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetActiveClusters(ctx context.Context) (*api.CallToolResult, error) {
	activeClusters, err := a.GetActiveClusters(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get active clusters: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{activeClusters},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetMCPServers(ctx context.Context) (*api.CallToolResult, error) {
	servers, err := a.GetMCPServers(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get MCP servers: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"servers": servers,
		"total":   len(servers),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetPortForwards(ctx context.Context) (*api.CallToolResult, error) {
	portForwards, err := a.GetPortForwards(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get port forwards: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"port_forwards": portForwards,
		"total":         len(portForwards),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetWorkflows(ctx context.Context) (*api.CallToolResult, error) {
	workflows, err := a.GetWorkflows(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get workflows: %v", err)},
			IsError: true,
		}, nil
	}

	result := map[string]interface{}{
		"workflows": workflows,
		"total":     len(workflows),
	}

	return &api.CallToolResult{
		Content: []interface{}{result},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetAggregator(ctx context.Context) (*api.CallToolResult, error) {
	aggregator, err := a.GetAggregatorConfig(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get aggregator config: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{aggregator},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigGetGlobalSettings(ctx context.Context) (*api.CallToolResult, error) {
	settings, err := a.GetGlobalSettings(ctx)
	if err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to get global settings: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{settings},
		IsError: false,
	}, nil
}

// Update handlers
func (a *ConfigAdapter) handleConfigUpdateMCPServer(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	serverData, ok := args["server"]
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"server is required"},
			IsError: true,
		}, nil
	}

	// Convert to config.MCPServerDefinition
	var server config.MCPServerDefinition
	if err := convertToStruct(serverData, &server); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse server definition: %v", err)},
			IsError: true,
		}, nil
	}

	if err := a.UpdateMCPServer(ctx, server); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update MCP server: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully updated MCP server '%s'", server.Name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigUpdatePortForward(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	portForwardData, ok := args["port_forward"]
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"port_forward is required"},
			IsError: true,
		}, nil
	}

	// Convert to config.PortForwardDefinition
	var portForward config.PortForwardDefinition
	if err := convertToStruct(portForwardData, &portForward); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse port forward definition: %v", err)},
			IsError: true,
		}, nil
	}

	if err := a.UpdatePortForward(ctx, portForward); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update port forward: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully updated port forward '%s'", portForward.Name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigUpdateWorkflow(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	workflowData, ok := args["workflow"]
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"workflow is required"},
			IsError: true,
		}, nil
	}

	// Convert to config.WorkflowDefinition
	var workflow config.WorkflowDefinition
	if err := convertToStruct(workflowData, &workflow); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse workflow definition: %v", err)},
			IsError: true,
		}, nil
	}

	if err := a.UpdateWorkflow(ctx, workflow); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update workflow: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully updated workflow '%s'", workflow.Name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigUpdateAggregator(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	aggregatorData, ok := args["aggregator"]
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"aggregator is required"},
			IsError: true,
		}, nil
	}

	// Convert to config.AggregatorConfig
	var aggregator config.AggregatorConfig
	if err := convertToStruct(aggregatorData, &aggregator); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse aggregator config: %v", err)},
			IsError: true,
		}, nil
	}

	if err := a.UpdateAggregatorConfig(ctx, aggregator); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update aggregator config: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"Successfully updated aggregator configuration"},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigUpdateGlobalSettings(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	settingsData, ok := args["settings"]
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"settings is required"},
			IsError: true,
		}, nil
	}

	// Convert to config.GlobalSettings
	var settings config.GlobalSettings
	if err := convertToStruct(settingsData, &settings); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to parse global settings: %v", err)},
			IsError: true,
		}, nil
	}

	if err := a.UpdateGlobalSettings(ctx, settings); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to update global settings: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{"Successfully updated global settings"},
		IsError: false,
	}, nil
}

// Delete handlers
func (a *ConfigAdapter) handleConfigDeleteMCPServer(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	if err := a.DeleteMCPServer(ctx, name); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete MCP server: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully deleted MCP server '%s'", name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigDeletePortForward(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	if err := a.DeletePortForward(ctx, name); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete port forward: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully deleted port forward '%s'", name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigDeleteWorkflow(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	if err := a.DeleteWorkflow(ctx, name); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete workflow: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully deleted workflow '%s'", name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigDeleteCluster(ctx context.Context, args map[string]interface{}) (*api.CallToolResult, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &api.CallToolResult{
			Content: []interface{}{"name is required"},
			IsError: true,
		}, nil
	}

	if err := a.DeleteCluster(ctx, name); err != nil {
		return &api.CallToolResult{
			Content: []interface{}{fmt.Sprintf("Failed to delete cluster: %v", err)},
			IsError: true,
		}, nil
	}

	return &api.CallToolResult{
		Content: []interface{}{fmt.Sprintf("Successfully deleted cluster '%s'", name)},
		IsError: false,
	}, nil
}

func (a *ConfigAdapter) handleConfigSave(ctx context.Context) (*api.CallToolResult, error) {
	err := a.SaveConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &api.CallToolResult{
		Content: []interface{}{
			"Configuration saved successfully",
		},
	}, nil
}

func (a *ConfigAdapter) handleConfigReload(ctx context.Context) (*api.CallToolResult, error) {
	// Reload main configuration
	if err := a.ReloadConfig(ctx); err != nil {
		return nil, err
	}

	// Trigger capability definitions reload if capability handler exists
	if capHandler := api.GetCapability(); capHandler != nil {
		if reloader, ok := capHandler.(interface{ ReloadDefinitions() error }); ok {
			if err := reloader.ReloadDefinitions(); err != nil {
				return nil, fmt.Errorf("failed to reload capability definitions: %w", err)
			}
		}
	}

	// Trigger workflow definitions reload if workflow handler exists
	if wfHandler := api.GetWorkflow(); wfHandler != nil {
		if reloader, ok := wfHandler.(interface{ ReloadWorkflows() error }); ok {
			if err := reloader.ReloadWorkflows(); err != nil {
				return nil, fmt.Errorf("failed to reload workflow definitions: %w", err)
			}
		}
	}

	return &api.CallToolResult{
		Content: []interface{}{
			"Configuration reloaded successfully",
		},
	}, nil
}

// Helper function to convert interface{} to struct
func convertToStruct(data interface{}, target interface{}) error {
	// For simplicity, we'll use JSON marshaling/unmarshaling
	// In production, you might want a more efficient approach
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, target)
}
