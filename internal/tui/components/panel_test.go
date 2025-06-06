package components

import (
	"envctl/internal/tui/design"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanel_Render_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		title   string
		content string
	}{
		{
			name:    "zero dimensions",
			width:   0,
			height:  0,
			title:   "Test Panel",
			content: "This is test content",
		},
		{
			name:    "negative dimensions",
			width:   -10,
			height:  -5,
			title:   "Test Panel",
			content: "This is test content",
		},
		{
			name:    "very small dimensions",
			width:   1,
			height:  1,
			title:   "Test Panel",
			content: "This is test content",
		},
		{
			name:    "empty content",
			width:   40,
			height:  10,
			title:   "Test Panel",
			content: "",
		},
		{
			name:    "very long content",
			width:   20,
			height:  5,
			title:   "Test Panel",
			content: strings.Repeat("This is a very long line that should be truncated. ", 10),
		},
		{
			name:    "multiline content exceeding height",
			width:   30,
			height:  5,
			title:   "Test Panel",
			content: "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create panel with edge case dimensions
			panel := NewPanel(tt.title).
				WithContent(tt.content).
				WithDimensions(tt.width, tt.height)

			// This should not panic
			output := panel.Render()

			// Verify output is generated
			assert.NotEmpty(t, output, "Panel should produce output even with edge case dimensions")

			// Verify the panel has minimum dimensions applied
			assert.True(t, panel.Width >= design.MinPanelWidth, "Panel width should be at least MinPanelWidth")
			assert.True(t, panel.Height >= design.MinPanelHeight, "Panel height should be at least MinPanelHeight")
		})
	}
}

func TestPanel_RenderTitle_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		icon      string
		width     int
		expectLen bool
	}{
		{
			name:      "very long title",
			title:     "This is a very long title that should be truncated when rendered",
			icon:      "📦",
			width:     20,
			expectLen: true,
		},
		{
			name:      "title with zero width",
			title:     "Test Title",
			icon:      "📦",
			width:     0,
			expectLen: true,
		},
		{
			name:      "empty title",
			title:     "",
			icon:      "📦",
			width:     20,
			expectLen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := NewPanel(tt.title).WithIcon(tt.icon)

			// This should not panic
			titleOutput := panel.renderTitle(tt.width)

			if tt.expectLen {
				assert.NotEmpty(t, titleOutput, "Title should be rendered")
			} else {
				assert.Empty(t, titleOutput, "Empty title should produce empty output")
			}
		})
	}
}

func TestPanel_Types(t *testing.T) {
	types := []PanelType{
		PanelTypeDefault,
		PanelTypeSuccess,
		PanelTypeError,
		PanelTypeWarning,
		PanelTypeInfo,
	}

	for _, pt := range types {
		t.Run(pt.String(), func(t *testing.T) {
			panel := NewPanel("Test Panel").
				WithType(pt).
				WithDimensions(40, 10).
				WithContent("Test content")

			// Should not panic
			output := panel.Render()
			assert.NotEmpty(t, output, "Panel should render with any type")
		})
	}
}

// Helper function to get panel type name
func (pt PanelType) String() string {
	switch pt {
	case PanelTypeDefault:
		return "Default"
	case PanelTypeSuccess:
		return "Success"
	case PanelTypeError:
		return "Error"
	case PanelTypeWarning:
		return "Warning"
	case PanelTypeInfo:
		return "Info"
	default:
		return "Unknown"
	}
}
