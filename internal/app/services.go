package app

import (
	"envctl/internal/adapters"
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/orchestrator"
	"envctl/internal/services"
	agg "envctl/internal/services/aggregator"
	"envctl/pkg/logging"
	"fmt"
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
	}

	// Use new config if clusters are defined
	if len(cfg.EnvctlConfig.Clusters) > 0 {
		orchConfig.Clusters = cfg.EnvctlConfig.Clusters
		orchConfig.ActiveClusters = cfg.EnvctlConfig.ActiveClusters
	}

	orch := orchestrator.New(orchConfig)

	// Get the service registry
	registry := orch.GetServiceRegistry()

	// Create APIs
	orchestratorAPI := api.NewOrchestratorAPI(orch, registry)
	mcpAPI := api.NewMCPServiceAPI(registry)
	portForwardAPI := api.NewPortForwardServiceAPI(registry)
	k8sAPI := api.NewK8sServiceAPI(registry)
	aggregatorAPI := api.NewAggregatorAPI(registry)

	// Register all APIs in the global registry
	api.SetAll(orchestratorAPI, mcpAPI, portForwardAPI, k8sAPI)

	// Initialize aggregator if enabled
	if err := initializeAggregator(cfg.EnvctlConfig, orchestratorAPI, mcpAPI, registry); err != nil {
		return nil, fmt.Errorf("failed to initialize aggregator: %w", err)
	}

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

// initializeAggregator creates and registers the aggregator service
func initializeAggregator(
	cfg *config.EnvctlConfig,
	orchestratorAPI api.OrchestratorAPI,
	mcpAPI api.MCPServiceAPI,
	registry services.ServiceRegistry,
) error {
	// Set default port if not configured
	aggPort := cfg.Aggregator.Port
	if aggPort == 0 {
		aggPort = 8080
	}

	// Create aggregator configuration
	aggConfig := aggregator.AggregatorConfig{
		Host: cfg.Aggregator.Host,
		Port: aggPort,
	}
	if aggConfig.Host == "" {
		aggConfig.Host = "localhost"
	}

	// Create adapters for the aggregator
	orchestratorEventAdapter := adapters.NewOrchestratorEventAdapter(orchestratorAPI)
	mcpServiceAdapter := adapters.NewMCPServiceAdapter(mcpAPI, registry)

	// Create aggregator service
	aggService := agg.NewAggregatorService(aggConfig, orchestratorEventAdapter, mcpServiceAdapter)

	// Register the aggregator service
	if err := registry.Register(aggService); err != nil {
		return fmt.Errorf("failed to register aggregator service: %w", err)
	}

	logging.Info("Services", "Registered MCP aggregator service on port %d", aggPort)
	return nil
}
