package app

import (
	"envctl/internal/api"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
)

// Services holds all the initialized services and APIs
type Services struct {
	Orchestrator    *orchestrator.Orchestrator
	OrchestratorAPI api.OrchestratorAPI
	MCPAPI          api.MCPServiceAPI
	PortForwardAPI  api.PortForwardServiceAPI
	K8sAPI          api.K8sServiceAPI
	AggregatorAPI   api.AggregatorAPI
	AggregatorPort  int
}

// InitializeServices creates and registers all required services
func InitializeServices(cfg *Config) (*Services, error) {
	// Create the orchestrator
	orchConfig := orchestrator.Config{
		MCName:       cfg.ManagementCluster,
		WCName:       cfg.WorkloadCluster,
		PortForwards: cfg.EnvctlConfig.PortForwards,
		MCPServers:   cfg.EnvctlConfig.MCPServers,
		Aggregator:   cfg.EnvctlConfig.Aggregator,
		Yolo:         cfg.Yolo,
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

	// Step 2: Create APIs that use the registered handlers
	orchestratorAPI := api.NewOrchestratorAPI()
	mcpAPI := api.NewMCPServiceAPI()
	portForwardAPI := api.NewPortForwardServiceAPI()
	k8sAPI := api.NewK8sServiceAPI()
	aggregatorAPI := api.NewAggregatorAPI()

	// Register all APIs in the global registry (for backward compatibility)
	api.SetAll(orchestratorAPI, mcpAPI, portForwardAPI, k8sAPI)

	return &Services{
		Orchestrator:    orch,
		OrchestratorAPI: orchestratorAPI,
		MCPAPI:          mcpAPI,
		PortForwardAPI:  portForwardAPI,
		K8sAPI:          k8sAPI,
		AggregatorAPI:   aggregatorAPI,
		AggregatorPort:  cfg.EnvctlConfig.Aggregator.Port,
	}, nil
}
