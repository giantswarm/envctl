package tui

// clearStatusBarMsg is sent internally to wipe the status-bar after a timeout.
// The timer is set via model.setStatusMessage.
type clearStatusBarMsg struct{} 