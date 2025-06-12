package app

import (
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	aggregatorService "envctl/internal/services/aggregator"
	"envctl/internal/services/mcpserver"

	// Import to trigger init() functions that register adapter factories
	_ "envctl/internal/capability"
	_ "envctl/internal/workflow"

	// Import ServiceClass manager
	"envctl/internal/serviceclass"
	"path/filepath"
)

// Services holds all the initialized services and APIs
type Services struct {
	Orchestrator    *orchestrator.Orchestrator
	OrchestratorAPI api.OrchestratorAPI
	MCPAPI          api.MCPServiceAPI
	AggregatorAPI   api.AggregatorAPI
	ConfigAPI       api.ConfigServiceAPI
	AggregatorPort  int
}

// InitializeServices creates and registers all required services
func InitializeServices(cfg *Config) (*Services, error) {
	// Create the orchestrator
	orchConfig := orchestrator.Config{
		MCPServers: cfg.EnvctlConfig.MCPServers,
		Aggregator: cfg.EnvctlConfig.Aggregator,
		Yolo:       cfg.Yolo,
	}

	orch := orchestrator.New(orchConfig)

	// Get the service registry
	registry := orch.GetServiceRegistry()

	// Step 1: Create and register adapters BEFORE creating APIs
	// This is critical - APIs need handlers to be registered first

	// Register service registry adapter
	registryAdapter := services.NewRegistryAdapter(registry)
	registryAdapter.Register()

	// Register orchestrator adapter
	orchAdapter := orchestrator.NewAPIAdapter(orch)
	orchAdapter.Register()

	// Register configuration adapter
	configAdapter := NewConfigAdapter(cfg.EnvctlConfig, "") // Empty path means auto-detect
	configAdapter.Register()

	// Register service adapters
	mcpAdapter := mcpserver.NewServiceAdapter()
	mcpAdapter.Register()

	// Initialize and register ServiceClass manager
	// This needs to be done before orchestrator starts to handle ServiceClass-based services
	configDir, err := config.GetUserConfigDir()
	if err != nil {
		// Use default if we can't get config dir
		configDir = ".config/envctl"
	}
	serviceClassPath := filepath.Join(configDir, "serviceclass", "definitions")

	// Create a simple tool checker that delegates to the aggregator when available
	// For now, we'll use a placeholder that always returns false
	toolChecker := &placeholderToolChecker{}

	// Create ServiceClass manager
	serviceClassManager := serviceclass.NewServiceClassManager(serviceClassPath, toolChecker)

	// Create and register ServiceClass adapter
	serviceClassAdapter := serviceclass.NewAdapter(serviceClassManager)
	serviceClassAdapter.Register()

	// Load ServiceClass definitions
	if err := serviceClassManager.LoadServiceDefinitions(); err != nil {
		// Log warning but don't fail - ServiceClass is optional
		// This would need proper logging in production
	}

	// Step 2: Create APIs that use the registered handlers
	orchestratorAPI := api.NewOrchestratorAPI()
	mcpAPI := api.NewMCPServiceAPI()
	aggregatorAPI := api.NewAggregatorAPI()
	configAPI := api.NewConfigServiceAPI()

	// Step 3: Create and register actual services
	// Create MCP server services based on configuration
	for _, mcpConfig := range cfg.EnvctlConfig.MCPServers {
		if mcpConfig.Enabled {
			mcpService := mcpserver.NewMCPServerService(mcpConfig)
			if mcpService != nil {
				registry.Register(mcpService)
			}
		}
	}

	// Create aggregator service - enable by default unless explicitly disabled
	// This ensures the aggregator starts even with no MCP servers configured
	aggregatorEnabled := true
	if cfg.EnvctlConfig.Aggregator.Port != 0 || cfg.EnvctlConfig.Aggregator.Host != "" {
		// If aggregator config exists, respect the enabled flag
		aggregatorEnabled = cfg.EnvctlConfig.Aggregator.Enabled
	}

	if aggregatorEnabled {
		// Need to get the service registry handler from the registry adapter
		registryHandler := api.GetServiceRegistry()
		if registryHandler != nil {
			// Auto-detect config directory
			configDir, err := config.GetUserConfigDir()
			if err != nil {
				// Fallback to empty string if auto-detection fails
				configDir = ""
			}

			// Convert config types
			aggConfig := aggregator.AggregatorConfig{
				Port:         cfg.EnvctlConfig.Aggregator.Port,
				Host:         cfg.EnvctlConfig.Aggregator.Host,
				EnvctlPrefix: cfg.EnvctlConfig.Aggregator.EnvctlPrefix,
				Yolo:         cfg.Yolo,
				ConfigDir:    configDir,
			}

			// Set defaults if not specified
			if aggConfig.Port == 0 {
				aggConfig.Port = 8090
			}
			if aggConfig.Host == "" {
				aggConfig.Host = "localhost"
			}

			aggService := aggregatorService.NewAggregatorService(
				aggConfig,
				orchestratorAPI,
				mcpAPI,
				registryHandler,
			)
			registry.Register(aggService)
		}
	}

	return &Services{
		Orchestrator:    orch,
		OrchestratorAPI: orchestratorAPI,
		MCPAPI:          mcpAPI,
		AggregatorAPI:   aggregatorAPI,
		ConfigAPI:       configAPI,
		AggregatorPort:  cfg.EnvctlConfig.Aggregator.Port,
	}, nil
}

// placeholderToolChecker is a temporary implementation of ToolAvailabilityChecker
// In production, this should check against the aggregator's tool registry
type placeholderToolChecker struct{}

func (p *placeholderToolChecker) IsToolAvailable(toolName string) bool {
	// For now, return false for all tools
	// This will be replaced with actual tool checking logic
	return false
}
