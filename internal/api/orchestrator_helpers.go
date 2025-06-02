package api

import (
	"envctl/internal/aggregator"
	"envctl/internal/services"
	agg "envctl/internal/services/aggregator"
)

// SetupAggregatorFactory creates and sets the aggregator service factory on the orchestrator.
// This helper function allows the orchestrator to create aggregator services with proper
// MCP client provider support without creating import cycles.
//
// Usage:
//
//	orchestrator := orchestrator.New(config)
//	api.SetupAggregatorFactory(orchestrator, registry)
type OrchestratorWithFactory interface {
	SetAggregatorServiceFactory(factory func(config aggregator.AggregatorConfig) services.Service)
}

func SetupAggregatorFactory(orch OrchestratorWithFactory, registry services.ServiceRegistry) {
	// Create the factory function that captures the registry
	factory := func(config aggregator.AggregatorConfig) services.Service {
		// Create MCP client provider using the registry
		clientProvider := NewMCPClientProvider(registry)

		// Create and return the aggregator service with the client provider
		return agg.NewAggregatorService(config, clientProvider)
	}

	// Set the factory on the orchestrator
	orch.SetAggregatorServiceFactory(factory)
}
