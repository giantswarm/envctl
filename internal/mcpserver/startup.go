package mcpserver

import (
	"sync"
	// "syscall" // Not directly used here
	"envctl/internal/config" // Added for new config type
)

// StartAllMCPServers iterates through the given list of MCPServerDefinition configurations,
// attempts to start each one using StartAndManageIndividualMcpServer, and sends information
// about each attempt (ManagedMcpServerInfo) on the returned channel.
// The provided McpUpdateFunc will be used for ongoing updates from each server.
// The master goroutine that launches individual servers will add to the WaitGroup for each server it attempts.
// StartAndManageIndividualMcpServer is then responsible for wg.Done() if it successfully launches its own goroutine,
// otherwise this function (StartAllMCPServers) must handle wg.Done() for initial startup failures.
func StartAllMCPServers(mcpServerConfigs []config.MCPServerDefinition, updateFn McpUpdateFunc, wg *sync.WaitGroup) <-chan ManagedMcpServerInfo {
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

			pid, stopChan, startErr := StartAndManageIndividualMcpServer(
				serverCfg,
				updateFn,
				wg, // Pass wg; StartAndManageIndividualMcpServer's goroutine will call Done.
			)

			// If StartAndManageIndividualMcpServer returns an error, its goroutine (which calls wg.Done)
			// was not started. So, we must call wg.Done() here if wg is in use.
			if startErr != nil && wg != nil {
				wg.Done()
			}

			infoChan <- ManagedMcpServerInfo{
				Label:    serverCfg.Name,
				PID:      pid,
				StopChan: stopChan,
				Err:      startErr,
			}
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

	managedMcpChan := StartAllMCPServers(configs, mcpUpdateFn, wg)

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
