package portforwarding

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

// StartAllConfiguredPortForwards starts and manages a list of port forwarding configurations.
// It's designed for non-TUI mode (e.g., CLI usage).
// It takes a slice of PortForwardConfig, a generic update function, and a global stop channel.
// The global stop channel can be used to signal all started port forwards to terminate.
func StartAllConfiguredPortForwards(
	configs []PortForwardConfig,
	updateFn PortForwardUpdateFunc,
	globalStopChan chan struct{}, // External signal to stop all port forwards
) ([]ManagedPortForwardInfo, error) {
	var startedPfs []ManagedPortForwardInfo
	var wg sync.WaitGroup

	// Buffered channel to collect results from each goroutine
	resultsChan := make(chan ManagedPortForwardInfo, len(configs))
	errorsChan := make(chan error, len(configs))

	for _, pfConfig := range configs {
		wg.Add(1)
		go func(cfg PortForwardConfig) {
			defer wg.Done()

			// Create a specific stop channel for this individual port-forward
			// This local stopChan will be closed if the globalStopChan is closed.
			localStopChan := make(chan struct{})
			cfg.StopChan = localStopChan // Assign to config if needed, though StartAndManageIndividualPortForward creates its own internal one.

			// cmd, returnedStopChan, err := StartAndManageIndividualPortForward(cfg, updateFn, nil)
			// The StartAndManageIndividualPortForward already returns its own stopChan that it listens to.
			// We need to respect that and use it.
			cmd, processSpecificStopChan, err := StartAndManageIndividualPortForward(cfg, updateFn, nil)

			pid := 0
			if cmd != nil && cmd.Process != nil {
				pid = cmd.Process.Pid
			}

			managedInfo := ManagedPortForwardInfo{
				Config:       cfg,
				PID:          pid,
				StopChan:     processSpecificStopChan, // This is the one to close to stop this specific PF
				InitialError: err,
			}

			if err != nil {
				errorsChan <- fmt.Errorf("error starting port-forward '%s': %w", cfg.Label, err)
				resultsChan <- managedInfo // Send info even on error for tracking
				return
			}

			resultsChan <- managedInfo

			// Wait for either the global stop signal or the individual process to stop on its own.
			select {
			case <-globalStopChan:
				if processSpecificStopChan != nil {
					// updateFn might log this as "Stopped (requested)"
					close(processSpecificStopChan) // Signal the specific port forward to stop
				}
				return
			case <-processSpecificStopChan: // This case might not be strictly necessary if StartAndManage handles its own termination logging
				// The process stopped on its own (e.g. error, or kubectl exited)
				// updateFn inside StartAndManageIndividualPortForward should have already reported this.
				return
			}
		}(pfConfig)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	var firstError error
	for res := range resultsChan {
		startedPfs = append(startedPfs, res)
		if res.InitialError != nil && firstError == nil {
			firstError = res.InitialError // Capture the first error encountered during startup
		}
	}

	// Check for errors sent explicitly to errorsChan as well
	for err := range errorsChan {
		if err != nil && firstError == nil {
			firstError = err
		}
		// Optionally collect all errors if needed
	}

	return startedPfs, firstError
}

// NonTUIUpdater is a simple PortForwardUpdateFunc for CLI mode that prints to stdout/stderr.
func NonTUIUpdater(update PortForwardProcessUpdate) {
	prefix := fmt.Sprintf("[%s PF %s:%s]", update.InstanceKey, update.LocalPort, update.RemotePort)
	if update.Error != nil {
		fmt.Fprintf(os.Stderr, "%s ERROR: %s - %s\n", prefix, update.StatusMsg, update.Error.Error())
	} else if update.OutputLog != "" {
		// Distinguish between ready message and other logs
		if strings.Contains(update.OutputLog, "Forwarding from") {
			fmt.Printf("%s STATUS: %s - %s\n", prefix, update.StatusMsg, update.OutputLog)
		} else {
			fmt.Printf("%s LOG: %s\n", prefix, update.OutputLog)
		}
	} else if update.StatusMsg != "" {
		fmt.Printf("%s STATUS: %s\n", prefix, update.StatusMsg)
	}
}

// SetupSignalHandler registers for SIGINT and SIGTERM and closes the provided channel when a signal is received.
// This is useful for graceful shutdown in CLI applications.
func SetupSignalHandler(stopChan chan struct{}) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c // Wait for a signal
		fmt.Println("\nShutting down port forwards...")
		close(stopChan) // Signal all goroutines to stop
	}()
}
