package model

import (
	"testing"
	"time"
)

func TestSetStatusMessage(t *testing.T) {
	// Create a model with some width
	m := &Model{
		Width:                100,
		StatusBarMessage:     "",
		StatusBarMessageType: StatusBarInfo,
	}

	// Test 1: First call to SetStatusMessage
	cmd1 := m.SetStatusMessage("First message", StatusBarSuccess, 1*time.Second)
	if m.StatusBarMessage != "First message" {
		t.Errorf("Expected StatusBarMessage 'First message', got '%s'", m.StatusBarMessage)
	}
	if m.StatusBarMessageType != StatusBarSuccess {
		t.Errorf("Expected StatusBarMessageType Success, got %v", m.StatusBarMessageType)
	}
	if m.StatusBarClearCancel == nil {
		t.Error("Expected StatusBarClearCancel to be non-nil after first call")
	}
	if cmd1 == nil {
		t.Error("Expected a non-nil tea.Cmd from SetStatusMessage")
	}
	cancelChan1 := m.StatusBarClearCancel

	// Test 2: Second call to SetStatusMessage (should cancel the first)
	cmd2 := m.SetStatusMessage("Second message", StatusBarError, 1*time.Second)
	if m.StatusBarMessage != "Second message" {
		t.Errorf("Expected StatusBarMessage 'Second message', got '%s'", m.StatusBarMessage)
	}
	if m.StatusBarMessageType != StatusBarError {
		t.Errorf("Expected StatusBarMessageType Error, got %v", m.StatusBarMessageType)
	}
	if m.StatusBarClearCancel == nil {
		t.Error("Expected StatusBarClearCancel to be non-nil after second call")
	}
	if m.StatusBarClearCancel == cancelChan1 {
		t.Error("Expected StatusBarClearCancel to be a new channel after second call")
	}
	// Check if the first channel was closed
	select {
	case <-cancelChan1:
		// Expected: channel is closed
	default:
		t.Error("Expected first StatusBarClearCancel channel to be closed")
	}
	if cmd2 == nil {
		t.Error("Expected a non-nil tea.Cmd from second SetStatusMessage call")
	}
}
