package reporting

import (
	// "envctl/internal/tui/model" // REMOVED import
	"envctl/pkg/logging"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TUIReporter is an implementation of ServiceReporter that sends updates to a channel
// for the TUI to process.
type TUIReporter struct {
	updateChan chan<- tea.Msg
}

// NewTUIReporter creates a new TUIReporter that sends updates to the provided TUI message channel.
func NewTUIReporter(updateChan chan<- tea.Msg) *TUIReporter {
	if updateChan == nil {
		logging.Error("TUIReporter", nil, "NewTUIReporter called with nil updateChan. Using a dummy channel.")
		dummyChan := make(chan tea.Msg)
		go func() {
			for range dummyChan {
			}
		}()
		return &TUIReporter{updateChan: dummyChan}
	}
	return &TUIReporter{updateChan: updateChan}
}

// Report processes a ManagedServiceUpdate by sending it to the TUI via the channel.
func (t *TUIReporter) Report(update ManagedServiceUpdate) {
	if t.updateChan == nil {
		return
	}

	// Set timestamp if not provided
	if update.Timestamp.IsZero() {
		update.Timestamp = time.Now()
	}

	// Send the update wrapped in a ReporterUpdateMsg
	select {
	case t.updateChan <- ReporterUpdateMsg{Update: update}:
		// Successfully sent
	default:
		// Channel is full or closed, drop the update
		// Log when service updates are dropped (but less verbosely than health updates)
		if update.State == StateFailed || update.State == StateRunning {
			// Only log important state changes
			logging.Warn("TUIReporter", "TUI channel full, dropping service update for %s (state=%s)",
				update.SourceLabel, update.State)
		}
	}
}

// ReportHealth sends a health status update to the TUI
func (t *TUIReporter) ReportHealth(update HealthStatusUpdate) {
	if t.updateChan == nil {
		logging.Error("TUIReporter", nil, "ReportHealth called with nil channel")
		return
	}

	// Send the health update wrapped in a HealthStatusMsg
	msg := HealthStatusMsg{Update: update}
	select {
	case t.updateChan <- msg:
		// Successfully sent
	default:
		// Channel is full or closed, drop the update
		logging.Warn("TUIReporter", "TUI channel full/closed, dropping health update for %s (IsMC=%v)",
			update.ClusterShortName, update.IsMC)
	}
}
