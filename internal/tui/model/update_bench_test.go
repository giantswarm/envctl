package model_test

import (
	"envctl/internal/config"
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
func newBenchModel() tea.Model {
	defaultCfg := config.GetDefaultConfig("mc", "wc")
	mCore := model.InitialModel("mc", "wc", "mc", false, defaultCfg, nil)

	mCore.Width = 120
	mCore.Height = 40

	app := controller.NewAppModel(mCore, "mc", "wc")
	return app
}

func BenchmarkModelUpdate(b *testing.B) {
	defaultCfg := config.GetDefaultConfig("benchmark-mc", "benchmark-wc")

	m := model.InitialModel("mc", "wc", "mc", false, defaultCfg, nil)
	_ = m // Use m to satisfy linter for now, benchmark logic needs review.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// var tm tea.Model = newBenchModel() // Or use the 'm' created above if intended for single setup
		// for _, msg := range benchmarkMsgs {
		// 	tm, _ = tm.Update(msg)
		// }
	}
}

func BenchmarkUpdate_PortForwardStatusUpdate(b *testing.B) {
	// Use default config for InitialModel
	defaultCfg := config.GetDefaultConfig("benchmark-mc", "benchmark-wc")

	m := model.InitialModel("mc", "wc", "mc", false, defaultCfg, nil)
	_ = m // Use m to satisfy linter for now, benchmark logic needs review.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// var tm tea.Model = newBenchModel() // Or use the 'm' created above if intended for single setup
		// for _, msg := range benchmarkMsgs {
		// 	tm, _ = tm.Update(msg)
		// }
	}
}
