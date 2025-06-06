package components

import (
	"envctl/internal/tui/design"
	"envctl/internal/tui/utils"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PanelType defines the visual style of a panel
type PanelType int

const (
	PanelTypeDefault PanelType = iota
	PanelTypeSuccess
	PanelTypeError
	PanelTypeWarning
	PanelTypeInfo
)

// Panel represents a reusable panel component
type Panel struct {
	Title    string
	Content  string
	Width    int
	Height   int
	Focused  bool
	Type     PanelType
	ShowIcon bool
	Icon     string
}

// NewPanel creates a new panel with default settings
func NewPanel(title string) *Panel {
	return &Panel{
		Title:    title,
		Width:    design.MinPanelWidth,
		Height:   design.MinPanelHeight,
		Type:     PanelTypeDefault,
		ShowIcon: true,
	}
}

// WithContent sets the panel content
func (p *Panel) WithContent(content string) *Panel {
	p.Content = content
	return p
}

// WithDimensions sets the panel dimensions
func (p *Panel) WithDimensions(width, height int) *Panel {
	p.Width = width
	p.Height = height
	return p
}

// WithType sets the panel type for styling
func (p *Panel) WithType(panelType PanelType) *Panel {
	p.Type = panelType
	return p
}

// WithIcon sets a custom icon for the panel
func (p *Panel) WithIcon(icon string) *Panel {
	p.Icon = icon
	return p
}

// SetFocused updates the focus state
func (p *Panel) SetFocused(focused bool) *Panel {
	p.Focused = focused
	return p
}

// Render returns the styled panel
func (p *Panel) Render() string {
	// Ensure minimum dimensions
	if p.Width < design.MinPanelWidth {
		p.Width = design.MinPanelWidth
	}
	if p.Height < design.MinPanelHeight {
		p.Height = design.MinPanelHeight
	}

	// Select appropriate style based on type and focus
	style := p.getStyle()

	// Calculate inner dimensions
	innerWidth := p.Width - style.GetHorizontalFrameSize()
	innerHeight := p.Height - style.GetVerticalFrameSize()

	// Ensure inner dimensions are positive
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Build content
	var lines []string

	// Add title line
	if p.Title != "" {
		titleLine := p.renderTitle(innerWidth)
		lines = append(lines, titleLine)
		if p.Content != "" {
			lines = append(lines, "") // Add separator
		}
	}

	// Add content lines
	if p.Content != "" {
		contentLines := strings.Split(p.Content, "\n")
		availableHeight := innerHeight - len(lines)

		// Only process content if we have space
		if availableHeight > 0 {
			// Truncate or pad content to fit
			if len(contentLines) > availableHeight {
				if availableHeight > 1 {
					contentLines = contentLines[:availableHeight-1]
					contentLines = append(contentLines, "...")
				} else {
					contentLines = []string{"..."}
				}
			}

			for _, line := range contentLines {
				if lipgloss.Width(line) > innerWidth && innerWidth > 3 {
					line = utils.TruncateString(line, innerWidth-3) + "..."
				} else if innerWidth <= 3 {
					line = "..."
				}
				lines = append(lines, line)
			}
		}
	}

	// Pad to fill height
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}

	// Truncate if too many lines
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}

	content := strings.Join(lines, "\n")
	return style.
		Width(p.Width).
		Height(p.Height).
		Render(content)
}

// getStyle returns the appropriate style based on panel state
func (p *Panel) getStyle() lipgloss.Style {
	baseStyle := design.PanelStyle
	if p.Focused {
		baseStyle = design.PanelFocusedStyle
	}

	// Apply type-specific styling
	switch p.Type {
	case PanelTypeSuccess:
		return baseStyle.Copy().BorderForeground(design.ColorSuccess)
	case PanelTypeError:
		return baseStyle.Copy().BorderForeground(design.ColorError)
	case PanelTypeWarning:
		return baseStyle.Copy().BorderForeground(design.ColorWarning)
	case PanelTypeInfo:
		return baseStyle.Copy().BorderForeground(design.ColorInfo)
	default:
		return baseStyle
	}
}

// renderTitle renders the panel title with optional icon
func (p *Panel) renderTitle(width int) string {
	// If title is empty, don't render anything
	if p.Title == "" {
		return ""
	}

	var titleParts []string

	// Add icon if specified
	if p.ShowIcon && p.Icon != "" {
		iconStyle := p.getIconStyle()
		titleParts = append(titleParts, iconStyle.Render(p.Icon))
	}

	// Add title text
	titleStyle := design.TitleStyle
	if p.Focused {
		titleStyle = titleStyle.Copy().Foreground(design.ColorPrimary)
	}
	titleParts = append(titleParts, titleStyle.Render(p.Title))

	title := strings.Join(titleParts, " ")

	// Truncate if too long
	if lipgloss.Width(title) > width {
		title = utils.TruncateString(title, width-3) + "..."
	}

	return title
}

// getIconStyle returns the appropriate icon style
func (p *Panel) getIconStyle() lipgloss.Style {
	switch p.Type {
	case PanelTypeSuccess:
		return design.IconSuccessStyle
	case PanelTypeError:
		return design.IconErrorStyle
	case PanelTypeWarning:
		return design.IconWarningStyle
	case PanelTypeInfo:
		return design.IconInfoStyle
	default:
		if p.Focused {
			return design.IconPrimaryStyle
		}
		return design.IconDefaultStyle
	}
}
