package portforwarding

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/pkg/logging"
	"errors"
	"fmt"
	"sync"
)

// GetPortForwardConfig function has been removed as its functionality is now part of config.LoadConfig
// and the default config generation in internal/config/types.go

// UpdateFunc defines the callback for port forwarding status updates.
// Signature changed to: serviceLabel, statusDetail string, isOpReady bool, operationErr error
// This type alias is here because it's used by startPortForwardingsInternal, which is in this file.
// Consider moving it if it's more broadly used or defined elsewhere with the same signature.
// type UpdateFunc func(serviceLabel string, statusDetail string, isOpReady bool, operationErr error)

// StartPortForwardings is an exported variable so it can be replaced for testing.
var StartPortForwardings = startPortForwardingsInternal

// startPortForwardingsInternal is the actual implementation.
// Updated to use []config.PortForwardDefinition
func startPortForwardingsInternal(
	configs []config.PortForwardDefinition, // Updated type
	updateCb PortForwardUpdateFunc,
	wg *sync.WaitGroup,
) map[string]chan struct{} {
	subsystemBase := "PortForwardingSys"
	logging.Debug(subsystemBase, ">>> ENTERED StartPortForwardings. Configs: %d", len(configs))

	individualStopChans := make(map[string]chan struct{})

	if len(configs) == 0 {
		logging.Info(subsystemBase, "No port forward configs to process.")
		return individualStopChans
	}

	for _, pfCfg := range configs { // pfCfg is now config.PortForwardDefinition
		currentPfCfg := pfCfg // Capture range variable
		if !currentPfCfg.Enabled { // Check if the port-forward is enabled in the config
			logging.Debug(subsystemBase, "Skipping disabled port-forward: %s", currentPfCfg.Name)
			continue
		}
		serviceSubsystem := "PortForward-" + currentPfCfg.Name // Use Name for label
		logging.Debug(serviceSubsystem, "Looping for: %s", currentPfCfg.Name)

		wg.Add(1)
		individualStopChan := make(chan struct{})
		individualStopChans[currentPfCfg.Name] = individualStopChan // Use Name for map key

		go func() {
			defer wg.Done()

			targetResource := fmt.Sprintf("%s/%s", currentPfCfg.TargetType, currentPfCfg.TargetName)
			logging.Info(serviceSubsystem, "Attempting to start port-forward for %s (%s) to %s:%s...", currentPfCfg.Name, targetResource, currentPfCfg.LocalPort, currentPfCfg.RemotePort)
			if updateCb != nil {
				updateCb(currentPfCfg.Name, StatusDetailInitializing, false, nil)
			}

			kubeUpdateCallback := func(kubeStatus, kubeOutputLog string, kubeIsError, kubeIsReady bool) {
				if kubeOutputLog != "" {
					if kubeIsError {
						logging.Error(serviceSubsystem+"-kube", nil, "%s", kubeOutputLog)
					} else {
						logging.Info(serviceSubsystem+"-kube", "%s", kubeOutputLog)
					}
				}

				if updateCb != nil {
					var opErr error
					if kubeIsError {
						if kubeOutputLog != "" {
							opErr = errors.New(kubeOutputLog)
						} else if kubeStatus != "" && kubeStatus != "Error" {
							opErr = fmt.Errorf("%s", kubeStatus)
						} else {
							opErr = fmt.Errorf("port-forward for %s failed", currentPfCfg.Name)
						}
					}
					mappedStatusDetail := mapKubeStatusToPortForwardStatusDetail(kubeStatus, kubeIsReady, kubeIsError, serviceSubsystem)
					updateCb(currentPfCfg.Name, mappedStatusDetail, kubeIsReady, opErr)
				}
			}

			portSpec := fmt.Sprintf("%s:%s", currentPfCfg.LocalPort, currentPfCfg.RemotePort)
			serviceArg := fmt.Sprintf("%s/%s", currentPfCfg.TargetType, currentPfCfg.TargetName)
			
			// Note: currentPfCfg.BindAddress is available here.
			// However, kube.StartPortForward currently hardcodes bind address to 127.0.0.1.
			// To use currentPfCfg.BindAddress, kube.StartPortForward would need to be updated.
			bindAddressForLog := currentPfCfg.BindAddress
			if bindAddressForLog == "" {
				bindAddressForLog = "127.0.0.1 (default)"
			}
			logging.Debug(serviceSubsystem, "Initiating kube.StartPortForward for %s with spec %s, serviceArg %s, context %s, configured bindAddress %s", currentPfCfg.Name, portSpec, serviceArg, currentPfCfg.KubeContextTarget, bindAddressForLog)

			pfStopChan, initialStatus, initialErr := kube.StartPortForward(
				context.Background(),
				currentPfCfg.KubeContextTarget,
				currentPfCfg.Namespace,
				serviceArg,
				portSpec,
				currentPfCfg.Name, 
				kubeUpdateCallback,
			)

			logging.Debug(serviceSubsystem, "kube.StartPortForward returned. InitialStatus: '%s', InitialErr: %v, pfStopChan_is_nil: %t", initialStatus, initialErr, pfStopChan == nil)

			if initialErr != nil {
				logging.Error(serviceSubsystem, initialErr, "Failed to start port-forward. Initial output: %s", initialStatus)
				if updateCb != nil {
					updateCb(currentPfCfg.Name, StatusDetailFailed, false, initialErr)
				}
				return
			}
			if pfStopChan == nil {
				critError := fmt.Errorf("critical setup error: stop channel is nil despite no initial error for %s", currentPfCfg.Name)
				logging.Error(serviceSubsystem, critError, "Critical setup error for port-forward. Initial output: %s", initialStatus)
				if updateCb != nil {
					updateCb(currentPfCfg.Name, StatusDetailFailed, false, critError)
				}
				return
			}

			logging.Info(serviceSubsystem, "Port-forwarding process initiated.")

			select {
			case <-pfStopChan:
				logging.Info(serviceSubsystem, "Stopped (internal signal from kube forwarder).")
				if updateCb != nil {
					updateCb(currentPfCfg.Name, StatusDetailStopped, false, nil)
				}
			case <-individualStopChan:
				logging.Info(serviceSubsystem, "Stopped (external signal from ServiceManager).")
				if pfStopChan != nil {
					close(pfStopChan)
				}
				if updateCb != nil {
					updateCb(currentPfCfg.Name, StatusDetailStopped, false, nil)
				}
			}
		}()
	}
	return individualStopChans
}

