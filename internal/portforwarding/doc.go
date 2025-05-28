// Package portforwarding provides kubectl port-forward functionality for envctl.
//
// This package manages the creation and lifecycle of port-forward tunnels to
// Kubernetes services, allowing local access to cluster resources. It handles
// the complexity of managing kubectl processes, monitoring their health, and
// recovering from failures.
//
// # Core Functionality
//
// The package provides functions to:
//   - Start port-forward tunnels using kubectl
//   - Monitor port-forward process health
//   - Automatically restart failed port-forwards
//   - Clean up resources on shutdown
//
// # Port Forward Configuration
//
// Port forwards are configured with:
//   - Name: Unique identifier for the port forward
//   - Namespace: Kubernetes namespace containing the service
//   - Service: Name of the Kubernetes service
//   - LocalPort: Port to bind on localhost
//   - RemotePort: Port on the remote service
//   - KubeContext: Kubernetes context to use
//
// # Process Management
//
// The package manages kubectl processes with:
//   - Graceful startup with readiness detection
//   - Health monitoring through process status
//   - Automatic restart on failure (with backoff)
//   - Clean shutdown with process termination
//
// # Error Handling
//
// Common error scenarios handled:
//   - Port already in use
//   - Service not found in namespace
//   - Context not available
//   - Network connectivity issues
//   - kubectl process crashes
//
// # Health Monitoring
//
// Port forward health is determined by:
//   - Process running status
//   - TCP connectivity to local port
//   - Recent error history
//   - Restart attempt count
//
// # Usage Example
//
//	// Start a port forward
//	ctx := context.Background()
//	config := PortForwardConfig{
//	    Name:        "prometheus",
//	    Namespace:   "monitoring",
//	    Service:     "prometheus-server",
//	    LocalPort:   9090,
//	    RemotePort:  9090,
//	    KubeContext: "my-cluster",
//	}
//
//	// Start the port forward
//	pf, err := StartPortForward(ctx, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pf.Stop()
//
//	// Check if it's ready
//	if err := pf.WaitReady(ctx, 30*time.Second); err != nil {
//	    log.Fatal("Port forward failed to become ready")
//	}
//
//	// Use the port forward
//	resp, err := http.Get("http://localhost:9090/metrics")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Integration with Services
//
// This package is used by the port forward service implementation
// (internal/services/portforward) which provides:
//   - Service interface implementation
//   - State management
//   - Health checking
//   - Integration with the orchestrator
//
// # Logging
//
// The package logs important events:
//   - Port forward start/stop
//   - Process output (stdout/stderr)
//   - Error conditions
//   - Restart attempts
//
// All logs use the centralized logging system with appropriate
// subsystem identification for filtering.
//
// # Thread Safety
//
// Port forward operations are thread-safe. Multiple port forwards
// can be managed concurrently without interference.
package portforwarding
