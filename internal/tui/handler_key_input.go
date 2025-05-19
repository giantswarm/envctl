package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsgInputMode processes key presses when the TUI is in the 'new connection input' mode.
// It deals with Enter/Ctrl+S submit, Esc cancel, Tab autocompletion and forwards all other
// keystrokes to the underlying bubbles/textinput component.
func handleKeyMsgInputMode(m model, keyMsg tea.KeyMsg) (model, tea.Cmd) {
    switch keyMsg.String() {
    case "ctrl+s": // Submit new connection (MC or WC)
        if m.currentInputStep == mcInputStep {
            mcName := m.newConnectionInput.Value()
            if mcName == "" {
                return m, nil // could set error feedback here
            }
            m.stashedMcName = mcName
            m.currentInputStep = wcInputStep
            m.newConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName)
            m.newConnectionInput.SetValue("")
            m.newConnectionInput.Focus()
            return m, nil
        }
        if m.currentInputStep == wcInputStep {
            wcName := m.newConnectionInput.Value()
            m.currentAppMode = ModeMainDashboard
            m.newConnectionInput.Blur()
            m.newConnectionInput.Reset()
            if len(m.portForwardOrder) > 0 {
                m.focusedPanelKey = m.portForwardOrder[0]
            }
            return m, func() tea.Msg { return submitNewConnectionMsg{mc: m.stashedMcName, wc: wcName} }
        }

    case "enter": // Confirm MC input and move to WC, or submit WC input
        if m.currentInputStep == mcInputStep {
            mcName := m.newConnectionInput.Value()
            if mcName == "" {
                return m, nil
            }
            m.stashedMcName = mcName
            m.currentInputStep = wcInputStep
            m.newConnectionInput.Prompt = fmt.Sprintf("Enter WC for %s (optional, Enter/Ctrl+S Submit, Esc Cancel, Tab Complete): ", mcName)
            m.newConnectionInput.SetValue("")
            m.newConnectionInput.Focus()
            return m, nil
        }
        if m.currentInputStep == wcInputStep {
            wcName := m.newConnectionInput.Value()
            m.currentAppMode = ModeMainDashboard
            m.newConnectionInput.Blur()
            m.newConnectionInput.Reset()
            if len(m.portForwardOrder) > 0 {
                m.focusedPanelKey = m.portForwardOrder[0]
            }
            return m, func() tea.Msg { return submitNewConnectionMsg{mc: m.stashedMcName, wc: wcName} }
        }

    case "esc": // Cancel new connection input
        m.currentAppMode = ModeMainDashboard
        m.newConnectionInput.Blur()
        m.newConnectionInput.Reset()
        m.currentInputStep = mcInputStep
        m.stashedMcName = ""
        if len(m.portForwardOrder) > 0 {
            m.focusedPanelKey = m.portForwardOrder[0]
        }
        return m, nil

    case "tab": // Autocompletion
        currentInput := m.newConnectionInput.Value()
        if m.clusterInfo != nil && currentInput != "" {
            var suggestions []string
            lower := strings.ToLower(currentInput)
            if m.currentInputStep == mcInputStep {
                for _, suggestion := range m.clusterInfo.ManagementClusters {
                    if strings.HasPrefix(strings.ToLower(suggestion), lower) {
                        suggestions = append(suggestions, suggestion)
                    }
                }
            } else if m.currentInputStep == wcInputStep && m.stashedMcName != "" {
                if wcs, ok := m.clusterInfo.WorkloadClusters[m.stashedMcName]; ok {
                    for _, suggestion := range wcs {
                        if strings.HasPrefix(strings.ToLower(suggestion), lower) {
                            suggestions = append(suggestions, suggestion)
                        }
                    }
                }
            }
            if len(suggestions) > 0 {
                m.newConnectionInput.SetValue(suggestions[0])
                m.newConnectionInput.SetCursor(len(suggestions[0]))
            }
        }
        return m, nil
    }

    // Default: forward to textinput
    var inputCmd tea.Cmd
    m.newConnectionInput, inputCmd = m.newConnectionInput.Update(keyMsg)
    return m, inputCmd
} 