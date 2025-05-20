package model_test

import (
	"testing"

	"envctl/internal/tui/controller"
	"envctl/internal/tui/model"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// benchmarkMsgBurst returns a slice of representative messages that exercise
// the Update loop heavily but deterministically. We build it once and reuse it
// across iterations to avoid measuring slice construction time.
var benchmarkMsgs = func() []tea.Msg {
	const burst = 1000 // total messages in a single burst
	msgs := make([]tea.Msg, 0, burst)
	for i := 0; i < burst/3; i++ {
		// Key press (generic rune)
		msgs = append(msgs, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a"), Alt: false})
		// Spinner tick (very frequent in real UI)
		msgs = append(msgs, spinner.TickMsg{})
		// Window resize (less frequent but expensive)
		msgs = append(msgs, tea.WindowSizeMsg{Width: 120, Height: 40})
	}
	return msgs
}()

// newBenchModel constructs a minimal but functional model for the benchmark.
// It mirrors the production InitialModel but disables debug logging to focus
// on Update-loop cost alone.
func newBenchModel() tea.Model {
	mCore := model.InitialModel("mc", "wc", "mc", false)
	// Seed terminal dimensions
	mCore.Width = 120
	mCore.Height = 40

	app := controller.NewAppModel(mCore, "mc", "wc")
	return app
}

func BenchmarkModelUpdate(b *testing.B) {
	msgs := benchmarkMsgs
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var tm tea.Model = newBenchModel()
		for _, msg := range msgs {
			tm, _ = tm.Update(msg)
		}
	}
}
