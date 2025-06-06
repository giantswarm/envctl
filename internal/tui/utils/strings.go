package utils

import "github.com/charmbracelet/lipgloss"

// TruncateString truncates a string to the specified width
func TruncateString(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}

	// Simple truncation - could be improved with proper rune handling
	for lipgloss.Width(s) > width && len(s) > 0 {
		s = s[:len(s)-1]
	}
	return s
}
