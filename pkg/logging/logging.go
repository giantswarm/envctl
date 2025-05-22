package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// LogLevel defines the severity of the log entry.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String makes LogLevel satisfy the fmt.Stringer interface.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func (l LogLevel) SlogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo // Default to INFO for unknown
	}
}

// LogEntry is the structured log entry passed to the TUI.
type LogEntry struct {
	Timestamp  time.Time
	Level      LogLevel
	Subsystem  string
	Message    string
	Err        error
	Attributes []slog.Attr // Using slog.Attr for flexibility
}

var (
	defaultLogger *slog.Logger
	tuiLogChannel chan LogEntry
	isTuiMode     bool
)

const tuiChannelBufferSize = 2048

// Initcommon initializes the logger for either TUI or CLI mode.
// This should be called once at application startup.
func Initcommon(mode string, level LogLevel, output io.Writer, channelBufferSize int) <-chan LogEntry {
	opts := &slog.HandlerOptions{
		Level: level.SlogLevel(),
	}

	var handler slog.Handler
	if mode == "tui" {
		isTuiMode = true
		if channelBufferSize <= 0 {
			channelBufferSize = tuiChannelBufferSize
		}
		tuiLogChannel = make(chan LogEntry, channelBufferSize)
		// For TUI, we primarily send to the channel.
		// We could also have a fallback handler for TUI initialization phase if needed,
		// or if some logs shouldn't go to the TUI display but to a file.
		// For now, TUI mode means logs are formatted and sent via tuiLogChannel.
		// The actual slog handler for TUI mode will be a custom one if we want
		// slog to directly send to our channel.
		// As a simpler start, the logging functions will manually send to tuiLogChannel.
		// And we can still set up a defaultLogger that might write to stderr during TUI init.
		handler = slog.NewTextHandler(os.Stderr, opts) // Fallback/debug handler for TUI mode
	} else { // cli mode
		isTuiMode = false
		// For CLI, we log directly to the provided output (e.g., os.Stdout, os.Stderr)
		// We can enhance this to support JSON later if outputFormat demands it.
		handler = slog.NewTextHandler(output, opts)
	}
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger) // Set for any global slog calls if necessary

	if isTuiMode {
		return tuiLogChannel
	}
	return nil
}

// InitForTUI initializes the logging system for TUI mode.
// It sets up a channel that the TUI will listen to for log entries.
func InitForTUI(filterLevel LogLevel) <-chan LogEntry {
	return Initcommon("tui", filterLevel, os.Stderr, tuiChannelBufferSize)
}

// InitForCLI initializes the logging system for CLI mode.
// Logs will be written to os.Stdout or os.Stderr based on level or configuration.
// outputFormat can be "text" or "json" in the future.
func InitForCLI(filterLevel LogLevel, output io.Writer) {
	Initcommon("cli", filterLevel, output, 0)
}

func logInternal(level LogLevel, subsystem string, err error, messageFmt string, args ...interface{}) {
	msg := messageFmt
	if len(args) > 0 {
		msg = fmt.Sprintf(messageFmt, args...)
	}

	now := time.Now()

	if isTuiMode {
		if tuiLogChannel != nil {
			entry := LogEntry{
				Timestamp: now,
				Level:     level,
				Subsystem: subsystem,
				Message:   msg,
				Err:       err,
			}
			// Blocking send to ensure FIFO and no loss, assuming TUI processes.
			// The channel is buffered, so it will only block if the buffer is full.
			tuiLogChannel <- entry 
		} else {
			// This case (TUI mode but channel is nil) should ideally not happen if InitForTUI was called.
			// Log to os.Stderr as an emergency fallback.
			fmt.Fprintf(os.Stderr, "[LOGGING_CRITICAL] TUI mode active but tuiLogChannel is nil. Log: %s [%s] %s\n", now.Format(time.RFC3339), level, msg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
		}
		return // Primary output for TUI mode is the channel.
	}

	// CLI mode logging
	if defaultLogger == nil {
		fmt.Fprintf(os.Stderr, "[LOGGING_ERROR] Logger not initialized. Log: %s [%s] %s\n", now.Format(time.RFC3339), level, msg)
		return
	}
	
	var slogAttrs []slog.Attr
	slogAttrs = append(slogAttrs, slog.String("subsystem", subsystem))
	if err != nil {
		slogAttrs = append(slogAttrs, slog.String("error", err.Error()))
	}

	defaultLogger.LogAttrs(context.Background(), level.SlogLevel(), msg, slogAttrs...)
}

// Debug logs a debug message.
func Debug(subsystem string, messageFmt string, args ...interface{}) {
	logInternal(LevelDebug, subsystem, nil, messageFmt, args...)
}

// Info logs an informational message.
func Info(subsystem string, messageFmt string, args ...interface{}) {
	logInternal(LevelInfo, subsystem, nil, messageFmt, args...)
}

// Warn logs a warning message.
func Warn(subsystem string, messageFmt string, args ...interface{}) {
	logInternal(LevelWarn, subsystem, nil, messageFmt, args...)
}

// Error logs an error message.
func Error(subsystem string, err error, messageFmt string, args ...interface{}) {
	logInternal(LevelError, subsystem, err, messageFmt, args...)
}

// Helper to convert our LogLevel to slog.Level
func (l LogLevel) toSlogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo // Default for safety
	}
}

// CloseTUIChannel closes the TUI log channel. Should be called on application shutdown.
func CloseTUIChannel() {
	if tuiLogChannel != nil {
		close(tuiLogChannel)
	}
} 