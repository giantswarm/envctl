package reporting

import (
	"sync"
	"time"
)

// ServiceStateSnapshot represents a complete snapshot of a service's state at a point in time
type ServiceStateSnapshot struct {
	Label         string
	SourceType    ServiceType
	State         ServiceState
	IsReady       bool
	ErrorDetail   error
	ProxyPort     int
	PID           int
	LastUpdated   time.Time
	CorrelationID string
	CausedBy      string
	ParentID      string
}

// StateChangeEvent represents a state change event with old and new states
type StateChangeEvent struct {
	Label    string
	OldState ServiceState
	NewState ServiceState
	Snapshot ServiceStateSnapshot
}

// StateSubscription represents a subscription to state changes
type StateSubscription struct {
	ID      string
	Label   string // Service label to watch, empty for all services
	Channel chan StateChangeEvent
	Closed  bool
	mu      sync.RWMutex
}

// Close closes the subscription channel
func (s *StateSubscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Closed {
		close(s.Channel)
		s.Closed = true
	}
}

// IsClosed returns whether the subscription is closed
func (s *StateSubscription) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Closed
}

// StateStore defines the interface for centralized state management
type StateStore interface {
	// GetServiceState returns the current state of a service
	GetServiceState(label string) (ServiceStateSnapshot, bool)

	// SetServiceState updates the state of a service
	SetServiceState(update ManagedServiceUpdate) (bool, error) // returns true if state changed

	// GetAllServiceStates returns all current service states
	GetAllServiceStates() map[string]ServiceStateSnapshot

	// GetServicesByType returns all services of a specific type
	GetServicesByType(serviceType ServiceType) map[string]ServiceStateSnapshot

	// GetServicesByState returns all services in a specific state
	GetServicesByState(state ServiceState) map[string]ServiceStateSnapshot

	// Subscribe creates a subscription to state changes for a specific service or all services
	Subscribe(label string) *StateSubscription // empty label subscribes to all changes

	// Unsubscribe removes a subscription
	Unsubscribe(subscription *StateSubscription)

	// Clear removes a service from the state store
	Clear(label string) bool // returns true if service existed

	// ClearAll removes all services from the state store
	ClearAll()

	// GetMetrics returns state store metrics
	GetMetrics() StateStoreMetrics
}

// StateStoreMetrics tracks state store performance and usage
type StateStoreMetrics struct {
	TotalServices       int
	TotalSubscriptions  int
	StateChanges        int64
	LastStateChange     time.Time
	ServicesByType      map[ServiceType]int
	ServicesByState     map[ServiceState]int
	SubscriptionMetrics SubscriptionMetrics
}

// SubscriptionMetrics tracks subscription-related metrics
type SubscriptionMetrics struct {
	ActiveSubscriptions  int
	TotalEventsDelivered int64
	DroppedEvents        int64
	LastEventTime        time.Time
}

// DefaultStateStore is the default implementation of StateStore
type DefaultStateStore struct {
	states        map[string]ServiceStateSnapshot
	subscriptions map[string]*StateSubscription
	metrics       StateStoreMetrics
	mu            sync.RWMutex
	subIDCounter  int64
}

// NewStateStore creates a new state store
func NewStateStore() StateStore {
	return &DefaultStateStore{
		states:        make(map[string]ServiceStateSnapshot),
		subscriptions: make(map[string]*StateSubscription),
		metrics: StateStoreMetrics{
			ServicesByType:  make(map[ServiceType]int),
			ServicesByState: make(map[ServiceState]int),
		},
	}
}

// GetServiceState returns the current state of a service
func (s *DefaultStateStore) GetServiceState(label string) (ServiceStateSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, exists := s.states[label]
	return snapshot, exists
}

// SetServiceState updates the state of a service
func (s *DefaultStateStore) SetServiceState(update ManagedServiceUpdate) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current state
	oldSnapshot, exists := s.states[update.SourceLabel]
	oldState := StateUnknown
	if exists {
		oldState = oldSnapshot.State
	}

	// Create new snapshot
	newSnapshot := ServiceStateSnapshot{
		Label:         update.SourceLabel,
		SourceType:    update.SourceType,
		State:         update.State,
		IsReady:       update.IsReady,
		ErrorDetail:   update.ErrorDetail,
		ProxyPort:     update.ProxyPort,
		PID:           update.PID,
		LastUpdated:   update.Timestamp,
		CorrelationID: update.CorrelationID,
		CausedBy:      update.CausedBy,
		ParentID:      update.ParentID,
	}

	// Check if state actually changed
	stateChanged := !exists || oldState != update.State

	// Update state
	s.states[update.SourceLabel] = newSnapshot

	// Update metrics
	s.updateMetrics(oldSnapshot, newSnapshot, exists, stateChanged)

	// Notify subscribers if state changed
	if stateChanged {
		event := StateChangeEvent{
			Label:    update.SourceLabel,
			OldState: oldState,
			NewState: update.State,
			Snapshot: newSnapshot,
		}
		s.notifySubscribers(event)
	}

	return stateChanged, nil
}

