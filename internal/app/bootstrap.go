package app

import (
	"context"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
	"os"
)

// Application is the main application structure that bootstraps and runs envctl
type Application struct {
	config   *Config
	services *Services
}

// NewApplication creates and initializes a new application instance
func NewApplication(cfg *Config) (*Application, error) {
	// Configure logging based on debug flag
	appLogLevel := logging.LevelInfo
	if cfg.Debug {
		appLogLevel = logging.LevelDebug
	}

	// Initialize logging for CLI output (will be replaced for TUI mode)
	logging.InitForCLI(appLogLevel, os.Stdout)

	// Load environment configuration
	var envctlCfg config.EnvctlConfig
	var err error

	if cfg.ConfigPath != "" {
		// Use single directory configuration loading
		envctlCfg, err = config.LoadConfigFromPath(cfg.ConfigPath)
		if err != nil {
			logging.Error("Bootstrap", err, "Failed to load envctl configuration from path: %s", cfg.ConfigPath)
			return nil, fmt.Errorf("failed to load envctl configuration from path %s: %w", cfg.ConfigPath, err)
		}
		logging.Info("Bootstrap", "Loaded configuration from custom path: %s", cfg.ConfigPath)
	} else {
		// Use layered configuration loading (default behavior)
		envctlCfg, err = config.LoadConfig()
		if err != nil {
			logging.Error("Bootstrap", err, "Failed to load envctl configuration")
			return nil, fmt.Errorf("failed to load envctl configuration: %w", err)
		}
		logging.Info("Bootstrap", "Loaded configuration using layered approach")
	}

	cfg.EnvctlConfig = &envctlCfg

	// Initialize services
	services, err := InitializeServices(cfg)
	if err != nil {
		logging.Error("Bootstrap", err, "Failed to initialize services")
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	return &Application{
		config:   cfg,
		services: services,
	}, nil
}

// Run executes the application in the appropriate mode
func (a *Application) Run(ctx context.Context) error {
	if a.config.NoTUI {
		return a.runCLIMode(ctx)
	}
	return a.runTUIMode(ctx)
}

// runCLIMode runs the application in non-interactive CLI mode
func (a *Application) runCLIMode(ctx context.Context) error {
	return runCLIMode(ctx, a.config, a.services)
}

// runTUIMode runs the application in interactive TUI mode
func (a *Application) runTUIMode(ctx context.Context) error {
	return runTUIMode(ctx, a.config, a.services)
}
