package tui

import (
	"envctl/internal/utils"
	"io"
	"os/exec"
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

// portForwardProcess holds information about a running port-forward command.
type portForwardProcess struct {
	label                 string
	cmd                   *exec.Cmd
	stdout                io.ReadCloser
	stderr                io.ReadCloser
	output                []string
	err                   error
	port                  string
	isWC                  bool
	context               string // The context name this port-forward targets
	namespace             string
	service               string
	active                bool
	statusMsg             string // More detailed status message
	stdoutClosed          bool   // Flag to indicate stdout stream has ended
	stderrClosed          bool   // Flag to indicate stderr stream has ended
	forwardingEstablished bool   // True if "Forwarding from..." detected
}

// Define messages for Bubble Tea
type portForwardOutputMsg struct {
	label      string
	streamType string // "stdout" or "stderr"
	line       string
}

type portForwardErrorMsg struct {
	label      string
	streamType string // "stdout", "stderr", or "general" for non-stream specific errors
	err        error
}

type portForwardStreamEndedMsg struct {
	label      string
	streamType string // "stdout" or "stderr"
}

type portForwardStartedMsg struct { // To signal the command has actually started
	label string
	pid   int
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
	clusterName string // The cluster name that was attempted.
	isMC        bool   // True if this login was for the Management Cluster.
	desiredWcShortName string // Store the originally desired WC short name, to carry through MC login success.
	loginStdout string // Captured stdout from tsh kube login
	loginStderr string // Captured stderr from tsh kube login
	err         error
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