// GetAllServiceStates returns all current service states
func (s *DefaultStateStore) GetAllServiceStates() map[string]ServiceStateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ServiceStateSnapshot, len(s.states))
	for label, snapshot := range s.states {
		result[label] = snapshot
	}
	return result
}

// GetServicesByType returns all services of a specific type
func (s *DefaultStateStore) GetServicesByType(serviceType ServiceType) map[string]ServiceStateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ServiceStateSnapshot)
	for label, snapshot := range s.states {
		if snapshot.SourceType == serviceType {
			result[label] = snapshot
		}
	}
	return result
}

// GetServicesByState returns all services in a specific state
func (s *DefaultStateStore) GetServicesByState(state ServiceState) map[string]ServiceStateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ServiceStateSnapshot)
	for label, snapshot := range s.states {
		if snapshot.State == state {
			result[label] = snapshot
		}
	}
	return result
}

// Subscribe creates a subscription to state changes
func (s *DefaultStateStore) Subscribe(label string) *StateSubscription {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subIDCounter++
	subID := GenerateCorrelationID() + "_sub"

	subscription := &StateSubscription{
		ID:      subID,
		Label:   label,
		Channel: make(chan StateChangeEvent, 100), // Buffered channel
		Closed:  false,
	}

	s.subscriptions[subID] = subscription
	s.metrics.TotalSubscriptions++
	s.metrics.SubscriptionMetrics.ActiveSubscriptions++

	return subscription
}

// Unsubscribe removes a subscription
func (s *DefaultStateStore) Unsubscribe(subscription *StateSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.subscriptions[subscription.ID]; exists {
		subscription.Close()
		delete(s.subscriptions, subscription.ID)
		s.metrics.SubscriptionMetrics.ActiveSubscriptions--
	}
}

// Clear removes a service from the state store
func (s *DefaultStateStore) Clear(label string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshot, exists := s.states[label]
	if !exists {
		return false
	}

	delete(s.states, label)

	// Update metrics
	s.metrics.TotalServices--
	s.metrics.ServicesByType[snapshot.SourceType]--
	s.metrics.ServicesByState[snapshot.State]--

	// Clean up zero counts
	if s.metrics.ServicesByType[snapshot.SourceType] == 0 {
		delete(s.metrics.ServicesByType, snapshot.SourceType)
	}
	if s.metrics.ServicesByState[snapshot.State] == 0 {
		delete(s.metrics.ServicesByState, snapshot.State)
	}

	return true
}

// ClearAll removes all services from the state store
func (s *DefaultStateStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states = make(map[string]ServiceStateSnapshot)
	s.metrics.TotalServices = 0
	s.metrics.ServicesByType = make(map[ServiceType]int)
	s.metrics.ServicesByState = make(map[ServiceState]int)
}

// GetMetrics returns state store metrics
func (s *DefaultStateStore) GetMetrics() StateStoreMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	metrics := s.metrics
	metrics.ServicesByType = make(map[ServiceType]int)
	metrics.ServicesByState = make(map[ServiceState]int)

	for k, v := range s.metrics.ServicesByType {
		metrics.ServicesByType[k] = v
	}
	for k, v := range s.metrics.ServicesByState {
		metrics.ServicesByState[k] = v
	}

	return metrics
}

// updateMetrics updates internal metrics after a state change
func (s *DefaultStateStore) updateMetrics(oldSnapshot, newSnapshot ServiceStateSnapshot, existed, stateChanged bool) {
	if !existed {
		s.metrics.TotalServices++
		s.metrics.ServicesByType[newSnapshot.SourceType]++
	} else if stateChanged {
		// Update state counts
		s.metrics.ServicesByState[oldSnapshot.State]--
		if s.metrics.ServicesByState[oldSnapshot.State] == 0 {
			delete(s.metrics.ServicesByState, oldSnapshot.State)
		}
	}

	if stateChanged {
		s.metrics.ServicesByState[newSnapshot.State]++
		s.metrics.StateChanges++
		s.metrics.LastStateChange = time.Now()
	}
}

// notifySubscribers sends state change events to all relevant subscribers
func (s *DefaultStateStore) notifySubscribers(event StateChangeEvent) {
	for subID, subscription := range s.subscriptions {
		// Check if subscription is for this specific service or all services
		if subscription.Label == "" || subscription.Label == event.Label {
			if subscription.IsClosed() {
				// Clean up closed subscriptions
				delete(s.subscriptions, subID)
				s.metrics.SubscriptionMetrics.ActiveSubscriptions--
				continue
			}

			// Try to send event without blocking
			select {
			case subscription.Channel <- event:
				s.metrics.SubscriptionMetrics.TotalEventsDelivered++
				s.metrics.SubscriptionMetrics.LastEventTime = time.Now()
			default:
				// Channel is full, drop the event
				s.metrics.SubscriptionMetrics.DroppedEvents++
			}
		}
	}
}
