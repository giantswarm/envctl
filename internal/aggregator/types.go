package aggregator

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClient defines the interface for MCP client operations
// This will be implemented by the client in the mcpserver package
type MCPClient interface {
	// Initialize establishes the connection and performs protocol handshake
	Initialize(ctx context.Context) error

	// Close cleanly shuts down the client connection
	Close() error

	// ListTools returns all available tools from the server
	ListTools(ctx context.Context) ([]mcp.Tool, error)

	// CallTool executes a specific tool and returns the result
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error)

	// ListResources returns all available resources from the server
	ListResources(ctx context.Context) ([]mcp.Resource, error)

	// ReadResource retrieves a specific resource
	ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error)

	// ListPrompts returns all available prompts from the server
	ListPrompts(ctx context.Context) ([]mcp.Prompt, error)

	// GetPrompt retrieves a specific prompt
	GetPrompt(ctx context.Context, name string, args map[string]interface{}) (*mcp.GetPromptResult, error)

	// Ping checks if the server is responsive
	Ping(ctx context.Context) error
}

// ServerInfo holds information about a registered MCP server
type ServerInfo struct {
	Name      string
	Client    MCPClient
	Tools     []mcp.Tool
	Resources []mcp.Resource
	Prompts   []mcp.Prompt
	Connected bool
	mu        sync.RWMutex
}

// UpdateTools updates the server's tool list
func (s *ServerInfo) UpdateTools(tools []mcp.Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tools = tools
}

// UpdateResources updates the server's resource list
func (s *ServerInfo) UpdateResources(resources []mcp.Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Resources = resources
}

// UpdatePrompts updates the server's prompt list
func (s *ServerInfo) UpdatePrompts(prompts []mcp.Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Prompts = prompts
}

// SetConnected updates the connection status
func (s *ServerInfo) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Connected = connected
}

// IsConnected returns the current connection status
func (s *ServerInfo) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Connected
}

// AggregatorConfig holds configuration for the aggregator
type AggregatorConfig struct {
	Port int    // Port to listen on for the aggregated SSE endpoint
	Host string // Host to bind to (default: localhost)
}

// RegistrationEvent represents a server registration/deregistration event
type RegistrationEvent struct {
	Type       EventType
	ServerName string
	Client     MCPClient
}

// EventType represents the type of registration event
type EventType int

const (
	EventRegister EventType = iota
	EventDeregister
)

// MCPClientProvider provides access to MCP clients from running servers
type MCPClientProvider interface {
	// GetMCPClient returns the MCP client for a specific service
	GetMCPClient(label string) (MCPClient, error)

	// GetAllMCPClients returns all available MCP clients
	GetAllMCPClients() map[string]MCPClient
}
