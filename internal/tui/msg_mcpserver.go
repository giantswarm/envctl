package tui

// -------------------- MCP proxy message types --------------------

type mcpServerSetupCompletedMsg struct {
	Label    string        // MCP name, e.g. "kubernetes"
	stopChan chan struct{} // stop channel if spawned successfully
	pid      int           // process PID
	status   string        // human-readable initial status
	err      error         // startup error (if any)
}

type mcpServerStatusUpdateMsg struct {
	Label     string // MCP name sending the update
	pid       int    // current PID (0 if unchanged)
	status    string // short status string (Running, Stopped, Error…)
	outputLog string // log line from stdout/stderr
	err       error  // error flag for terminal problems
}

// restartMcpServerMsg is emitted when the user requests a manual restart
// of a specific MCP proxy panel.
type restartMcpServerMsg struct {
	Label string // panel key / MCP name
}
