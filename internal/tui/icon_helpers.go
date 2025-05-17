package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// SafeIcon wraps an icon with proper spacing to prevent rendering issues
// It ensures that an icon doesn't "swallow" the next character by adding
// spaces depending on the display width of the icon:
//   - If the icon occupies a single cell we append 1 space.
//   - If the icon occupies two cells (common for many emojis / NerdFont glyphs)
//     we append 2 spaces so that at least one space is visible after the icon.
func SafeIcon(icon string) string {
	// Determine how many cells the icon will occupy
	w := runewidth.StringWidth(icon)
	// Default to 1 trailing space
	spaces := 1
	// If the icon is wide (2 cells) then add an extra space to avoid swallowing
	if w >= 2 {
		spaces = 2
	}
	return fmt.Sprintf("%s%s", icon, strings.Repeat(" ", spaces))
}

// IconText formats an icon with text, handling spacing properly
func IconText(icon string, text string) string {
	return fmt.Sprintf("%s%s", SafeIcon(icon), text)
}

// RenderIconWithNodes formats health node information with proper icon spacing
func RenderIconWithNodes(icon string, readyNodes, totalNodes int, warningSuffix string) string {
	if warningSuffix != "" {
		return fmt.Sprintf("%sNodes: %d/%d %s", SafeIcon(icon), readyNodes, totalNodes, warningSuffix)
	}
	return fmt.Sprintf("%sNodes: %d/%d", SafeIcon(icon), readyNodes, totalNodes)
}

// RenderIconWithMessage formats a standard message with icon
func RenderIconWithMessage(icon string, message string) string {
	return fmt.Sprintf("%s%s", SafeIcon(icon), message)
} 