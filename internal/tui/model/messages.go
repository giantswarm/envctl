package model

import (
	"envctl/internal/k8smanager"
	"envctl/internal/managers"
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
	Info *k8smanager.ClusterList
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

// ---- MCP proxy messages ----

type RestartMcpServerMsg struct {
	Label string
}

// ---- Misc overlay / status bar ----

type ClearStatusBarMsg struct{}

// NopMsg is a message that performs no operation, useful for testing
// or triggering updates without specific side effects.
type NopMsg struct{}

// ---- ServiceManager related messages (NEW - KEEP THESE) ----
type ServiceUpdateMsg struct {
	Update managers.ManagedServiceUpdate
}

// ServiceErrorMsg is a more specific message for critical errors from a service,
// though ServiceUpdateMsg can also carry error information.
// This could be used for errors that need special handling or distinct logging.
type ServiceErrorMsg struct {
	Label string
	Err   error
}

// AllServicesStartedMsg can be sent after the initial batch of services has been processed by StartServices.
type AllServicesStartedMsg struct {
	InitialStartupErrors []error
}

// ServiceStopResultMsg is sent after an attempt to stop a service.
type ServiceStopResultMsg struct {
	Label string
	Err   error // nil if successful
}
