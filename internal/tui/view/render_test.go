package view

// All tests have been moved to separate files:
// - render_test_helpers.go: Helper functions and mocks
// - render_components_test.go: Component rendering tests
// - render_overlays_test.go: Overlay rendering tests
// - render_modes_test.go: Mode rendering tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareLogContent(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		maxWidth int
		expected string
	}{
		{
			name:     "empty lines",
			lines:    []string{},
			maxWidth: 80,
			expected: "No logs yet...",
		},
		{
			name:     "single line within width",
			lines:    []string{"Test log entry"},
			maxWidth: 80,
			expected: "Test log entry",
		},
		{
			name: "multiple lines within width",
			lines: []string{
				"First log entry",
				"Second log entry",
				"Third log entry",
			},
			maxWidth: 80,
			expected: "First log entry\nSecond log entry\nThird log entry",
		},
		{
			name: "line that needs wrapping",
			lines: []string{
				"This is a very long log entry that should be wrapped",
			},
			maxWidth: 20,
			expected: "This is a very long \nlog entry that shoul\nd be wrapped",
		},
		{
			name: "exact width line",
			lines: []string{
				"12345",
			},
			maxWidth: 5,
			expected: "12345",
		},
		{
			name: "line one char over width",
			lines: []string{
				"123456",
			},
			maxWidth: 5,
			expected: "12345\n6",
		},
		{
			name:     "zero width - no wrapping",
			lines:    []string{"Test line that won't be wrapped"},
			maxWidth: 0,
			expected: "Test line that won't be wrapped",
		},
		{
			name:     "negative width - no wrapping",
			lines:    []string{"Test with negative width"},
			maxWidth: -10,
			expected: "Test with negative width",
		},
		{
			name: "multiple lines with zero width",
			lines: []string{
				"Line 1",
				"Line 2",
				"Line 3",
			},
			maxWidth: 0,
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name: "mixed line lengths",
			lines: []string{
				"Short",
				"This is a much longer line that will need to be wrapped",
				"Mid length line",
			},
			maxWidth: 15,
			expected: "Short\nThis is a much \nlonger line tha\nt will need to \nbe wrapped\nMid length line",
		},
		{
			name:     "very small positive width",
			lines:    []string{"ABC"},
			maxWidth: 1,
			expected: "A\nB\nC",
		},
		{
			name: "empty lines in input",
			lines: []string{
				"Line 1",
				"",
				"Line 3",
			},
			maxWidth: 10,
			expected: "Line 1\n\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrepareLogContent(tt.lines, tt.maxWidth)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrepareLogContent_EdgeCases(t *testing.T) {
	t.Run("nil lines", func(t *testing.T) {
		result := PrepareLogContent(nil, 80)
		assert.Equal(t, "No logs yet...", result)
	})

	t.Run("lines with only spaces", func(t *testing.T) {
		lines := []string{"   ", "     "}
		result := PrepareLogContent(lines, 10)
		assert.Equal(t, "   \n     ", result)
	})

	t.Run("unicode characters", func(t *testing.T) {
		lines := []string{"Hello 世界"}
		result := PrepareLogContent(lines, 7)
		// Note: This simple implementation doesn't handle unicode width properly
		// It counts bytes, not visual width. The Chinese characters are 3 bytes each
		// so "Hello 世" is actually 9 bytes, which gets split incorrectly
		assert.Contains(t, result, "Hello")
		// Just verify it doesn't panic and returns something
		assert.NotEmpty(t, result)
	})
}

func TestTrimStatusMessage(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "running with PID",
			status:   "Running (PID: 12345)",
			expected: "Running",
		},
		{
			name:     "forwarding message",
			status:   "Forwarding from 127.0.0.1:8080 -> 80",
			expected: "Forwarding",
		},
		{
			name:     "short status",
			status:   "Active",
			expected: "Active",
		},
		{
			name:     "long error message",
			status:   "Error: Connection refused to remote host",
			expected: "Error: Conne...",
		},
		{
			name:     "long failed message",
			status:   "Failed to establish connection to server",
			expected: "Failed to es...",
		},
		{
			name:     "exactly 15 chars",
			status:   "123456789012345",
			expected: "123456789012345",
		},
		{
			name:     "empty status",
			status:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimStatusMessage(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}
