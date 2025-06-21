package app

import (
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/capability"
	"envctl/internal/config"
	mcpserverPkg "envctl/internal/mcpserver"
	"envctl/internal/orchestrator"
	"envctl/internal/serviceclass"
	"envctl/internal/services"
	aggregatorService "envctl/internal/services/aggregator"
	"envctl/internal/services/mcpserver"
	"envctl/internal/workflow"
	"envctl/pkg/logging"
	"fmt"
)

// Services holds all the initialized services and APIs
type Services struct {
	Orchestrator    *orchestrator.Orchestrator
	OrchestratorAPI api.OrchestratorAPI
	AggregatorAPI   api.AggregatorAPI
	ConfigAPI       api.ConfigServiceAPI
	AggregatorPort  int
}

// InitializeServices creates and registers all required services
func InitializeServices(cfg *Config) (*Services, error) {
	// Create storage for shared use across services including orchestrator persistence
	var storage *config.Storage
	if cfg.ConfigPath != "" {
		storage = config.NewStorageWithPath(cfg.ConfigPath)
	} else {
		storage = config.NewStorage()
	}

	// Create API-based tool checker and caller
	toolChecker := api.NewAPIToolChecker()
	apiToolCaller := api.NewAPIToolCaller()

	// Create orchestrator without ToolCaller initially
	orchConfig := orchestrator.Config{
		Aggregator: cfg.EnvctlConfig.Aggregator,
		Yolo:       cfg.Yolo,
		ToolCaller: nil, // Will be set up after aggregator is available
		Storage:    storage,
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

	// Initialize and register ServiceClass manager
	// This needs to be done before orchestrator starts to handle ServiceClass-based services

	// Get the service registry handler from the API
	registryHandler := api.GetServiceRegistry()
	if registryHandler == nil {
		return nil, fmt.Errorf("service registry handler not available")
	}

	// Use the shared storage created earlier
	serviceClassManager, err := serviceclass.NewServiceClassManager(toolChecker, storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create ServiceClass manager: %w", err)
	}

	// Create and register ServiceClass adapter
	serviceClassAdapter := serviceclass.NewAdapter(serviceClassManager)
	serviceClassAdapter.Register()

	// Load ServiceClass definitions
	if cfg.ConfigPath != "" {
		serviceClassManager.SetConfigPath(cfg.ConfigPath)
	}
	if err := serviceClassManager.LoadServiceDefinitions(); err != nil {
		// Log warning but don't fail - ServiceClass is optional
		logging.Warn("Services", "Failed to load ServiceClass definitions: %v", err)
	}

	// Initialize and register Capability adapter
	capabilityAdapter, err := capability.NewAdapter(toolChecker, nil, storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create Capability adapter: %w", err)
	}
	capabilityAdapter.Register()

	// Load Capability definitions
	if cfg.ConfigPath != "" {
		capabilityAdapter.SetConfigPath(cfg.ConfigPath)
	}
	if err := capabilityAdapter.LoadDefinitions(); err != nil {
		// Log warning but don't fail - Capability is optional
		logging.Warn("Services", "Failed to load Capability definitions: %v", err)
	}

	// Create and register Workflow adapter
	workflowManager, err := workflow.NewWorkflowManager(storage, nil, toolChecker)
	if err != nil {
		return nil, fmt.Errorf("failed to create Workflow manager: %w", err)
	}

	workflowAdapter := workflow.NewAdapter(workflowManager, nil)
	workflowAdapter.Register()

	// Load Workflow definitions
	if cfg.ConfigPath != "" {
		workflowManager.SetConfigPath(cfg.ConfigPath)
	}
	if err := workflowManager.LoadDefinitions(); err != nil {
		// Log warning but don't fail - Workflow is optional
		logging.Warn("Services", "Failed to load Workflow definitions: %v", err)
	}

	// Initialize and register MCPServer manager (new unified configuration approach)
	mcpServerManager, err := mcpserverPkg.NewMCPServerManager(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server manager: %w", err)
	}

	// Create and register MCPServer adapter
	mcpServerAdapter := mcpserverPkg.NewAdapter(mcpServerManager)
	mcpServerAdapter.Register()

	// Load MCP server definitions
	if cfg.ConfigPath != "" {
		mcpServerManager.SetConfigPath(cfg.ConfigPath)
	}
	if err := mcpServerManager.LoadDefinitions(); err != nil {
		// Log warning but don't fail - MCP servers are optional
		logging.Warn("Services", "Failed to load MCP server definitions: %v", err)
	}

	// Step 2: Create APIs that use the registered handlers
	orchestratorAPI := api.NewOrchestratorAPI()
	aggregatorAPI := api.NewAggregatorAPI()
	configAPI := api.NewConfigServiceAPI()

	// Step 3: Create and register actual services
	// Create MCP server services using the new directory-based loading
	mcpServerDefinitions := mcpServerManager.ListDefinitions()
	for _, mcpDef := range mcpServerDefinitions {
		if mcpDef.Enabled {
			mcpService, err := mcpserver.NewService(&mcpDef, mcpServerManager)
			if err != nil {
				logging.Warn("Services", "Failed to create MCP server service %s: %v", mcpDef.Name, err)
				continue
			}
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
			// Auto-detect config directory or use custom path
			var configDir string
			if cfg.ConfigPath != "" {
				configDir = cfg.ConfigPath
			} else {
				userConfigDir, err := config.GetUserConfigDir()
				if err != nil {
					// Fallback to empty string if auto-detection fails
					configDir = ""
				} else {
					configDir = userConfigDir
				}
			}

			// Convert config types
			aggConfig := aggregator.AggregatorConfig{
				Port:         cfg.EnvctlConfig.Aggregator.Port,
				Host:         cfg.EnvctlConfig.Aggregator.Host,
				Transport:    cfg.EnvctlConfig.Aggregator.Transport,
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
			if aggConfig.Transport == "" {
				aggConfig.Transport = config.MCPTransportStreamableHTTP
			}

			aggService := aggregatorService.NewAggregatorService(
				aggConfig,
				orchestratorAPI,
				registryHandler,
			)
			registry.Register(aggService)

			// Create aggregator API adapter
			aggAdapter := aggregatorService.NewAPIAdapter(aggService)
			aggAdapter.Register()

			// Set up ToolCaller for ServiceClass-based services using orchestrator's event system
			// This follows our architectural principles by using event-driven patterns instead of time.Sleep
			setupToolCaller := func() {
				logging.Info("Bootstrap", "Setting up ToolCaller for ServiceClass-based services")

				// Set the API-based tool caller on the orchestrator
				orch.SetToolCaller(apiToolCaller)
				logging.Info("Bootstrap", "Set up API-based ToolCaller for ServiceClass-based services")

				// Also set the toolCaller on the workflow manager
				workflowHandler := api.GetWorkflow()
				if workflowHandler != nil {
					logging.Debug("Bootstrap", "Found workflow handler, attempting to set ToolCaller")
					if workflowSetter, ok := workflowHandler.(interface{ SetToolCaller(interface{}) }); ok {
						workflowSetter.SetToolCaller(apiToolCaller)
						logging.Info("Bootstrap", "Set up API-based ToolCaller for workflow execution")
					} else {
						logging.Warn("Bootstrap", "Workflow handler does not support SetToolCaller")
					}
				} else {
					logging.Warn("Bootstrap", "Workflow handler not found when setting up ToolCaller")
				}

				// Phase 1: Availability refresh is now transparent, no manual refresh needed
				logging.Info("Bootstrap", "ServiceClass and Capability availability is managed transparently")
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
		AggregatorAPI:   aggregatorAPI,
		ConfigAPI:       configAPI,
		AggregatorPort:  cfg.EnvctlConfig.Aggregator.Port,
	}, nil
}
