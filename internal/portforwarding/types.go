package portforwarding

import "os/exec"

// PortForwardConfig holds the configuration for a port forwarding setup.
type PortForwardConfig struct {
	Label          string // User-defined label for this port-forward
	ServiceName    string
	Namespace      string
	LocalPort      string
	RemotePort     string
	KubeContext    string // Kubernetes context to use
	BindAddress    string // Address to bind to locally (e.g., "127.0.0.1", "0.0.0.0")
	InstanceKey    string // A unique key to identify this port-forward instance
	StopChan       chan struct{}
	ReadyChan      chan struct{}
	KubeConfigPath string // Path to the kubeconfig file
}

// PortForwardProcessUpdate encapsulates status updates from a port forwarding process.
type PortForwardProcessUpdate struct {
	InstanceKey string // Key identifying the port-forward instance
	ServiceName string
	Namespace   string
	LocalPort   string
	RemotePort  string
	StatusMsg   string    // e.g., "Starting", "Running", "Stopped", "Error"
	OutputLog   string    // Log output from the process
	Error       error     // Any error encountered
	PID         int       // Process ID, if applicable (might not be directly a separate process)
	Cmd         *exec.Cmd // The command being executed, if applicable
	Running     bool
}

// PortForwardUpdateFunc is the function signature for callbacks that receive updates
// from the port forwarding process.
type PortForwardUpdateFunc func(update PortForwardProcessUpdate)

// ManagedPortForwardInfo holds information about a managed port-forward process,
// typically returned after initiating it.
type ManagedPortForwardInfo struct {
	Config       PortForwardConfig
	PID          int
	StopChan     chan struct{} // Channel to signal termination to the process
	InitialError error         // Any error that occurred during startup
}
