package reporting

import (
	"bytes"
	"envctl/pkg/logging"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestConsoleReporter_Report(t *testing.T) {
	tests := []struct {
		name            string
		update          ManagedServiceUpdate
		expectedLevel   string // e.g., "DEBUG", "INFO", "WARN", "ERROR"
		expectedSubstr  string // Substring to find in the log message part
		expectErrorInLog bool   // Whether the error detail should be in the log message
	}{
		{
			name: "StateStarting - no error",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypePortForward,
				SourceLabel: "PF1",
				State:       StateStarting,
			},
			expectedLevel: "DEBUG", // ConsoleReporter maps Starting to Debug
			expectedSubstr:  "State: Starting",
		},
		{
			name: "StateRunning - no error",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypeMCPServer,
				SourceLabel: "MCP1",
				State:       StateRunning,
				IsReady:     true,
			},
			expectedLevel: "INFO",
			expectedSubstr:  "State: Running",
		},
		{
			name: "StateFailed - with error detail",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypeKube,
				SourceLabel: "LoginOp",
				State:       StateFailed,
				ErrorDetail: errors.New("epic fail"),
			},
			expectedLevel: "ERROR",
			expectedSubstr:  "State: Failed",
			expectErrorInLog: true, // ErrorDetail should be logged by logging.Error
		},
		{
			name: "StateFailed - no error detail",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypeSystem,
				SourceLabel: "SysCheck",
				State:       StateFailed,
			},
			expectedLevel: "ERROR",
			expectedSubstr:  "State: Failed",
		},
		{
			name: "StateUnknown - no error",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypePortForward,
				SourceLabel: "PF-Unknown",
				State:       StateUnknown,
			},
			expectedLevel: "WARN", // ConsoleReporter maps Unknown to Warn
			expectedSubstr:  "State: Unknown",
		},
		{
			name: "StateStopped - no error",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypeMCPServer,
				SourceLabel: "MCP-Stopped",
				State:       StateStopped,
			},
			expectedLevel: "INFO",
			expectedSubstr:  "State: Stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			// Setup logger to use buffer for this test run
			// This assumes InitForCLI can be used to redirect a global logger.
			// For more robust testing, dependency injection for the logger in ConsoleReporter would be better,
			// or pkg/logging should provide a way to get/set a test logger instance.
			originalLoggerOut := os.Stdout // Placeholder for actual original output
			originalLoggerLevel := logging.LevelInfo // Placeholder
			// Ideally, pkg/logging would have GetLevel() and GetOutput() for proper stashing and restoring.
			defer logging.InitForCLI(originalLoggerLevel, originalLoggerOut) // Attempt to restore

			// Set level for logging package based on what ConsoleReporter is expected to produce
			// To capture all levels ConsoleReporter might output based on its internal logic:
			logging.InitForCLI(logging.LevelDebug, &logBuf) 

			reporter := NewConsoleReporter()
			reporter.Report(tt.update)

			logOutput := logBuf.String()

			assert.Contains(t, logOutput, fmt.Sprintf("level=%s", tt.expectedLevel), "Expected log level does not match")
			assert.Contains(t, logOutput, tt.expectedSubstr, "Expected substring not found in log message")
			
			expectedSubsystem := string(tt.update.SourceType)
			if tt.update.SourceLabel != "" {
				expectedSubsystem = fmt.Sprintf("%s-%s", tt.update.SourceType, tt.update.SourceLabel)
			}
			assert.Contains(t, logOutput, fmt.Sprintf("subsystem=%s", expectedSubsystem), "Expected subsystem does not match")

			if tt.expectErrorInLog {
				assert.Contains(t, logOutput, tt.update.ErrorDetail.Error(), "Expected error detail not found in log")
			} else if tt.update.ErrorDetail != nil {
				// If ErrorDetail was present in update but not expected in log (e.g. if Warn didn't include it)
				// For current ConsoleReporter, ErrorDetail always makes it an Error log with the detail.
				// So this else if might not be hit with current ConsoleReporter logic if ErrorDetail means it becomes ERROR.
			}
		})
	}
}

func TestTUIReporter_Report(t *testing.T) {
	tests := []struct {
		name         string
		update       ManagedServiceUpdate
		chanIsNil    bool
		expectSend   bool // Whether we expect a message to be sent on the channel
		blockChannel bool // Whether to simulate a full/blocked channel
	}{
		{
			name: "Valid update, valid channel",
			update: ManagedServiceUpdate{
				Timestamp:   time.Now(), 
				SourceType:  ServiceTypeSystem,
				SourceLabel: "Test1",
				State:       StateRunning,
			},
			expectSend: true,
		},
		{
			name: "Timestamp is zero, should be set",
			update: ManagedServiceUpdate{
				SourceType:  ServiceTypeKube,
				SourceLabel: "LoginTime",
				State:       StateStarting,
			},
			expectSend: true, // Timestamp will be set by Report method
		},
		{
			name: "Nil channel",
			update: ManagedServiceUpdate{
				SourceType: ServiceTypePortForward, State: StateFailed,
			},
			chanIsNil:  true,
			expectSend: false,
		},
		{
			name: "Blocked channel",
			update: ManagedServiceUpdate{
				SourceType: ServiceTypeMCPServer, State: StateStopped,
			},
			blockChannel: true, // Make channel unbuffered and don't read
			expectSend:    false, // Send should fail or timeout (select default)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ch chan tea.Msg
			if !tt.chanIsNil {
				if tt.blockChannel {
					ch = make(chan tea.Msg) // Unbuffered, will block if no receiver
				} else {
					ch = make(chan tea.Msg, 1) // Buffered, can receive one message
				}
			}
			// If ch is nil, tt.chanIsNil is true, NewTUIReporter handles this.

			reporter := NewTUIReporter(ch)
			reporter.Report(tt.update)

			if tt.expectSend {
				select {
				case msg := <-ch:
					assert.IsType(t, ReporterUpdateMsg{}, msg, "Message should be ReporterUpdateMsg")
					reportedUpdate := msg.(ReporterUpdateMsg).Update
					
					// Check if timestamp was set if originally zero
					if tt.update.Timestamp.IsZero() {
						assert.False(t, reportedUpdate.Timestamp.IsZero(), "Timestamp should have been set by Report")
						tt.update.Timestamp = reportedUpdate.Timestamp // Set to received for DeepEqual
					}
					assert.Equal(t, tt.update, reportedUpdate, "Reported update does not match original")
				case <-time.After(50 * time.Millisecond): // Increased timeout slightly
					t.Errorf("Expected a message on the channel, but timed out")
				}
			} else {
				// If not expecting send, ensure channel is empty (or wasn't sent to if blocked)
				select {
				case msg := <-ch:
					t.Errorf("Did not expect a message on the channel, but got: %+v", msg)
				default:
					// Channel is empty or blocked as expected, good.
				}
			}
		})
	}
} 