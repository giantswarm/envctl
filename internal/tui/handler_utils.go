package tui

// safeCloseChan closes a channel if it hasn't been closed yet.
func safeCloseChan(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}
