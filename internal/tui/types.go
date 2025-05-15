package tui

import (
	"envctl/internal/utils"
	"time"
)

// clusterHealthInfo holds basic health data for a cluster.
// Will be expanded for more detailed health checks.
type clusterHealthInfo struct {
	ReadyNodes  int
	TotalNodes  int
	StatusError error
	IsLoading   bool
	LastUpdated time.Time
}

// portForwardProcess holds information about a running port-forward.
// Adapted for client-go based port forwarding.
type portForwardProcess struct {
	label                 string
	// cmd                   *exec.Cmd // Removed for client-go
	// stdout                io.ReadCloser // Removed for client-go
	// stderr                io.ReadCloser // Removed for client-go
	pid                   int             // May become informational, e.g., for logging association if available
	stopChan              chan struct{}   // To signal the port-forwarding goroutine to stop
	output                []string        // General output/log for this PF, less direct stdout/stderr
	err                   error
	port                  string // e.g., "8080:80" or just "8080" if local and remote are same
	isWC                  bool
	context               string // The context name this port-forward targets
	namespace             string
	service               string // Service name to target
	active                bool   // Whether this PF is configured to be active
	statusMsg             string // Detailed status message (e.g., "Running", "Stopped", "Error")
	// stdoutClosed          bool   // Less relevant with client-go direct stream handling
	// stderrClosed          bool   // Less relevant with client-go direct stream handling
	forwardingEstablished bool // True if client-go signals forwarding is active
}

// Define messages for Bubble Tea

// portForwardStatusUpdateMsg can be used for various status changes from client-go PFs
type portForwardStatusUpdateMsg struct {
	label     string
	status    string // e.g., "Forwarding Active (Local Port: 1234)", "Error: ...", "Stopped"
	outputLog string // Optional line to add to the PF's specific output log
	isError   bool
	isReady   bool // Specifically indicates the PF is now listening
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
// The PID might not be relevant anymore.
// This can be consolidated with portForwardStatusUpdateMsg by using its isReady field.
type portForwardStartedMsg struct {
	label     string
	localPort string // The actual local port it's listening on
}

// portForwardRestartCompletedMsg carries the result of an async restart attempt with client-go.
// The PID of the kubectl command is no longer relevant.
// The command, stdout, stderr are also not relevant as client-go handles this internally.
type portForwardRestartCompletedMsg struct {
	label       string
	newStopChan chan struct{} // The stop channel for the newly started port-forward
	localPort   string        // Actual local port if successfully (re)started
	err         error
}

// portForwardSetupCompletedMsg is sent when the initial setup command for a client-go port-forward completes.
// It carries the stop channel for the new port-forward and any immediate setup error.
// Ongoing status updates will come via other messages like portForwardStatusUpdateMsg.
type portForwardSetupCompletedMsg struct {
	label    string
	stopChan chan struct{}
	status   string // Initial status message, e.g., "Initializing..." or error if initialError is nil but status indicates issue
	err      error  // For immediate errors from the setup command itself
}

type kubeContextResultMsg struct {
	context string
	err     error
}

// nodeStatusMsg carries node health information.
type nodeStatusMsg struct {
	clusterShortName string // The short name of the cluster this status is for (e.g., "alba" or "deu01")
	forMC            bool   // True if this status is for the Management Cluster
	readyNodes       int
	totalNodes       int
	err              error
}

// requestClusterHealthUpdate is an internal message to trigger a health refresh.
type requestClusterHealthUpdate struct{}

// --- New Connection Flow Messages ---

// startNewConnectionInputMsg signals the TUI to enter the mode for inputting new cluster names.
type startNewConnectionInputMsg struct{}

// submitNewConnectionMsg carries the new cluster names to re-initialize the TUI.
type submitNewConnectionMsg struct {
	mc string
	wc string
}

// cancelNewConnectionInputMsg signals the TUI to exit the new cluster input mode.
type cancelNewConnectionInputMsg struct{}

// mcNameEnteredMsg signals that the management cluster name has been entered,
// and we should now prompt for the workload cluster.
type mcNameEnteredMsg struct {
	mc string
}

// kubeLoginResultMsg signals the result of a single utils.LoginToKubeCluster attempt.
// It indicates whether it was for MC or WC for context.
type kubeLoginResultMsg struct {
	clusterName        string // The cluster name that was attempted.
	isMC               bool   // True if this login was for the Management Cluster.
	desiredWcShortName string // Store the originally desired WC short name, to carry through MC login success.
	loginStdout        string // Captured stdout from tsh kube login
	loginStderr        string // Captured stderr from tsh kube login
	err                error
}

// contextSwitchAndReinitializeResultMsg signals the result of attempting to switch context
// and contains diagnostic info. If successful, the model will proceed with full re-initialization.
type contextSwitchAndReinitializeResultMsg struct {
	switchedContext string // The context that was actually switched to.
	desiredMcName   string // The MC name the user intended.
	desiredWcName   string // The WC name the user intended.
	diagnosticLog   string
	err             error
}

// kubeContextSwitchedMsg is a message to indicate the result of a context switch attempt.
type kubeContextSwitchedMsg struct {
	TargetContext string // The context name that was attempted to switch to.
	err           error
}

// clusterListResultMsg carries the list of MCs and WCs from utils.GetClusterInfo.
type clusterListResultMsg struct {
	info *utils.ClusterInfo
	err  error
}
