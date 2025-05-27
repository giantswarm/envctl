package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderNewConnectionInputViewV2 renders the new connection input view
func renderNewConnectionInputViewV2(m *model.ModelV2, width int) string {
	return renderNewConnectionInputView(&model.Model{
		NewConnectionInput: m.NewConnectionInput,
		Width:              width,
	}, width)
}

// renderHelpOverlayV2 renders the help overlay
func renderHelpOverlayV2(m *model.ModelV2) string {
	// Match v1 exactly
	titleView := color.HelpTitleStyle.Render("KEYBOARD SHORTCUTS")

	// Build help content matching v1 format
	var helpLines []string

	// Column 1 - Navigation
	col1 := []string{
		"↑/k        Move focus up",
		"↓/j        Move focus down",
		"Tab        Next panel",
		"Shift+Tab  Previous panel",
	}

	// Column 2 - Service Control
	col2 := []string{
		"n          New connection",
		"r          Restart service",
		"x          Stop service",
		"s          Switch K8s context",
		"y          Copy to clipboard",
	}

	// Column 3 - View Controls
	col3 := []string{
		"h/?        Toggle help",
		"L          Activity log",
		"C          MCP configuration",
		"M          MCP tools",
		"D          Toggle dark mode",
		"z          Toggle debug mode",
		"q          Quit",
	}

	// Find max lines
	maxLines := len(col1)
	if len(col2) > maxLines {
		maxLines = len(col2)
	}
	if len(col3) > maxLines {
		maxLines = len(col3)
	}

	// Pad columns to same length
	for len(col1) < maxLines {
		col1 = append(col1, "")
	}
	for len(col2) < maxLines {
		col2 = append(col2, "")
	}
	for len(col3) < maxLines {
		col3 = append(col3, "")
	}

	// Build lines with proper spacing
	helpLines = append(helpLines, "") // Empty line after title
	for i := 0; i < maxLines; i++ {
		line := fmt.Sprintf("%-25s%-30s%-25s", col1[i], col2[i], col3[i])
		helpLines = append(helpLines, line)
	}

	helpContent := strings.Join(helpLines, "\n")
	finalContent := titleView + "\n" + helpContent

	container := color.CenteredOverlayContainerStyle.Render(finalContent)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, container,
		lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
}

// renderLogOverlayV2 renders the log overlay
func renderLogOverlayV2(m *model.ModelV2) string {
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
	statusBar := renderStatusBarV2(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// renderMcpConfigOverlayV2 renders the MCP config overlay
func renderMcpConfigOverlayV2(m *model.ModelV2) string {
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
	statusBar := renderStatusBarV2(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// renderMcpToolsOverlayV2 renders the MCP tools overlay
func renderMcpToolsOverlayV2(m *model.ModelV2) string {
	// Match v1 exactly
	toolsTitleText := SafeIcon(IconGear) + " MCP Server Tools  (↑/↓ scroll  •  Esc close)"
	toolsTitleView := color.LogPanelTitleStyle.Render(toolsTitleText)
	toolsTitleHeight := lipgloss.Height(toolsTitleView)

	toolsOverlayTotalWidth := int(float64(m.Width) * 0.8)
	toolsOverlayTotalHeight := int(float64(m.Height) * 0.7)

	newToolsViewportWidth := toolsOverlayTotalWidth - color.McpConfigOverlayStyle.GetHorizontalFrameSize()
	newToolsViewportHeight := toolsOverlayTotalHeight - color.McpConfigOverlayStyle.GetVerticalFrameSize() - toolsTitleHeight

	if newToolsViewportWidth < 0 {
		newToolsViewportWidth = 0
	}
	if newToolsViewportHeight < 0 {
		newToolsViewportHeight = 0
	}

	// Update viewport dimensions
	m.McpToolsViewport.Width = newToolsViewportWidth
	m.McpToolsViewport.Height = newToolsViewportHeight

	// Generate and set content
	toolsContent := GenerateMcpToolsContentV2(m)
	m.McpToolsViewport.SetContent(toolsContent)

	viewportView := m.McpToolsViewport.View()
	content := lipgloss.JoinVertical(lipgloss.Left, toolsTitleView, viewportView)
	toolsOverlay := color.McpConfigOverlayStyle.Copy().
		Width(toolsOverlayTotalWidth - color.McpConfigOverlayStyle.GetHorizontalFrameSize()).
		Height(toolsOverlayTotalHeight - color.McpConfigOverlayStyle.GetVerticalFrameSize()).
		Render(content)

	overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, toolsOverlay,
		lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
	statusBar := renderStatusBarV2(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// GenerateMcpToolsContentV2 generates MCP tools content for ModelV2
func GenerateMcpToolsContentV2(m *model.ModelV2) string {
	var content []string

	for serverName, tools := range m.MCPTools {
		content = append(content, fmt.Sprintf("=== %s ===", serverName))
		if len(tools) == 0 {
			content = append(content, "  No tools available")
		} else {
			for _, tool := range tools {
				content = append(content, fmt.Sprintf("  • %s: %s", tool.Name, tool.Description))
			}
		}
		content = append(content, "")
	}

	if len(content) == 0 {
		return "No MCP servers with tools available"
	}

	return strings.Join(content, "\n")
}
