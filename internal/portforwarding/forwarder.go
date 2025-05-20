package portforwarding

import (
	"envctl/internal/kube"
	"fmt"
	"os/exec"
	"strings"
)

// StartAndManageIndividualPortForward starts a single port-forward using the Kubernetes Go
// client.  No external `kubectl` process is spawned – the returned *exec.Cmd is always nil
// (kept for backwards-compatibility so that existing code storing PIDs continues to compile).
//
// The legacy *exec.Cmd input parameter has been removed – callers no longer need to pass `nil`.
func StartAndManageIndividualPortForward(
	cfg PortForwardConfig,
	updateFn PortForwardUpdateFunc,
) (*exec.Cmd, chan struct{}, error) {
	// Notify caller that initialisation has begun.
	updateFn(PortForwardProcessUpdate{
		InstanceKey: cfg.InstanceKey,
		ServiceName: cfg.ServiceName,
		Namespace:   cfg.Namespace,
		LocalPort:   cfg.LocalPort,
		RemotePort:  cfg.RemotePort,
		StatusMsg:   "Initializing",
		Running:     false,
	})

	// Translate utils-level updates into package-level updates expected by the TUI / CLI.
	bridge := func(status, output string, isError, isReady bool) {
		upd := PortForwardProcessUpdate{
			InstanceKey: cfg.InstanceKey,
			ServiceName: cfg.ServiceName,
			Namespace:   cfg.Namespace,
			LocalPort:   cfg.LocalPort,
			RemotePort:  cfg.RemotePort,
			StatusMsg:   status,
			OutputLog:   strings.TrimSpace(output),
			Running:     !(isError || strings.Contains(status, "Stopped")),
		}
		if isError {
			upd.Error = fmt.Errorf("%s", output)
		}
		if isReady {
			upd.Running = true
		}
		updateFn(upd)
	}

	portMap := fmt.Sprintf("%s:%s", cfg.LocalPort, cfg.RemotePort)

	stop, initialStatus, err := kube.StartPortForwardClientGo(
		cfg.KubeContext,
		cfg.Namespace,
		cfg.ServiceName,
		portMap,
		cfg.Label,
		bridge,
	)

	if initialStatus != "" {
		bridge(initialStatus, "", false, false)
	}
	if err != nil {
		bridge("Error", err.Error(), true, false)
		return nil, stop, err
	}
	return nil, stop, nil
}
