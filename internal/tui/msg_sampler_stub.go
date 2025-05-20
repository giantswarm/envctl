//go:build !msgsample
// +build !msgsample

// This stub provides a no-op implementation of recordMsgSample when the
// `msgsample` build tag is NOT enabled.
package tui

import tea "github.com/charmbracelet/bubbletea"

// recordMsgSample is a no-op in normal builds. Activate the msg sampling
// feature by building with the `msgsample` build tag, which provides a real
// implementation.
func recordMsgSample(msg tea.Msg) {}

// finalizeMsgSampling is a no-op when msg sampling is disabled.
func finalizeMsgSampling() {}
