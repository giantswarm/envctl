package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func handleWindowSizeMsg(m model, msg tea.WindowSizeMsg) (model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true
	return m, nil
} 