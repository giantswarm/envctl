package tui

import (
	"os/exec"

	"envctl/internal/portforwarding"
)

// -------------------- Port-forward message types --------------------

// portForwardCoreUpdateMsg wraps an update coming from the core port-forwarding
// package and is routed through the main Bubble-Tea update loop.
type portForwardCoreUpdateMsg struct {
	update portforwarding.PortForwardProcessUpdate
}

// portForwardSetupResultMsg reports the synchronous result of attempting to
// start a port-forward (success or immediate failure).
// On success StopChan/Cmd may be nil if the underlying implementation had no
// long-running process (rare).
//
// InstanceKey always matches PortForwardConfig.InstanceKey and thus the
// key used in model.portForwards.
type portForwardSetupResultMsg struct {
	InstanceKey string        // port-forward identifier
	Cmd         *exec.Cmd     // running command (if any)
	StopChan    chan struct{} // channel to stop the PF (nil on failure)
	InitialPID  int           // PID of Cmd if available
	Err         error         // immediate startup error
}
