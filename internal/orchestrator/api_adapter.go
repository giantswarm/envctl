package orchestrator

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"time"
)

// APIAdapter adapts the Orchestrator to implement api.OrchestratorHandler
type APIAdapter struct {
	orch *Orchestrator
}

// NewAPIAdapter creates a new orchestrator API adapter
func NewAPIAdapter(o *Orchestrator) *APIAdapter {
	return &APIAdapter{orch: o}
}

// StartService starts a specific service
func (a *APIAdapter) StartService(label string) error {
	return a.orch.StartService(label)
}

// StopService stops a specific service
func (a *APIAdapter) StopService(label string) error {
	return a.orch.StopService(label)
}

// RestartService restarts a specific service
func (a *APIAdapter) RestartService(label string) error {
	return a.orch.RestartService(label)
}

// SubscribeToStateChanges returns a channel for receiving service state change events
func (a *APIAdapter) SubscribeToStateChanges() <-chan api.ServiceStateChangedEvent {
	// Convert internal events to API events
	internalChan := a.orch.SubscribeToStateChanges()
	apiChan := make(chan api.ServiceStateChangedEvent, 100)

	go func() {
		for event := range internalChan {
			apiChan <- api.ServiceStateChangedEvent{
				Label:       event.Label,
				ServiceType: event.ServiceType,
				OldState:    event.OldState,
				NewState:    event.NewState,
				Health:      event.Health,
				Error:       event.Error,
				Timestamp:   time.Now(),
			}
		}
		close(apiChan)
	}()

	return apiChan
}

// GetAvailableClusters returns all clusters configured for a specific role
func (a *APIAdapter) GetAvailableClusters(role api.ClusterRole) []api.ClusterDefinition {
	// Convert API role to config role
	configRole := config.ClusterRole(role)
	configClusters := a.orch.GetAvailableClusters(configRole)

	// Convert config clusters to API clusters
	result := make([]api.ClusterDefinition, 0, len(configClusters))
	for _, c := range configClusters {
		result = append(result, api.ClusterDefinition{
			Name:        c.Name,
			Context:     c.Context,
			Role:        api.ClusterRole(c.Role),
			DisplayName: c.DisplayName,
			Icon:        c.Icon,
		})
	}

	return result
}

// GetActiveCluster returns the currently active cluster for a role
func (a *APIAdapter) GetActiveCluster(role api.ClusterRole) (string, bool) {
	configRole := config.ClusterRole(role)
	return a.orch.GetActiveCluster(configRole)
}

// SwitchCluster changes the active cluster for a role and restarts affected services
func (a *APIAdapter) SwitchCluster(role api.ClusterRole, clusterName string) error {
	configRole := config.ClusterRole(role)
	return a.orch.SwitchCluster(configRole, clusterName)
}

// Register registers this adapter with the API package
func (a *APIAdapter) Register() {
	api.RegisterOrchestrator(a)
}
