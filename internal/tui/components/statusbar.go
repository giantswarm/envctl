package components

import (
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"envctl/internal/tui/utils"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar represents the bottom status bar
type StatusBar struct {
	Width       int
	Message     string
	MessageType model.MessageType
	LeftText    string
	RightText   string
	ShowMessage bool
}

// NewStatusBar creates a new status bar
func NewStatusBar(width int) *StatusBar {
	return &StatusBar{
		Width:       width,
		ShowMessage: false,
	}
}

// WithMessage sets a status message
func (s *StatusBar) WithMessage(message string, msgType model.MessageType) *StatusBar {
	s.Message = message
	s.MessageType = msgType
	s.ShowMessage = true
	return s
}

// WithLeftText sets the left side text
func (s *StatusBar) WithLeftText(text string) *StatusBar {
	s.LeftText = text
	return s
}

// WithRightText sets the right side text
func (s *StatusBar) WithRightText(text string) *StatusBar {
	s.RightText = text
	return s
}

// ClearMessage removes the status message
func (s *StatusBar) ClearMessage() *StatusBar {
	s.ShowMessage = false
	s.Message = ""
	return s
}

// Render returns the styled status bar
func (s *StatusBar) Render() string {
	// Select style based on message type
	style := s.getStyle()

	// Build content
	var content string

	if s.ShowMessage && s.Message != "" {
		// Show message
		content = s.Message
	} else {
		// Show left/right text
		if s.LeftText != "" && s.RightText != "" {
			leftWidth := lipgloss.Width(s.LeftText)
			rightWidth := lipgloss.Width(s.RightText)
			padding := s.Width - leftWidth - rightWidth - design.SpaceSM*2 // Account for style padding

			if padding > 0 {
				content = s.LeftText + strings.Repeat(" ", padding) + s.RightText
			} else {
				// Not enough space, just show left text
				content = utils.TruncateString(s.LeftText, s.Width-design.SpaceSM*2)
			}
		} else if s.LeftText != "" {
			content = s.LeftText
		} else if s.RightText != "" {
			content = s.RightText
		}
	}

	return style.
		Width(s.Width).
		MaxWidth(s.Width).
		Render(content)
}

// getStyle returns the appropriate style based on message type
func (s *StatusBar) getStyle() lipgloss.Style {
	if s.ShowMessage {
		switch s.MessageType {
		case model.StatusBarSuccess:
			return design.StatusBarSuccessStyle
		case model.StatusBarError:
			return design.StatusBarErrorStyle
		case model.StatusBarWarning:
			return design.StatusBarWarningStyle
		case model.StatusBarInfo:
			return design.StatusBarInfoStyle
		default:
			return design.StatusBarStyle
		}
	}
	return design.StatusBarStyle
}

// FormatClusterInfo formats cluster information for the status bar
func FormatClusterInfo(mcName, wcName string) string {
	if mcName == "" {
		return "No clusters connected"
	}

	result := fmt.Sprintf("MC: %s", mcName)
	if wcName != "" {
		result += fmt.Sprintf(" / WC: %s", wcName)
	}

	return result
}
