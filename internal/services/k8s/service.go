package k8s

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
)

// K8sConnectionService implements the Service interface for Kubernetes connections
type K8sConnectionService struct {
	*services.BaseService

	mu          sync.RWMutex
	label       string
	contextName string
	isMC        bool
	kubeMgr     kube.Manager

	// Health check state
	lastHealthCheck time.Time
	readyNodes      int
	totalNodes      int
	healthError     error

	// Context for cancellation
	cancelFunc context.CancelFunc
}

// NewK8sConnectionService creates a new K8s connection service
func NewK8sConnectionService(label, contextName string, isMC bool, kubeMgr kube.Manager) *K8sConnectionService {
	// K8s connections have no dependencies
	deps := []string{}

	return &K8sConnectionService{
		BaseService: services.NewBaseService(label, services.TypeKubeConnection, deps),
		label:       label,
		contextName: contextName,
		isMC:        isMC,
		kubeMgr:     kubeMgr,
		readyNodes:  -1,
		totalNodes:  -1,
	}
}

// Start starts monitoring the K8s connection
func (s *K8sConnectionService) Start(ctx context.Context) error {
	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

	// Create a cancellable context
	monitorCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancelFunc = cancel
	s.mu.Unlock()

	// Start monitoring in background
	go s.monitorConnection(monitorCtx)

	// Do initial health check
	health, err := s.CheckHealth(ctx)
	if err != nil {
		s.UpdateState(services.StateFailed, health, err)
		return fmt.Errorf("initial health check failed for %s: %w", s.label, err)
	}

	s.UpdateState(services.StateRunning, health, nil)
	return nil
}

// Stop stops monitoring the K8s connection
func (s *K8sConnectionService) Stop(ctx context.Context) error {
	s.UpdateState(services.StateStopping, s.GetHealth(), nil)

	s.mu.Lock()
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
	s.mu.Unlock()

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	return nil
}

// Restart restarts the K8s connection monitoring
func (s *K8sConnectionService) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop K8s connection %s: %w", s.label, err)
	}

	return s.Start(ctx)
}

// GetServiceData implements ServiceDataProvider
func (s *K8sConnectionService) GetServiceData() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := map[string]interface{}{
		"label":      s.label,
		"context":    s.contextName,
		"isMC":       s.isMC,
		"readyNodes": s.readyNodes,
		"totalNodes": s.totalNodes,
		"lastCheck":  s.lastHealthCheck,
	}

	if s.healthError != nil {
		data["healthError"] = s.healthError.Error()
	}

	return data
}

// CheckHealth implements HealthChecker
func (s *K8sConnectionService) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	// Use kube manager to check cluster health
	health, err := s.kubeMgr.GetClusterNodeHealth(ctx, s.contextName)

	s.mu.Lock()
	s.lastHealthCheck = time.Now()
	s.healthError = err

	if err != nil {
		s.readyNodes = -1
		s.totalNodes = -1
		s.mu.Unlock()

		logging.Error("K8sConnection-"+s.label, err, "Health check failed")
		return services.HealthUnhealthy, err
	}

	// Update node counts
	s.readyNodes = health.ReadyNodes
	s.totalNodes = health.TotalNodes
	s.mu.Unlock()

	// Determine health status
	if health.ReadyNodes == health.TotalNodes && health.TotalNodes > 0 {
		logging.Debug("K8sConnection-"+s.label, "Health check passed: %d/%d nodes ready", health.ReadyNodes, health.TotalNodes)
		return services.HealthHealthy, nil
	} else if health.ReadyNodes > 0 {
		logging.Warn("K8sConnection-"+s.label, "Cluster degraded: %d/%d nodes ready", health.ReadyNodes, health.TotalNodes)
		return services.HealthUnhealthy, fmt.Errorf("cluster degraded: %d/%d nodes ready", health.ReadyNodes, health.TotalNodes)
	} else {
		logging.Error("K8sConnection-"+s.label, nil, "No nodes ready: %d/%d", health.ReadyNodes, health.TotalNodes)
		return services.HealthUnhealthy, fmt.Errorf("no nodes ready")
	}
}

// GetHealthCheckInterval implements HealthChecker
func (s *K8sConnectionService) GetHealthCheckInterval() time.Duration {
	// K8s connections should be checked every 15 seconds
	return 15 * time.Second
}

// monitorConnection continuously monitors the K8s connection
func (s *K8sConnectionService) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(s.GetHealthCheckInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			health, err := s.CheckHealth(ctx)

			// Update health status
			if err != nil {
				s.UpdateHealth(health)
				s.UpdateError(err)
			} else {
				s.UpdateHealth(health)
			}

		case <-ctx.Done():
			logging.Debug("K8sConnection-"+s.label, "Monitoring stopped")
			return
		}
	}
}
