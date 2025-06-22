package mcpserver

import (
	"context"
	"fmt"
	"time"

	"envctl/internal/api"
	"envctl/internal/mcpserver"
	"envctl/internal/services"
	"envctl/pkg/logging"
)

// Service implements the Service interface for MCP server management
type Service struct {
	*services.BaseService
	definition *api.MCPServer
	runner     Runner
	manager    *mcpserver.MCPServerManager
}

// Runner interface for different MCP server execution types
type Runner interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning(ctx context.Context) bool
}

// NewService creates a new MCP server service
func NewService(definition *api.MCPServer, manager *mcpserver.MCPServerManager) (*Service, error) {
	baseService := services.NewBaseService(definition.Name, services.TypeMCPServer, []string{})

	service := &Service{
		BaseService: baseService,
		definition:  definition,
		manager:     manager,
	}

	// Create appropriate runner based on type
	var err error
	service.runner, err = service.createRunner()
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	return service, nil
}

// createRunner creates the appropriate runner for the MCP server type
func (s *Service) createRunner() (Runner, error) {
	switch s.definition.Type {
	case api.MCPServerTypeLocalCommand:
		return mcpserver.NewProcessRunner(s.definition), nil
	case api.MCPServerTypeContainer:
		return mcpserver.NewContainerRunner(s.definition)
	default:
		return nil, fmt.Errorf("unsupported MCP server type: %s", s.definition.Type)
	}
}

// Start starts the MCP server service
func (s *Service) Start(ctx context.Context) error {
	if s.IsRunning() {
		return fmt.Errorf("service %s is already running", s.GetName())
	}

	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)
	s.LogInfo("Starting MCP server service")

	// Start the runner
	if err := s.runner.Start(ctx); err != nil {
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Wait a moment for the server to initialize
	time.Sleep(100 * time.Millisecond)

	// Check if it's actually running
	if s.runner.IsRunning(ctx) {
		s.UpdateState(services.StateRunning, services.HealthHealthy, nil)
		s.LogInfo("MCP server started successfully")
	} else {
		err := fmt.Errorf("MCP server failed to start properly")
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return err
	}

	return nil
}

// Stop stops the MCP server service
func (s *Service) Stop(ctx context.Context) error {
	if !s.IsRunning() {
		s.LogDebug("Service %s is not running, nothing to stop", s.GetName())
		return nil
	}

	s.UpdateState(services.StateStopping, s.GetHealth(), nil)
	s.LogInfo("Stopping MCP server service")

	// Stop the runner
	if err := s.runner.Stop(ctx); err != nil {
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to stop MCP server: %w", err)
	}

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	s.LogInfo("MCP server stopped successfully")

	return nil
}

// Restart restarts the MCP server service
func (s *Service) Restart(ctx context.Context) error {
	s.LogInfo("Restarting MCP server service")

	if s.IsRunning() {
		if err := s.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop service during restart: %w", err)
		}
	}

	// Wait a moment between stop and start
	time.Sleep(200 * time.Millisecond)

	return s.Start(ctx)
}

// IsRunning checks if the MCP server is running
func (s *Service) IsRunning() bool {
	return s.GetState() == services.StateRunning
}

// IsHealthy checks if the MCP server is healthy
func (s *Service) IsHealthy() bool {
	return s.GetHealth() == services.HealthHealthy && s.IsRunning()
}

// GetServiceType returns the service type
func (s *Service) GetServiceType() string {
	return "mcpserver"
}

// GetConfiguration returns the MCP server configuration
func (s *Service) GetConfiguration() interface{} {
	return s.definition
}

// ValidateConfiguration validates the MCP server configuration
func (s *Service) ValidateConfiguration() error {
	if s.definition == nil {
		return fmt.Errorf("MCP server definition is nil")
	}

	if s.definition.Name == "" {
		return fmt.Errorf("MCP server name is required")
	}

	// Type-specific validation
	switch s.definition.Type {
	case api.MCPServerTypeLocalCommand:
		if len(s.definition.Command) == 0 {
			return fmt.Errorf("command is required for localCommand type")
		}
	case api.MCPServerTypeContainer:
		if s.definition.Image == "" {
			return fmt.Errorf("image is required for container type")
		}
	default:
		return fmt.Errorf("unsupported MCP server type: %s", s.definition.Type)
	}

	return nil
}

// UpdateConfiguration updates the MCP server configuration
func (s *Service) UpdateConfiguration(newConfig interface{}) error {
	newDef, ok := newConfig.(*api.MCPServer)
	if !ok {
		return fmt.Errorf("invalid configuration type for MCP server")
	}

	s.definition = newDef

	// Recreate runner if type changed
	var err error
	s.runner, err = s.createRunner()
	if err != nil {
		return fmt.Errorf("failed to recreate runner: %w", err)
	}

	return nil
}

// GetServiceData implements ServiceDataProvider
func (s *Service) GetServiceData() map[string]interface{} {
	data := map[string]interface{}{
		"name":      s.definition.Name,
		"type":      s.definition.Type,
		"autoStart": s.definition.AutoStart,
		"state":     s.GetState(),
		"health":    s.GetHealth(),
	}

	if s.definition.Type == api.MCPServerTypeLocalCommand {
		data["command"] = s.definition.Command
	} else if s.definition.Type == api.MCPServerTypeContainer {
		data["image"] = s.definition.Image
	}

	if s.GetLastError() != nil {
		data["error"] = s.GetLastError().Error()
	}

	return data
}

// CheckHealth implements HealthChecker
func (s *Service) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	if !s.runner.IsRunning(ctx) {
		s.UpdateHealth(services.HealthUnhealthy)
		return services.HealthUnhealthy, fmt.Errorf("MCP server process is not running")
	}

	s.UpdateHealth(services.HealthHealthy)
	return services.HealthHealthy, nil
}

// GetHealthCheckInterval implements HealthChecker
func (s *Service) GetHealthCheckInterval() time.Duration {
	if s.definition.HealthCheckInterval > 0 {
		return s.definition.HealthCheckInterval
	}
	// Default health check interval
	return 30 * time.Second
}

// GetLogContext returns the logging context for this service
func (s *Service) GetLogContext() string {
	return fmt.Sprintf("MCPServerService-%s", s.GetName())
}

// LogInfo logs an info message with service context
func (s *Service) LogInfo(format string, args ...interface{}) {
	logging.Info(s.GetLogContext(), format, args...)
}

// LogDebug logs a debug message with service context
func (s *Service) LogDebug(format string, args ...interface{}) {
	logging.Debug(s.GetLogContext(), format, args...)
}

// LogError logs an error message with service context
func (s *Service) LogError(err error, format string, args ...interface{}) {
	logging.Error(s.GetLogContext(), err, format, args...)
}

// LogWarn logs a warning message with service context
func (s *Service) LogWarn(format string, args ...interface{}) {
	logging.Warn(s.GetLogContext(), format, args...)
}
