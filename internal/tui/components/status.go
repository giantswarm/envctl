package components

import (
	"envctl/internal/tui/design"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// StatusType represents different status states
type StatusType int

const (
	StatusTypeRunning StatusType = iota
	StatusTypeStopped
	StatusTypeFailed
	StatusTypeStarting
	StatusTypeUnknown
	StatusTypeHealthy
	StatusTypeUnhealthy
	StatusTypeDegraded
	StatusTypeChecking
)

// StatusIndicator represents a status with icon and text
type StatusIndicator struct {
	Type           StatusType
	Text           string
	ShowIcon       bool
	ShowText       bool
	CustomIcon     string
	CustomStyle    lipgloss.Style
	HasCustomStyle bool
}

// NewStatusIndicator creates a new status indicator
func NewStatusIndicator(statusType StatusType) *StatusIndicator {
	return &StatusIndicator{
		Type:     statusType,
		ShowIcon: true,
		ShowText: true,
	}
}

// WithText sets custom text for the status
func (s *StatusIndicator) WithText(text string) *StatusIndicator {
	s.Text = text
	return s
}

// WithIcon sets a custom icon
func (s *StatusIndicator) WithIcon(icon string) *StatusIndicator {
	s.CustomIcon = icon
	return s
}

// IconOnly shows only the icon
func (s *StatusIndicator) IconOnly() *StatusIndicator {
	s.ShowIcon = true
	s.ShowText = false
	return s
}

// TextOnly shows only the text
func (s *StatusIndicator) TextOnly() *StatusIndicator {
	s.ShowIcon = false
	s.ShowText = true
	return s
}

// WithStyle sets a custom style
func (s *StatusIndicator) WithStyle(style lipgloss.Style) *StatusIndicator {
	s.CustomStyle = style
	s.HasCustomStyle = true
	return s
}

// Render returns the styled status indicator
func (s *StatusIndicator) Render() string {
	var parts []string

	icon := s.getIcon()
	text := s.getText()
	style := s.getStyle()

	if s.ShowIcon && icon != "" {
		parts = append(parts, style.Render(icon))
	}

	if s.ShowText && text != "" {
		parts = append(parts, style.Render(text))
	}

	if len(parts) == 0 {
		return ""
	}

	if len(parts) == 1 {
		return parts[0]
	}

	return fmt.Sprintf("%s %s", parts[0], parts[1])
}

// getIcon returns the appropriate icon for the status
func (s *StatusIndicator) getIcon() string {
	if s.CustomIcon != "" {
		return s.CustomIcon
	}

	switch s.Type {
	case StatusTypeRunning, StatusTypeHealthy:
		return design.SafeIcon(design.IconCheck)
	case StatusTypeFailed, StatusTypeUnhealthy:
		return design.SafeIcon(design.IconCross)
	case StatusTypeStarting, StatusTypeChecking:
		return design.SafeIcon(design.IconHourglass)
	case StatusTypeStopped:
		return design.SafeIcon(design.IconStop)
	case StatusTypeDegraded:
		return design.SafeIcon(design.IconWarning)
	default:
		return design.SafeIcon(design.IconQuestion)
	}
}

// getText returns the appropriate text for the status
func (s *StatusIndicator) getText() string {
	if s.Text != "" {
		return s.Text
	}

	switch s.Type {
	case StatusTypeRunning:
		return "Running"
	case StatusTypeStopped:
		return "Stopped"
	case StatusTypeFailed:
		return "Failed"
	case StatusTypeStarting:
		return "Starting"
	case StatusTypeHealthy:
		return "Healthy"
	case StatusTypeUnhealthy:
		return "Unhealthy"
	case StatusTypeDegraded:
		return "Degraded"
	case StatusTypeChecking:
		return "Checking"
	default:
		return "Unknown"
	}
}

// getStyle returns the appropriate style for the status
func (s *StatusIndicator) getStyle() lipgloss.Style {
	if s.HasCustomStyle {
		return s.CustomStyle
	}

	switch s.Type {
	case StatusTypeRunning, StatusTypeHealthy:
		return design.TextSuccessStyle
	case StatusTypeFailed, StatusTypeUnhealthy:
		return design.TextErrorStyle
	case StatusTypeStarting, StatusTypeChecking:
		return design.TextWarningStyle
	case StatusTypeStopped:
		return design.TextSecondaryStyle
	case StatusTypeDegraded:
		return design.TextWarningStyle
	default:
		return design.TextStyle
	}
}

// StatusFromString converts a string status to StatusType
func StatusFromString(status string) StatusType {
	switch status {
	case "running", "Running", "connected", "Connected":
		return StatusTypeRunning
	case "stopped", "Stopped", "disconnected", "Disconnected":
		return StatusTypeStopped
	case "failed", "Failed", "error", "Error":
		return StatusTypeFailed
	case "starting", "Starting", "connecting", "Connecting":
		return StatusTypeStarting
	case "healthy", "Healthy":
		return StatusTypeHealthy
	case "unhealthy", "Unhealthy":
		return StatusTypeUnhealthy
	case "degraded", "Degraded":
		return StatusTypeDegraded
	case "checking", "Checking":
		return StatusTypeChecking
	default:
		return StatusTypeUnknown
	}
}
