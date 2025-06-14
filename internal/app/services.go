package app

import (
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/capability"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	aggregatorService "envctl/internal/services/aggregator"
	"envctl/internal/services/mcpserver"
	"envctl/pkg/logging"

	// Import to trigger init() functions that register adapter factories
	_ "envctl/internal/workflow"

	// Import ServiceClass manager
	"envctl/internal/serviceclass"
	"fmt"
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

// aggregatorToolChecker checks tool availability using the aggregator server
type aggregatorToolChecker struct {
	serviceRegistry services.ServiceRegistry
}

func (a *aggregatorToolChecker) IsToolAvailable(toolName string) bool {
	// Get the aggregator service
	service, exists := a.serviceRegistry.Get("mcp-aggregator")
	if !exists {
		return false
	}

	// Cast to aggregator service to get the manager
	if aggService, ok := service.(interface{ GetManager() *aggregator.AggregatorManager }); ok {
		manager := aggService.GetManager()
		if manager != nil {
			server := manager.GetAggregatorServer()
			if server != nil {
				return server.IsToolAvailable(toolName)
			}
		}
	}
	return false
}

// InitializeServices creates and registers all required services
func InitializeServices(cfg *Config) (*Services, error) {
	// Create orchestrator without ToolCaller initially
	orchConfig := orchestrator.Config{
		MCPServers: cfg.EnvctlConfig.MCPServers,
		Aggregator: cfg.EnvctlConfig.Aggregator,
		Yolo:       cfg.Yolo,
		ToolCaller: nil, // Will be set up after aggregator is available
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

	// Get the service registry handler from the API
	registryHandler := api.GetServiceRegistry()
	if registryHandler == nil {
		return nil, fmt.Errorf("service registry handler not available")
	}

	// Create the tool checker using the API handler
	toolChecker := &aggregatorToolChecker{serviceRegistry: registry}

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
	// Register the MCP service API so mcp_server_* tools can work
	api.SetMCPServiceAPI(mcpAPI)
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

							// Set up ToolCaller for ServiceClass-based services using orchestrator's event system
		// This follows our architectural principles by using event-driven patterns instead of time.Sleep
		setupToolCaller := func() {
			logging.Info("Bootstrap", "Setting up ToolCaller for ServiceClass-based services")
			
			// Get the aggregator service and create a ToolCaller
			if service, exists := registry.Get("mcp-aggregator"); exists {
				if aggSvc, ok := service.(interface{ GetManager() *aggregator.AggregatorManager }); ok {
					manager := aggSvc.GetManager()
					if manager != nil {
						server := manager.GetAggregatorServer()
						if server != nil {
							// Create AggregatorToolCaller and set it on the orchestrator
							toolCaller := capability.NewAggregatorToolCaller(server)
							orch.SetToolCaller(toolCaller)
							logging.Info("Bootstrap", "Set up ToolCaller for ServiceClass-based services")
							
							// Refresh ServiceClass availability now that ToolCaller is available
							if serviceClassHandler := api.GetServiceClassManager(); serviceClassHandler != nil {
								serviceClassHandler.RefreshAvailability()
								logging.Info("Bootstrap", "Refreshed ServiceClass availability after ToolCaller setup")
							}
						}
					}
				}
			}
		}
		
		// Subscribe to orchestrator state change events to detect when aggregator becomes running
		go func() {
			stateChanges := orch.SubscribeToStateChanges()
			for event := range stateChanges {
				if event.Label == "mcp-aggregator" && event.NewState == "Running" && event.OldState != "Running" {
					logging.Info("Bootstrap", "Aggregator service transitioned to running state via orchestrator event")
					setupToolCaller()
					return // Exit goroutine after setting up ToolCaller
				}
			}
		}()
		
		// Check if aggregator is already running and set up ToolCaller immediately
		if aggService.GetState() == services.StateRunning {
			logging.Info("Bootstrap", "Aggregator service is already running")
			setupToolCaller()
		}
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
