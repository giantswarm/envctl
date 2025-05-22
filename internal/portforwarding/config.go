package portforwarding

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/utils"
	"envctl/pkg/logging"
	"errors"
	"fmt"
	"sync"
)

// GetPortForwardConfig generates the default list of port forwarding configurations
// based on the management cluster and workload cluster arguments.
func GetPortForwardConfig(mcShortName, workloadClusterArg string) []PortForwardingConfig {
	configs := make([]PortForwardingConfig, 0)
	mcKubeContext := utils.BuildMcContext(mcShortName)

	// Determine alloyMetricsTargetContext internally
	var alloyMetricsTargetContext string
	if workloadClusterArg != "" && mcShortName != "" { // WC is specified
		alloyMetricsTargetContext = utils.BuildWcContext(mcShortName, workloadClusterArg)
	} else if mcShortName != "" { // Only MC specified
		alloyMetricsTargetContext = utils.BuildMcContext(mcShortName)
	}

	if mcShortName != "" {
		configs = append(configs, PortForwardingConfig{
			Label:       "Prometheus (MC)",
			InstanceKey: "Prometheus (MC)",
			ServiceName: "service/mimir-query-frontend",
			Namespace:   "mimir",
			LocalPort:   "8080",
			RemotePort:  "8080",
			KubeContext: mcKubeContext,
			BindAddress: "127.0.0.1",
		})
		configs = append(configs, PortForwardingConfig{
			Label:       "Grafana (MC)",
			InstanceKey: "Grafana (MC)",
			ServiceName: "service/grafana",
			Namespace:   "monitoring",
			LocalPort:   "3000",
			RemotePort:  "3000",
			KubeContext: mcKubeContext,
			BindAddress: "127.0.0.1",
		})
	}

	alloyLabel := "Alloy Metrics"
	// Use workloadClusterArg to determine if it's a WC or MC context for Alloy label
	if workloadClusterArg != "" && mcShortName != "" {
		alloyLabel += " (WC)"
	} else if mcShortName != "" {
		alloyLabel += " (MC)"
	}

	if alloyMetricsTargetContext != "" {
		configs = append(configs, PortForwardingConfig{
			Label:       alloyLabel,
			InstanceKey: alloyLabel,
			ServiceName: "service/alloy-metrics-cluster",
			Namespace:   "kube-system",
			LocalPort:   "12345",
			RemotePort:  "12345",
			KubeContext: alloyMetricsTargetContext,
			BindAddress: "127.0.0.1",
		})
	}
	return configs
}

// UpdateFunc defines the callback for port forwarding status updates.
// Signature changed to: serviceLabel, statusDetail string, isOpReady bool, operationErr error

// StartPortForwardings is an exported variable so it can be replaced for testing.
var StartPortForwardings = startPortForwardingsInternal

