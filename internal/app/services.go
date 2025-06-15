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
	if aggService, ok := service.(interface {
		GetManager() *aggregator.AggregatorManager
	}); ok {
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

func (a *aggregatorToolChecker) GetAvailableTools() []string {
	// Get the aggregator service
	service, exists := a.serviceRegistry.Get("mcp-aggregator")
	if !exists {
		return []string{}
	}

	// Cast to aggregator service to get the manager
	if aggService, ok := service.(interface {
		GetManager() *aggregator.AggregatorManager
	}); ok {
		manager := aggService.GetManager()
		if manager != nil {
			server := manager.GetAggregatorServer()
			if server != nil {
				return server.GetAvailableTools()
			}
		}
	}
	return []string{}
}

// InitializeServices creates and registers all required services
func InitializeServices(cfg *Config) (*Services, error) {
	// Create orchestrator without ToolCaller initially
	orchConfig := orchestrator.Config{
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

	// Get the service registry handler from the API
	registryHandler := api.GetServiceRegistry()
	if registryHandler == nil {
		return nil, fmt.Errorf("service registry handler not available")
	}

	// Create the tool checker using the API handler
	toolChecker := &aggregatorToolChecker{serviceRegistry: registry}

	// Create ServiceClass manager (now uses layered configuration loading)
	serviceClassManager, err := serviceclass.NewServiceClassManager(toolChecker)
	if err != nil {
		return nil, fmt.Errorf("failed to create ServiceClass manager: %w", err)
	}

	// Create and register ServiceClass adapter
	serviceClassAdapter := serviceclass.NewAdapter(serviceClassManager)
	serviceClassAdapter.Register()

	// Load ServiceClass definitions
	if err := serviceClassManager.LoadServiceDefinitions(); err != nil {
		// Log warning but don't fail - ServiceClass is optional
		logging.Warn("Services", "Failed to load ServiceClass definitions: %v", err)
	}

	// Initialize and register Capability manager
	capabilityManager, err := capability.NewCapabilityManager(toolChecker, capability.NewRegistry())
	if err != nil {
		return nil, fmt.Errorf("failed to create Capability manager: %w", err)
	}

	// Create and register Capability adapter
	// Note: We'll need to pass a ToolCaller when it becomes available
	capabilityAdapter, err := capability.NewAdapter(toolChecker, nil) // ToolCaller will be set later
	if err != nil {
		return nil, fmt.Errorf("failed to create Capability adapter: %w", err)
	}
	capabilityAdapter.Register()

	// Load Capability definitions
	if err := capabilityManager.LoadDefinitions(); err != nil {
		// Log warning but don't fail - Capability is optional
		logging.Warn("Services", "Failed to load Capability definitions: %v", err)
	}

	// Initialize and register Workflow storage
	// Auto-detect config directory for workflow storage
	configDir, err := config.GetUserConfigDir()
	if err != nil {
		// Fallback to empty string if auto-detection fails
		configDir = ""
	}

	workflowStorage, err := workflow.NewWorkflowStorage(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create Workflow storage: %w", err)
	}

	// Create and register Workflow adapter
	// Note: We'll need to pass a ToolCaller when it becomes available
	workflowAdapter, err := workflow.NewAdapter(configDir, nil, toolChecker) // ToolCaller will be set later
	if err != nil {
		return nil, fmt.Errorf("failed to create Workflow adapter: %w", err)
	}
	workflowAdapter.Register()

	// Load Workflow definitions (already done in NewWorkflowStorage, but we can reload if needed)
	if err := workflowStorage.LoadWorkflows(); err != nil {
		// Log warning but don't fail - Workflow is optional
		logging.Warn("Services", "Failed to load Workflow definitions: %v", err)
	}

	// Initialize and register MCPServer manager (new unified configuration approach)
	mcpServerManager, err := mcpserverPkg.NewMCPServerManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP server manager: %w", err)
	}

	// Create and register MCPServer adapter
	mcpServerAdapter := mcpserverPkg.NewAdapter(mcpServerManager)
	mcpServerAdapter.Register()

	// Load MCP server definitions from directories
	if err := mcpServerManager.LoadDefinitions(); err != nil {
		// Log warning but don't fail - MCP servers are optional
		logging.Warn("Services", "Failed to load MCP server definitions: %v", err)
	}

	// Step 2: Create APIs that use the registered handlers
	orchestratorAPI := api.NewOrchestratorAPI()
	mcpAPI := api.NewMCPServiceAPI()
	// Register the MCP service API so mcp_server_* tools can work
	api.SetMCPServiceAPI(mcpAPI)
	aggregatorAPI := api.NewAggregatorAPI()
	configAPI := api.NewConfigServiceAPI()

	// Step 3: Create and register actual services
	// Create MCP server services using the new directory-based loading
	mcpServerDefinitions := mcpServerManager.ListDefinitions()
	for _, mcpDef := range mcpServerDefinitions {
		if mcpDef.Enabled {
			mcpService := mcpserver.NewMCPServerService(mcpDef)
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
					if aggSvc, ok := service.(interface {
						GetManager() *aggregator.AggregatorManager
					}); ok {
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

								// Refresh Capability availability now that ToolCaller is available
								capabilityManager.RefreshAvailability()
								logging.Info("Bootstrap", "Refreshed Capability availability after ToolCaller setup")
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
