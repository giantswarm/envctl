package app

import (
	"envctl/internal/api"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	"envctl/internal/services/mcpserver"

	// Import to trigger init() functions that register adapter factories
	_ "envctl/internal/capability"
	_ "envctl/internal/workflow"
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
		MCName:     cfg.ManagementCluster,
		WCName:     cfg.WorkloadCluster,
		MCPServers: cfg.EnvctlConfig.MCPServers,
		Aggregator: cfg.EnvctlConfig.Aggregator,
		Yolo:       cfg.Yolo,
	}

	// Use new config if clusters are defined
	if len(cfg.EnvctlConfig.Clusters) > 0 {
		orchConfig.Clusters = cfg.EnvctlConfig.Clusters
		orchConfig.ActiveClusters = cfg.EnvctlConfig.ActiveClusters
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

	// Step 2: Create APIs that use the registered handlers
	orchestratorAPI := api.NewOrchestratorAPI()
	mcpAPI := api.NewMCPServiceAPI()
	aggregatorAPI := api.NewAggregatorAPI()
	configAPI := api.NewConfigServiceAPI()

	return &Services{
		Orchestrator:    orch,
		OrchestratorAPI: orchestratorAPI,
		MCPAPI:          mcpAPI,
		AggregatorAPI:   aggregatorAPI,
		ConfigAPI:       configAPI,
		AggregatorPort:  cfg.EnvctlConfig.Aggregator.Port,
	}, nil
}
