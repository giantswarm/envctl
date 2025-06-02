package mcpserver

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/containerizer"
	"envctl/internal/mcpserver"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MCPServerService implements the Service interface for MCP servers
type MCPServerService struct {
	*services.BaseService

	// Immutable configuration (no mutex needed)
	config config.MCPServerDefinition

	// Runtime state (minimal mutex protection needed)
	mu            sync.RWMutex
	managedServer *mcpserver.ManagedMcpServer
	containerID   string
	stopChan      chan struct{}

	// Lifecycle management
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Container runtime (initialized once, then read-only)
	containerRuntime atomic.Pointer[containerizer.ContainerRuntime]
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
	}
}

// Start starts the MCP server
func (s *MCPServerService) Start(ctx context.Context) error {
	// Quick state check - BaseService handles its own locking
	if s.GetState() == services.StateRunning {
		return nil
	}

	// Create cancellable context
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)
	logging.Debug("MCPServer", "Starting MCP server %s (type: %s)", s.config.Name, s.config.Type)

	// Update function to receive status updates
	updateFn := func(update mcpserver.McpDiscreteStatusUpdate) {
		logging.Debug("MCPServer", "Status update for %s: %s", update.Label, update.ProcessStatus)

		// Convert MCP status to common status
		var status string
		switch update.ProcessStatus {
		case "ProcessInitializing", "ContainerInitializing":
			status = services.StatusInitializing
		case "ProcessRunning", "ContainerRunning":
			status = services.StatusRunning
		case "ProcessUnhealthy", "ContainerUnhealthy":
			status = services.StatusUnhealthy
		case "ProcessStartFailed", "ContainerStartFailed":
			status = services.StatusFailed
		case "ProcessExitedWithError", "ContainerExited":
			status = services.StatusFailed
		case "ProcessStoppedByUser", "ContainerStoppedByUser":
			status = services.StatusStopped
		default:
			status = update.ProcessStatus
		}

		// Create common status update
		statusUpdate := services.StatusUpdate{
			Label:   update.Label,
			Status:  status,
			IsError: update.ProcessErr != nil,
			IsReady: status == services.StatusRunning,
			Error:   update.ProcessErr,
		}

		// Map to service state and health
		newState := services.MapStatusToState(statusUpdate.Status)
		newHealth := services.MapStatusToHealth(statusUpdate.Status, statusUpdate.IsError)

		// Special case: if status is "unhealthy", only update health, not state
		if status == services.StatusUnhealthy {
			s.UpdateHealth(newHealth)
		} else {
			s.UpdateState(newState, newHealth, statusUpdate.Error)
		}
	}

	var err error
	switch s.config.Type {
	case config.MCPServerTypeLocalCommand:
		err = s.startLocalCommand(updateFn)
	case config.MCPServerTypeContainer:
		err = s.startContainer(updateFn)
	default:
		err = fmt.Errorf("unsupported server type: %s", s.config.Type)
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
	}

	if err != nil {
		return err
	}

	logging.Info("MCPServer", "Started MCP server process: %s", s.config.Name)
	return nil
}

// startLocalCommand starts a local command MCP server
func (s *MCPServerService) startLocalCommand(updateFn mcpserver.McpUpdateFunc) error {
	s.wg.Add(1)
	managedServer, err := mcpserver.StartAndManageIndividualMcpServer(s.config, updateFn, &s.wg)
	if err != nil {
		s.wg.Done()
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to start local MCP server: %w", err)
	}

	// Only lock when setting the runtime state
	s.mu.Lock()
	s.managedServer = managedServer
	s.stopChan = managedServer.StopChan
	s.mu.Unlock()

	return nil
}

// startContainer starts a containerized MCP server
func (s *MCPServerService) startContainer(updateFn mcpserver.McpUpdateFunc) error {
	// Get or create container runtime
	runtime := s.containerRuntime.Load()
	if runtime == nil {
		newRuntime, err := containerizer.NewContainerRuntime("docker") // TODO: Get from global config
		if err != nil {
			s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
			return fmt.Errorf("failed to initialize container runtime: %w", err)
		}
		// Try to set it atomically
		if !s.containerRuntime.CompareAndSwap(nil, &newRuntime) {
			// Another goroutine set it first, use theirs
			runtime = s.containerRuntime.Load()
		} else {
			runtime = &newRuntime
		}
	}

	s.wg.Add(1)
	containerID, stopChan, err := mcpserver.StartAndManageContainerizedMcpServer(
		s.config,
		*runtime,
		updateFn,
		&s.wg,
	)
	if err != nil {
		s.wg.Done()
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to start containerized MCP server: %w", err)
	}

	// Only lock when setting the runtime state
	s.mu.Lock()
	s.containerID = containerID
	s.stopChan = stopChan
	s.mu.Unlock()

	return nil
}

// Stop stops the MCP server
func (s *MCPServerService) Stop(ctx context.Context) error {
	// Always transition to stopping state first
	s.UpdateState(services.StateStopping, s.GetHealth(), nil)

	// Get the stop channel and cancel func
	s.mu.RLock()
	stopChan := s.stopChan
	s.mu.RUnlock()

	// Cancel context
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// Signal stop through the stop channel
	if stopChan != nil {
		close(stopChan)
	}

	// Wait for the management goroutine to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// Clean shutdown
	case <-time.After(10 * time.Second):
		logging.Warn("MCPServer", "Timeout waiting for MCP server %s to stop", s.config.Name)
	}

	// Clean up references
	s.mu.Lock()
	s.managedServer = nil
	s.stopChan = nil
	s.containerID = ""
	s.mu.Unlock()

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	logging.Info("MCPServer", "Stopped MCP server: %s", s.config.Name)
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
	managedServer := s.managedServer
	containerID := s.containerID
	s.mu.RUnlock()

	data := map[string]interface{}{
		"name":    s.config.Name,
		"command": s.config.Command,
		"icon":    s.config.Icon,
		"enabled": s.config.Enabled,
		"type":    s.config.Type,
	}

	if managedServer != nil && managedServer.PID > 0 {
		data["pid"] = managedServer.PID
	}

	if containerID != "" {
		data["containerID"] = containerID[:12] // Short ID
	}

	return data
}

// CheckHealth implements HealthChecker using MCP client ping
func (s *MCPServerService) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	if s.GetState() != services.StateRunning {
		return services.HealthUnknown, nil
	}

	// For local commands, check if we have a client
	// The client is persistent and already connected, so if we have it, we're healthy
	client := s.GetMCPClient()
	if client == nil {
		s.UpdateHealth(services.HealthUnhealthy)
		return services.HealthUnhealthy, fmt.Errorf("MCP client not available")
	}

	// TODO: In the future, we could add actual ping functionality
	// For now, having a client means we're healthy
	s.UpdateHealth(services.HealthHealthy)
	return services.HealthHealthy, nil
}

// GetHealthCheckInterval implements HealthChecker
func (s *MCPServerService) GetHealthCheckInterval() time.Duration {
	if s.config.HealthCheckInterval > 0 {
		return s.config.HealthCheckInterval
	}
	// Default: MCP servers should be checked every 30 seconds
	return 30 * time.Second
}

// GetMCPClient returns the persistent MCP client for this server
func (s *MCPServerService) GetMCPClient() interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.GetState() != services.StateRunning {
		return nil
	}

	switch s.config.Type {
	case config.MCPServerTypeLocalCommand:
		if s.managedServer != nil {
			return s.managedServer.Client
		}
	case config.MCPServerTypeContainer:
		// TODO: Implement container client support
		return nil
	}

	return nil
}
