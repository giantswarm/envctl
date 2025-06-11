package app

import (
	"context"
	"envctl/internal/tui/controller"
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"envctl/pkg/logging"
	"os"
	"os/signal"
	"syscall"
)

// runCLIMode executes the non-interactive command line mode
func runCLIMode(ctx context.Context, config *Config, services *Services) error {
	logging.Info("CLI", "Running in no-TUI mode.")
	logging.Info("CLI", "--- Setting up orchestrator for service management ---")

	// Start all configured services
	if err := services.Orchestrator.Start(ctx); err != nil {
		logging.Error("CLI", err, "Failed to start orchestrator")
		return err
	}

	logging.Info("CLI", "Services started. Press Ctrl+C to stop all services and exit.")

	// Wait for interrupt signal to gracefully shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown sequence
	logging.Info("CLI", "\n--- Shutting down services ---")
	services.Orchestrator.Stop()

	return nil
}

// runTUIMode executes the interactive terminal UI mode
func runTUIMode(ctx context.Context, config *Config, services *Services) error {
	logging.Info("CLI", "Starting TUI mode...")

	// Initialize design system for TUI (dark mode by default)
	design.Initialize(true)

	// Switch logging to channel-based system for TUI integration
	logLevel := logging.LevelInfo
	if config.Debug {
		logLevel = logging.LevelDebug
	}
	logChan := logging.InitForTUI(logLevel)
	defer logging.CloseTUIChannel()

	// Create and configure the TUI program
	p, err := controller.NewProgram(model.TUIConfig{
		DebugMode:        config.Debug,
		ColorMode:        "auto",
		MCPServerConfig:  config.EnvctlConfig.MCPServers,
		AggregatorConfig: config.EnvctlConfig.Aggregator,
		Orchestrator:     services.Orchestrator,
		OrchestratorAPI:  services.OrchestratorAPI,
		MCPServiceAPI:    services.MCPAPI,
		AggregatorAPI:    services.AggregatorAPI,
	}, logChan)
	if err != nil {
		logging.Error("TUI-Lifecycle", err, "Error creating TUI program")
		return err
	}

	// Run the TUI until user exits
	if _, err := p.Run(); err != nil {
		logging.Error("TUI-Lifecycle", err, "Error running TUI program")
		return err
	}
	logging.Info("TUI-Lifecycle", "TUI exited.")

	return nil
}
