package tui

import (
	"envctl/internal/utils"
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

// portForwardProcess represents the state and configuration of a single port-forwarding operation.
// It is designed for use with client-go based port forwarding and holds necessary details
// like the target service, ports, Kubernetes context, and its current operational status.
type portForwardProcess struct {
	label                 string        // User-friendly label for the port-forward (e.g., "Prometheus (MC)").
	pid                   int           // PID of the process, mainly for informational/logging purposes if available (less critical with client-go).
	stopChan              chan struct{} // Channel used to signal the port-forwarding goroutine to stop.
	output                []string      // Stores general output or log messages specific to this port-forward.
	err                   error         // Any error encountered by this port-forwarding process.
	port                  string        // Port mapping string (e.g., "8080:8080").
	isWC                  bool          // True if this port-forward targets a workload cluster service.
	context               string        // The Kubernetes context name this port-forward targets.
	namespace             string        // Kubernetes namespace of the target service.
	service               string        // Name of the Kubernetes service to port-forward to.
	active                bool          // Whether this port-forward is configured to be active (i.e., should be running).
	statusMsg             string        // Detailed status message for display in the TUI (e.g., "Running", "Error").
	forwardingEstablished bool          // True if the client-go port-forwarder has successfully established the connection.
}

// Define messages for Bubble Tea

// portForwardStatusUpdateMsg is sent by an active port-forwarding goroutine to update the TUI
// about its status, provide log output, or signal readiness/errors.
type portForwardStatusUpdateMsg struct {
	label     string // Identifies which port-forward this update is for.
	status    string // Current status text (e.g., "Forwarding from 127.0.0.1:8080 -> 8080").
	outputLog string // An optional single line of log output from the port-forward process.
	isError   bool   // True if this update indicates an error state.
	isReady   bool   // True if this update indicates the port-forward is established and ready.
}

// portForwardOutputMsg might be used for verbose logging from the PF wrapper if needed.
// Its direct relevance decreases as client-go might log differently.
type portForwardOutputMsg struct {
	label      string
	streamType string // "stdout" or "stderr" (less direct, more for wrapper logs)
	line       string
}

// portForwardErrorMsg for critical errors from the port-forwarding goroutine itself (not just stream errors)
// This can be consolidated with portForwardStatusUpdateMsg by using its isError field.
type portForwardErrorMsg struct {
	label string
	err   error
}

// portForwardStreamEndedMsg is less relevant with client-go, which manages its own streams.
// We might have a general "portForwardStoppedMsg" instead.
type portForwardStreamEndedMsg struct {
	label      string
	streamType string // "stdout" or "stderr"
}

// portForwardStartedMsg could signal that the client-go ForwardPorts() is up and listening.
// This can be consolidated with portForwardStatusUpdateMsg by using its isReady field.
type portForwardStartedMsg struct {
	label     string
	localPort string // The actual local port it's listening on
}

// portForwardRestartCompletedMsg carries the result of an async restart attempt with client-go.
type portForwardRestartCompletedMsg struct {
	label       string
	newStopChan chan struct{} // The stop channel for the newly started port-forward
	localPort   string        // Actual local port if successfully (re)started
	err         error
}

// portForwardSetupCompletedMsg is sent after the initial synchronous setup of a client-go port-forward completes.
// It informs the TUI whether the setup was successful (providing a stopChan) or if an immediate error occurred.
type portForwardSetupCompletedMsg struct {
	label    string        // Identifies the port-forward.
	stopChan chan struct{} // Channel to stop the port-forward if setup was successful; nil otherwise.
	status   string        // Initial status message (e.g., "Initializing...").
	err      error         // Error encountered during the synchronous setup phase, if any.
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
