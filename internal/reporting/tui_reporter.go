package reporting

import (
	// "envctl/internal/tui/model" // REMOVED import
	"envctl/pkg/logging"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIReporter is an implementation of ServiceReporter that sends updates to a buffered channel
// for the TUI to process with configurable overflow behavior and maintains state in a StateStore.
type TUIReporter struct {
	bufferedChan *BufferedChannel
	updateChan   chan<- tea.Msg // Keep for backwards compatibility
	stateStore   StateStore     // Centralized state management

	// Enhanced error handling
	retryQueue    []ManagedServiceUpdate // Queue for retrying failed updates
	retryAttempts map[string]int         // Track retry attempts per service
	maxRetries    int                    // Maximum retry attempts
	mu            sync.RWMutex           // Protect retry state
}

// TUIReporterConfig configures the TUI reporter behavior
type TUIReporterConfig struct {
	BufferSize     int
	BufferStrategy BufferStrategy
	StateStore     StateStore // Optional: if nil, a new one will be created
	MaxRetries     int        // Maximum retry attempts for critical updates
}

// DefaultTUIReporterConfig returns a sensible default configuration
func DefaultTUIReporterConfig() TUIReporterConfig {
	// Create a priority strategy that never drops critical messages
	strategy := NewPriorityBufferStrategy(BufferActionEvictOldest)
	strategy.SetPriority("ReporterUpdateMsg", BufferActionEvictOldest) // Service updates are important
	strategy.SetPriority("HealthStatusMsg", BufferActionEvictOldest)   // Health updates are important

	return TUIReporterConfig{
		BufferSize:     1000,
		BufferStrategy: strategy,
		StateStore:     nil, // Will be created automatically
		MaxRetries:     3,   // Retry critical updates up to 3 times
	}
}

// NewTUIReporter creates a new TUIReporter that sends updates to the provided TUI message channel.
// For backwards compatibility, this uses the default configuration.
func NewTUIReporter(updateChan chan<- tea.Msg) *TUIReporter {
	return NewTUIReporterWithConfig(updateChan, DefaultTUIReporterConfig())
}

// NewTUIReporterWithConfig creates a new TUIReporter with custom configuration
func NewTUIReporterWithConfig(updateChan chan<- tea.Msg, config TUIReporterConfig) *TUIReporter {
	if updateChan == nil {
		logging.Error("TUIReporter", nil, "NewTUIReporter called with nil updateChan. Using a dummy channel.")
		dummyChan := make(chan tea.Msg)
		go func() {
			for range dummyChan {
			}
		}()
		updateChan = dummyChan
	}

	// Create state store if not provided
	stateStore := config.StateStore
	if stateStore == nil {
		stateStore = NewStateStore()
	}

	bufferedChan := NewBufferedChannel(config.BufferSize, config.BufferStrategy)

	reporter := &TUIReporter{
		bufferedChan:  bufferedChan,
		updateChan:    updateChan,
		stateStore:    stateStore,
		retryQueue:    make([]ManagedServiceUpdate, 0),
		retryAttempts: make(map[string]int),
		maxRetries:    config.MaxRetries,
	}

	// Start a goroutine to forward messages from buffered channel to the original channel
	go func() {
		for {
			msg := bufferedChan.Receive()
			if msg == nil {
				break // Channel closed
			}
			select {
			case updateChan <- msg:
				// Successfully forwarded
			default:
				// Original channel is full or closed, log the issue
				logging.Warn("TUIReporter", "Original TUI channel full/closed, dropping forwarded message")
			}
		}
	}()

	// Start retry processor
	go reporter.processRetryQueue()

	return reporter
}

// Report processes a ManagedServiceUpdate by updating the state store and sending it to the TUI via the buffered channel.
func (t *TUIReporter) Report(update ManagedServiceUpdate) {
	if t.bufferedChan == nil {
		return
	}

	// Set timestamp if not provided
	if update.Timestamp.IsZero() {
		update.Timestamp = time.Now()
	}

	// Ensure correlation ID is set
	if update.CorrelationID == "" {
		update.CorrelationID = GenerateCorrelationID()
	}

	// Update the centralized state store first
	stateChanged := false
	if t.stateStore != nil {
		changed, err := t.stateStore.SetServiceState(update)
		if err != nil {
			logging.Error("TUIReporter", err, "Failed to update state store for service %s", update.SourceLabel)
		}
		stateChanged = changed
	}

	// Only send TUI updates for actual state changes to reduce noise
	if stateChanged || update.State == StateFailed || update.ErrorDetail != nil {
		// Send the update wrapped in a ReporterUpdateMsg
		msg := ReporterUpdateMsg{Update: update}
		sent := t.bufferedChan.Send(msg)

		if !sent {
			// Message was dropped, handle based on importance
			if t.isCriticalUpdate(update) {
				t.handleDroppedCriticalUpdate(update)
			} else {
				// Only log important state changes when dropped
				if update.State == StateFailed || update.State == StateRunning {
					logging.Warn("TUIReporter", "TUI buffer full, dropped service update for %s (state=%s, correlationID=%s)",
						update.SourceLabel, update.State, update.CorrelationID)
				}
			}
		}
	}
}

// isCriticalUpdate determines if an update is critical and should be retried
func (t *TUIReporter) isCriticalUpdate(update ManagedServiceUpdate) bool {
	return update.State == StateFailed ||
		update.State == StateRunning ||
		update.ErrorDetail != nil
}

// handleDroppedCriticalUpdate handles critical updates that were dropped
func (t *TUIReporter) handleDroppedCriticalUpdate(update ManagedServiceUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := update.SourceLabel + ":" + string(update.State)
	attempts := t.retryAttempts[key]

	if attempts < t.maxRetries {
		t.retryQueue = append(t.retryQueue, update)
		t.retryAttempts[key] = attempts + 1
		logging.Warn("TUIReporter", "Critical update dropped, queued for retry (attempt %d/%d): %s",
			attempts+1, t.maxRetries, update.SourceLabel)
	} else {
		logging.Error("TUIReporter", nil, "Critical update permanently dropped after %d attempts: %s",
			t.maxRetries, update.SourceLabel)

		// Send a notification to the user about the permanent failure
		notificationMsg := BackpressureNotificationMsg{
			ServiceLabel: update.SourceLabel,
			DroppedState: update.State,
			Reason:       "Buffer overflow - too many updates",
		}

		// Try to send notification, but don't retry if it fails
		t.bufferedChan.Send(notificationMsg)
	}
}

// processRetryQueue processes the retry queue periodically
func (t *TUIReporter) processRetryQueue() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		t.mu.Lock()
		if len(t.retryQueue) == 0 {
			t.mu.Unlock()
			continue
		}

		// Process up to 10 retries per cycle to avoid overwhelming
		batchSize := 10
		if len(t.retryQueue) < batchSize {
			batchSize = len(t.retryQueue)
		}

		batch := make([]ManagedServiceUpdate, batchSize)
		copy(batch, t.retryQueue[:batchSize])
		t.retryQueue = t.retryQueue[batchSize:]
		t.mu.Unlock()

		// Try to send each update in the batch
		for _, update := range batch {
			msg := ReporterUpdateMsg{Update: update}
			sent := t.bufferedChan.Send(msg)

			if !sent {
				// Still can't send, put it back in the queue
				t.handleDroppedCriticalUpdate(update)
			} else {
				// Successfully sent, clear retry count
				t.mu.Lock()
				key := update.SourceLabel + ":" + string(update.State)
				delete(t.retryAttempts, key)
				t.mu.Unlock()
			}
		}
	}
}

// GetStateStore returns the underlying state store
func (t *TUIReporter) GetStateStore() StateStore {
	return t.stateStore
}

// GetMetrics returns the current buffer metrics for monitoring
func (t *TUIReporter) GetMetrics() ChannelStats {
	if t.bufferedChan == nil {
		return ChannelStats{}
	}
	return t.bufferedChan.GetMetrics()
}

// GetStateStoreMetrics returns the state store metrics
func (t *TUIReporter) GetStateStoreMetrics() StateStoreMetrics {
	if t.stateStore == nil {
		return StateStoreMetrics{}
	}
	return t.stateStore.GetMetrics()
}

// Close closes the buffered channel
func (t *TUIReporter) Close() {
	if t.bufferedChan != nil {
		t.bufferedChan.Close()
	}
}
