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

	// Test 3: Test message truncation
	longMessage := "This is a very long message that should be truncated because it exceeds the available width in the status bar"
	cmd3 := m.SetStatusMessage(longMessage, StatusBarInfo, 1*time.Second)
	if len(m.StatusBarMessage) > 100 { // Should be truncated based on width
		t.Errorf("Expected message to be truncated, but got length %d", len(m.StatusBarMessage))
	}
	if cmd3 == nil {
		t.Error("Expected a non-nil tea.Cmd from SetStatusMessage with long message")
	}

	// Test 4: Test with very small width
	m.Width = 10
	cmd4 := m.SetStatusMessage("Short", StatusBarWarning, 1*time.Second)
	if len(m.StatusBarMessage) > 10 {
		t.Errorf("Expected message to be truncated for small width, got '%s' (length %d)", m.StatusBarMessage, len(m.StatusBarMessage))
	}
	if cmd4 == nil {
		t.Error("Expected a non-nil tea.Cmd from SetStatusMessage with small width")
	}
}
