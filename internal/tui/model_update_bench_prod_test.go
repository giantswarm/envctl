package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// buildProductionMsgs assembles a slice of messages that reflects the sample
// distribution captured from a real session (see msg_sample.json).
func buildProductionMsgs() []tea.Msg {
	msgs := make([]tea.Msg, 0,
		408+4+15+1+2+1+2+1+3+3+2+5+3+1) // preallocate exact capacity

	appendN := func(n int, m tea.Msg) {
		for i := 0; i < n; i++ {
			msgs = append(msgs, m)
		}
	}

	appendN(408, spinner.TickMsg{})
	appendN(4, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	appendN(15, tea.MouseMsg{})
	appendN(1, tea.WindowSizeMsg{Width: 120, Height: 40})
	appendN(2, clearStatusBarMsg{})
	appendN(1, clusterListResultMsg{})
	appendN(2, kubeContextResultMsg{})
	appendN(1, kubeContextSwitchedMsg{})
	appendN(3, mcpServerSetupCompletedMsg{})
	appendN(3, mcpServerStatusUpdateMsg{})
	appendN(2, nodeStatusMsg{})
	appendN(5, portForwardCoreUpdateMsg{})
	appendN(3, portForwardSetupResultMsg{})
	appendN(1, requestClusterHealthUpdate{})

	return msgs
}

func BenchmarkModelUpdateProduction(b *testing.B) {
	msgs := buildProductionMsgs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mod := newBenchModel() // reuse existing helper from previous benchmark
		var tm tea.Model = mod
		for _, msg := range msgs {
			tm, _ = tm.Update(msg)
		}
	}
}
