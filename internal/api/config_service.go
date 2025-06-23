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
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration sections
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

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

// ConfigHandler provides configuration management functionality
type ConfigHandler interface {
	// Get configuration
	GetConfig(ctx context.Context) (*config.EnvctlConfig, error)
	GetAggregatorConfig(ctx context.Context) (*config.AggregatorConfig, error)
	GetGlobalSettings(ctx context.Context) (*config.GlobalSettings, error)

	// Update configuration
	UpdateAggregatorConfig(ctx context.Context, aggregator config.AggregatorConfig) error
	UpdateGlobalSettings(ctx context.Context, settings config.GlobalSettings) error

	// Save configuration
	SaveConfig(ctx context.Context) error

	// Reload configuration from disk
	ReloadConfig(ctx context.Context) error

	ToolProvider
}

// GetConfig returns the entire configuration
func (c *configServiceAPI) GetConfig(ctx context.Context) (*config.EnvctlConfig, error) {
	handler := GetConfigHandler()
	if handler == nil {
		return nil, fmt.Errorf("config handler not registered")
	}
	return handler.GetConfig(ctx)
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
