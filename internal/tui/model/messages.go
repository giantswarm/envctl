package model

import (
	"envctl/internal/portforwarding"
	"envctl/internal/utils"
	"os/exec"
)

// ---- New connection flow messages ----

type StartNewConnectionInputMsg struct{}

// SubmitNewConnectionMsg signals that new MC/WC names have been submitted.
// Controller will use this to (re)initialize contexts, PFs, etc.
type SubmitNewConnectionMsg struct {
	MC string
	WC string
}

type CancelNewConnectionInputMsg struct{}

type MCNameEnteredMsg struct {
	MC string
}

type KubeLoginResultMsg struct {
	ClusterName        string
	IsMC               bool
	DesiredWCShortName string
	LoginStdout        string
	LoginStderr        string
	Err                error
}

type ContextSwitchAndReinitializeResultMsg struct {
	SwitchedContext string
	DesiredMCName   string
	DesiredWCName   string
	DiagnosticLog   string
	Err             error
}

type KubeContextSwitchedMsg struct {
	TargetContext string
	Err           error
	DebugInfo     string
}

type ClusterListResultMsg struct {
	Info *utils.ClusterInfo
	Err  error
}

// ---- Cluster / kube-context messages ----

type KubeContextResultMsg struct {
	Context string
	Err     error
}

type NodeStatusMsg struct {
	ClusterShortName string
	ForMC            bool
	ReadyNodes       int
	TotalNodes       int
	Err              error
	DebugInfo        string
}

type RequestClusterHealthUpdate struct{}

// ---- Port-forward messages ----

type PortForwardCoreUpdateMsg struct {
	Update portforwarding.PortForwardProcessUpdate
}

type PortForwardSetupResultMsg struct {
	InstanceKey string
	Cmd         *exec.Cmd
	StopChan    chan struct{}
	InitialPID  int
	Err         error
}

// ---- MCP proxy messages ----

type McpServerSetupCompletedMsg struct {
	Label    string
	StopChan chan struct{}
	PID      int
	Status   string
	Err      error
}

type McpServerStatusUpdateMsg struct {
	Label     string
	PID       int
	Status    string
	OutputLog string
	Err       error
}

type RestartMcpServerMsg struct {
	Label string
}

// ---- Misc overlay / status bar ----

type ClearStatusBarMsg struct{}

// NopMsg is a message that performs no operation, useful for testing
// or triggering updates without specific side effects.
type NopMsg struct{}

// TODO: Add more message types as needed for TUI interactions.
