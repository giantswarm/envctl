package reporting

import (
	// "envctl/internal/tui/model" // REMOVED import
	"envctl/pkg/logging"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIReporter is an implementation of ServiceReporter that sends updates to a buffered channel
// for the TUI to process with configurable overflow behavior.
type TUIReporter struct {
	bufferedChan *BufferedChannel
	updateChan   chan<- tea.Msg // Keep for backwards compatibility
}

// TUIReporterConfig configures the TUI reporter behavior
type TUIReporterConfig struct {
	BufferSize     int
	BufferStrategy BufferStrategy
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

	bufferedChan := NewBufferedChannel(config.BufferSize, config.BufferStrategy)

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

	return &TUIReporter{
		bufferedChan: bufferedChan,
		updateChan:   updateChan,
	}
}

// Report processes a ManagedServiceUpdate by sending it to the TUI via the buffered channel.
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

	// Send the update wrapped in a ReporterUpdateMsg
	msg := ReporterUpdateMsg{Update: update}
	sent := t.bufferedChan.Send(msg)

	if !sent {
		// Message was dropped, log based on importance
		if update.State == StateFailed || update.State == StateRunning {
			// Only log important state changes when dropped
			logging.Warn("TUIReporter", "TUI buffer full, dropped service update for %s (state=%s, correlationID=%s)",
				update.SourceLabel, update.State, update.CorrelationID)
		}
	}
}

// ReportHealth sends a health status update to the TUI
func (t *TUIReporter) ReportHealth(update HealthStatusUpdate) {
	if t.bufferedChan == nil {
		logging.Error("TUIReporter", nil, "ReportHealth called with nil buffered channel")
		return
	}

	// Send the health update wrapped in a HealthStatusMsg
	msg := HealthStatusMsg{Update: update}
	sent := t.bufferedChan.Send(msg)

	if !sent {
		// Health update was dropped
		logging.Warn("TUIReporter", "TUI buffer full, dropped health update for %s (IsMC=%v)",
			update.ClusterShortName, update.IsMC)
	}
}

// GetMetrics returns the current buffer metrics for monitoring
func (t *TUIReporter) GetMetrics() ChannelStats {
	if t.bufferedChan == nil {
		return ChannelStats{}
	}
	return t.bufferedChan.GetMetrics()
}

// Close closes the buffered channel
func (t *TUIReporter) Close() {
	if t.bufferedChan != nil {
		t.bufferedChan.Close()
	}
}
