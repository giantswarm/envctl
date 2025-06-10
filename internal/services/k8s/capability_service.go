// Package k8s provides a Kubernetes connection service that can work with capabilities
package k8s

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"envctl/internal/capability"
	"envctl/internal/kube"
	"envctl/internal/services"
	"envctl/pkg/logging"
)

// CapabilityK8sConnectionService is a future-ready K8s connection service that will use capability-based authentication
// For now, it falls back to the traditional kube.Manager for authentication
type CapabilityK8sConnectionService struct {
	*services.CapabilityService
	
	clusterName  string
	contextName  string
	isMC         bool // Is this a management cluster?
	
	// Traditional kube manager for backward compatibility
	kubeMgr kube.Manager
	
	// Future: auth capability handle
	authHandle   *capability.CapabilityHandle
	
	// Monitoring
	mu         sync.Mutex
	cancelFunc context.CancelFunc
}

// NewCapabilityK8sConnectionService creates a new K8s connection service ready for capabilities
func NewCapabilityK8sConnectionService(clusterName, contextName string, isMC bool, kubeMgr kube.Manager) *CapabilityK8sConnectionService {
	label := fmt.Sprintf("k8s-%s", clusterName)
	
	// Define future capability requirements
	// When auth providers are ready, this service will automatically use them
	requirements := []capability.CapabilityRequirement{
		{
			Type:     capability.CapabilityTypeAuth,
			Optional: true, // Optional for now to maintain backward compatibility
			Config: map[string]interface{}{
				"cluster": clusterName,
				"context": contextName,
			},
		},
	}
	
	service := &CapabilityK8sConnectionService{
		CapabilityService: services.NewCapabilityService(label, services.TypeKubeConnection, []string{}, requirements),
		clusterName:       clusterName,
		contextName:       contextName,
		isMC:              isMC,
		kubeMgr:          kubeMgr,
	}
	
	// Set capability callbacks for when auth capabilities become available
	service.SetCapabilityCallbacks(
		service.onCapabilityProvided,
		service.onCapabilityLost,
	)
	
	return service
}

// Start starts the K8s connection service
func (s *CapabilityK8sConnectionService) Start(ctx context.Context) error {
	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)
	
	// Check if we have auth capability (future functionality)
	handle, hasCapability := s.GetCapabilityHandleByType(capability.CapabilityTypeAuth)
	if hasCapability {
		s.authHandle = &handle
		logging.Info(s.GetLabel(), "Using capability-based authentication from provider: %s", handle.Provider)
		// TODO: Implement capability-based authentication when auth providers are ready
		// For now, we still use the traditional method below
	}
	
	// Traditional authentication (current implementation)
	// Extract cluster name from context (remove teleport prefix)
	clusterName := s.kubeMgr.StripTeleportPrefix(s.contextName)
	if clusterName != "" && clusterName != s.contextName {
		// Only login if we have a teleport context
		logging.Info(s.GetLabel(), "Logging in to cluster: %s", clusterName)
		stdout, stderr, err := s.kubeMgr.Login(clusterName)
		if err != nil {
			logging.Error(s.GetLabel(), err, "Failed to login to cluster %s. Stderr: %s", clusterName, stderr)
			s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
			return fmt.Errorf("failed to login to cluster %s: %w", clusterName, err)
		}
		if stdout != "" {
			logging.Debug(s.GetLabel(), "Login stdout: %s", stdout)
		}
	}
	
	// Create a cancellable context for monitoring
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancelFunc = monitorCancel
	s.mu.Unlock()
	
	// Start monitoring in background
	go s.monitorConnection(monitorCtx)
	
	// Do initial health check
	health, err := s.CheckHealth(ctx)
	if err != nil {
		s.UpdateState(services.StateFailed, health, err)
		return fmt.Errorf("initial health check failed for %s: %w", s.clusterName, err)
	}
	
	s.UpdateState(services.StateRunning, health, nil)
	return nil
}

// Stop stops the K8s connection service
func (s *CapabilityK8sConnectionService) Stop(ctx context.Context) error {
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

// onCapabilityProvided handles when auth capability is provided
func (s *CapabilityK8sConnectionService) onCapabilityProvided(handle capability.CapabilityHandle) error {
	if handle.Type == capability.CapabilityTypeAuth {
		s.authHandle = &handle
		logging.Info(s.GetLabel(), "Auth capability provided by: %s (will be used in future versions)", handle.Provider)
		// TODO: Switch to capability-based auth when providers implement the required tools
	}
	return nil
}

// onCapabilityLost handles when auth capability is lost
func (s *CapabilityK8sConnectionService) onCapabilityLost(handleID string) error {
	if s.authHandle != nil && s.authHandle.ID == handleID {
		logging.Warn(s.GetLabel(), "Lost auth capability (falling back to traditional auth)")
		s.authHandle = nil
		// Continue with traditional auth for now
	}
	return nil
}

// CheckHealth checks the health of the K8s connection
func (s *CapabilityK8sConnectionService) CheckHealth(ctx context.Context) (services.HealthStatus, error) {
	// For now, use traditional health check
	// In the future, this will use capability-based validation
	
	// Try to check cluster health
	nodeHealth, err := s.kubeMgr.GetClusterNodeHealth(ctx, s.contextName)
	if err != nil {
		return services.HealthUnhealthy, err
	}
	
	// Check if all nodes are ready
	if nodeHealth.Error != nil {
		return services.HealthUnhealthy, nodeHealth.Error
	}
	
	if nodeHealth.TotalNodes == 0 {
		return services.HealthUnhealthy, fmt.Errorf("no nodes found in cluster")
	}
	
	if nodeHealth.ReadyNodes < nodeHealth.TotalNodes {
		return services.HealthUnhealthy, fmt.Errorf("%d/%d nodes ready", nodeHealth.ReadyNodes, nodeHealth.TotalNodes)
	}
	
	return services.HealthHealthy, nil
}

// monitorConnection continuously monitors the K8s connection health
func (s *CapabilityK8sConnectionService) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health, err := s.CheckHealth(ctx)
			if err != nil {
				logging.Debug(s.GetLabel(), "Health check failed: %v", err)
			}
			s.UpdateState(s.GetState(), health, err)
		}
	}
}

// GetClusterName returns the cluster name
func (s *CapabilityK8sConnectionService) GetClusterName() string {
	return s.clusterName
}

// GetContextName returns the Kubernetes context name
func (s *CapabilityK8sConnectionService) GetContextName() string {
	return s.contextName
}

// IsMC returns whether this is a management cluster
func (s *CapabilityK8sConnectionService) IsMC() bool {
	return s.isMC
}

// Restart restarts the K8s connection
func (s *CapabilityK8sConnectionService) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	
	// Give it a moment to fully stop
	time.Sleep(100 * time.Millisecond)
	
	return s.Start(ctx)
}

// IsForCluster checks if this service is for the specified cluster
func (s *CapabilityK8sConnectionService) IsForCluster(clusterName string) bool {
	return s.clusterName == clusterName || 
		   strings.HasSuffix(s.contextName, clusterName)
}

// GetServiceData returns service-specific data
func (s *CapabilityK8sConnectionService) GetServiceData() map[string]interface{} {
	data := map[string]interface{}{
		"cluster_name": s.clusterName,
		"context_name": s.contextName,
		"is_mc":        s.isMC,
	}
	
	// Add capability information if available
	if s.authHandle != nil {
		data["auth_provider"] = s.authHandle.Provider
		data["has_capability_auth"] = true
	} else {
		data["has_capability_auth"] = false
	}
	
	return data
} 