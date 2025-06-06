package components

import (
	"envctl/internal/tui/design"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Layout helps organize the dashboard into sections
type Layout struct {
	Width  int
	Height int
}

// NewLayout creates a new layout manager
func NewLayout(width, height int) *Layout {
	return &Layout{
		Width:  width,
		Height: height,
	}
}

// SplitHorizontal splits the area horizontally by percentage
func (l *Layout) SplitHorizontal(topPercent float64) (topHeight, bottomHeight int) {
	// Ensure minimum layout dimensions
	if l.Height < design.MinPanelHeight*2 {
		l.Height = design.MinPanelHeight * 2
	}

	if topPercent <= 0 || topPercent >= 1 {
		topPercent = 0.5
	}

	topHeight = int(float64(l.Height) * topPercent)
	bottomHeight = l.Height - topHeight

	// Ensure minimum heights
	if topHeight < design.MinPanelHeight {
		topHeight = design.MinPanelHeight
		bottomHeight = l.Height - topHeight
	}
	if bottomHeight < design.MinPanelHeight {
		bottomHeight = design.MinPanelHeight
		topHeight = l.Height - bottomHeight
	}

	// Final safety check
	if topHeight < design.MinPanelHeight {
		topHeight = design.MinPanelHeight
	}
	if bottomHeight < design.MinPanelHeight {
		bottomHeight = design.MinPanelHeight
	}

	return topHeight, bottomHeight
}

// SplitVertical splits the area vertically by percentage
func (l *Layout) SplitVertical(leftPercent float64) (leftWidth, rightWidth int) {
	// Ensure minimum layout dimensions
	if l.Width < design.MinPanelWidth*2 {
		l.Width = design.MinPanelWidth * 2
	}

	if leftPercent <= 0 || leftPercent >= 1 {
		leftPercent = 0.5
	}

	leftWidth = int(float64(l.Width) * leftPercent)
	rightWidth = l.Width - leftWidth

	// Ensure minimum widths
	if leftWidth < design.MinPanelWidth {
		leftWidth = design.MinPanelWidth
		rightWidth = l.Width - leftWidth
	}
	if rightWidth < design.MinPanelWidth {
		rightWidth = design.MinPanelWidth
		leftWidth = l.Width - rightWidth
	}

	// Final safety check
	if leftWidth < design.MinPanelWidth {
		leftWidth = design.MinPanelWidth
	}
	if rightWidth < design.MinPanelWidth {
		rightWidth = design.MinPanelWidth
	}

	return leftWidth, rightWidth
}

// CalculateContentArea returns the available content area after accounting for header and status bar
func (l *Layout) CalculateContentArea(headerHeight, statusBarHeight int) int {
	contentHeight := l.Height - headerHeight - statusBarHeight
	if contentHeight < 0 {
		contentHeight = 0
	}
	return contentHeight
}

// JoinHorizontal joins components horizontally with optional gap
func JoinHorizontal(gap int, components ...string) string {
	if gap > 0 {
		spacer := strings.Repeat(" ", gap)
		parts := make([]string, 0, len(components)*2-1)
		for i, comp := range components {
			if i > 0 {
				parts = append(parts, spacer)
			}
			parts = append(parts, comp)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, components...)
}

// JoinVertical joins components vertically
func JoinVertical(components ...string) string {
	return lipgloss.JoinVertical(lipgloss.Left, components...)
}

// CenterContent centers content within the given dimensions
func CenterContent(width, height int, content string) string {
	return design.CenterVertical(height, design.CenterHorizontal(width, content))
}
