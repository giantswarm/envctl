package view

import (
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"fmt"
	"strings"
)

// Render(renders the UI for Model
func Render(m *model.Model) string {
	switch m.CurrentAppMode {
	case model.ModeQuitting:
		return design.TextStyle.Render(m.QuittingMessage)
	case model.ModeInitializing:
		if m.Width == 0 || m.Height == 0 {
			return design.TextStyle.Render("Initializing... (waiting for window size)")
		}
		return design.TextStyle.Render(fmt.Sprintf("%s Initializing...", m.Spinner.View()))
	case model.ModeNewConnectionInput:
		return renderNewConnectionInputView(m, m.Width)
	case model.ModeMainDashboard:
		// Initialize lists if not already done
		initializeLists(m)
		return renderMainDashboard(m)
	case model.ModeHelpOverlay:
		return renderHelpOverlay(m)
	case model.ModeLogOverlay:
		return renderLogOverlay(m)
	case model.ModeMcpConfigOverlay:
		return renderMcpConfigOverlay(m)
	case model.ModeMcpToolsOverlay:
		return renderMcpToolsOverlay(m)
	case model.ModeAgentREPLOverlay:
		return renderAgentREPLOverlay(m)
	default:
		return design.TextStyle.Render(fmt.Sprintf("Unhandled application mode: %s", m.CurrentAppMode.String()))
	}
}

// PrepareLogContent prepares log content for display in a viewport
func PrepareLogContent(lines []string, maxWidth int) string {
	if len(lines) == 0 {
		return "No logs yet..."
	}

	// Handle edge cases for maxWidth
	if maxWidth <= 0 {
		// If maxWidth is invalid, just join lines without wrapping
		return strings.Join(lines, "\n")
	}

	// Join lines and handle wrapping
	content := strings.Join(lines, "\n")

	// Simple word wrapping for long lines
	var wrapped []string
	for _, line := range strings.Split(content, "\n") {
		if len(line) <= maxWidth {
			wrapped = append(wrapped, line)
		} else {
			// Wrap long lines
			for len(line) > maxWidth {
				wrapped = append(wrapped, line[:maxWidth])
				line = line[maxWidth:]
			}
			if len(line) > 0 {
				wrapped = append(wrapped, line)
			}
		}
	}

	return strings.Join(wrapped, "\n")
}

// trimStatusMessage shortens long status strings for panel display.
func trimStatusMessage(status string) string {
	if strings.HasPrefix(status, "Forwarding from") {
		return "Forwarding"
	}
	if len(status) > 15 && (strings.Contains(status, "Error") || strings.Contains(status, "Failed")) {
		return status[:12] + "..."
	}
	return status
}

// initializeLists ensures the list models are initialized
func initializeLists(m *model.Model) {
	// Initialize MCP servers list if needed
	if m.MCPServersList == nil && len(m.MCPServerConfig) > 0 {
		// Calculate reasonable dimensions
		width := m.Width * 2 / 3
		height := m.Height / 2
		m.MCPServersList = BuildMCPServersList(m, width, height, m.FocusedPanelKey == "mcpservers")
	} else if m.MCPServersList != nil {
		// Update focus state
		listModel := m.MCPServersList.(*ServiceListModel)
		listModel.SetFocused(m.FocusedPanelKey == "mcpservers")
	}

	// Initialize MCP tools list if needed
	if m.MCPToolsList == nil && len(m.MCPToolsWithStatus) > 0 {
		// Calculate reasonable dimensions for overlay
		width := m.Width - 4
		height := m.Height - 10
		m.MCPToolsList = BuildMCPToolsList(m, width, height, false)
	}
}
