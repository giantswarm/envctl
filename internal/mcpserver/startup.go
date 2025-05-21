package mcpserver

import (
	"sync"
	// "syscall" // Not directly used here
)

// StartAllMCPServers iterates through the given list of MCPServerConfig configurations,
// attempts to start each one using StartAndManageIndividualMcpServer, and sends information
// about each attempt (ManagedMcpServerInfo) on the returned channel.
// The provided McpUpdateFunc will be used for ongoing updates from each server.
// The master goroutine that launches individual servers will add to the WaitGroup for each server it attempts.
// StartAndManageIndividualMcpServer is then responsible for wg.Done() if it successfully launches its own goroutine,
// otherwise this function (StartAllMCPServers) must handle wg.Done() for initial startup failures.
func StartAllMCPServers(mcpServerConfigs []MCPServerConfig, updateFn McpUpdateFunc, wg *sync.WaitGroup) <-chan ManagedMcpServerInfo {
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

// StartAndManageMCPServersFunc is the type for the StartAndManageMCPServers function, for mocking.
var StartAndManageMCPServers = StartMCPServers

// defaultStartAndManageMCPServers is the actual implementation.
func StartMCPServers(
	configs []MCPServerConfig,
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
