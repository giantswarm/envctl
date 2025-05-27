package view

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"envctl/internal/api"
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"sort"
)

// RenderMCPToolsPanel renders a panel showing the tools available for an MCP server
func RenderMCPToolsPanel(m *model.Model, serverName string, width, height int) string {
	// Get tools for this server
	tools, exists := m.MCPTools[serverName]
	if !exists || len(tools) == 0 {
		return renderEmptyToolsPanel(serverName, width, height)
	}

	// Create styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(width - 2).
		Height(height - 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	toolNameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	descriptionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	// Build content
	var content strings.Builder
	content.WriteString(titleStyle.Render(fmt.Sprintf("🔧 %s Tools (%d)", serverName, len(tools))))
	content.WriteString("\n\n")

	// List tools
	for i, tool := range tools {
		if i > 0 {
			content.WriteString("\n")
		}

		toolLine := fmt.Sprintf("• %s", toolNameStyle.Render(tool.Name))
		if tool.Description != "" {
			toolLine += fmt.Sprintf(" - %s", descriptionStyle.Render(truncateString(tool.Description, width-10)))
		}
		content.WriteString(toolLine)

		// Stop if we're running out of space
		if i > height-6 {
			content.WriteString(fmt.Sprintf("\n... and %d more", len(tools)-i-1))
			break
		}
	}

	return borderStyle.Render(content.String())
}

// renderEmptyToolsPanel renders a panel when no tools are available
func renderEmptyToolsPanel(serverName string, width, height int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(width-2).
		Height(height-2).
		Align(lipgloss.Center, lipgloss.Center)

	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	content := fmt.Sprintf("🔧 %s Tools\n\n%s",
		serverName,
		emptyStyle.Render("No tools available or not yet loaded"))

	return borderStyle.Render(content)
}

// RenderMCPToolsList renders a simple list of MCP tools for a server
func RenderMCPToolsList(tools []api.MCPTool, maxWidth int) string {
	if len(tools) == 0 {
		return "No tools available"
	}

	var b strings.Builder
	for i, tool := range tools {
		if i > 0 {
			b.WriteString("\n")
		}

		name := tool.Name
		desc := tool.Description

		// Truncate if needed
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		if len(desc) > maxWidth-35 {
			desc = desc[:maxWidth-38] + "..."
		}

		b.WriteString(fmt.Sprintf("  • %-30s %s", name, desc))
	}

	return b.String()
}

// truncateString truncates a string to a maximum length, adding ellipsis if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

// GenerateMcpToolsContent generates the content for the MCP tools overlay
func GenerateMcpToolsContent(m *model.Model) string {
	if len(m.MCPTools) == 0 {
		return color.LogDebugStyle.Render("No MCP server tools available. Start an MCP server to see its tools.")
	}

	var content strings.Builder

	// Sort server names for consistent display
	serverNames := make([]string, 0, len(m.MCPTools))
	for name := range m.MCPTools {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)

	for i, serverName := range serverNames {
		if i > 0 {
			content.WriteString("\n\n")
		}

		// Server header
		serverHeader := color.PortTitleStyle.Render(fmt.Sprintf("🖥 %s", serverName))
		content.WriteString(serverHeader)
		content.WriteString("\n")

		tools := m.MCPTools[serverName]
		if len(tools) == 0 {
			content.WriteString(color.LogDebugStyle.Render("  No tools available"))
			continue
		}

		// Sort tools by name for consistent display
		sort.Slice(tools, func(i, j int) bool {
			return tools[i].Name < tools[j].Name
		})

		for _, tool := range tools {
			// Tool name
			toolName := color.HealthGoodStyle.Render(fmt.Sprintf("  • %s", tool.Name))
			content.WriteString(toolName)
			content.WriteString("\n")

			// Tool description
			if tool.Description != "" {
				desc := wrapText(fmt.Sprintf("    %s", tool.Description), 70)
				content.WriteString(color.LogDebugStyle.Render(desc))
				content.WriteString("\n")
			}

			// Parameters - parse the JSON schema
			if len(tool.InputSchema) > 0 {
				var schema struct {
					Properties map[string]map[string]interface{} `json:"properties"`
					Required   []string                          `json:"required"`
				}

				if err := json.Unmarshal(tool.InputSchema, &schema); err == nil && len(schema.Properties) > 0 {
					content.WriteString(color.LogInfoStyle.Render("    Parameters:\n"))

					// Sort parameter names
					paramNames := make([]string, 0, len(schema.Properties))
					for name := range schema.Properties {
						paramNames = append(paramNames, name)
					}
					sort.Strings(paramNames)

					for _, paramName := range paramNames {
						param := schema.Properties[paramName]

						// Check if parameter is required
						isRequired := false
						for _, req := range schema.Required {
							if req == paramName {
								isRequired = true
								break
							}
						}

						paramStr := fmt.Sprintf("      - %s", paramName)
						if isRequired {
							paramStr += " (required)"
						}
						paramStr += fmt.Sprintf(": %s", getParamType(param))

						if desc, ok := param["description"].(string); ok && desc != "" {
							paramStr += fmt.Sprintf(" - %s", desc)
						}

						content.WriteString(color.LogDebugStyle.Render(paramStr))
						content.WriteString("\n")
					}
				}
			}
		}
	}

	return content.String()
}

// renderMcpToolsOverlay renders the MCP tools overlay
func renderMcpToolsOverlay(m *model.Model, width, height int) string {
	titleText := SafeIcon(IconGear) + " MCP Server Tools  (↑/↓ scroll  •  Esc close)"
	titleView := color.LogPanelTitleStyle.Render(titleText)

	viewportView := m.McpToolsViewport.View()

	// Combine title and viewport
	content := lipgloss.JoinVertical(lipgloss.Left, titleView, viewportView)

	// Apply overlay styling
	return color.McpConfigOverlayStyle.Width(width).Height(height).Render(content)
}

// getParamType extracts the type information from a parameter schema
func getParamType(param map[string]interface{}) string {
	if typeVal, ok := param["type"].(string); ok {
		return typeVal
	}
	return "unknown"
}

// wrapText wraps text to a specified width
func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder
	currentLine.WriteString("    ") // Maintain indentation

	for i, word := range words {
		if i > 0 && currentLine.Len()+1+len(word) > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString("    ") // Maintain indentation
		}
		if i > 0 && currentLine.Len() > 4 { // More than just indentation
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}

	if currentLine.Len() > 4 { // More than just indentation
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}