// Helper function to map kubeStatus to PortForwardStatusDetail
// This should be similar to the logic in forwarder.go's bridgeCallback
func mapKubeStatusToPortForwardStatusDetail(kubeStatus string, kubeIsReady bool, kubeIsError bool, subsystem string) PortForwardStatusDetail {
	var statusDetail PortForwardStatusDetail
	switch kubeStatus {
	case string(StatusDetailInitializing):
		statusDetail = StatusDetailInitializing
	case string(StatusDetailForwardingActive), "Forwarding from": // "Forwarding from" is a common stdout message from client-go
		statusDetail = StatusDetailForwardingActive
	case string(StatusDetailStopped):
		statusDetail = StatusDetailStopped
	case string(StatusDetailFailed):
		statusDetail = StatusDetailFailed
	case string(StatusDetailError):
		statusDetail = StatusDetailError
	default:
		if kubeIsReady {
			statusDetail = StatusDetailForwardingActive
		} else if kubeIsError {
			statusDetail = StatusDetailFailed
		} else {
			statusDetail = StatusDetailUnknown
			logging.Debug(subsystem, "Unknown kubeStatus in mapKubeStatusToPortForwardStatusDetail: '%s', IsReady: %t, IsError: %t", kubeStatus, kubeIsReady, kubeIsError)
		}
	}

	if kubeIsError {
		statusDetail = StatusDetailFailed
	} else if kubeIsReady {
		statusDetail = StatusDetailForwardingActive
	}
	return statusDetail
}
