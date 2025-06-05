package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderNewConnectionInputView(renders the new connection input view
func renderNewConnectionInputView(m *model.Model, width int) string {
	return renderNewConnectionInputView(&model.Model{
		NewConnectionInput: m.NewConnectionInput,
		Width:              width,
	}, width)
}

// renderHelpOverlay(renders the help overlay
func renderHelpOverlay(m *model.Model) string {
	// Match v1 exactly
	titleView := color.HelpTitleStyle.Render("KEYBOARD SHORTCUTS")

	// Build help content
	var helpLines []string
	helpLines = append(helpLines, "")
	helpLines = append(helpLines, "Navigation:")
	helpLines = append(helpLines, "  Tab/Shift+Tab  Navigate between panels")
	helpLines = append(helpLines, "  ↑/↓ or j/k     Navigate within lists")
	helpLines = append(helpLines, "  /              Filter list (when focused)")
	helpLines = append(helpLines, "  q              Quit application")
	helpLines = append(helpLines, "")
	helpLines = append(helpLines, "Service Control:")
	helpLines = append(helpLines, "  Enter          Start stopped service")
	helpLines = append(helpLines, "  r              Restart focused service")
	helpLines = append(helpLines, "  x              Stop focused service")
	helpLines = append(helpLines, "  s              Switch to focused K8s context")
	helpLines = append(helpLines, "")
	helpLines = append(helpLines, "View Controls:")
	helpLines = append(helpLines, "  h or ?         Show/hide this help")
	helpLines = append(helpLines, "  L              Show activity log overlay")
	helpLines = append(helpLines, "  C              Show MCP configuration")
	helpLines = append(helpLines, "  M              Show MCP tools")
	helpLines = append(helpLines, "  A              Show Agent REPL")
	helpLines = append(helpLines, "  D              Toggle dark mode")
	helpLines = append(helpLines, "  z              Toggle debug mode")
	helpLines = append(helpLines, "")
	helpLines = append(helpLines, "In Overlays:")
	helpLines = append(helpLines, "  Esc            Close overlay")
	helpLines = append(helpLines, "  y              Copy content to clipboard")
	helpLines = append(helpLines, "  ↑/↓            Scroll content")

	helpContent := strings.Join(helpLines, "\n")

	container := color.CenteredOverlayContainerStyle.Render(titleView + "\n" + helpContent)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, container)
}

