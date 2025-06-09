package portforward

import (
	"context"
	"envctl/internal/capability"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/internal/services"
	"envctl/pkg/logging"
	"fmt"
)

// CapabilityPortForwardService is an example of a port forward service
// that uses capabilities instead of hard dependencies
type CapabilityPortForwardService struct {
	*services.CapabilityService

	config   config.PortForwardDefinition
	stopChan chan struct{}
}

// NewCapabilityPortForwardService creates a new capability-aware port forward service
func NewCapabilityPortForwardService(pfConfig config.PortForwardDefinition) *CapabilityPortForwardService {
	// Define capability requirements
	requirements := []capability.CapabilityRequirement{
		{
			Type:     capability.CapabilityTypeAuth,
			Features: []string{"login", "validate"},
			Config: map[string]interface{}{
				"cluster": pfConfig.ClusterName,
			},
			Optional: false,
		},
		{
			Type:     capability.CapabilityTypeDiscovery,
			Features: []string{"find_service", "find_pod"},
			Config: map[string]interface{}{
				"namespace": pfConfig.Namespace,
			},
			Optional: false,
		},
		{
			Type:     capability.CapabilityTypePortForward,
			Features: []string{"create", "manage"},
			Config: map[string]interface{}{
				"local_port":  pfConfig.LocalPort,
				"remote_port": pfConfig.RemotePort,
			},
			Optional: false,
		},
	}

	// Create the service with no hard dependencies
	service := &CapabilityPortForwardService{
		CapabilityService: services.NewCapabilityService(
			pfConfig.Name,
			services.TypePortForward,
			[]string{}, // No hard dependencies
			requirements,
		),
		config: pfConfig,
	}

	// Set capability callbacks
	service.SetCapabilityCallbacks(
		service.onCapabilityProvided,
		service.onCapabilityLost,
	)

	return service
}

// Start starts the port forward using capabilities
func (s *CapabilityPortForwardService) Start(ctx context.Context) error {
	// Update state to starting
	s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

	// Wait for all required capabilities
	if err := s.WaitForCapabilities(ctx); err != nil {
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return fmt.Errorf("failed to acquire capabilities: %w", err)
	}

	// Get auth capability handle
	authHandle, ok := s.GetCapabilityHandleByType(capability.CapabilityTypeAuth)
	if !ok {
		err := fmt.Errorf("auth capability not found")
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return err
	}

	// Get discovery capability handle
	discoveryHandle, ok := s.GetCapabilityHandleByType(capability.CapabilityTypeDiscovery)
	if !ok {
		err := fmt.Errorf("discovery capability not found")
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return err
	}

	// Get port forward capability handle
	pfHandle, ok := s.GetCapabilityHandleByType(capability.CapabilityTypePortForward)
	if !ok {
		err := fmt.Errorf("port forward capability not found")
		s.UpdateState(services.StateFailed, services.HealthUnhealthy, err)
		return err
	}

	logging.Info(s.GetLabel(), "Starting port forward using capabilities: auth=%s, discovery=%s, pf=%s",
		authHandle.Provider, discoveryHandle.Provider, pfHandle.Provider)

	// The actual port forwarding would be handled by the capability provider
	// This is just a placeholder showing how to use capabilities

	// Update state to running
	s.UpdateState(services.StateRunning, services.HealthHealthy, nil)

	// Create stop channel
	s.stopChan = make(chan struct{})

	// Wait for stop signal
	<-s.stopChan

	return nil
}

// Stop stops the port forward
func (s *CapabilityPortForwardService) Stop(ctx context.Context) error {
	if s.stopChan != nil {
		close(s.stopChan)
	}

	s.UpdateState(services.StateStopped, services.HealthUnknown, nil)
	return nil
}

// Restart restarts the port forward
func (s *CapabilityPortForwardService) Restart(ctx context.Context) error {
	if err := s.Stop(ctx); err != nil {
		return err
	}
	return s.Start(ctx)
}

