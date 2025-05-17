package tui

import (
	"envctl/internal/portforwarding"
	"envctl/internal/utils"
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

// Define messages for Bubble Tea

// portForwardCoreUpdateMsg is sent by the TUI's adapter callback to the main TUI channel.
// It wraps the core PortForwardProcessUpdate.
type portForwardCoreUpdateMsg struct {
	update portforwarding.PortForwardProcessUpdate
}

// portForwardSetupResultMsg is sent after the initial call to portforwarding.StartAndManageIndividualPortForward.
// It informs the TUI whether the setup was successful (providing a stopChan and cmd) or if an immediate error occurred.
type portForwardSetupResultMsg struct {
	InstanceKey string        // Identifies the port-forward (matches PortForwardConfig.InstanceKey).
	Cmd         *exec.Cmd     // The command object if successfully initiated.
	StopChan    chan struct{} // Channel to stop the port-forward if setup was successful; nil otherwise.
	InitialPID  int           // PID if available at startup
	Err         error         // Error encountered during the synchronous setup phase, if any.
}

// mcpServerSetupCompletedMsg is sent after the initial synchronous setup of the MCP server process completes.
// It informs the TUI whether the setup was successful (providing a stopChan and PID) or if an immediate error occurred.
type mcpServerSetupCompletedMsg struct {
	Label    string        // Identifies which MCP proxy/server this message is for.
	stopChan chan struct{} // Channel to stop the MCP server process if setup was successful; nil otherwise.
	pid      int           // PID of the MCP server process if successfully started.
	status   string        // Initial status message (e.g., "Starting...").
	err      error         // Error encountered during the synchronous setup phase, if any.
}

type kubeContextResultMsg struct {
	context string // The current Kubernetes context name.
	err     error  // Error encountered while fetching the context, if any.
}

// nodeStatusMsg carries node health information (ready/total nodes) for a specific cluster.
type nodeStatusMsg struct {
	clusterShortName string // Short name of the cluster (e.g., "myinstallation" or "deu01").
	forMC            bool   // True if this status is for the Management Cluster, false for Workload Cluster.
	readyNodes       int    // Number of ready nodes.
	totalNodes       int    // Total number of nodes.
	err              error  // Error encountered while fetching node status, if any.
}

// requestClusterHealthUpdate is an empty message used to trigger a refresh of cluster health information.
type requestClusterHealthUpdate struct{}

// --- New Connection Flow Messages ---

// Messages related to the UI flow for establishing a new connection to different clusters.

// startNewConnectionInputMsg signals the TUI to switch to the input mode for a new connection.
type startNewConnectionInputMsg struct{}

// submitNewConnectionMsg carries the management and workload cluster names entered by the user
// to initiate a new connection sequence.
type submitNewConnectionMsg struct {
	mc string // Management Cluster name.
	wc string // Workload Cluster name (optional).
}

// cancelNewConnectionInputMsg signals the TUI to exit the new connection input mode and revert.
type cancelNewConnectionInputMsg struct{}

// mcNameEnteredMsg signals that the user has finished entering the Management Cluster name,
// and the TUI should now prompt for the Workload Cluster name (if applicable).
type mcNameEnteredMsg struct {
	mc string // The entered Management Cluster name.
}

// kubeLoginResultMsg reports the outcome of a `tsh kube login` attempt for a single cluster.
type kubeLoginResultMsg struct {
	clusterName        string // The name of the cluster for which login was attempted.
	isMC               bool   // True if the login was for the Management Cluster.
	desiredWcShortName string // If MC login was successful, this carries the WC name for the next step.
	loginStdout        string // Captured stdout from the `tsh kube login` command.
	loginStderr        string // Captured stderr from the `tsh kube login` command.
	err                error  // Error encountered during the login attempt, if any.
}

// contextSwitchAndReinitializeResultMsg reports the result of the overall new connection process,
// including context switching and readiness for TUI re-initialization.
type contextSwitchAndReinitializeResultMsg struct {
	switchedContext string // The Kubernetes context that was ultimately set.
	desiredMcName   string // The Management Cluster name targeted by the user.
	desiredWcName   string // The Workload Cluster name targeted by the user.
	diagnosticLog   string // A log of actions taken during the connection attempt.
	err             error  // Any error that prevented successful connection and re-initialization.
}

// kubeContextSwitchedMsg indicates the result of an explicit attempt to switch the Kubernetes context.
type kubeContextSwitchedMsg struct {
	TargetContext string // The Kubernetes context that was the target of the switch attempt.
	err           error  // Error encountered during the context switch, if any.
}

// clusterListResultMsg carries the list of available management and workload clusters,
// typically fetched for autocompletion purposes.
type clusterListResultMsg struct {
	info *utils.ClusterInfo // Pointer to the struct containing cluster lists.
	err  error              // Error encountered while fetching the cluster list, if any.
}
