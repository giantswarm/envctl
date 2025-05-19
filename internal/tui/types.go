package tui

import (
	"envctl/internal/portforwarding"
	"os/exec"
	"time"
)

// clusterHealthInfo holds basic health data for a Kubernetes cluster, specifically node readiness.
// It includes when the data was last updated and if it's currently being loaded.
// This struct is used to display health status in the TUI.
type clusterHealthInfo struct {
	ReadyNodes  int       // Number of nodes in a Ready state.
	TotalNodes  int       // Total number of nodes in the cluster.
	StatusError error     // Any error encountered while fetching health status.
	IsLoading   bool      // True if health information is currently being fetched.
	LastUpdated time.Time // Timestamp of the last successful health update.
}

// portForwardProcess represents the TUI-specific state for a port-forwarding operation.
// The core process management is handled by the `portforwarding` package.
type portForwardProcess struct {
	label     string                           // User-friendly label (e.g., "Prometheus (MC)"). Used as an InstanceKey.
	config    portforwarding.PortForwardConfig // Core configuration
	stopChan  chan struct{}                    // Channel to signal the port-forwarding goroutine to stop.
	cmd       *exec.Cmd                        // Reference to the running command
	output    []string                         // Stores general output or log messages specific to this port-forward for display
	err       error                            // Any error encountered by this port-forwarding process.
	active    bool                             // Whether this port-forward is configured to be active (i.e., should be running).
	statusMsg string                           // Detailed status message for display in the TUI (e.g., "Running", "Error").
	pid       int                              // PID for display
	running   bool                             // Reflects the 'Running' field from PortForwardProcessUpdate
}

// OverallAppStatus defines the high-level operational status of the application.
type OverallAppStatus int

const (
	AppStatusUnknown OverallAppStatus = iota // Or AppStatusInitializing
	AppStatusUp
	AppStatusConnecting
	AppStatusDegraded
	AppStatusFailed
)

// String provides a human-readable representation of the OverallAppStatus.
func (s OverallAppStatus) String() string {
	// Make sure this slice is ordered consistently with the const definitions.
	statuses := []string{"Initializing", "Up", "Connecting", "Degraded", "Failed", "Unknown"}
	if s < 0 || int(s) >= len(statuses)-1 { // -1 because Unknown is an extra fallback
		return statuses[len(statuses)-1] // Return "Unknown" for out-of-bounds
	}
	return statuses[s]
}
