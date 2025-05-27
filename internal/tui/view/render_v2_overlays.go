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

	var helpLines []string

	columnSeparator := "  "
	interColumnGap := "   " // Space between logical columns (key+desc pairs)
	descColumnWidth := 20   // Approximate width for description text for inter-column padding

	keyBindingColumns := m.Keys.FullHelp() // [][]key.Binding, outer slice is columns

	if len(keyBindingColumns) == 0 {
		helpLines = append(helpLines, "No keybindings configured.")
	} else {
		// Pre-calculate the maximum width needed for keys IN EACH COLUMN
		maxKeyWidths := make([]int, len(keyBindingColumns))
		for c, column := range keyBindingColumns {
			currentMax := 0
			for _, binding := range column {
				keyWidth := lipgloss.Width(binding.Help().Key)
				if keyWidth > currentMax {
					currentMax = keyWidth
				}
			}
			maxKeyWidths[c] = currentMax
		}

		maxRows := 0
		for _, column := range keyBindingColumns {
			if len(column) > maxRows {
				maxRows = len(column)
			}
		}

		for r := 0; r < maxRows; r++ { // Iterate down the visual rows
			var currentLineStrBuilder strings.Builder
			for c := 0; c < len(keyBindingColumns); c++ { // Iterate across the columns
				if r < len(keyBindingColumns[c]) { // Check if current column has a binding for this row
					binding := keyBindingColumns[c][r]
					keyText := binding.Help().Key
					descText := binding.Help().Desc

					currentColKeyDisplayWidth := maxKeyWidths[c]
					currentKeyActualWidth := lipgloss.Width(keyText)
					paddingForKey := ""
					if currentKeyActualWidth < currentColKeyDisplayWidth {
						paddingForKey = strings.Repeat(" ", currentColKeyDisplayWidth-currentKeyActualWidth)
					}
					currentLineStrBuilder.WriteString(keyText)
					currentLineStrBuilder.WriteString(paddingForKey)
					currentLineStrBuilder.WriteString(columnSeparator)
					currentLineStrBuilder.WriteString(descText)

					if c < len(keyBindingColumns)-1 {
						currentDescActualWidth := lipgloss.Width(descText)
						paddingForDesc := ""
						if currentDescActualWidth < descColumnWidth {
							paddingForDesc = strings.Repeat(" ", descColumnWidth-currentDescActualWidth)
						}
						currentLineStrBuilder.WriteString(paddingForDesc)
						currentLineStrBuilder.WriteString(interColumnGap)
					}
				} else {
					if c < len(keyBindingColumns)-1 {
						fullCellWidthEstimate := maxKeyWidths[c] + len(columnSeparator) + descColumnWidth + len(interColumnGap)
						currentLineStrBuilder.WriteString(strings.Repeat(" ", fullCellWidthEstimate))
					}
				}
			}
			helpLines = append(helpLines, currentLineStrBuilder.String())
		}
	}

	helpContent := strings.Join(helpLines, "\n")

	finalContentString := titleView + "\n" + helpContent

	container := color.CenteredOverlayContainerStyle.Render(finalContentString)

	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, container, lipgloss.WithWhitespaceBackground(lipgloss.AdaptiveColor{Light: "rgba(0,0,0,0.1)", Dark: "rgba(0,0,0,0.6)"}))
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
