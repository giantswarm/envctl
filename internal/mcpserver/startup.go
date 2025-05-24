package mcpserver

import (
	"sync"
	// "syscall" // Not directly used here
	"envctl/internal/config" // Added for new config type
	"envctl/internal/containerizer"
	"envctl/pkg/logging"
	"fmt"
)

// StartAllMCPServers iterates through the given list of MCPServerDefinition configurations,
// attempts to start each one using the appropriate method (local command or container),
// and sends information about each attempt (ManagedMcpServerInfo) on the returned channel.
func StartAllMCPServers(
	mcpServerConfigs []config.MCPServerDefinition,
	updateFn McpUpdateFunc,
	wg *sync.WaitGroup,
	containerRuntime containerizer.ContainerRuntime,
) <-chan ManagedMcpServerInfo {
	infoChan := make(chan ManagedMcpServerInfo)

	go func() {
		defer close(infoChan)

		if len(mcpServerConfigs) == 0 {
			// Caller can check if the channel closes immediately to know no servers were defined/attempted.
			return
		}

		for _, serverCfg := range mcpServerConfigs {
			if wg != nil {
				wg.Add(1) // Add before attempting to start, corresponding Done is crucial.
			}

			var pid int
			var containerID string
			var stopChan chan struct{}
			var startErr error

			// Choose the appropriate startup method based on server type
			switch serverCfg.Type {
			case config.MCPServerTypeLocalCommand:
				pid, stopChan, startErr = StartAndManageIndividualMcpServer(
					serverCfg,
					updateFn,
					wg, // Pass wg; StartAndManageIndividualMcpServer's goroutine will call Done.
				)

			case config.MCPServerTypeContainer:
				if containerRuntime == nil {
					startErr = fmt.Errorf("container runtime not available for server %s", serverCfg.Name)
					logging.Error("MCPStartup", startErr, "Cannot start containerized server")
				} else {
					containerID, stopChan, startErr = StartAndManageContainerizedMcpServer(
						serverCfg,
						containerRuntime,
						updateFn,
						wg,
					)
					// For consistency, we could use a hash of containerID as "PID"
					// but keeping it as 0 for containers is cleaner
					pid = 0
				}

			default:
				startErr = fmt.Errorf("unknown server type %s for server %s", serverCfg.Type, serverCfg.Name)
				logging.Error("MCPStartup", startErr, "Cannot start server with unknown type")
			}

			// If startup failed and goroutine wasn't started, we must call wg.Done()
			if startErr != nil && wg != nil {
				wg.Done()
			}

			info := ManagedMcpServerInfo{
				Label:       serverCfg.Name,
				PID:         pid,
				ContainerID: containerID,
				StopChan:    stopChan,
				Err:         startErr,
			}

			infoChan <- info
		}
	}()

	return infoChan
}

// StartMCPServers is an exported variable so it can be replaced for testing.
var StartMCPServers = startMCPServersInternal

// startMCPServersInternal is the actual implementation for starting MCP servers.
func startMCPServersInternal(
	configs []config.MCPServerDefinition,
	mcpUpdateFn McpUpdateFunc,
	wg *sync.WaitGroup,
) (map[string]chan struct{}, []error) {
	stopChans := make(map[string]chan struct{})
	var startupErrors []error

	if len(configs) == 0 {
		return stopChans, startupErrors
	}

	// Get global container runtime if needed
	var containerRuntime containerizer.ContainerRuntime
	hasContainerServers := false
	for _, cfg := range configs {
		if cfg.Type == config.MCPServerTypeContainer {
			hasContainerServers = true
			break
		}
	}

	if hasContainerServers {
		// TODO: Get runtime type from global config
		runtime, err := containerizer.NewContainerRuntime("docker")
		if err != nil {
			logging.Error("MCPStartup", err, "Failed to initialize container runtime")
			// Continue anyway, individual servers will fail
		} else {
			containerRuntime = runtime
		}
	}

	managedMcpChan := StartAllMCPServers(configs, mcpUpdateFn, wg, containerRuntime)

	for serverInfo := range managedMcpChan {
		if serverInfo.Err != nil {
			startupErrors = append(startupErrors, serverInfo.Err)
		}
		if serverInfo.StopChan != nil {
			stopChans[serverInfo.Label] = serverInfo.StopChan
		} else if serverInfo.Err == nil {
			// This case (no error but nil stopChan) should ideally not happen.
		}
	}

	return stopChans, startupErrors
}

// StartAllPredefinedMcpServers launches multiple MCP servers based on the provided configurations.
// It uses a channel to communicate the results of each server initialization asynchronously.
// Each server is launched in parallel using goroutines.
// 'wg' can be nil; if provided, each server increment the WaitGroup counter and decrement it when their lifecycle goroutine ends.
// 'updateFn' is called whenever there's a status update for any MCP server.
func StartAllPredefinedMcpServers(
	serverConfigs []config.MCPServerDefinition, // Updated type
	updateFn McpUpdateFunc,
	wg *sync.WaitGroup,
) <-chan ManagedMcpServerInfo {

	ch := make(chan ManagedMcpServerInfo, len(serverConfigs))
	var internalWg sync.WaitGroup

	for _, cfg := range serverConfigs {
		internalWg.Add(1)
		serverConfig := cfg // Capture loop variable
		if wg != nil {
			wg.Add(1)
		}

		// Create a channel to capture port updates for this server
		portChan := make(chan int, 1)
		serverLabel := serverConfig.Name

		// Wrap the updateFn to capture port updates
		wrappedUpdateFn := func(update McpDiscreteStatusUpdate) {
			if update.ProxyPort > 0 && update.Label == serverLabel {
				select {
				case portChan <- update.ProxyPort:
				default:
					// Channel already has a port, don't block
				}
			}
			if updateFn != nil {
				updateFn(update)
			}
		}

		go func() {
			defer internalWg.Done()

			pid, stopChan, err := StartAndManageIndividualMcpServer(serverConfig, wrappedUpdateFn, wg)

			// Try to get the port if it was detected
			var detectedPort int
			select {
			case detectedPort = <-portChan:
				// Got a port
			default:
				// No port detected yet, use configured port as fallback
				detectedPort = serverConfig.ProxyPort
			}

			ch <- ManagedMcpServerInfo{
				Label:     serverConfig.Name,
				PID:       pid,
				ProxyPort: detectedPort,
				StopChan:  stopChan,
				Err:       err,
			}
		}()
	}

	go func() {
		internalWg.Wait()
		close(ch)
	}()

	return ch
}
