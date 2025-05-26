package reporting

import (
	"sync"
	"time"
)

// EventHandler is a function that processes events
type EventHandler func(Event)

// EventFilter is a function that determines if an event should be processed
type EventFilter func(Event) bool

// Subscription represents a subscription to events
type EventSubscription struct {
	ID      string
	Filter  EventFilter
	Handler EventHandler
	Channel chan Event
	Closed  bool
	mu      sync.RWMutex
}

// Close closes the subscription
func (s *EventSubscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Closed {
		if s.Channel != nil {
			close(s.Channel)
		}
		s.Closed = true
	}
}

// IsClosed returns whether the subscription is closed
func (s *EventSubscription) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Closed
}

// EventBus provides publish/subscribe functionality for events
type EventBus interface {
	// Publish publishes an event to all subscribers
	Publish(event Event)

	// Subscribe creates a subscription with a handler function
	Subscribe(filter EventFilter, handler EventHandler) *EventSubscription

	// SubscribeChannel creates a subscription with a channel
	SubscribeChannel(filter EventFilter, bufferSize int) *EventSubscription

	// Unsubscribe removes a subscription
	Unsubscribe(subscription *EventSubscription)

	// GetMetrics returns event bus metrics
	GetMetrics() EventBusMetrics

	// Close closes the event bus and all subscriptions
	Close()
}

// EventBusMetrics tracks event bus performance
type EventBusMetrics struct {
	TotalSubscriptions  int
	ActiveSubscriptions int
	EventsPublished     int64
	EventsDelivered     int64
	EventsDropped       int64
	LastEventTime       time.Time
	SubscriptionsByType map[EventType]int
	EventsByType        map[EventType]int64
	AverageDeliveryTime time.Duration
}

// DefaultEventBus is the default implementation of EventBus
type DefaultEventBus struct {
	subscriptions map[string]*EventSubscription
	metrics       EventBusMetrics
	mu            sync.RWMutex
	subIDCounter  int64
	closed        bool
}

// NewEventBus creates a new event bus
func NewEventBus() EventBus {
	return &DefaultEventBus{
		subscriptions: make(map[string]*EventSubscription),
		metrics: EventBusMetrics{
			SubscriptionsByType: make(map[EventType]int),
			EventsByType:        make(map[EventType]int64),
		},
	}
}

// Publish publishes an event to all subscribers
func (eb *DefaultEventBus) Publish(event Event) {
	eb.mu.RLock()
	if eb.closed {
		eb.mu.RUnlock()
		return
	}

	// Create a copy of subscriptions to avoid holding the lock during delivery
	subscriptionsCopy := make(map[string]*EventSubscription)
	for k, v := range eb.subscriptions {
		subscriptionsCopy[k] = v
	}
	eb.mu.RUnlock()

	startTime := time.Now()
	delivered := 0
	dropped := 0

	// Deliver to all matching subscriptions
	for subID, subscription := range subscriptionsCopy {
		if subscription.IsClosed() {
			// Clean up closed subscriptions
			eb.mu.Lock()
			delete(eb.subscriptions, subID)
			eb.metrics.ActiveSubscriptions--
			eb.mu.Unlock()
			continue
		}

		// Check if event matches filter
		if subscription.Filter != nil && !subscription.Filter(event) {
			continue
		}

		// Deliver via handler if available
		if subscription.Handler != nil {
			go func(handler EventHandler, evt Event) {
				defer func() {
					if r := recover(); r != nil {
						// Log panic in handler but don't crash the bus
					}
				}()
				handler(evt)
			}(subscription.Handler, event)
			delivered++
		}

		// Deliver via channel if available
		if subscription.Channel != nil {
			select {
			case subscription.Channel <- event:
				delivered++
			default:
				// Channel is full, drop the event
				dropped++
			}
		}
	}

	// Update metrics with proper synchronization
	eb.mu.Lock()
	eb.metrics.EventsPublished++
	if eb.metrics.EventsByType == nil {
		eb.metrics.EventsByType = make(map[EventType]int64)
	}
	eb.metrics.EventsByType[event.Type()]++
	eb.metrics.LastEventTime = event.Timestamp()
	eb.metrics.EventsDelivered += int64(delivered)
	eb.metrics.EventsDropped += int64(dropped)

	if delivered > 0 {
		deliveryTime := time.Since(startTime)
		// Simple moving average for delivery time
		if eb.metrics.AverageDeliveryTime == 0 {
			eb.metrics.AverageDeliveryTime = deliveryTime
		} else {
			eb.metrics.AverageDeliveryTime = (eb.metrics.AverageDeliveryTime + deliveryTime) / 2
		}
	}
	eb.mu.Unlock()
}

// Subscribe creates a subscription with a handler function
func (eb *DefaultEventBus) Subscribe(filter EventFilter, handler EventHandler) *EventSubscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return nil
	}

	eb.subIDCounter++
	subID := GenerateCorrelationID() + "_sub"

	subscription := &EventSubscription{
		ID:      subID,
		Filter:  filter,
		Handler: handler,
		Closed:  false,
	}

	eb.subscriptions[subID] = subscription
	eb.metrics.TotalSubscriptions++
	eb.metrics.ActiveSubscriptions++

	return subscription
}

// SubscribeChannel creates a subscription with a channel
func (eb *DefaultEventBus) SubscribeChannel(filter EventFilter, bufferSize int) *EventSubscription {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return nil
	}

	eb.subIDCounter++
	subID := GenerateCorrelationID() + "_sub"

	subscription := &EventSubscription{
		ID:      subID,
		Filter:  filter,
		Channel: make(chan Event, bufferSize),
		Closed:  false,
	}

	eb.subscriptions[subID] = subscription
	eb.metrics.TotalSubscriptions++
	eb.metrics.ActiveSubscriptions++

	return subscription
}

