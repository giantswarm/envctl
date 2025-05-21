package portforwarding

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

// startAndManageIndividualPortForwardFn allows mocking of StartAndManageIndividualPortForward for testing StartAllConfiguredPortForwards.
var startAndManageIndividualPortForwardFn = StartAndManageIndividualPortForward

// StartAllConfiguredPortForwards starts and manages a list of port forwarding configurations.
// It's designed for non-TUI mode (e.g., CLI usage).
// It takes a slice of PortForwardConfig, a generic update function, and a global stop channel.
// The global stop channel can be used to signal all started port forwards to terminate.
func StartAllConfiguredPortForwards(
	configs []PortForwardConfig,
	updateFn PortForwardUpdateFunc,
	globalStopChan chan struct{}, // External signal to stop all port forwards
) ([]ManagedPortForwardInfo, error) {
	log.Printf("[StartAll] BEGIN. Num configs: %d", len(configs))
	var startedPfs []ManagedPortForwardInfo
	var wg sync.WaitGroup

	resultsChan := make(chan ManagedPortForwardInfo, len(configs))
	errorsChan := make(chan error, len(configs))

	for i, pfConfig := range configs {
		wg.Add(1)
		log.Printf("[StartAll] Goroutine %d for %s: wg.Add(1) called.", i, pfConfig.InstanceKey)
		go func(cfg PortForwardConfig, idx int) {
			log.Printf("[StartAll GOROUTINE %d - %s] START. Calling defer wg.Done().", idx, cfg.InstanceKey)
			defer func() {
				log.Printf("[StartAll GOROUTINE %d - %s] wg.Done() CALLED.", idx, cfg.InstanceKey)
				wg.Done()
			}()

			localStopChan := make(chan struct{})
			cfg.StopChan = localStopChan

			log.Printf("[StartAll GOROUTINE %d - %s] Calling startAndManageIndividualPortForwardFn.", idx, cfg.InstanceKey)
			processSpecificStopChan, err := startAndManageIndividualPortForwardFn(cfg, updateFn)
			log.Printf("[StartAll GOROUTINE %d - %s] startAndManageIndividualPortForwardFn returned. Error: %v", idx, cfg.InstanceKey, err)

			managedInfo := ManagedPortForwardInfo{
				Config:       cfg,
				StopChan:     processSpecificStopChan,
				InitialError: err,
			}

			if err != nil {
				log.Printf("[StartAll GOROUTINE %d - %s] Startup error: %v. Sending to errorsChan.", idx, cfg.InstanceKey, err)
				errorsChan <- fmt.Errorf("error starting port-forward '%s': %w", cfg.Label, err)
				resultsChan <- managedInfo
				log.Printf("[StartAll GOROUTINE %d - %s] RETURN after error.", idx, cfg.InstanceKey)
				return
			}

			log.Printf("[StartAll GOROUTINE %d - %s] Startup success. Sending to resultsChan.", idx, cfg.InstanceKey)
			resultsChan <- managedInfo
			log.Printf("[StartAll GOROUTINE %d - %s] Entering select block.", idx, cfg.InstanceKey)

			select {
			case <-globalStopChan:
				log.Printf("[StartAll GOROUTINE %d - %s] SELECT CASE: Received from globalStopChan.", idx, cfg.InstanceKey)
				if processSpecificStopChan != nil {
					log.Printf("[StartAll GOROUTINE %d - %s] SELECT CASE: Closing processSpecificStopChan.", idx, cfg.InstanceKey)
					close(processSpecificStopChan)
				} else {
					log.Printf("[StartAll GOROUTINE %d - %s] SELECT CASE: processSpecificStopChan is nil, not closing.", idx, cfg.InstanceKey)
				}
				log.Printf("[StartAll GOROUTINE %d - %s] SELECT CASE: RETURN after globalStopChan.", idx, cfg.InstanceKey)
				return
			case <-processSpecificStopChan:
				log.Printf("[StartAll GOROUTINE %d - %s] SELECT CASE: Received from processSpecificStopChan. Process stopped on its own.", idx, cfg.InstanceKey)
				log.Printf("[StartAll GOROUTINE %d - %s] SELECT CASE: RETURN after processSpecificStopChan.", idx, cfg.InstanceKey)
				return
			}
		}(pfConfig, i)
	}

	go func() {
		log.Printf("[StartAll wg.Wait GOROUTINE] Waiting on wg.Wait().")
		wg.Wait()
		log.Printf("[StartAll wg.Wait GOROUTINE] wg.Wait() completed. Closing resultsChan and errorsChan.")
		close(resultsChan)
		close(errorsChan)
	}()

	var firstError error // Declare firstError here, once.
	log.Printf("[StartAll] Reading from resultsChan...")
	for res := range resultsChan {
		log.Printf("[StartAll] Received from resultsChan: %+v", res)
		startedPfs = append(startedPfs, res)
		if res.InitialError != nil && firstError == nil { // firstError is now in scope
			firstError = res.InitialError
		}
	}
	log.Printf("[StartAll] Finished reading from resultsChan. Num startedPfs: %d", len(startedPfs))

	log.Printf("[StartAll] Reading from errorsChan...")
	for err := range errorsChan {
		log.Printf("[StartAll] Received from errorsChan: %v", err)
		if err != nil && firstError == nil { // firstError is in scope
			firstError = err
		}
	}
	log.Printf("[StartAll] Finished reading from errorsChan.")

	log.Printf("[StartAll] END. Returning %d startedPfs, error: %v", len(startedPfs), firstError)
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
