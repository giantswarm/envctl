package aggregator

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/services"
	"envctl/internal/services/mcpserver"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// AggregatorService implements the Service interface for the MCP aggregator
type AggregatorService struct {
	*services.BaseService

	mu       sync.RWMutex
	config   aggregator.AggregatorConfig
	server   *aggregator.AggregatorServer
	registry services.ServiceRegistry
}

// NewAggregatorService creates a new aggregator service
func NewAggregatorService(config aggregator.AggregatorConfig, registry services.ServiceRegistry) *AggregatorService {
	// The aggregator depends on all MCP servers being ready
	// We'll dynamically add dependencies based on registered MCP servers
	deps := []string{}

	return &AggregatorService{
		BaseService: services.NewBaseService("mcp-aggregator", services.ServiceType("Aggregator"), deps),
		config:      config,
		registry:    registry,
	}
}

// Start starts the aggregator service
func (s *AggregatorService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.GetState() == services.StateRunning {
		return nil
	}

	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

	// Create the aggregator server
	s.server = aggregator.NewAggregatorServer(s.config)

	// Start the aggregator server
	if err := s.server.Start(ctx); err != nil {
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to start aggregator server: %w", err)
	}

	// Register all MCP servers with the aggregator
	if err := s.registerMCPServers(ctx); err != nil {
		s.server.Stop(ctx)
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to register MCP servers: %w", err)
	}

	s.UpdateState(services.StateRunning, services.HealthHealthy, nil)

	// Start monitoring for MCP server changes
	go s.monitorMCPServers(ctx)

	logging.Info("Aggregator", "Started MCP aggregator service on %s", s.server.GetEndpoint())
	return nil
}

// Stop stops the aggregator service
func (s *AggregatorService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.GetState() != services.StateRunning {
		return nil
	}

	s.UpdateState(services.StateStopping, s.GetHealth(), nil)

	if s.server != nil {
		if err := s.server.Stop(ctx); err != nil {
			logging.Error("Aggregator", err, "Error stopping aggregator server")
		}
		s.server = nil
	}

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)

	logging.Info("Aggregator", "Stopped MCP aggregator service")
	return nil
}

// Restart restarts the aggregator service
func (s *AggregatorService) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop aggregator service: %w", err)
	}

	// Small delay before restarting
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	return s.Start(ctx)
}

// CheckHealth implements HealthChecker
func (s *AggregatorService) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.GetState() != services.StateRunning {
		return services.HealthUnknown, nil
	}

	// The aggregator is healthy if it's running
	// We could add more sophisticated health checks here
	s.UpdateHealth(services.HealthHealthy)
	return services.HealthHealthy, nil
}

// GetHealthCheckInterval implements HealthChecker
func (s *AggregatorService) GetHealthCheckInterval() time.Duration {
	return 30 * time.Second
}

// GetServiceData implements ServiceDataProvider
func (s *AggregatorService) GetServiceData() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := map[string]interface{}{
		"port": s.config.Port,
		"host": s.config.Host,
	}

	if s.server != nil {
		data["endpoint"] = s.server.GetEndpoint()
		data["tools"] = len(s.server.GetTools())
		data["resources"] = len(s.server.GetResources())
		data["prompts"] = len(s.server.GetPrompts())

		// Get connected servers count
		registry := s.server.GetRegistry()
		if registry != nil {
			servers := registry.GetAllServers()
			connected := 0
			for _, info := range servers {
				if info.IsConnected() {
					connected++
				}
			}
			data["servers_total"] = len(servers)
			data["servers_connected"] = connected
		}
	}

	return data
}

// registerMCPServers registers all running MCP servers with the aggregator
func (s *AggregatorService) registerMCPServers(ctx context.Context) error {
	// Get all MCP server services from the registry
	mcpServers := s.registry.GetByType(services.TypeMCPServer)

	for _, svc := range mcpServers {
		// Only register running servers
		if svc.GetState() != services.StateRunning {
			continue
		}

		// Get the MCP client from the service
		if mcpSvc, ok := svc.(*mcpserver.MCPServerService); ok {
			client := mcpSvc.GetMCPClient()
			if client != nil {
				// Register with the aggregator
				if err := s.server.RegisterServer(ctx, svc.GetLabel(), client); err != nil {
					logging.Warn("Aggregator", "Failed to register MCP server %s: %v", svc.GetLabel(), err)
					// Continue with other servers
				} else {
					logging.Info("Aggregator", "Registered MCP server %s with aggregator", svc.GetLabel())
				}
			}
		}
	}

	return nil
}

// monitorMCPServers monitors for MCP server state changes
func (s *AggregatorService) monitorMCPServers(ctx context.Context) {
	// This is a simplified version - in production, we'd want to
	// subscribe to service state change events
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			server := s.server
			s.mu.RUnlock()

			if server == nil {
				continue
			}

			// Check for new MCP servers to register
			mcpServers := s.registry.GetByType(services.TypeMCPServer)
			registry := server.GetRegistry()

			for _, svc := range mcpServers {
				label := svc.GetLabel()

				// Check if already registered
				if _, exists := registry.GetServerInfo(label); exists {
					continue
				}

				// Register new running servers
				if svc.GetState() == services.StateRunning {
					if mcpSvc, ok := svc.(*mcpserver.MCPServerService); ok {
						client := mcpSvc.GetMCPClient()
						if client != nil {
							if err := server.RegisterServer(ctx, label, client); err != nil {
								logging.Warn("Aggregator", "Failed to register new MCP server %s: %v", label, err)
							} else {
								logging.Info("Aggregator", "Registered new MCP server %s with aggregator", label)
							}
						}
					}
				}
			}

			// Check for servers to deregister
			for name := range registry.GetAllServers() {
				found := false
				for _, svc := range mcpServers {
					if svc.GetLabel() == name && svc.GetState() == services.StateRunning {
						found = true
						break
					}
				}

				if !found {
					if err := server.DeregisterServer(name); err != nil {
						logging.Warn("Aggregator", "Failed to deregister MCP server %s: %v", name, err)
					} else {
						logging.Info("Aggregator", "Deregistered MCP server %s from aggregator", name)
					}
				}
			}
		}
	}
}

// GetEndpoint returns the aggregator's SSE endpoint URL
func (s *AggregatorService) GetEndpoint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.server != nil {
		return s.server.GetEndpoint()
	}

	return ""
}