// Unsubscribe removes a subscription
func (eb *DefaultEventBus) Unsubscribe(subscription *EventSubscription) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if _, exists := eb.subscriptions[subscription.ID]; exists {
		subscription.Close()
		delete(eb.subscriptions, subscription.ID)
		eb.metrics.ActiveSubscriptions--
	}
}

// GetMetrics returns event bus metrics
func (eb *DefaultEventBus) GetMetrics() EventBusMetrics {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Return a copy to prevent external modification
	metrics := eb.metrics
	metrics.SubscriptionsByType = make(map[EventType]int)
	metrics.EventsByType = make(map[EventType]int64)

	for k, v := range eb.metrics.SubscriptionsByType {
		metrics.SubscriptionsByType[k] = v
	}
	for k, v := range eb.metrics.EventsByType {
		metrics.EventsByType[k] = v
	}

	return metrics
}

// Close closes the event bus and all subscriptions
func (eb *DefaultEventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.closed = true

	// Close all subscriptions
	for _, subscription := range eb.subscriptions {
		subscription.Close()
	}

	eb.subscriptions = make(map[string]*EventSubscription)
	eb.metrics.ActiveSubscriptions = 0
}

// Common event filters

// FilterByType creates a filter that matches events of specific types
func FilterByType(eventTypes ...EventType) EventFilter {
	typeMap := make(map[EventType]bool)
	for _, t := range eventTypes {
		typeMap[t] = true
	}

	return func(event Event) bool {
		return typeMap[event.Type()]
	}
}

// FilterBySource creates a filter that matches events from specific sources
func FilterBySource(sources ...string) EventFilter {
	sourceMap := make(map[string]bool)
	for _, s := range sources {
		sourceMap[s] = true
	}

	return func(event Event) bool {
		return sourceMap[event.Source()]
	}
}

// FilterBySeverity creates a filter that matches events with minimum severity
func FilterBySeverity(minSeverity EventSeverity) EventFilter {
	severityLevels := map[EventSeverity]int{
		SeverityTrace: 0,
		SeverityDebug: 1,
		SeverityInfo:  2,
		SeverityWarn:  3,
		SeverityError: 4,
		SeverityFatal: 5,
	}

	minLevel := severityLevels[minSeverity]

	return func(event Event) bool {
		eventLevel, exists := severityLevels[event.Severity()]
		return exists && eventLevel >= minLevel
	}
}

// FilterByCorrelation creates a filter that matches events with specific correlation ID
func FilterByCorrelation(correlationID string) EventFilter {
	return func(event Event) bool {
		return event.CorrelationID() == correlationID
	}
}

// CombineFilters combines multiple filters with AND logic
func CombineFilters(filters ...EventFilter) EventFilter {
	return func(event Event) bool {
		for _, filter := range filters {
			if !filter(event) {
				return false
			}
		}
		return true
	}
}

// AnyFilter combines multiple filters with OR logic
func AnyFilter(filters ...EventFilter) EventFilter {
	return func(event Event) bool {
		for _, filter := range filters {
			if filter(event) {
				return true
			}
		}
		return false
	}
}

// EventBusAdapter adapts the event bus to work with existing ServiceReporter interface
type EventBusAdapter struct {
	eventBus   EventBus
	stateStore StateStore
}

// NewEventBusAdapter creates a new adapter
func NewEventBusAdapter(eventBus EventBus, stateStore StateStore) *EventBusAdapter {
	if eventBus == nil {
		eventBus = NewEventBus()
	}
	if stateStore == nil {
		stateStore = NewStateStore()
	}

	return &EventBusAdapter{
		eventBus:   eventBus,
		stateStore: stateStore,
	}
}

// Report converts ManagedServiceUpdate to ServiceStateEvent and publishes it
func (eba *EventBusAdapter) Report(update ManagedServiceUpdate) {
	// Get old state from state store
	oldSnapshot, exists := eba.stateStore.GetServiceState(update.SourceLabel)
	oldState := StateUnknown
	if exists {
		oldState = oldSnapshot.State
	}

	// Update state store
	stateChanged, err := eba.stateStore.SetServiceState(update)
	if err != nil {
		// Log error but continue
		return
	}

	// Only publish events for actual state changes
	if stateChanged {
		// Create service state event
		event := NewServiceStateEvent(update.SourceType, update.SourceLabel, oldState, update.State)
		event.BaseEvent.WithCorrelation(update.CorrelationID, update.CausedBy, update.ParentID)

		if update.ErrorDetail != nil {
			event.WithError(update.ErrorDetail)
		}

		if update.ProxyPort > 0 || update.PID > 0 {
			event.WithServiceData(update.ProxyPort, update.PID)
		}

		// Publish the event
		eba.eventBus.Publish(event)
	}
}

// ReportHealth converts HealthStatusUpdate to HealthEvent and publishes it
func (eba *EventBusAdapter) ReportHealth(update HealthStatusUpdate) {
	event := NewHealthEvent(update.ContextName, update.ClusterShortName, update.IsMC, update.IsHealthy, update.ReadyNodes, update.TotalNodes)

	if update.Error != nil {
		event.WithError(update.Error)
	}

	// Publish the event
	eba.eventBus.Publish(event)
}

// GetStateStore returns the underlying state store
func (eba *EventBusAdapter) GetStateStore() StateStore {
	return eba.stateStore
}

// GetEventBus returns the underlying event bus
func (eba *EventBusAdapter) GetEventBus() EventBus {
	return eba.eventBus
}

// Close closes the adapter and underlying event bus
func (eba *EventBusAdapter) Close() {
	if eba.eventBus != nil {
		eba.eventBus.Close()
	}
}
