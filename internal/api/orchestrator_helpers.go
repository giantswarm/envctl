package api

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/services"
	agg "envctl/internal/services/aggregator"
	"envctl/pkg/logging"
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
	SetGlobalStateChangeCallback(callback services.StateChangeCallback)
}

// OrchestratorWithGetter extends OrchestratorWithFactory to get the current callback
type OrchestratorWithGetter interface {
	OrchestratorWithFactory
	GetGlobalStateChangeCallback() services.StateChangeCallback
}

func SetupAggregatorFactory(orch OrchestratorWithFactory, registry services.ServiceRegistry) {
	var aggregatorService *agg.AggregatorService

	// Try to get the existing callback if the orchestrator supports it
	var existingCallback services.StateChangeCallback
	if orchWithGetter, ok := orch.(OrchestratorWithGetter); ok {
		existingCallback = orchWithGetter.GetGlobalStateChangeCallback()
	}

	// Create the factory function that captures the registry
	factory := func(config aggregator.AggregatorConfig) services.Service {
		// Create MCP client provider using the registry
		clientProvider := NewMCPClientProvider(registry)

		// Create and return the aggregator service with the client provider
		return agg.NewAggregatorService(config, clientProvider)
	}

	// Set the factory on the orchestrator
	orch.SetAggregatorServiceFactory(factory)

	// Wrap the global state change callback to monitor MCP server changes
	orch.SetGlobalStateChangeCallback(func(label string, oldState, newState services.ServiceState, health services.HealthStatus, err error) {
		// Log all state changes for debugging
		logging.Debug("Aggregator", "State change callback fired: %s changed from %s to %s (health: %s)",
			label, oldState, newState, health)

		// Call the existing callback if it exists
		if existingCallback != nil {
			existingCallback(label, oldState, newState, health, err)
		}

		// Check if this is an MCP server state change
		if service, exists := registry.Get(label); exists && service.GetType() == services.TypeMCPServer {
			// Only refresh if the aggregator is running
			if aggregatorService != nil && aggregatorService.GetState() == services.StateRunning {
				logging.Info("Aggregator", "MCP server %s changed state from %s to %s, refreshing aggregator",
					label, oldState, newState)

				// Refresh in a goroutine to avoid blocking
				go func() {
					ctx := context.Background()
					if err := aggregatorService.RefreshMCPServers(ctx); err != nil {
						logging.Error("Aggregator", err, "Failed to refresh MCP servers after state change")
					}
				}()
			} else {
				if aggregatorService != nil {
					logging.Debug("Aggregator", "MCP server %s changed state but aggregator is not running (state: %v)",
						label, aggregatorService.GetState())
				} else {
					logging.Debug("Aggregator", "MCP server %s changed state but aggregator service is nil", label)
				}
			}
		}

		// Also check if this is the aggregator starting up
		if label == "mcp-aggregator" && newState == services.StateRunning {
			// Initial refresh when aggregator starts
			go func() {
				ctx := context.Background()
				if err := aggregatorService.RefreshMCPServers(ctx); err != nil {
					logging.Error("Aggregator", err, "Failed to refresh MCP servers after aggregator startup")
				}
			}()
		}
	})
}
