package portforward

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/portforwarding"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// PortForwardService implements the Service interface for port forwards
type PortForwardService struct {
	*services.BaseService

	mu         sync.RWMutex
	config     config.PortForwardDefinition
	kubeMgr    kube.Manager
	stopChan   chan struct{}
	localPort  int
	remotePort int
	targetPod  string

	// Internal state from port forwarding
	statusChan chan string
}

// NewPortForwardService creates a new port forward service
func NewPortForwardService(cfg config.PortForwardDefinition, kubeMgr kube.Manager) *PortForwardService {
	// No dependencies for port forwards (they depend on K8s connections which are handled separately)
	deps := []string{}

	// Parse ports
	localPort, remotePort := parsePortSpec(cfg.LocalPort, cfg.RemotePort)

	return &PortForwardService{
		BaseService: services.NewBaseService(cfg.Name, services.TypePortForward, deps),
		config:      cfg,
		kubeMgr:     kubeMgr,
		localPort:   localPort,
		remotePort:  remotePort,
		statusChan:  make(chan string, 10),
	}
}

// Start starts the port forward
func (s *PortForwardService) Start(ctx context.Context) error {
	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

	// Create update function
	updateFn := func(label string, detail portforwarding.PortForwardStatusDetail, isReady bool, err error) {
		// Convert status detail to string for handling
		var status string
		switch detail {
		case portforwarding.StatusDetailInitializing:
			status = "Initializing"
		case portforwarding.StatusDetailForwardingActive:
			status = "Running"
		case portforwarding.StatusDetailStopped:
			status = "Stopped"
		case portforwarding.StatusDetailFailed:
			status = "Failed"
		default:
			status = "Unknown"
		}

		select {
		case s.statusChan <- status:
		default:
			// Channel full, drop update
		}
	}

	// Use the existing port forwarding package
	stopChan, err := portforwarding.StartAndManageIndividualPortForward(
		s.config,
		updateFn,
	)

	if err != nil {
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to start port forward %s: %w", s.config.Name, err)
	}

	s.mu.Lock()
	s.stopChan = stopChan
	s.mu.Unlock()

	// Monitor status updates
	go s.monitorStatus(ctx)

	return nil
}

// Stop stops the port forward
func (s *PortForwardService) Stop(ctx context.Context) error {
	s.UpdateState(services.StateStopping, s.GetHealth(), nil)

	s.mu.RLock()
	stopChan := s.stopChan
	s.mu.RUnlock()

	if stopChan != nil {
		close(stopChan)

		// Wait for port forward to stop with context timeout
		done := make(chan struct{})
		go func() {
			// Give the port forward some time to stop gracefully
			time.Sleep(500 * time.Millisecond)
			close(done)
		}()

		select {
		case <-done:
			// Port forward stopped gracefully
		case <-ctx.Done():
			logging.Warn("PortForwardService", "Context cancelled while stopping port forward %s", s.config.Name)
			return ctx.Err()
		}
	}

	s.mu.Lock()
	s.stopChan = nil
	s.targetPod = ""
	s.mu.Unlock()

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	return nil
}

// Restart restarts the port forward
func (s *PortForwardService) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop port forward %s: %w", s.config.Name, err)
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
func (s *PortForwardService) GetServiceData() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := map[string]interface{}{
		"name":        s.config.Name,
		"namespace":   s.config.Namespace,
		"targetType":  s.config.TargetType,
		"targetName":  s.config.TargetName,
		"localPort":   s.localPort,
		"remotePort":  s.remotePort,
		"bindAddress": s.config.BindAddress,
		"enabled":     s.config.Enabled,
		"icon":        s.config.Icon,
		"category":    s.config.Category,
		"context":     s.config.KubeContextTarget,
	}

	if s.targetPod != "" {
		data["targetPod"] = s.targetPod
	}

	return data
}

// CheckHealth implements HealthChecker
func (s *PortForwardService) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	// For port forwards, health is based on the current state
	// A more sophisticated check could try to connect to the local port
	state := s.GetState()

	switch state {
	case services.StateRunning:
		// TODO: Could implement actual port connectivity check here
		return services.HealthHealthy, nil
	case services.StateFailed:
		return services.HealthUnhealthy, fmt.Errorf("port forward is in failed state")
	case services.StateStarting:
		return services.HealthChecking, nil
	case services.StateStopped, services.StateStopping:
		return services.HealthUnknown, nil
	default:
		return services.HealthUnknown, nil
	}
}

// GetHealthCheckInterval implements HealthChecker
func (s *PortForwardService) GetHealthCheckInterval() time.Duration {
	if s.config.HealthCheckInterval > 0 {
		return s.config.HealthCheckInterval
	}
	// Default to 30 seconds for port forwards
	return 30 * time.Second
}

// monitorStatus monitors the port forward status channel
func (s *PortForwardService) monitorStatus(ctx context.Context) {
	for {
		select {
		case status := <-s.statusChan:
			s.handleStatusUpdate(status)
		case <-ctx.Done():
			return
		}
	}
}

// handleStatusUpdate handles status updates from the port forward
func (s *PortForwardService) handleStatusUpdate(status string) {
	// Map status to service state
	switch status {
	case "Initializing", "Starting":
		s.UpdateState(services.StateStarting, services.HealthUnknown, nil)
	case "Running", "ForwardingActive":
		s.UpdateState(services.StateRunning, services.HealthHealthy, nil)
	case "Failed", "Error":
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, fmt.Errorf("port forward failed"))
	case "Stopped":
		s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	}
}

// parsePortSpec parses port specifications
func parsePortSpec(localPortStr, remotePortStr string) (int, int) {
	var localPort, remotePort int
	fmt.Sscanf(localPortStr, "%d", &localPort)
	fmt.Sscanf(remotePortStr, "%d", &remotePort)
	return localPort, remotePort
}
