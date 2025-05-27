package mcpserver

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/mcpserver"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// mcpServerStarter is an interface for starting MCP servers (used for testing)
type mcpServerStarter interface {
	StartAndManageIndividualMcpServer(
		serverConfig config.MCPServerDefinition,
		updateFn mcpserver.McpUpdateFunc,
		wg *sync.WaitGroup,
	) (pid int, stopChan chan struct{}, initialError error)
}

// defaultMCPServerStarter implements mcpServerStarter using the real mcpserver package
type defaultMCPServerStarter struct{}

func (d *defaultMCPServerStarter) StartAndManageIndividualMcpServer(
	serverConfig config.MCPServerDefinition,
	updateFn mcpserver.McpUpdateFunc,
	wg *sync.WaitGroup,
) (pid int, stopChan chan struct{}, initialError error) {
	return mcpserver.StartAndManageIndividualMcpServer(serverConfig, updateFn, wg)
}

// MCPServerService implements the Service interface for MCP servers
type MCPServerService struct {
	*services.BaseService

	mu       sync.RWMutex
	config   config.MCPServerDefinition
	pid      int
	port     int
	stopChan chan struct{}

	// Internal state from the mcpserver package
	updateChan chan mcpserver.McpDiscreteStatusUpdate

	// For testing
	starter mcpServerStarter
}

// NewMCPServerService creates a new MCP server service
func NewMCPServerService(cfg config.MCPServerDefinition) *MCPServerService {
	deps := []string{}
	// Add dependencies based on configuration
	if cfg.RequiresPortForwards != nil {
		deps = append(deps, cfg.RequiresPortForwards...)
	}

	return &MCPServerService{
		BaseService: services.NewBaseService(cfg.Name, services.TypeMCPServer, deps),
		config:      cfg,
		updateChan:  make(chan mcpserver.McpDiscreteStatusUpdate, 10),
		starter:     &defaultMCPServerStarter{},
	}
}

// Start starts the MCP server
func (s *MCPServerService) Start(ctx context.Context) error {
	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

	// Create update function that converts internal updates to service updates
	updateFn := func(update mcpserver.McpDiscreteStatusUpdate) {
		select {
		case s.updateChan <- update:
		case <-ctx.Done():
		}
	}

	// Start the process monitor goroutine
	go s.monitorProcess(ctx)

	// Start the MCP server process
	wg := &sync.WaitGroup{}
	wg.Add(1)

	pid, stopChan, err := s.starter.StartAndManageIndividualMcpServer(
		s.config,
		updateFn,
		wg,
	)

	if err != nil {
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to start MCP server %s: %w", s.config.Name, err)
	}

	s.mu.Lock()
	s.pid = pid
	s.stopChan = stopChan
	s.mu.Unlock()

	return nil
}

// Stop stops the MCP server
func (s *MCPServerService) Stop(ctx context.Context) error {
	s.UpdateState(services.StateStopping, s.GetHealth(), nil)

	s.mu.RLock()
	stopChan := s.stopChan
	s.mu.RUnlock()

	if stopChan != nil {
		// Send stop signal
		close(stopChan)

		// Wait for process to stop with timeout from context
		done := make(chan struct{})
		go func() {
			// Give the process some time to stop gracefully
			time.Sleep(500 * time.Millisecond)
			close(done)
		}()

		select {
		case <-done:
			// Process stopped gracefully
		case <-ctx.Done():
			// Context cancelled, force stop
			logging.Warn("MCPServerService", "Context cancelled while stopping MCP server %s", s.config.Name)
			return ctx.Err()
		}
	}

	s.mu.Lock()
	s.pid = 0
	s.port = 0
	s.stopChan = nil
	s.mu.Unlock()

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	return nil
}

// Restart restarts the MCP server
func (s *MCPServerService) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop MCP server %s: %w", s.config.Name, err)
	}

	// Small delay before restarting
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	return s.Start(ctx)
}

// GetServiceData implements ServiceDataProvider
func (s *MCPServerService) GetServiceData() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := map[string]interface{}{
		"name":    s.config.Name,
		"command": s.config.Command,
		"icon":    s.config.Icon,
		"enabled": s.config.Enabled,
	}

	if s.pid > 0 {
		data["pid"] = s.pid
	}

	if s.port > 0 {
		data["port"] = s.port
	} else if s.config.ProxyPort > 0 {
		// Use configured port if actual port not yet detected
		data["port"] = s.config.ProxyPort
	}

	return data
}

// CheckHealth implements HealthChecker
func (s *MCPServerService) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	s.mu.RLock()
	port := s.port
	if port == 0 && s.config.ProxyPort > 0 {
		port = s.config.ProxyPort
	}
	s.mu.RUnlock()

	var health services.HealthStatus
	var err error

	if port == 0 {
		health = services.HealthUnknown
		err = fmt.Errorf("MCP server port not available yet")
	} else if s.GetState() == services.StateRunning {
		// TODO: Implement actual health check against the MCP server endpoint
		// For now, just check if the process is running
		health = services.HealthHealthy
		err = nil
	} else {
		health = services.HealthUnhealthy
		err = nil
	}

	// Update the service's health status
	s.UpdateHealth(health)

	return health, err
}

// GetHealthCheckInterval implements HealthChecker
func (s *MCPServerService) GetHealthCheckInterval() time.Duration {
	// MCP servers should be checked every 10 seconds for more responsive health updates
	return 10 * time.Second
}

// monitorProcess monitors the MCP server process for updates
func (s *MCPServerService) monitorProcess(ctx context.Context) {
	for {
		select {
		case update := <-s.updateChan:
			s.handleProcessUpdate(update)
		case <-ctx.Done():
			return
		}
	}
}

// handleProcessUpdate handles updates from the MCP server process
func (s *MCPServerService) handleProcessUpdate(update mcpserver.McpDiscreteStatusUpdate) {
	// Update internal state
	s.mu.Lock()
	if update.PID > 0 {
		s.pid = update.PID
	}
	if update.ProxyPort > 0 {
		s.port = update.ProxyPort
	}
	s.mu.Unlock()

	// Map process status to service state
	var state services.ServiceState
	var health services.HealthStatus

	switch update.ProcessStatus {
	case "ProcessInitializing", "ProcessStarting":
		state = services.StateStarting
		health = services.HealthUnknown
	case "ProcessRunning":
		state = services.StateRunning
		health = services.HealthHealthy
	case "ProcessStoppedByUser", "ProcessExitedGracefully":
		state = services.StateStopped
		health = services.HealthUnknown
	case "ProcessStartFailed", "ProcessExitedWithError", "ProcessKillFailed":
		state = services.StateFailed
		health = services.HealthUnhealthy
	default:
		state = services.StateUnknown
		health = services.HealthUnknown
	}

	s.UpdateState(state, health, update.ProcessErr)
}
