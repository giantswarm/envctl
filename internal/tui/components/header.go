package components

import (
	"envctl/internal/tui/design"
	"envctl/internal/tui/utils"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Header represents the application header
type Header struct {
	Title        string
	Subtitle     string
	ShowSpinner  bool
	SpinnerView  string
	Width        int
	RightContent string
}

// NewHeader creates a new header
func NewHeader(title string) *Header {
	return &Header{
		Title: title,
		Width: 80, // Default width
	}
}

// WithSubtitle adds a subtitle
func (h *Header) WithSubtitle(subtitle string) *Header {
	h.Subtitle = subtitle
	return h
}

// WithSpinner shows a spinner in the header
func (h *Header) WithSpinner(spinnerView string) *Header {
	h.ShowSpinner = true
	h.SpinnerView = spinnerView
	return h
}

// WithRightContent adds content to the right side
func (h *Header) WithRightContent(content string) *Header {
	h.RightContent = content
	return h
}

// WithWidth sets the header width
func (h *Header) WithWidth(width int) *Header {
	h.Width = width
	return h
}

// Render returns the styled header
func (h *Header) Render() string {
	// Build left side content
	var leftParts []string

	if h.ShowSpinner && h.SpinnerView != "" {
		leftParts = append(leftParts, h.SpinnerView)
	}

	leftParts = append(leftParts, h.Title)

	if h.Subtitle != "" {
		leftParts = append(leftParts, design.TextSecondaryStyle.Render(h.Subtitle))
	}

	leftContent := strings.Join(leftParts, " ")

	// Calculate padding for right alignment
	var content string
	if h.RightContent != "" {
		leftWidth := lipgloss.Width(leftContent)
		rightWidth := lipgloss.Width(h.RightContent)
		availableWidth := h.Width - design.SpaceSM*2 // Account for padding

		if leftWidth+rightWidth+2 <= availableWidth {
			// We have space for both
			padding := availableWidth - leftWidth - rightWidth
			content = leftContent + strings.Repeat(" ", padding) + h.RightContent
		} else {
			// Not enough space, prioritize left content
			content = utils.TruncateString(leftContent, availableWidth)
		}
	} else {
		content = leftContent
	}

	return design.HeaderStyle.Copy().
		Width(h.Width).
		MaxWidth(h.Width).
		Render(content)
}
