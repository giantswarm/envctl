package mcpserver

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ManagedMcpServer represents a running MCP server with its client
type ManagedMcpServer struct {
	Label    string
	Client   aggregator.MCPClient
	StopChan chan struct{}
}

// StartAndManageIndividualMcpServer prepares, starts, and manages a single MCP server process.
// It returns the managed server info including the stdio client for communication.
func StartAndManageIndividualMcpServer(
	serverConfig config.MCPServerDefinition,
	updateFn McpUpdateFunc,
	wg *sync.WaitGroup,
) (*ManagedMcpServer, error) {

	label := serverConfig.Name
	subsystem := "MCPServer-" + label

	logging.Info(subsystem, "Starting MCP server %s: %s", label, strings.Join(serverConfig.Command, " "))

	if len(serverConfig.Command) == 0 {
		errMsg := fmt.Errorf("command not defined for MCP server %s", label)
		logging.Error(subsystem, errMsg, "Cannot start MCP server")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	// Create the command
	cmdName := serverConfig.Command[0]
	cmdArgs := serverConfig.Command[1:]

	// Create the stdio client with environment variables
	client := NewStdioClientWithEnv(cmdName, cmdArgs, serverConfig.Env)

	// Initialize the MCP protocol with a shorter timeout to fail fast for authentication issues
	// Use a separate context just for initialization
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	// Send initializing status
	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessInitializing"})
	}

	if err := client.Initialize(initCtx); err != nil {
		errMsg := fmt.Errorf("failed to initialize MCP client for %s: %w", label, err)
		logging.Error(subsystem, errMsg, "Failed to initialize MCP protocol")

		// Ensure the client is closed to clean up any processes
		if closeErr := client.Close(); closeErr != nil {
			logging.Debug(subsystem, "Error closing failed client for %s: %v", label, closeErr)
		}

		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	logging.Debug(subsystem, "MCP server started successfully: %s", label)

	stopChan := make(chan struct{})

	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{
			Label:         label,
			ProcessStatus: "ProcessRunning",
		})
	}

	// Monitor the server in a goroutine
	go func() {
		if wg != nil {
			defer wg.Done()
		}

		// Monitor stderr for logging
		if stderrReader, ok := client.GetStderr(); ok && stderrReader != nil {
			go func() {
				buf := make([]byte, 1024)
				for {
					n, err := stderrReader.Read(buf)
					if err != nil {
						if err != io.EOF {
							logging.Debug(subsystem, "Error reading stderr: %v", err)
						}
						return
					}
					if n > 0 {
						logging.Debug(subsystem, "MCP server %s stderr: %s", label, string(buf[:n]))
					}
				}
			}()
		}

		// Wait for stop signal only - don't use the initialization context
		<-stopChan
		logging.Debug(subsystem, "Received stop signal for MCP server %s", label)

		// Close the client (this should gracefully shut down the process)
		if err := client.Close(); err != nil {
			logging.Error(subsystem, err, "Error closing MCP client for %s", label)
		}

		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{
				Label:         label,
				ProcessStatus: "ProcessStoppedByUser",
			})
		}
	}()

	return &ManagedMcpServer{
		Label:    label,
		Client:   client,
		StopChan: stopChan,
	}, nil
}

// ProcessManager manages MCP server processes
type ProcessManager struct {
	servers map[string]*ManagedMcpServer
	mu      sync.RWMutex
}

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		servers: make(map[string]*ManagedMcpServer),
	}
}

// AddServer adds a managed server to the process manager
func (pm *ProcessManager) AddServer(server *ManagedMcpServer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.servers[server.Label] = server
}

// RemoveServer removes a managed server from the process manager
func (pm *ProcessManager) RemoveServer(label string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.servers, label)
}

// GetServer retrieves a managed server by label
func (pm *ProcessManager) GetServer(label string) (*ManagedMcpServer, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	server, exists := pm.servers[label]
	return server, exists
}

// StopServer stops a specific server
func (pm *ProcessManager) StopServer(label string) error {
	pm.mu.RLock()
	server, exists := pm.servers[label]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not found", label)
	}

	// Signal the server to stop
	close(server.StopChan)

	// Remove from manager
	pm.RemoveServer(label)

	return nil
}

// StopAll stops all managed servers
func (pm *ProcessManager) StopAll() {
	pm.mu.RLock()
	servers := make([]*ManagedMcpServer, 0, len(pm.servers))
	for _, server := range pm.servers {
		servers = append(servers, server)
	}
	pm.mu.RUnlock()

	// Stop all servers
	for _, server := range servers {
		close(server.StopChan)
	}

	// Clear the map
	pm.mu.Lock()
	pm.servers = make(map[string]*ManagedMcpServer)
	pm.mu.Unlock()
}