// renderLogOverlay(renders the log overlay
func renderLogOverlay(m *model.Model) string {
	// Match v1 exactly
	titleText := SafeIcon(IconScroll) + " Activity Log  (↑/↓ scroll  •  y copy  •  Esc close)"
	titleView := color.LogPanelTitleStyle.Render(titleText)
	titleHeight := lipgloss.Height(titleView)

	overlayTotalWidth := int(float64(m.Width) * 0.8)
	overlayTotalHeight := int(float64(m.Height) * 0.7)

	// Calculate actual content area for the viewport within the overlay
	newViewportWidth := overlayTotalWidth - color.LogOverlayStyle.GetHorizontalFrameSize()
	newViewportHeight := overlayTotalHeight - color.LogOverlayStyle.GetVerticalFrameSize() - titleHeight

	if newViewportWidth < 0 {
		newViewportWidth = 0
	}
	if newViewportHeight < 0 {
		newViewportHeight = 0
	}

	// Check if viewport dimensions or content needs updating
	dimensionsChanged := m.LogViewport.Width != newViewportWidth || m.LogViewport.Height != newViewportHeight

	m.LogViewport.Width = newViewportWidth
	m.LogViewport.Height = newViewportHeight

	// If dimensions changed OR if the activity log itself is dirty, re-prepare and re-set the content
	if m.ActivityLogDirty || dimensionsChanged {
		preparedLogOverlayContent := PrepareLogContent(m.ActivityLog, m.LogViewport.Width)
		m.LogViewport.SetContent(preparedLogOverlayContent)
	}

	// Render the overlay content
	title := color.LogPanelTitleStyle.Render(SafeIcon(IconScroll) + " Activity Log  (↑/↓ scroll  •  y copy  •  Esc close)")
	viewportView := m.LogViewport.View()
	content := lipgloss.JoinVertical(lipgloss.Left, title, viewportView)
	overlay := color.LogOverlayStyle.Copy().
		Width(overlayTotalWidth - color.LogOverlayStyle.GetHorizontalFrameSize()).
		Height(overlayTotalHeight - color.LogOverlayStyle.GetVerticalFrameSize()).
		Render(content)

	// Use lipgloss.Place to center the overlay
	overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))

	// Add status bar at the bottom
	statusBar := renderStatusBar(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// renderMcpConfigOverlay(renders the MCP config overlay
func renderMcpConfigOverlay(m *model.Model) string {
	// Match v1 exactly
	cfgTitleText := SafeIcon(IconGear) + " MCP Configuration  (↑/↓ scroll  •  y copy  •  Esc close)"
	cfgTitleView := color.LogPanelTitleStyle.Render(cfgTitleText)
	cfgTitleHeight := lipgloss.Height(cfgTitleView)

	cfgOverlayTotalWidth := int(float64(m.Width) * 0.8)
	cfgOverlayTotalHeight := int(float64(m.Height) * 0.7)

	newMcpViewportWidth := cfgOverlayTotalWidth - color.McpConfigOverlayStyle.GetHorizontalFrameSize()
	newMcpViewportHeight := cfgOverlayTotalHeight - color.McpConfigOverlayStyle.GetVerticalFrameSize() - cfgTitleHeight

	if newMcpViewportWidth < 0 {
		newMcpViewportWidth = 0
	}
	if newMcpViewportHeight < 0 {
		newMcpViewportHeight = 0
	}

	// Update viewport dimensions
	m.McpConfigViewport.Width = newMcpViewportWidth
	m.McpConfigViewport.Height = newMcpViewportHeight

	// Content is set by controller when mode changes
	viewportView := m.McpConfigViewport.View()
	content := lipgloss.JoinVertical(lipgloss.Left, cfgTitleView, viewportView)
	cfgOverlay := color.McpConfigOverlayStyle.Copy().
		Width(cfgOverlayTotalWidth - color.McpConfigOverlayStyle.GetHorizontalFrameSize()).
		Height(cfgOverlayTotalHeight - color.McpConfigOverlayStyle.GetVerticalFrameSize()).
		Render(content)

	overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, cfgOverlay,
		lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
	statusBar := renderStatusBar(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// renderMcpToolsOverlay(renders the MCP tools overlay
func renderMcpToolsOverlay(m *model.Model) string {
	// Initialize list if needed
	if m.MCPToolsList == nil && len(m.MCPToolsWithStatus) > 0 {
		width := int(float64(m.Width) * 0.8)
		height := int(float64(m.Height) * 0.7)
		m.MCPToolsList = BuildMCPToolsList(m, width-4, height-6, true)
	}

	if m.MCPToolsList == nil {
		return renderMainDashboard(m)
	}

	// Update list size and focus
	listModel := m.MCPToolsList.(*ServiceListModel)
	overlayWidth := int(float64(m.Width) * 0.8)
	overlayHeight := int(float64(m.Height) * 0.7)
	listModel.SetSize(overlayWidth-4, overlayHeight-6)
	listModel.SetFocused(true)

	// Title
	toolsTitleText := SafeIcon(IconGear) + " MCP Server Tools"
	if m.AggregatorInfo != nil && m.AggregatorInfo.YoloMode {
		toolsTitleText += "  [YOLO MODE ACTIVE]"
	}
	toolsTitleText += "  (↑/↓ navigate  •  / filter  •  Esc close)"
	toolsTitleView := color.LogPanelTitleStyle.Render(toolsTitleText)

	// Get list view
	listView := listModel.View()

	// Combine title and list
	content := lipgloss.JoinVertical(lipgloss.Left, toolsTitleView, listView)

	// Create overlay
	toolsOverlay := color.McpConfigOverlayStyle.Copy().
		Width(overlayWidth - color.McpConfigOverlayStyle.GetHorizontalFrameSize()).
		Height(overlayHeight - color.McpConfigOverlayStyle.GetVerticalFrameSize()).
		Render(content)

	overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, toolsOverlay,
		lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
	statusBar := renderStatusBar(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// GenerateMcpToolsContent generates MCP tools content for Model
func GenerateMcpToolsContent(m *model.Model) string {
	var content []string

	// Calculate available width for wrapping (viewport width minus some padding)
	wrapWidth := m.McpToolsViewport.Width - 4 // Leave some margin
	if wrapWidth < 40 {
		wrapWidth = 40 // Minimum width
	}

	for serverName, tools := range m.MCPTools {
		content = append(content, fmt.Sprintf("=== %s ===", serverName))
		if len(tools) == 0 {
			content = append(content, "  No tools available")
		} else {
			for _, tool := range tools {
				// Format tool name on its own line
				content = append(content, fmt.Sprintf("  • %s", tool.Name))

				// Wrap and indent the description
				if tool.Description != "" {
					wrapped := wrapText(tool.Description, wrapWidth-6) // Account for "    " indentation
					for _, line := range wrapped {
						content = append(content, fmt.Sprintf("    %s", line))
					}
				}
				content = append(content, "") // Empty line between tools
			}
		}
		content = append(content, "") // Empty line between servers
	}

	if len(content) == 0 {
		return "No MCP servers with tools available"
	}

	return strings.Join(content, "\n")
}

// wrapText wraps text to fit within the specified width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
