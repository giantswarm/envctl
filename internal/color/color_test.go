package color

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestInitialize(t *testing.T) {
	tests := []struct {
		name       string
		isDarkMode bool
		expected   bool
	}{
		{"set dark mode", true, true},
		{"set light mode", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Initialize(tt.isDarkMode)
			if lipgloss.HasDarkBackground() != tt.expected {
				t.Errorf("lipgloss.HasDarkBackground() got %v, want %v after Initialize(%v)", lipgloss.HasDarkBackground(), tt.expected, tt.isDarkMode)
			}
		})
	}

	// Styles are global vars, their non-zero value is guaranteed by declaration.
	// A more involved test might check specific properties if needed.
} 