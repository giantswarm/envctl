package managers

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sConnectionConfig represents the configuration for a K8s connection service
type K8sConnectionConfig struct {
	Name                string // e.g., "mc" or "wc"
	ContextName         string // The actual kubectl context name
	IsMC                bool   // True if this is a management cluster
	HealthCheckInterval time.Duration
}

// K8sConnectionService manages a Kubernetes connection as a service
type K8sConnectionService struct {
	config       K8sConnectionConfig
	reporter     reporting.ServiceReporter
	stopChan     chan struct{}
	healthCtx    context.Context
	healthCancel context.CancelFunc
	mu           sync.Mutex
}

// StartK8sConnectionServices starts K8s connection services
func StartK8sConnectionServices(
	configs []K8sConnectionConfig,
	reporter reporting.ServiceReporter,
	wg *sync.WaitGroup,
) map[string]chan struct{} {
	stopChannels := make(map[string]chan struct{})

	for _, cfg := range configs {
		currentCfg := cfg // Capture range variable
		stopChan := make(chan struct{})
		stopChannels[currentCfg.Name] = stopChan

		wg.Add(1)
		go func() {
			defer wg.Done()

			service := &K8sConnectionService{
				config:   currentCfg,
				reporter: reporter,
				stopChan: stopChan,
			}

			service.Run()
		}()
	}

	return stopChannels
}

// Run starts the K8s connection service
func (s *K8sConnectionService) Run() {
	subsystem := fmt.Sprintf("K8sConnection-%s", s.config.Name)

	// Report initial state
	s.reportState(reporting.StateStarting, nil)

	// Create health check context
	s.mu.Lock()
	s.healthCtx, s.healthCancel = context.WithCancel(context.Background())
	s.mu.Unlock()

	// Start health monitoring
	go s.monitorHealth()

	// Perform initial health check
	s.checkHealth()

	// Wait for stop signal
	<-s.stopChan

	// Cancel health monitoring
	s.mu.Lock()
	if s.healthCancel != nil {
		s.healthCancel()
	}
	s.mu.Unlock()

	// Report stopped state
	s.reportState(reporting.StateStopped, nil)
	logging.Info(subsystem, "K8s connection service stopped")
}

// monitorHealth continuously monitors the K8s connection health
func (s *K8sConnectionService) monitorHealth() {
	ticker := time.NewTicker(s.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.healthCtx.Done():
			return
		case <-ticker.C:
			s.checkHealth()
		}
	}
}

// checkHealth performs a health check on the K8s connection
func (s *K8sConnectionService) checkHealth() {
	subsystem := fmt.Sprintf("K8sConnection-%s", s.config.Name)

	// Create clientset for the specific context
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: s.config.ContextName}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		s.handleHealthCheckError(subsystem, err, "Failed to get REST config")
		return
	}
	restConfig.Timeout = 15 * time.Second

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		s.handleHealthCheckError(subsystem, err, "Failed to create clientset")
		return
	}

	// Perform the health check using kube package
	readyNodes, totalNodes, statusErr := kube.GetNodeStatus(clientset)

	isHealthy := statusErr == nil

	// Determine state based on health
	var state reporting.ServiceState
	var healthErr error

	if statusErr != nil {
		state = reporting.StateFailed
		healthErr = statusErr
		logging.Error(subsystem, statusErr, "Health check failed")
	} else {
		state = reporting.StateRunning
		logging.Debug(subsystem, "Health check passed: %d/%d nodes ready", readyNodes, totalNodes)
	}

	// Report the state
	s.reportState(state, healthErr)

	// Also report health status for UI
	if s.reporter != nil {
		clusterShortName := s.config.Name
		if s.config.IsMC {
			// Extract MC name from context (simplified - you might need better parsing)
			clusterShortName = s.config.Name
		}

		healthUpdate := reporting.HealthStatusUpdate{
			Timestamp:        time.Now(),
			ContextName:      s.config.ContextName,
			ClusterShortName: clusterShortName,
			IsMC:             s.config.IsMC,
			IsHealthy:        isHealthy,
			ReadyNodes:       readyNodes,
			TotalNodes:       totalNodes,
			Error:            healthErr,
		}

		s.reporter.ReportHealth(healthUpdate)
	}
}

// handleHealthCheckError is a helper to handle health check errors
func (s *K8sConnectionService) handleHealthCheckError(subsystem string, err error, message string) {
	logging.Error(subsystem, err, "%s", message)
	s.reportState(reporting.StateFailed, err)

	if s.reporter != nil {
		healthUpdate := reporting.HealthStatusUpdate{
			Timestamp:        time.Now(),
			ContextName:      s.config.ContextName,
			ClusterShortName: s.config.Name,
			IsMC:             s.config.IsMC,
			IsHealthy:        false,
			ReadyNodes:       0,
			TotalNodes:       0,
			Error:            err,
		}
		s.reporter.ReportHealth(healthUpdate)
	}
}

// reportState reports the current state of the K8s connection
func (s *K8sConnectionService) reportState(state reporting.ServiceState, err error) {
	if s.reporter == nil {
		return
	}

	update := reporting.NewManagedServiceUpdate(
		reporting.ServiceTypeKube,
		s.config.Name,
		state,
	).WithCause("k8s_connection_health")

	if err != nil {
		update = update.WithError(err)
	}

	s.reporter.Report(update)
}
