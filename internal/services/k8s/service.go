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

	// First, ensure we're logged in to the cluster
	// Extract cluster name from context (remove teleport prefix)
	clusterName := s.kubeMgr.StripTeleportPrefix(s.contextName)
	if clusterName != "" && clusterName != s.contextName {
		// Only login if we have a teleport context
		logging.Info("K8sConnection-"+s.label, "Logging in to cluster: %s", clusterName)
		stdout, stderr, err := s.kubeMgr.Login(clusterName)
		if err != nil {
			logging.Error("K8sConnection-"+s.label, err, "Failed to login to cluster %s. Stderr: %s", clusterName, stderr)
			s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
			return fmt.Errorf("failed to login to cluster %s: %w", clusterName, err)
		}
		if stdout != "" {
			logging.Debug("K8sConnection-"+s.label, "Login stdout: %s", stdout)
		}
	}

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
	// First check if API is responsive - this is the primary health indicator
	apiErr := s.kubeMgr.CheckAPIHealth(ctx, s.contextName)

	// Also get node health for informational purposes
	nodeHealth, nodeErr := s.kubeMgr.GetClusterNodeHealth(ctx, s.contextName)

	s.mu.Lock()
	s.lastHealthCheck = time.Now()

	if apiErr != nil {
		// API is not responsive - this means the cluster is unhealthy
		s.healthError = apiErr
		s.readyNodes = -1
		s.totalNodes = -1
		s.mu.Unlock()

		logging.Error("K8sConnection-"+s.label, apiErr, "API health check failed")
		return services.HealthUnhealthy, apiErr
	}

	// API is responsive, so the cluster is healthy
	// Update node counts if available
	if nodeErr != nil {
		// Can't get node info, but API is still healthy
		logging.Warn("K8sConnection-"+s.label, "Could not retrieve node information: %v", nodeErr)
		s.readyNodes = -1
		s.totalNodes = -1
		s.healthError = nodeErr
	} else {
		s.readyNodes = nodeHealth.ReadyNodes
		s.totalNodes = nodeHealth.TotalNodes
		s.healthError = nil

		// Log warnings for degraded nodes, but don't affect health status
		if nodeHealth.ReadyNodes < nodeHealth.TotalNodes && nodeHealth.TotalNodes > 0 {
			logging.Warn("K8sConnection-"+s.label, "Cluster has degraded nodes: %d/%d ready",
				nodeHealth.ReadyNodes, nodeHealth.TotalNodes)
		} else if nodeHealth.ReadyNodes == 0 && nodeHealth.TotalNodes > 0 {
			logging.Warn("K8sConnection-"+s.label, "No nodes are ready: %d/%d",
				nodeHealth.ReadyNodes, nodeHealth.TotalNodes)
		}
	}
	s.mu.Unlock()

	// As long as the API is responsive, the cluster is considered healthy
	logging.Debug("K8sConnection-"+s.label, "API is healthy, nodes: %d/%d ready",
		s.readyNodes, s.totalNodes)
	return services.HealthHealthy, nil
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
