package portforwarding

// import "envctl/internal/reporting" // Not needed here if using simple signature

// PortForwardingConfig holds the configuration for a port forwarding setup.
type PortForwardingConfig struct {
	Label          string // User-defined label for this port-forward
	ServiceName    string
	Namespace      string
	LocalPort      string
	RemotePort     string
	KubeContext    string // Kubernetes context to use
	BindAddress    string // Address to bind to locally (e.g., "127.0.0.1", "0.0.0.0")
	InstanceKey    string // A unique key to identify this port-forward instance
	KubeConfigPath string // Path to the kubeconfig file
}

// PortForwardUpdateFunc is the function signature for callbacks that receive updates
// from the port forwarding process.
// Signature changed to: label, statusDetail, isOpReady, operationErr
type PortForwardUpdateFunc func(serviceLabel, statusDetail string, isOpReady bool, operationErr error)

// ManagedPortForwardInfo holds information about a managed port-forward process,
// typically returned after initiating it.
type ManagedPortForwardInfo struct {
	Config       PortForwardingConfig
	StopChan     chan struct{} // Channel to signal termination to the process
	InitialError error         // Any error that occurred during startup
}
