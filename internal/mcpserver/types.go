package mcpserver

// MCPServerConfig defines the configuration for an MCP server.
type MCPServerConfig struct {
	Name      string            // Unique name for the server (e.g., "kubernetes", "prometheus")
	ProxyPort int               // Port on which mcp-proxy will listen
	Command   string            // The actual command to run (e.g., "npx", "uvx")
	Args      []string          // Arguments for the command
	Env       map[string]string // Environment variables to set for the command
}

// McpDiscreteStatusUpdate is used to report discrete status changes from a running MCP process.
// It focuses on the state, not verbose logs.
type McpDiscreteStatusUpdate struct {
	Label         string // The unique label of the MCP server instance
	ProcessStatus string // A string indicating the npx process status, e.g., "NpxStarting", "NpxRunning", "NpxExitedWithError"
	ProcessErr    error  // The actual Go error object if the process failed or exited with an error
	PID           int    // Process ID, can be useful for diagnostics
}

// McpUpdateFunc is a callback function type for receiving McpDiscreteStatusUpdate messages.
type McpUpdateFunc func(update McpDiscreteStatusUpdate)

// ManagedMcpServerInfo holds information about an MCP server that has been initiated.
// It's sent over a channel by StartAllPredefinedMcpServers.
type ManagedMcpServerInfo struct {
	Label    string        // Name of the server
	PID      int           // Process ID if successfully started, otherwise 0
	StopChan chan struct{} // Channel to signal this server to stop; nil if startup failed badly
	Err      error         // Initial error during startup, if any
}
