package portforwarding

import "envctl/internal/config"

// import "envctl/internal/reporting" // Not needed here if using simple signature

// PortForwardingConfig struct removed

// PortForwardStatusDetail defines specific status details from the port-forwarding operation.
// These are more granular statuses that the ServiceManager can then map to reporting.ServiceState.
type PortForwardStatusDetail string

const (
	// StatusDetailUnknown is for when the status from the underlying mechanism is not recognized.
	StatusDetailUnknown PortForwardStatusDetail = "Unknown"
	// StatusDetailInitializing indicates the port-forward is being set up.
	StatusDetailInitializing PortForwardStatusDetail = "Initializing"
	// StatusDetailForwardingActive indicates that port forwarding is active and ready.
	StatusDetailForwardingActive PortForwardStatusDetail = "ForwardingActive"
	// StatusDetailStopped indicates the port-forward has been stopped.
	StatusDetailStopped PortForwardStatusDetail = "Stopped"
	// StatusDetailFailed indicates a failure in the port-forwarding operation.
	// operationErr in the updateFn will likely contain more details.
	StatusDetailFailed PortForwardStatusDetail = "Failed"
	// StatusDetailError is often used when the underlying kubectl command reports an error state.
	StatusDetailError PortForwardStatusDetail = "Error"
)

// PortForwardUpdateFunc is the function signature for callbacks that receive updates
// from the port forwarding process.
// Signature changed to: label, statusDetail, isOpReady, operationErr
type PortForwardUpdateFunc func(serviceLabel string, statusDetail PortForwardStatusDetail, isOpReady bool, operationErr error)

// ManagedPortForwardInfo holds information about a managed port-forward process,
// typically returned after initiating it.
type ManagedPortForwardInfo struct {
	Config       config.PortForwardDefinition // Updated type
	StopChan     chan struct{} // Channel to signal termination to the process
	InitialError error         // Any error that occurred during startup
}
