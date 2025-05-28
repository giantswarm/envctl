// Package api provides the API layer for envctl's service management.
//
// This package defines the interfaces and types used for communication between
// the orchestrator, services, and external consumers (TUI, CLI). It provides
// a clean abstraction layer that allows different components to interact without
// tight coupling.
//
// # Core Interfaces
//
// The package defines several key interfaces that components must implement:
//
// OrchestratorAPI: Provides methods for managing service lifecycle
//   - StartService: Start a specific service by label
//   - StopService: Stop a specific service by label
//   - RestartService: Restart a specific service
//   - GetServiceStatus: Get current status of a service
//   - ListServices: List all registered services
//
// K8sServiceAPI: Provides Kubernetes connection information
//   - ListConnections: Get all K8s connections
//   - GetConnectionInfo: Get details for a specific connection
//   - GetConnectionByContext: Find connection by context name
//
// PortForwardServiceAPI: Manages port forward services
//   - ListForwards: Get all port forwards
//   - GetForwardInfo: Get details for a specific forward
//
// MCPServiceAPI: Manages MCP server services
//   - ListServers: Get all MCP servers
//   - GetServerInfo: Get details for a specific server
//   - GetTools: Get available tools for an MCP server
//
// # Data Types
//
// The package provides data transfer objects for service information:
//
// ServiceStatus: Current state and health of any service
//   - Label: Unique identifier
//   - Type: Service type (K8s, PortForward, MCP)
//   - State: Current state (stopped, starting, running, etc.)
//   - Health: Health status (unknown, healthy, unhealthy)
//   - Error: Any error information
//
// K8sConnectionInfo: Kubernetes connection details
//   - Context: Kubernetes context name
//   - Cluster: Cluster name
//   - IsManagement: Whether it's a management cluster
//   - ReadyNodes: Number of ready nodes
//   - TotalNodes: Total number of nodes
//
// PortForwardServiceInfo: Port forward details
//   - Name: Service name
//   - Namespace: Kubernetes namespace
//   - Service: Kubernetes service name
//   - LocalPort: Local port number
//   - RemotePort: Remote port number
//   - Context: Target Kubernetes context
//
// MCPServerInfo: MCP server details
//   - Name: Server name
//   - Type: Server type (localCommand, container)
//   - ProxyPort: Local proxy port
//   - Command: Command or image used
//   - Dependencies: Required services
//
// # Event System
//
// The package includes an event system for state changes:
//
// ServiceStateChangedEvent: Emitted when a service changes state
//   - Label: Service that changed
//   - OldState: Previous state
//   - NewState: Current state
//   - Error: Any associated error
//   - Timestamp: When the change occurred
//
// # Usage Example
//
//	// Using the orchestrator API
//	var orch api.OrchestratorAPI = orchestrator.New(config)
//
//	// Start a service
//	if err := orch.StartService(ctx, "mc-prometheus"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get service status
//	status, err := orch.GetServiceStatus(ctx, "mc-prometheus")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Service state: %s, health: %s\n", status.State, status.Health)
//
//	// Subscribe to state changes
//	events := orch.SubscribeToStateChanges()
//	for event := range events {
//	    fmt.Printf("Service %s: %s -> %s\n",
//	        event.Label, event.OldState, event.NewState)
//	}
//
// # Thread Safety
//
// All interfaces in this package must be implemented in a thread-safe manner
// as they may be called concurrently from multiple goroutines (TUI updates,
// health checkers, user actions, etc.).
package api
