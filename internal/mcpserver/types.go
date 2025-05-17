package mcpserver

// PredefinedMcpServer holds the configuration for an MCP server that envctl knows how to start.
// The actual server (e.g., mcp-server-kubernetes) is run via mcp-proxy.
type PredefinedMcpServer struct {
	Name      string            // e.g., "kubernetes", "prometheus"
	ProxyPort int               // Port for mcp-proxy to listen on
	Command   string            // The underlying MCP server command (e.g., "npx", "uvx")
	Args      []string          // Arguments for the underlying MCP server command
	Env       map[string]string // Environment variables for the underlying MCP server command
}

// McpProcessUpdate is used to report status, logs, and errors from a running MCP process.
type McpProcessUpdate struct {
	Label     string
	PID       int
	Status    string // e.g., "Running", "Stopped", "Error", or empty if just a log line
	OutputLog string // Log line from stdout/stderr
	IsError   bool   // True if this update represents a critical error for the process itself
	Err       error  // The actual error if IsError is true
}

// McpUpdateFunc is a callback function type for receiving McpProcessUpdate messages.
type McpUpdateFunc func(update McpProcessUpdate)

// ManagedMcpServerInfo holds information about an MCP server that has been initiated.
// It's sent over a channel by StartAllPredefinedMcpServers.
type ManagedMcpServerInfo struct {
	Label    string        // Name of the server
	PID      int           // Process ID if successfully started, otherwise 0
	StopChan chan struct{} // Channel to signal this server to stop; nil if startup failed badly
	Err      error         // Initial error during startup, if any
}
