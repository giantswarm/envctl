package portforward

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/portforwarding"
	"envctl/internal/services"
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
	}
}

// Start starts the port forward
func (s *PortForwardService) Start(ctx context.Context) error {
	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

	// Create update function that directly updates service state
	updateFn := func(label string, detail portforwarding.PortForwardStatusDetail, isReady bool, err error) {
		// Convert portforwarding status to common status
		var status string
		switch detail {
		case portforwarding.StatusDetailInitializing:
			status = services.StatusInitializing
		case portforwarding.StatusDetailForwardingActive:
			status = services.StatusActive
		case portforwarding.StatusDetailStopped:
			status = services.StatusStopped
		case portforwarding.StatusDetailFailed:
			status = services.StatusFailed
		default:
			status = string(detail)
		}

		// Create status update
		update := services.StatusUpdate{
			Label:   label,
			Status:  status,
			IsError: err != nil,
			IsReady: isReady,
			Error:   err,
		}

		// Map to service state and health
		newState := services.MapStatusToState(update.Status)
		newHealth := services.MapStatusToHealth(update.Status, update.IsError)

		// Update service state
		s.UpdateState(newState, newHealth, update.Error)
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

	var health services.HealthStatus
	var err error

	switch state {
	case services.StateRunning:
		// TODO: Could implement actual port connectivity check here
		health = services.HealthHealthy
		err = nil
	case services.StateFailed:
		health = services.HealthUnhealthy
		err = fmt.Errorf("port forward is in failed state")
	case services.StateStarting:
		health = services.HealthChecking
		err = nil
	case services.StateStopped, services.StateStopping:
		health = services.HealthUnknown
		err = nil
	default:
		health = services.HealthUnknown
		err = nil
	}

	// Update the service's health status
	s.UpdateHealth(health)

	return health, err
}

// GetHealthCheckInterval implements HealthChecker
func (s *PortForwardService) GetHealthCheckInterval() time.Duration {
	if s.config.HealthCheckInterval > 0 {
		return s.config.HealthCheckInterval
	}
	// Default to 10 seconds for port forwards for more responsive health updates
	return 10 * time.Second
}

// parsePortSpec parses port specifications
func parsePortSpec(localPortStr, remotePortStr string) (int, int) {
	var localPort, remotePort int
	fmt.Sscanf(localPortStr, "%d", &localPort)
	fmt.Sscanf(remotePortStr, "%d", &remotePort)
	return localPort, remotePort
}
