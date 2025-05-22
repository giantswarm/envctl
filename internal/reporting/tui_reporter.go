package reporting

import (
	// "envctl/internal/tui/model" // REMOVED import
	"fmt"
	"os"
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
		fmt.Fprintf(os.Stderr, "CRITICAL_SETUP_ERROR: NewTUIReporter called with nil updateChan. Using a dummy channel.\n")
		dummyChan := make(chan tea.Msg)
		go func() {
			for range dummyChan {
			}
		}()
		return &TUIReporter{updateChan: dummyChan}
	}
	return &TUIReporter{updateChan: updateChan}
}

// Report sends the ManagedServiceUpdate wrapped in a ReporterUpdateMsg to the TUI's update channel.
func (tr *TUIReporter) Report(update ManagedServiceUpdate) {
	if update.Timestamp.IsZero() {
		update.Timestamp = time.Now()
	}

	if tr.updateChan == nil {
		// This fallback should ideally use pkg/logging if the TUI channel itself is broken.
		// However, TUIReporter's role is to send to the TUI. If it can't, a direct print is a last resort.
		fmt.Fprintf(os.Stderr, "TUIReporter: updateChan is nil. Dropping update: %v\n", update)
		return
	}

	select {
	case tr.updateChan <- ReporterUpdateMsg{Update: update}:
		// Update sent successfully
	default:
		// This fallback also indicates a problem with the TUI consuming messages.
		// Using update.State for the message part, as update.Message no longer exists.
		fmt.Fprintf(os.Stderr, "TUIReporter: Failed to send update to TUI channel (full or closed?). Dropping update for %s - %s (State: %s)\n", update.SourceType, update.SourceLabel, update.State)
	}
}
