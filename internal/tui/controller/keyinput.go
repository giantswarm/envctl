package controller

import (
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsgInputMode processes key presses when the TUI is in the 'new connection input' mode.
// It deals with Enter/Ctrl+S submit, Esc cancel, Tab autocompletion and forwards all other
// keystrokes to the underlying bubbles/textinput component.
func handleKeyMsgInputMode(m *model.Model, keyMsg tea.KeyMsg) (*model.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "ctrl+s": // Submit new connection (MC or WC)
		if m.CurrentInputStep == model.McInputStep {
			mcName := m.NewConnectionInput.Value()
			if mcName == "" {
				return m, nil // could set error feedback here
			}
			m.StashedMcName = mcName
			m.CurrentInputStep = model.WcInputStep
			m.NewConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName)
			m.NewConnectionInput.SetValue("")
			m.NewConnectionInput.Focus()
			return m, nil
		}
		if m.CurrentInputStep == model.WcInputStep {
			wcName := m.NewConnectionInput.Value()
			m.CurrentAppMode = model.ModeMainDashboard
			m.NewConnectionInput.Blur()
			m.NewConnectionInput.Reset()
			if len(m.PortForwardOrder) > 0 {
				m.FocusedPanelKey = m.PortForwardOrder[0]
			}
			return m, func() tea.Msg { return model.SubmitNewConnectionMsg{MC: m.StashedMcName, WC: wcName} }
		}

	case "enter": // Confirm MC input and move to WC, or submit WC input
		if m.CurrentInputStep == model.McInputStep {
			mcName := m.NewConnectionInput.Value()
			if mcName == "" {
				return m, nil
			}
			m.StashedMcName = mcName
			m.CurrentInputStep = model.WcInputStep
			m.NewConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName)
			m.NewConnectionInput.SetValue("")
			m.NewConnectionInput.Focus()
			return m, nil
		}
		if m.CurrentInputStep == model.WcInputStep {
			wcName := m.NewConnectionInput.Value()
			m.CurrentAppMode = model.ModeMainDashboard
			m.NewConnectionInput.Blur()
			m.NewConnectionInput.Reset()
			if len(m.PortForwardOrder) > 0 {
				m.FocusedPanelKey = m.PortForwardOrder[0]
			}
			return m, func() tea.Msg { return model.SubmitNewConnectionMsg{MC: m.StashedMcName, WC: wcName} }
		}

	case "esc": // Cancel new connection input
		m.CurrentAppMode = model.ModeMainDashboard
		m.NewConnectionInput.Blur()
		m.NewConnectionInput.Reset()
		m.CurrentInputStep = model.McInputStep
		m.StashedMcName = ""
		if len(m.PortForwardOrder) > 0 {
			m.FocusedPanelKey = m.PortForwardOrder[0]
		}
		return m, nil

	case "tab": // Autocompletion
		currentInput := m.NewConnectionInput.Value()
		if m.ClusterInfo != nil && currentInput != "" {
			var suggestions []string
			lower := strings.ToLower(currentInput)
			if m.CurrentInputStep == model.McInputStep {
				for _, suggestion := range m.ClusterInfo.ManagementClusters {
					if strings.HasPrefix(strings.ToLower(suggestion), lower) {
						suggestions = append(suggestions, suggestion)
					}
				}
			} else if m.CurrentInputStep == model.WcInputStep && m.StashedMcName != "" {
				if wcs, ok := m.ClusterInfo.WorkloadClusters[m.StashedMcName]; ok {
					for _, suggestion := range wcs {
						if strings.HasPrefix(strings.ToLower(suggestion), lower) {
							suggestions = append(suggestions, suggestion)
						}
					}
				}
			}
			if len(suggestions) > 0 {
				m.NewConnectionInput.SetValue(suggestions[0])
				m.NewConnectionInput.SetCursor(len(suggestions[0]))
			}
		}
		return m, nil
	}

	// Default: forward to textinput
	var inputCmd tea.Cmd
	m.NewConnectionInput, inputCmd = m.NewConnectionInput.Update(keyMsg)
	return m, inputCmd
}
