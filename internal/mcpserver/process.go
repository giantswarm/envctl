package mcpserver

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/config"
	"envctl/pkg/logging"
	"fmt"
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

	cmdName := serverConfig.Command[0]
	cmdArgs := serverConfig.Command[1:]

	// Create stdio client with environment variables
	client := NewStdioClientWithEnv(cmdName, cmdArgs, serverConfig.Env)

	// The client will manage the process when initialized
	ctx := context.Background()

	// Use a longer timeout for initialization to handle first-time npx downloads
	initCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := client.Initialize(initCtx); err != nil {
		errMsg := fmt.Errorf("failed to initialize MCP client for %s: %w", label, err)
		logging.Error(subsystem, errMsg, "Failed to start MCP server")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ProcessStartFailed", ProcessErr: errMsg})
		}
		return nil, errMsg
	}

	// For now, we don't have direct access to the process PID when using stdio client
	// This would require changes to the mcp-go library
	pid := 0

	logging.Debug(subsystem, "MCP server started successfully: %s", label)

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

		// Wait for stop signal
		<-stopChan

		logging.Debug(subsystem, "Received stop signal for MCP server %s", label)

		// Close the client (this stops the process)
		if err := client.Close(); err != nil {
			logging.Error(subsystem, err, "Error closing MCP client")
		}

		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{
				Label:         label,
				PID:           pid,
				ProcessStatus: "ProcessStoppedByUser",
			})
		}
	}()

	return &ManagedMcpServer{
		Label:    label,
		PID:      pid,
		Client:   client,
		StopChan: stopChan,
	}, nil
}
