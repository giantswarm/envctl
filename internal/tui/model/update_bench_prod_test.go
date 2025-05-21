package model_test

import (
	"testing"

	"envctl/internal/tui/model"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// buildProductionMsgs assembles a slice of messages that reflects the sample
// distribution captured from a real session (see msg_sample.json).
func buildProductionMsgs() []tea.Msg {
	msgs := make([]tea.Msg, 0,
		// Adjusted capacity estimate after removing old messages
		408+4+15+1+2+1+2+1+2+1) // Removed counts for McpServerSetup, McpServerStatus, PortForwardCore, PortForwardSetup

	appendN := func(n int, m tea.Msg) {
		for i := 0; i < n; i++ {
			msgs = append(msgs, m)
		}
	}

	appendN(408, spinner.TickMsg{})
	appendN(4, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	appendN(15, tea.MouseMsg{})
	appendN(1, tea.WindowSizeMsg{Width: 120, Height: 40})
	appendN(2, model.ClearStatusBarMsg{})
	appendN(1, model.ClusterListResultMsg{})
	appendN(2, model.KubeContextResultMsg{})
	appendN(1, model.KubeContextSwitchedMsg{})
	appendN(2, model.NodeStatusMsg{})
	appendN(1, model.RequestClusterHealthUpdate{})

	return msgs
}

// newBenchModel should be defined once, or this test should call the one from update_bench_test.go
// For simplicity, let's assume it uses the same newBenchModel logic.
// If newBenchModel is not in this file, this benchmark would call the one from the other _test.go file
// if they are in the same package (model_test), which they are.

func BenchmarkModelUpdateProduction(b *testing.B) {
	msgs := buildProductionMsgs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// newBenchModel now returns *controller.AppModel which is a tea.Model
		var tm tea.Model = newBenchModel() // Uses newBenchModel from update_bench_test.go
		for _, msg := range msgs {
			tm, _ = tm.Update(msg)
		}
	}
}
