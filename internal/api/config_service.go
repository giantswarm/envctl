package api

import (
	"context"
	"fmt"

	"envctl/internal/config"
)

// ConfigServiceAPI defines the interface for managing configuration at runtime
type ConfigServiceAPI interface {
	// Get entire configuration
	GetConfig(ctx context.Context) (*config.EnvctlConfig, error)

	// Get specific configuration sections
	GetClusters(ctx context.Context) ([]config.ClusterDefinition, error)
	GetActiveClusters(ctx context.Context) (map[config.ClusterRole]string, error)
	GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error)
	GetPortForwards(ctx context.Context) ([]config.PortForwardDefinition, error)
	GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error)
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration sections
	UpdateClusters(ctx context.Context, clusters []config.ClusterDefinition) error
	UpdateActiveClusters(ctx context.Context, activeClusters map[config.ClusterRole]string) error
	UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error
	UpdatePortForward(ctx context.Context, portForward config.PortForwardDefinition) error
	UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

	// Delete configuration items
	DeleteMCPServer(ctx context.Context, name string) error
	DeletePortForward(ctx context.Context, name string) error
	DeleteWorkflow(ctx context.Context, name string) error
	DeleteCluster(ctx context.Context, name string) error

	// Save configuration to file
	SaveConfig(ctx context.Context) error
}

// configServiceAPI implements the ConfigServiceAPI interface
type configServiceAPI struct {
	// No fields - uses handlers from registry
}

// NewConfigServiceAPI creates a new ConfigServiceAPI instance
func NewConfigServiceAPI() ConfigServiceAPI {
	return &configServiceAPI{}
}

// GetConfig returns the entire configuration
func (c *configServiceAPI) GetConfig(ctx context.Context) (*config.EnvctlConfig, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetConfig()
}

// GetClusters returns all configured clusters
func (c *configServiceAPI) GetClusters(ctx context.Context) ([]config.ClusterDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Clusters, nil
}

// GetActiveClusters returns the active clusters map
func (c *configServiceAPI) GetActiveClusters(ctx context.Context) (map[config.ClusterRole]string, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return cfg.ActiveClusters, nil
}

// GetMCPServers returns all MCP server definitions
func (c *configServiceAPI) GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return cfg.MCPServers, nil
}

// GetPortForwards returns all port forward definitions
func (c *configServiceAPI) GetPortForwards(ctx context.Context) ([]config.PortForwardDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return cfg.PortForwards, nil
}

// GetWorkflows returns all workflow definitions
func (c *configServiceAPI) GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Workflows, nil
}

// GetAggregatorConfig returns the aggregator configuration
func (c *configServiceAPI) GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return &cfg.Aggregator, nil
}

// GetGlobalSettings returns the global settings
func (c *configServiceAPI) GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	cfg, err := handler.GetConfig()
	if err != nil {
		return nil, err
	}
	return &cfg.GlobalSettings, nil
}

// UpdateClusters updates the clusters configuration
func (c *configServiceAPI) UpdateClusters(ctx context.Context, clusters []config.ClusterDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateClusters(clusters)
}

// UpdateActiveClusters updates the active clusters mapping
func (c *configServiceAPI) UpdateActiveClusters(ctx context.Context, activeClusters map[config.ClusterRole]string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateActiveClusters(activeClusters)
}

// UpdateMCPServer updates or adds an MCP server definition
func (c *configServiceAPI) UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateMCPServer(server)
}

// UpdatePortForward updates or adds a port forward definition
func (c *configServiceAPI) UpdatePortForward(ctx context.Context, portForward config.PortForwardDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdatePortForward(portForward)
}

// UpdateWorkflow updates or adds a workflow definition
func (c *configServiceAPI) UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateWorkflow(workflow)
}

// UpdateAggregatorConfig updates the aggregator configuration
func (c *configServiceAPI) UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateAggregatorConfig(aggregator)
}

// UpdateGlobalSettings updates the global settings
func (c *configServiceAPI) UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateGlobalSettings(settings)
}

// DeleteMCPServer removes an MCP server by name
func (c *configServiceAPI) DeleteMCPServer(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeleteMCPServer(name)
}

// DeletePortForward removes a port forward by name
func (c *configServiceAPI) DeletePortForward(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeletePortForward(name)
}

// DeleteWorkflow removes a workflow by name
func (c *configServiceAPI) DeleteWorkflow(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeleteWorkflow(name)
}

// DeleteCluster removes a cluster by name
func (c *configServiceAPI) DeleteCluster(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeleteCluster(name)
}

// SaveConfig persists the configuration to file
func (c *configServiceAPI) SaveConfig(ctx context.Context) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.SaveConfig()
}
