//go:build !msgsample
// +build !msgsample

// This stub provides a no-op implementation of RecordMsgSample when the
// `msgsample` build tag is NOT enabled.
package model

import tea "github.com/charmbracelet/bubbletea"

// RecordMsgSample is a no-op in normal builds. Activate the msg sampling
// feature by building with the `msgsample` build tag, which provides a real
// implementation.
func RecordMsgSample(msg tea.Msg) {}

// FinalizeMsgSampling is a no-op when msg sampling is disabled.
func FinalizeMsgSampling() {}
