package mcpserver

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var execCommand = exec.Command

// ManagedMcpServer represents a running MCP server with its client
type ManagedMcpServer struct {
	Label    string
	PID      int
	Client   aggregator.MCPClient
	StopChan chan struct{}
	Cmd      *exec.Cmd
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
	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessInitializing", PID: 0})
	}

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
	cmd := execCommand(cmdName, cmdArgs...)

	// Set environment variables
	if len(serverConfig.Env) > 0 {
		env := cmd.Environ()
		for k, v := range serverConfig.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	// Create pipes for stdin, stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		errMsg := fmt.Errorf("failed to create stdin pipe: %w", err)
		logging.Error(subsystem, errMsg, "Failed to create pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := fmt.Errorf("failed to create stdout pipe: %w", err)
		logging.Error(subsystem, errMsg, "Failed to create pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		errMsg := fmt.Errorf("failed to create stderr pipe: %w", err)
		logging.Error(subsystem, errMsg, "Failed to create pipe")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	// Start the process
	logging.Debug(subsystem, "Starting process: %s %s", cmdName, strings.Join(cmdArgs, " "))
	if err := cmd.Start(); err != nil {
		errMsg := fmt.Errorf("failed to start process: %w", err)
		logging.Error(subsystem, errMsg, "Failed to start MCP server process")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	// Get the PID directly from our process
	pid := cmd.Process.Pid
	logging.Info(subsystem, "Started MCP server process with PID %d", pid)

	// Create client connected to the running process
	client := NewProcessClient(stdin, stdout, stderr)

	// Initialize the MCP protocol
	ctx := context.Background()
	initCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := client.Initialize(initCtx); err != nil {
		// Kill the process if initialization fails
		cmd.Process.Kill()
		cmd.Wait()

		errMsg := fmt.Errorf("failed to initialize MCP client for %s: %w", label, err)
		logging.Error(subsystem, errMsg, "Failed to initialize MCP protocol")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	logging.Debug(subsystem, "MCP server started successfully: %s with PID %d", label, pid)

	stopChan := make(chan struct{})

	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{
			Label:         label,
			PID:           pid,
			ProcessStatus: "ProcessRunning",
		})
	}

	// Monitor the server in a goroutine
	go func() {
		if wg != nil {
			defer wg.Done()
		}

		// Monitor stderr for logging
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := stderr.Read(buf)
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

		// Wait for either stop signal or process exit
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-stopChan:
			logging.Debug(subsystem, "Received stop signal for MCP server %s", label)

			// Close the client (this should gracefully shut down the protocol)
			if err := client.Close(); err != nil {
				logging.Error(subsystem, err, "Error closing MCP client")
			}

			// Give the process a moment to exit gracefully
			select {
			case <-done:
				// Process exited on its own
			case <-time.After(5 * time.Second):
				// Force kill if it doesn't exit
				logging.Warn(subsystem, "Process didn't exit gracefully, forcing kill")
				cmd.Process.Kill()
				<-done
			}

			if updateFn != nil {
				updateFn(McpDiscreteStatusUpdate{
					Label:         label,
					PID:           pid,
					ProcessStatus: "ProcessStoppedByUser",
				})
			}

		case err := <-done:
			// Process exited on its own
			logging.Info(subsystem, "MCP server process %s exited: %v", label, err)

			// Close the client
			client.Close()

			status := "ProcessExited"
			if err != nil {
				status = "ProcessExitedWithError"
			}

			if updateFn != nil {
				updateFn(McpDiscreteStatusUpdate{
					Label:         label,
					PID:           pid,
					ProcessStatus: status,
					ProcessErr:    err,
				})
			}
		}
	}()

	return &ManagedMcpServer{
		Label:    label,
		PID:      pid,
		Client:   client,
		StopChan: stopChan,
		Cmd:      cmd,
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
