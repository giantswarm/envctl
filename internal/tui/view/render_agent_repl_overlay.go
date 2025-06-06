package view

import (
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderAgentREPLOverlay renders the agent REPL overlay
func renderAgentREPLOverlay(m *model.Model) string {
	// Title text with instructions
	titleText := design.SafeIcon(design.IconTerminal) + " Agent REPL  (↑/↓ scroll or history  •  Tab complete  •  Enter execute  •  Esc close)"
	titleView := design.LogPanelTitleStyle.Render(titleText)
	titleHeight := lipgloss.Height(titleView)

	// Calculate overlay dimensions
	overlayTotalWidth := int(float64(m.Width) * 0.85)
	overlayTotalHeight := int(float64(m.Height) * 0.8)

	// Calculate viewport and input dimensions
	borderFrameSize := design.LogOverlayStyle.GetHorizontalFrameSize()
	verticalFrameSize := design.LogOverlayStyle.GetVerticalFrameSize()

	viewportWidth := overlayTotalWidth - borderFrameSize
	inputHeight := 3 // Height for the input field
	viewportHeight := overlayTotalHeight - verticalFrameSize - titleHeight - inputHeight

	if viewportWidth < 0 {
		viewportWidth = 0
	}
	if viewportHeight < 0 {
		viewportHeight = 0
	}

	// Update viewport dimensions
	m.AgentREPLViewport.Width = viewportWidth
	m.AgentREPLViewport.Height = viewportHeight

	// Render viewport
	viewportView := m.AgentREPLViewport.View()

	// Render input field with prompt
	prompt := design.AgentPromptStyle.Render("MCP> ")
	inputView := lipgloss.JoinHorizontal(lipgloss.Left, prompt, m.AgentREPLInput.View())

	// Add a separator line above the input
	separator := design.DimStyle.Render(strings.Repeat("─", viewportWidth))

	// Combine all elements
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleView,
		viewportView,
		separator,
		inputView,
	)

	// Apply the overlay style
	overlay := design.LogOverlayStyle.Copy().
		Width(overlayTotalWidth - borderFrameSize).
		Height(overlayTotalHeight - verticalFrameSize).
		Render(content)

	// Center the overlay on screen with background
	overlayCanvas := lipgloss.Place(m.Width, m.Height-1, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(design.ColorOverlay))

	// Add status bar at the bottom
	statusBar := renderStatusBar(m, m.Width)
	return lipgloss.JoinVertical(lipgloss.Left, overlayCanvas, statusBar)
}

// PrepareAgentREPLContent prepares the REPL output for display in the viewport
func PrepareAgentREPLContent(output []string, width int) string {
	if len(output) == 0 {
		welcomeMsg := design.DimStyle.Render("Welcome to the Agent REPL. Type 'help' for available commands.")
		return welcomeMsg
	}

	// Join all output lines
	content := strings.Join(output, "\n")

	// Wrap long lines if needed
	var wrappedLines []string
	for _, line := range strings.Split(content, "\n") {
		if len(line) <= width {
			wrappedLines = append(wrappedLines, line)
		} else {
			// Simple word wrap
			wrapped := wrapText(line, width)
			wrappedLines = append(wrappedLines, wrapped...)
		}
	}

	return strings.Join(wrappedLines, "\n")
}