// startPortForwardingsInternal is the actual implementation.
func startPortForwardingsInternal(
	configs []PortForwardingConfig,
	updateCb PortForwardUpdateFunc,
	wg *sync.WaitGroup,
) map[string]chan struct{} {
	subsystemBase := "PortForwardingSys"
	logging.Debug(subsystemBase, ">>> ENTERED StartPortForwardings. Configs: %d", len(configs))

	individualStopChans := make(map[string]chan struct{})

	if len(configs) == 0 {
		logging.Info(subsystemBase, "No port forward configs to process.")
		if updateCb != nil {
			// Send a generic system status if needed, though ServiceManager might handle this.
			// For now, relying on logging only for this case.
		}
		return individualStopChans
	}

	for _, pfCfg := range configs {
		config := pfCfg // Capture range variable
		serviceSubsystem := "PortForward-" + config.Label
		logging.Debug(serviceSubsystem, "Looping for: %s", config.Label)

		wg.Add(1)
		individualStopChan := make(chan struct{})
		individualStopChans[config.Label] = individualStopChan

		go func() {
			defer wg.Done()

			logging.Info(serviceSubsystem, "Attempting to start port-forward for %s to %s:%s...", config.ServiceName, config.LocalPort, config.RemotePort)
			if updateCb != nil {
				updateCb(config.Label, StatusDetailInitializing, false, nil)
			}

			// kubeUpdateCallback translates kube.SendUpdateFunc to our new UpdateFunc needs
			kubeUpdateCallback := func(kubeStatus, kubeOutputLog string, kubeIsError, kubeIsReady bool) {
				// Log raw output from kube forwarder
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
						// Try to form a meaningful error from kubeStatus or kubeOutputLog
						if kubeOutputLog != "" {
							opErr = errors.New(kubeOutputLog)
						} else if kubeStatus != "" && kubeStatus != "Error" {
							opErr = fmt.Errorf("%s", kubeStatus)
						} else {
							opErr = fmt.Errorf("port-forward for %s failed", config.Label)
						}
					}
					// Pass kubeStatus as statusDetail. ServiceManager adapter will map this to ServiceState.
					mappedStatusDetail := mapKubeStatusToPortForwardStatusDetail(kubeStatus, kubeIsReady, kubeIsError, serviceSubsystem)
					updateCb(config.Label, mappedStatusDetail, kubeIsReady, opErr)
				}
			}

			portSpec := fmt.Sprintf("%s:%s", config.LocalPort, config.RemotePort)
			logging.Debug(serviceSubsystem, "Initiating kube.StartPortForward for %s with spec %s", config.Label, portSpec)

			pfStopChan, initialStatus, initialErr := kube.StartPortForward(
				context.Background(),
				config.KubeContext,
				config.Namespace,
				config.ServiceName,
				portSpec,
				config.Label,
				kubeUpdateCallback,
			)

			logging.Debug(serviceSubsystem, "kube.StartPortForward returned. InitialStatus: '%s', InitialErr: %v, pfStopChan_is_nil: %t", initialStatus, initialErr, pfStopChan == nil)

			if initialErr != nil {
				logging.Error(serviceSubsystem, initialErr, "Failed to start port-forward. Initial output: %s", initialStatus)
				if updateCb != nil {
					updateCb(config.Label, StatusDetailFailed, false, initialErr)
				}
				return
			}
			if pfStopChan == nil {
				critError := fmt.Errorf("critical setup error: stop channel is nil despite no initial error for %s", config.Label)
				logging.Error(serviceSubsystem, critError, "Critical setup error for port-forward. Initial output: %s", initialStatus)
				if updateCb != nil {
					updateCb(config.Label, StatusDetailFailed, false, critError)
				}
				return
			}

			// Successfully initiated, further status updates will come via kubeUpdateCallback.
			// The initial "Starting" status was already sent.
			logging.Info(serviceSubsystem, "Port-forwarding process initiated.")

			select {
			case <-pfStopChan: // Internal stop signal from the port-forwarding goroutine in kube package
				logging.Info(serviceSubsystem, "Stopped (internal signal from kube forwarder).")
				if updateCb != nil {
					updateCb(config.Label, StatusDetailStopped, false, nil)
				}
			case <-individualStopChan: // External stop signal from ServiceManager
				logging.Info(serviceSubsystem, "Stopped (external signal from ServiceManager).")
				if pfStopChan != nil {
					close(pfStopChan)
				}
				if updateCb != nil {
					updateCb(config.Label, StatusDetailStopped, false, nil)
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
	case string(StatusDetailForwardingActive), "Forwarding from":
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
	// If error is present (opErr in the calling context, or implied by kubeIsError), it should be Failed.
	// This helper only uses kubeIsError. The caller (kubeUpdateCallback) will handle opErr.
	if kubeIsError {
		statusDetail = StatusDetailFailed
	} else if kubeIsReady { // Ensure if kube reports ready, we reflect it as active.
		statusDetail = StatusDetailForwardingActive
	}
	return statusDetail
}
