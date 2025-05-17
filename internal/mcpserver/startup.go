package mcpserver

import (
	"sync"
	// "syscall" // Not directly used here
)

// StartAllPredefinedMcpServers iterates through PredefinedMcpServers, attempts to start each one
// using StartAndManageIndividualMcpServer, and sends information about each attempt (ManagedMcpServerInfo)
// on the returned channel. The provided McpUpdateFunc will be used for ongoing updates from each server.
// The master goroutine that launches individual servers will add to the WaitGroup for each server it attempts.
// StartAndManageIndividualMcpServer is then responsible for wg.Done() if it successfully launches its own goroutine,
// otherwise this function (StartAllPredefinedMcpServers) must handle wg.Done() for initial startup failures.
func StartAllPredefinedMcpServers(updateFn McpUpdateFunc, wg *sync.WaitGroup) <-chan ManagedMcpServerInfo {
	infoChan := make(chan ManagedMcpServerInfo)

	go func() {
		defer close(infoChan)

		if len(PredefinedMcpServers) == 0 {
			// Caller can check if the channel closes immediately to know no servers were defined/attempted.
			return
		}

		for _, serverCfg := range PredefinedMcpServers {
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
