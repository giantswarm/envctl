package adapters

import (
	"envctl/internal/aggregator"
	"envctl/internal/api"
)

// OrchestratorEventAdapter adapts the OrchestratorAPI to the OrchestratorEventProvider interface
type OrchestratorEventAdapter struct {
	api api.OrchestratorAPI
}

// NewOrchestratorEventAdapter creates a new orchestrator event adapter
func NewOrchestratorEventAdapter(api api.OrchestratorAPI) *OrchestratorEventAdapter {
	return &OrchestratorEventAdapter{api: api}
}

// SubscribeToStateChanges returns a channel for service state change events
func (a *OrchestratorEventAdapter) SubscribeToStateChanges() <-chan aggregator.ServiceStateChangedEvent {
	// Create a channel to forward events
	eventChan := make(chan aggregator.ServiceStateChangedEvent, 100)

	// Subscribe to API events
	apiEvents := a.api.SubscribeToStateChanges()

	// Forward events in a goroutine
	go func() {
		for apiEvent := range apiEvents {
			// Convert API event to aggregator event
			aggEvent := aggregator.ServiceStateChangedEvent{
				Label:    apiEvent.Label,
				OldState: apiEvent.OldState,
				NewState: apiEvent.NewState,
				Health:   apiEvent.Health,
				Error:    apiEvent.Error,
			}

			select {
			case eventChan <- aggEvent:
				// Event forwarded successfully
			default:
				// Channel full, drop event
			}
		}
		close(eventChan)
	}()

	return eventChan
}
