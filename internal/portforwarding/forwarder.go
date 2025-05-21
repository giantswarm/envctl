package portforwarding

import (
	"envctl/internal/kube"
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
	if updateFn != nil {
		updateFn(cfg.Label, "Initializing", "", false, false)
	}

	var bridgeCallback kube.SendUpdateFunc = func(status, outputLog string, isError, isReady bool) {
		if updateFn != nil {
			updateFn(cfg.Label, status, outputLog, isError, isReady)
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

	if initialStatus != "" && initialErr == nil {
		if updateFn != nil {
			updateFn(cfg.Label, initialStatus, "", false, false)
		}
	}

	if initialErr != nil {
		if updateFn != nil {
			updateFn(cfg.Label, fmt.Sprintf("Failed to initialize port-forward: %v", initialErr), initialStatus, true, false)
		}
		return stopChan, initialErr
	}

	return stopChan, nil
}
