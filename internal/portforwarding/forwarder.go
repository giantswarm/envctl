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

// KubeStartPortForwardFn allows mocking of kube.StartPortForward for testing.
var KubeStartPortForwardFn = kube.StartPortForward

// Structure to hold the last reported state for debouncing
type portForwardReportedState struct {
	detail  PortForwardStatusDetail
	isReady bool
	err     error
}

var (
	lastPortForwardStates      = make(map[string]portForwardReportedState)
	lastPortForwardStatesMutex = &sync.Mutex{}
)

// StartAndManageIndividualPortForward starts a single port-forward using the Kubernetes Go
// client via the internal kube package.
// It does not spawn any external processes.
func StartAndManageIndividualPortForward(
	cfg config.PortForwardDefinition,
	updateFn PortForwardUpdateFunc,
) (chan struct{}, error) {
	subsystem := "PortForward-" + cfg.Name
	// Initial log for this specific forwarder starting its management. Kube operations log separately.
	targetResource := fmt.Sprintf("%s/%s", cfg.TargetType, cfg.TargetName)
	logging.Info(subsystem, "Attempting to start and manage port-forward for %s (%s:%s -> %s/%s)", cfg.Name, cfg.LocalPort, cfg.RemotePort, cfg.Namespace, targetResource)

	// Helper function to report state if it has changed
	reportIfChanged := func(label string, detail PortForwardStatusDetail, isReady bool, opErr error) {
		lastPortForwardStatesMutex.Lock()
		defer lastPortForwardStatesMutex.Unlock()

		lastState, known := lastPortForwardStates[label]
		errorChanged := (lastState.err != nil) != (opErr != nil) || (lastState.err != nil && opErr != nil && lastState.err.Error() != opErr.Error())

		if !known || lastState.detail != detail || lastState.isReady != isReady || errorChanged {
			newState := portForwardReportedState{detail: detail, isReady: isReady, err: opErr}
			lastPortForwardStates[label] = newState

			if updateFn != nil {
				// Log the state change being reported TO ServiceManager by this forwarder module.
				logging.Debug(subsystem, "Reporting state change for %s: Detail: %s, Ready: %t, Error: %v (Previously: Detail: %s, Ready: %t, Error: %v)",
					label, detail, isReady, opErr, lastState.detail, lastState.isReady, lastState.err)
				updateFn(label, detail, isReady, opErr)
			}

			if detail == StatusDetailStopped || detail == StatusDetailFailed {
				logging.Debug(subsystem, "Terminal state %s for %s, removing from lastPortForwardStates.", detail, label)
				delete(lastPortForwardStates, label)
			}
		} else {
			logging.Debug(subsystem, "State for %s is unchanged (Detail: %s, Ready: %t, Error: %v), not reporting to ServiceManager.", label, detail, isReady, opErr)
		}
	}

	var bridgeCallback kube.SendUpdateFunc = func(kubeStatus, kubeOutputLog string, kubeIsError, kubeIsReady bool) {
		// If kubeStatus is empty, it's a raw log line from tuiLogWriter, ignore it here.
		// tuiLogWriter should handle its own direct logging if those lines are desired.
		// As per current setup (tuiLogWriter unchanged by user request), these raw lines are being sent.
		// We MUST filter them here to prevent them from being treated as state changes.
		if kubeStatus == "" {
			// This is a raw line from tuiLogWriter. Do not process as a state update.
			// If these lines are needed for debugging envctl itself, they should be logged by tuiLogWriter directly.
			// For now, we are dropping them here to adhere to "bridge only for state transitions".
			return
		}

		var operationErr error
		if kubeIsError {
			if kubeOutputLog != "" {
				operationErr = errors.New(kubeOutputLog)
			} else {
				operationErr = fmt.Errorf("port-forward operation for %s indicated error for status '%s' without specific detail", cfg.Name, kubeStatus)
			}
		}

		var statusDetail PortForwardStatusDetail
		switch kubeStatus {
		case "Initializing":
			statusDetail = StatusDetailInitializing
		case "ForwardingActive":
			statusDetail = StatusDetailForwardingActive
		case "Stopped":
			statusDetail = StatusDetailStopped
		case "Failed":
			statusDetail = StatusDetailFailed
		default:
			logging.Warn(subsystem, "Unknown kubeStatus '%s' received in bridgeCallback for %s. Detail: %s, IsError: %t, IsReady: %t",
				kubeStatus, cfg.Name, kubeOutputLog, kubeIsError, kubeIsReady)
			statusDetail = StatusDetailUnknown
			if kubeIsError && operationErr == nil {
				operationErr = fmt.Errorf("unknown status '%s' with error flag for %s", kubeStatus, cfg.Name)
			}
		}

		reportIfChanged(cfg.Name, statusDetail, kubeIsReady, operationErr)
	}

	portMap := fmt.Sprintf("%s:%s", cfg.LocalPort, cfg.RemotePort)
	serviceArg := fmt.Sprintf("%s/%s", cfg.TargetType, cfg.TargetName)

	stopChan, initialStatusFromKube, initialErr := KubeStartPortForwardFn(
		context.Background(),
		cfg.KubeContextTarget,
		cfg.Namespace,
		serviceArg,
		portMap,
		cfg.Name,
		bridgeCallback,
	)

	if initialErr != nil {
		logging.Error(subsystem, initialErr, "Failed to initialize port-forward start sequence for %s. Initial status reported by kube: %s", cfg.Name, initialStatusFromKube)
		reportIfChanged(cfg.Name, StatusDetailFailed, false, initialErr)
		return stopChan, initialErr
	}

	logging.Debug(subsystem, "Port-forward management process initiated for %s. Kube reported initial status: %s", cfg.Name, initialStatusFromKube)

	return stopChan, nil
}
