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

	// Reload configuration from disk
	ReloadConfig(ctx context.Context) error
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
	return handler.GetConfig(ctx)
}

// GetClusters returns all configured clusters
func (c *configServiceAPI) GetClusters(ctx context.Context) ([]config.ClusterDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetClusters(ctx)
}

// GetActiveClusters returns the active clusters map
func (c *configServiceAPI) GetActiveClusters(ctx context.Context) (map[config.ClusterRole]string, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetActiveClusters(ctx)
}

// GetMCPServers returns all MCP server definitions
func (c *configServiceAPI) GetMCPServers(ctx context.Context) ([]config.MCPServerDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetMCPServers(ctx)
}

// GetPortForwards returns all port forward definitions
func (c *configServiceAPI) GetPortForwards(ctx context.Context) ([]config.PortForwardDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetPortForwards(ctx)
}

// GetWorkflows returns all workflow definitions
func (c *configServiceAPI) GetWorkflows(ctx context.Context) ([]config.WorkflowDefinition, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetWorkflows(ctx)
}

// GetAggregatorConfig returns the aggregator configuration
func (c *configServiceAPI) GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetAggregatorConfig(ctx)
}

// GetGlobalSettings returns the global settings
func (c *configServiceAPI) GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetGlobalSettings(ctx)
}

// UpdateClusters updates the clusters configuration
func (c *configServiceAPI) UpdateClusters(ctx context.Context, clusters []config.ClusterDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	// Get current config
	cfg, err := handler.GetConfig(ctx)
	if err != nil {
		return err
	}
	// Update clusters
	cfg.Clusters = clusters
	// Save config
	return handler.SaveConfig(ctx)
}

// UpdateActiveClusters updates the active clusters mapping
func (c *configServiceAPI) UpdateActiveClusters(ctx context.Context, activeClusters map[config.ClusterRole]string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	// Get current config
	cfg, err := handler.GetConfig(ctx)
	if err != nil {
		return err
	}
	// Update active clusters
	cfg.ActiveClusters = activeClusters
	// Save config
	return handler.SaveConfig(ctx)
}

// UpdateMCPServer updates or adds an MCP server definition
func (c *configServiceAPI) UpdateMCPServer(ctx context.Context, server config.MCPServerDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateMCPServer(ctx, server)
}

// UpdatePortForward updates or adds a port forward definition
func (c *configServiceAPI) UpdatePortForward(ctx context.Context, portForward config.PortForwardDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdatePortForward(ctx, portForward)
}

// UpdateWorkflow updates or adds a workflow definition
func (c *configServiceAPI) UpdateWorkflow(ctx context.Context, workflow config.WorkflowDefinition) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateWorkflow(ctx, workflow)
}

// UpdateAggregatorConfig updates the aggregator configuration
func (c *configServiceAPI) UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateAggregatorConfig(ctx, aggregator)
}

// UpdateGlobalSettings updates the global settings
func (c *configServiceAPI) UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.UpdateGlobalSettings(ctx, settings)
}

// DeleteMCPServer removes an MCP server by name
func (c *configServiceAPI) DeleteMCPServer(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeleteMCPServer(ctx, name)
}

// DeletePortForward removes a port forward by name
func (c *configServiceAPI) DeletePortForward(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeletePortForward(ctx, name)
}

// DeleteWorkflow removes a workflow by name
func (c *configServiceAPI) DeleteWorkflow(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeleteWorkflow(ctx, name)
}

// DeleteCluster removes a cluster by name
func (c *configServiceAPI) DeleteCluster(ctx context.Context, name string) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.DeleteCluster(ctx, name)
}

// SaveConfig persists the configuration to file
func (c *configServiceAPI) SaveConfig(ctx context.Context) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.SaveConfig(ctx)
}

// ReloadConfig reloads the configuration from disk
func (c *configServiceAPI) ReloadConfig(ctx context.Context) error {
	handler := GetConfigHandler()
	if handler == nil {
		return fmt.Errorf("config handler not registered")
	}
	return handler.ReloadConfig(ctx)
}
