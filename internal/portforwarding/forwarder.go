package portforwarding

import (
	"envctl/internal/kube"
	"envctl/pkg/logging"
	"errors"
	"fmt"
)

// KubeStartPortForwardClientGoFn allows mocking of kube.StartPortForwardClientGo for testing.
// It is a variable so it can be replaced by tests.
var KubeStartPortForwardFn = kube.StartPortForwardClientGo // Test comment to force re-evaluation

// StartAndManageIndividualPortForward starts a single port-forward using the Kubernetes Go
// client via the internal kube package.
// It does not spawn any external processes.
func StartAndManageIndividualPortForward(
	cfg PortForwardingConfig,
	updateFn PortForwardUpdateFunc,
) (chan struct{}, error) {
	subsystem := "PortForward-" + cfg.Label
	logging.Info(subsystem, "Initializing port-forward for %s to %s:%s in namespace %s (context: %s)", cfg.ServiceName, cfg.LocalPort, cfg.RemotePort, cfg.Namespace, cfg.KubeContext)

	if updateFn != nil {
		updateFn(cfg.Label, StatusDetailInitializing, false, nil)
	}

	var bridgeCallback kube.SendUpdateFunc = func(kubeStatus, kubeOutputLog string, kubeIsError, kubeIsReady bool) {
		if kubeOutputLog != "" {
			if kubeIsError {
				logging.Error(subsystem, nil, "kubectl: %s", kubeOutputLog)
			} else {
				logging.Info(subsystem, "kubectl: %s", kubeOutputLog)
			}
		}

		if updateFn != nil {
			var operationErr error
			if kubeIsError {
				if kubeOutputLog != "" {
					operationErr = errors.New(kubeOutputLog)
				} else if kubeStatus != "" && kubeStatus != string(StatusDetailError) {
					operationErr = fmt.Errorf("status: %s", kubeStatus)
				} else {
					operationErr = fmt.Errorf("port-forward operation for %s failed", cfg.Label)
				}
			}

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
					logging.Debug(subsystem, "Unknown kubeStatus received: '%s', IsReady: %t, IsError: %t", kubeStatus, kubeIsReady, kubeIsError)
				}
			}
			
			if operationErr != nil {
				statusDetail = StatusDetailFailed
			} else if kubeIsReady {
				statusDetail = StatusDetailForwardingActive
			}

			updateFn(cfg.Label, statusDetail, kubeIsReady, operationErr)
		}
	}

	portMap := fmt.Sprintf("%s:%s", cfg.LocalPort, cfg.RemotePort)

	stopChan, initialStatus, initialErr := KubeStartPortForwardFn(
		cfg.KubeContext,
		cfg.Namespace,
		cfg.ServiceName,
		portMap,
		cfg.Label,
		bridgeCallback,
	)

	if initialErr != nil {
		logging.Error(subsystem, initialErr, "Failed to initialize port-forward: %v. Initial output: %s", initialErr, initialStatus)
		if updateFn != nil {
			updateFn(cfg.Label, StatusDetailFailed, false, initialErr)
		}
		return stopChan, initialErr
	}

	logging.Info(subsystem, "Port-forward process initiated. Initial status: %s", initialStatus)

	return stopChan, nil
}
