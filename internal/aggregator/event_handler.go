package aggregator

import (
	"context"
	"envctl/pkg/logging"
	"sync"
	"time"
)

// ServiceStateEvent represents a service state change event
// This is a minimal interface to avoid import cycles while maintaining compatibility
type ServiceStateEvent struct {
	Label       string
	ServiceType string
	OldState    string
	NewState    string
	Health      string
	Error       error
}

// StateEventProvider defines the interface for subscribing to service state changes
type StateEventProvider interface {
	SubscribeToStateChanges() <-chan ServiceStateEvent
}

// EventHandler handles orchestrator events and updates the aggregator accordingly
type EventHandler struct {
	stateProvider StateEventProvider
	refreshFunc   func(context.Context) error
	ctx           context.Context
	cancelFunc    context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
	running       bool
}

// NewEventHandler creates a new event handler
func NewEventHandler(stateProvider StateEventProvider, refreshFunc func(context.Context) error) *EventHandler {
	return &EventHandler{
		stateProvider: stateProvider,
		refreshFunc:   refreshFunc,
	}
}

// Start begins listening for orchestrator events
func (eh *EventHandler) Start(ctx context.Context) error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if eh.running {
		return nil
	}

	eh.ctx, eh.cancelFunc = context.WithCancel(ctx)
	eh.running = true

	// Subscribe to state changes from the orchestrator
	eventChan := eh.stateProvider.SubscribeToStateChanges()

	eh.wg.Add(1)
	go eh.handleEvents(eventChan)

	logging.Info("Aggregator-EventHandler", "Started event handler for MCP service state changes")
	return nil
}

// Stop stops the event handler
func (eh *EventHandler) Stop() error {
	eh.mu.Lock()
	if !eh.running {
		eh.mu.Unlock()
		return nil
	}

	eh.running = false
	cancelFunc := eh.cancelFunc
	eh.mu.Unlock()

	if cancelFunc != nil {
		cancelFunc()
	}

	// Wait for the event handling goroutine to finish
	eh.wg.Wait()

	logging.Info("Aggregator-EventHandler", "Stopped event handler")
	return nil
}

// IsRunning returns whether the event handler is currently running
func (eh *EventHandler) IsRunning() bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.running
}

// handleEvents processes orchestrator events in a background goroutine
func (eh *EventHandler) handleEvents(eventChan <-chan ServiceStateEvent) {
	defer eh.wg.Done()
	defer func() {
		// Mark as not running when goroutine exits
		eh.mu.Lock()
		eh.running = false
		eh.mu.Unlock()
	}()

	for {
		select {
		case <-eh.ctx.Done():
			logging.Debug("Aggregator-EventHandler", "Event handler context cancelled, stopping")
			return

		case event, ok := <-eventChan:
			if !ok {
				logging.Warn("Aggregator-EventHandler", "Event channel closed, stopping event handler")
				return
			}

			eh.processEvent(event)
		}
	}
}

// processEvent handles a single orchestrator event
func (eh *EventHandler) processEvent(event ServiceStateEvent) {
	// Filter for MCP service events only
	if !eh.isMCPServiceEvent(event) {
		return
	}

	logging.Debug("Aggregator-EventHandler", "Processing MCP service event: %s %s->%s",
		event.Label, event.OldState, event.NewState)

	// Check if this is a state change that requires aggregator refresh
	if eh.shouldRefreshAggregator(event) {
		eh.triggerRefresh(event)
	}
}

// isMCPServiceEvent checks if the event is related to an MCP service
func (eh *EventHandler) isMCPServiceEvent(event ServiceStateEvent) bool {
	return event.ServiceType == "MCPServer"
}

// shouldRefreshAggregator determines if the event requires aggregator refresh
func (eh *EventHandler) shouldRefreshAggregator(event ServiceStateEvent) bool {
	// Refresh when:
	// 1. MCP service becomes running (needs to be registered)
	// 2. MCP service stops being running (needs to be deregistered)
	// 3. MCP service fails (might need to be deregistered)

	oldState := event.OldState
	newState := event.NewState

	// Debug logging to track state changes
	logging.Debug("Aggregator-EventHandler", "Checking if refresh needed for %s: oldState='%s', newState='%s'",
		event.Label, oldState, newState)

	// Service became running - register it
	if oldState != "Running" && newState == "Running" {
		logging.Debug("Aggregator-EventHandler", "Service %s became running, will refresh", event.Label)
		return true
	}

	// Service stopped being running - deregister it
	if oldState == "Running" && newState != "Running" {
		logging.Debug("Aggregator-EventHandler", "Service %s stopped being running, will refresh", event.Label)
		return true
	}

	// Service failed - might need deregistration
	if newState == "Failed" {
		logging.Debug("Aggregator-EventHandler", "Service %s failed, will refresh", event.Label)
		return true
	}

	logging.Debug("Aggregator-EventHandler", "No refresh needed for %s state change", event.Label)
	return false
}

// triggerRefresh calls the refresh function to update the aggregator
func (eh *EventHandler) triggerRefresh(event ServiceStateEvent) {
	logging.Info("Aggregator-EventHandler", "Refreshing aggregator due to MCP service change: %s (%s->%s)",
		event.Label, event.OldState, event.NewState)

	// Create a context with timeout for the refresh operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Call the refresh function
	if err := eh.refreshFunc(ctx); err != nil {
		logging.Error("Aggregator-EventHandler", err, "Failed to refresh aggregator after MCP service change: %s", event.Label)

		// Log additional error details if available
		if event.Error != nil {
			logging.Debug("Aggregator-EventHandler", "Original service error: %v", event.Error)
		}
	} else {
		logging.Debug("Aggregator-EventHandler", "Successfully refreshed aggregator after %s state change", event.Label)
	}
}