// CheckHealth checks the health of the port forward
func (s *CapabilityPortForwardService) CheckHealth(ctx context.Context) error {
	// Check if all capabilities are still healthy
	if !s.HasRequiredCapabilities() {
		return fmt.Errorf("missing required capabilities")
	}

	// Additional health checks would go here

	return nil
}

// onCapabilityProvided handles when a capability is provided
func (s *CapabilityPortForwardService) onCapabilityProvided(handle capability.CapabilityHandle) error {
	logging.Info(s.GetLabel(), "Capability provided: %s from %s", handle.Type, handle.Provider)

	// If we're already running and a capability was updated, we might need to restart
	if s.GetState() == services.StateRunning {
		// In a real implementation, we would check if this affects our operation
		// and potentially trigger a restart
	}

	return nil
}

// onCapabilityLost handles when a capability is lost
func (s *CapabilityPortForwardService) onCapabilityLost(handleID string) error {
	logging.Warn(s.GetLabel(), "Capability lost: %s", handleID)

	// If we're running and lost a required capability, we need to stop
	if s.GetState() == services.StateRunning {
		// Check if this was a required capability
		for _, req := range s.GetRequiredCapabilities() {
			if !req.Optional {
				// Stop the service
				go func() {
					if err := s.Stop(context.Background()); err != nil {
						logging.Error(s.GetLabel(), err, "Failed to stop after capability loss")
					}
				}()
				break
			}
		}
	}

	return nil
}

// Example of how to provide a port forward capability from the kubectl MCP
type KubectlPortForwardProvider struct {
	kubeManager kube.Manager
}

// GetCapabilityType returns the type of capability this provider offers
func (p *KubectlPortForwardProvider) GetCapabilityType() capability.CapabilityType {
	return capability.CapabilityTypePortForward
}

// GetCapabilityFeatures returns the list of features this provider supports
func (p *KubectlPortForwardProvider) GetCapabilityFeatures() []string {
	return []string{"create", "manage", "list", "delete"}
}

// ValidateCapabilityRequest validates if this provider can fulfill a request
func (p *KubectlPortForwardProvider) ValidateCapabilityRequest(req capability.CapabilityRequest) error {
	if req.Type != capability.CapabilityTypePortForward {
		return fmt.Errorf("unsupported capability type: %s", req.Type)
	}

	// Check if all required features are supported
	for _, feature := range req.Features {
		supported := false
		for _, supportedFeature := range p.GetCapabilityFeatures() {
			if feature == supportedFeature {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("unsupported feature: %s", feature)
		}
	}

	// Validate config
	if req.Config != nil {
		if _, ok := req.Config["local_port"]; !ok {
			return fmt.Errorf("missing required config: local_port")
		}
		if _, ok := req.Config["remote_port"]; !ok {
			return fmt.Errorf("missing required config: remote_port")
		}
	}

	return nil
}

// ProvideCapability attempts to fulfill a capability request
func (p *KubectlPortForwardProvider) ProvideCapability(ctx context.Context, req capability.CapabilityRequest) (capability.CapabilityHandle, error) {
	// Validate the request
	if err := p.ValidateCapabilityRequest(req); err != nil {
		return capability.CapabilityHandle{}, err
	}

	// Extract config
	localPort := req.Config["local_port"].(string)
	remotePort := req.Config["remote_port"].(string)

	// In a real implementation, this would create the actual port forward
	// using kubectl or the Kubernetes API

	// Create handle
	handle := capability.CapabilityHandle{
		ID:       fmt.Sprintf("kubectl-pf-%s-%s", localPort, remotePort),
		Provider: "kubectl-mcp",
		Type:     capability.CapabilityTypePortForward,
		Config: map[string]interface{}{
			"local_port":  localPort,
			"remote_port": remotePort,
			"command":     fmt.Sprintf("kubectl port-forward ... %s:%s", localPort, remotePort),
		},
	}

	return handle, nil
}
