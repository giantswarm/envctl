package managers

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"sync"
	"time"
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
	isReady      bool // Track readiness separately from state
	// Track last reported values to avoid redundant updates
	lastState      reporting.ServiceState
	lastReadyNodes int
	lastTotalNodes int
	lastError      error
	mu             sync.Mutex
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
				config:         currentCfg,
				reporter:       reporter,
				stopChan:       stopChan,
				lastState:      reporting.StateUnknown,
				lastReadyNodes: -1,
				lastTotalNodes: -1,
			}

			service.Run()
		}()
	}

	return stopChannels
}

// Run starts the K8s connection service
func (s *K8sConnectionService) Run() {
	subsystem := fmt.Sprintf("K8sConnection-%s", s.config.Name)

	// Report initial state with IsReady=false
	s.reportState(reporting.StateStarting, nil)

	// Create health check context
	s.mu.Lock()
	s.healthCtx, s.healthCancel = context.WithCancel(context.Background())
	s.mu.Unlock()

	// Start health monitoring
	go s.monitorHealth()

	// Perform initial health check with retries
	go func() {
		// Retry initial health check up to 3 times with delays
		for i := 0; i < 3; i++ {
			if i > 0 {
				// Wait before retry (exponential backoff)
				time.Sleep(time.Duration(i) * time.Second)
			}

			// Check if service is stopping
			select {
			case <-s.stopChan:
				return
			default:
			}

			s.checkHealth()

			// If health check succeeded, stop retrying
			s.mu.Lock()
			isReady := s.isReady
			s.mu.Unlock()

			if isReady {
				logging.Info(subsystem, "Initial health check succeeded on attempt %d", i+1)
				break
			} else {
				logging.Debug(subsystem, "Initial health check failed on attempt %d, will retry", i+1)
			}
		}
	}()

	// Wait for stop signal
	<-s.stopChan

	// Cancel health monitoring
	s.mu.Lock()
	if s.healthCancel != nil {
		s.healthCancel()
	}
	s.mu.Unlock()

	// Report stopped state with IsReady=false
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

	// Use kube package to create clientset for the specific context
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientset, err := kube.GetClientsetForContext(ctx, s.config.ContextName)
	if err != nil {
		s.handleHealthCheckError(subsystem, err, "Failed to create clientset")
		return
	}

	// Perform the health check using kube package
	readyNodes, totalNodes, statusErr := kube.GetNodeStatus(clientset)

	// Determine state and readiness based on health
	var state reporting.ServiceState
	var healthErr error
	var isReady bool

	if statusErr != nil {
		state = reporting.StateFailed
		healthErr = statusErr
		isReady = false
		logging.Error(subsystem, statusErr, "Health check failed")
	} else {
		state = reporting.StateRunning
		isReady = true // K8s connection is ready when health check passes
		logging.Debug(subsystem, "Health check passed: %d/%d nodes ready", readyNodes, totalNodes)
	}

	// Check if anything has changed
	s.mu.Lock()
	previousIsReady := s.isReady
	previousState := s.lastState
	previousReadyNodes := s.lastReadyNodes
	previousTotalNodes := s.lastTotalNodes
	previousError := s.lastError

	// Update internal state
	s.isReady = isReady
	s.lastState = state
	s.lastReadyNodes = readyNodes
	s.lastTotalNodes = totalNodes
	s.lastError = healthErr
	s.mu.Unlock()

	// Only report if something has changed
	hasChanged := previousIsReady != isReady ||
		previousState != state ||
		previousReadyNodes != readyNodes ||
		previousTotalNodes != totalNodes ||
		(previousError == nil && healthErr != nil) ||
		(previousError != nil && healthErr == nil) ||
		(previousError != nil && healthErr != nil && previousError.Error() != healthErr.Error())

	if hasChanged {
		logging.Debug(subsystem, "Health status changed - State: %s->%s, Ready: %v->%v, Nodes: %d/%d->%d/%d",
			previousState, state, previousIsReady, isReady,
			previousReadyNodes, previousTotalNodes, readyNodes, totalNodes)
		s.reportStateWithHealth(state, healthErr, readyNodes, totalNodes)
	}
}

// handleHealthCheckError is a helper to handle health check errors
func (s *K8sConnectionService) handleHealthCheckError(subsystem string, err error, message string) {
	logging.Error(subsystem, err, "%s", message)

	// Check if anything has changed
	s.mu.Lock()
	previousIsReady := s.isReady
	previousState := s.lastState
	previousReadyNodes := s.lastReadyNodes
	previousTotalNodes := s.lastTotalNodes
	previousError := s.lastError

	// Update internal state
	s.isReady = false
	s.lastState = reporting.StateFailed
	s.lastReadyNodes = 0
	s.lastTotalNodes = 0
	s.lastError = err
	s.mu.Unlock()

	// Only report if something has changed
	hasChanged := previousIsReady != false ||
		previousState != reporting.StateFailed ||
		previousReadyNodes != 0 ||
		previousTotalNodes != 0 ||
		(previousError == nil && err != nil) ||
		(previousError != nil && err == nil) ||
		(previousError != nil && err != nil && previousError.Error() != err.Error())

	if hasChanged {
		// Report failed state with no nodes ready
		s.reportStateWithHealth(reporting.StateFailed, err, 0, 0)
	}
}

// reportState reports the current state of the K8s connection
func (s *K8sConnectionService) reportState(state reporting.ServiceState, err error) {
	// Default to no health data when not from a health check
	s.reportStateWithHealth(state, err, -1, -1)
}

// reportStateWithHealth reports the current state with optional K8s health data
func (s *K8sConnectionService) reportStateWithHealth(state reporting.ServiceState, err error, readyNodes, totalNodes int) {
	if s.reporter == nil {
		return
	}

	update := reporting.NewManagedServiceUpdate(
		reporting.ServiceTypeKube,
		s.config.Name,
		state,
	).WithCause("k8s_connection_health")

	// Set IsReady based on our tracked state
	s.mu.Lock()
	update.IsReady = s.isReady
	s.mu.Unlock()

	if err != nil {
		update = update.WithError(err)
	}

	// Add K8s health data if available (readyNodes >= 0 indicates health data is present)
	if readyNodes >= 0 {
		update = update.WithK8sHealth(readyNodes, totalNodes, s.config.IsMC)
	}

	s.reporter.Report(update)
}
