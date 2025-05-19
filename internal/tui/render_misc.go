package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderNewConnectionInputView(m model, width int) string {
    var inputPrompt strings.Builder
    inputPrompt.WriteString("Enter new cluster information (ESC to cancel, Enter to confirm/next)\n\n")
    inputPrompt.WriteString(m.newConnectionInput.View())
    if m.currentInputStep == mcInputStep {
        inputPrompt.WriteString("\n\n[Input: Management Cluster Name]")
    } else {
        inputPrompt.WriteString(fmt.Sprintf("\n\n[Input: Workload Cluster Name for MC: %s (optional)]", m.stashedMcName))
    }
    return lipgloss.NewStyle().Padding(1,2).Border(lipgloss.RoundedBorder()).Width(width-4).Align(lipgloss.Center).Render(inputPrompt.String())
}

func renderHeader(m model, contentWidth int) string {
    if contentWidth < 40 {
        title := "envctl TUI"
        if m.isLoading { title = m.spinner.View() + " " + title }
        return headerStyle.Copy().Width(contentWidth).Render(title)
    }
    title := "envctl TUI - Press h for Help | Tab to Navigate | q to Quit"
    if m.isLoading { title = m.spinner.View() + " " + title }
    if m.debugMode {
        title += fmt.Sprintf(" | Mode: %s | Toggle Dark: D | Debug: z", m.colorMode)
    }
    frame := headerStyle.GetHorizontalFrameSize()
    if contentWidth <= frame { return "envctl TUI" }
    return headerStyle.Copy().Width(contentWidth-frame).Render(title)
} 