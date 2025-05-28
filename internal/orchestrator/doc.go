// Package orchestrator provides the core service orchestration functionality for envctl.
//
// The orchestrator is responsible for managing the lifecycle of all services in envctl,
// including Kubernetes connections, port forwards, and MCP servers. It ensures services
// are started in the correct dependency order and handles automatic recovery when
// dependencies fail.
//
// # Architecture
//
// The orchestrator uses a service registry pattern combined with a dependency graph
// to manage services. Each service implements the services.Service interface and can
// declare dependencies on other services.
//
// # Service Types
//
// The orchestrator manages three main types of services:
//
//   - K8s Connections: Establish and maintain connections to Kubernetes clusters via Teleport
//   - Port Forwards: Create kubectl port-forward tunnels to cluster services
//   - MCP Servers: Run Model Context Protocol servers for AI assistant integration
//
// # Dependency Management
//
// Services are organized in a dependency hierarchy:
//
//  1. K8s connections (foundation - no dependencies)
//  2. Port forwards (depend on K8s connections)
//  3. MCP servers (may depend on port forwards)
//
// When a service fails, all dependent services are automatically stopped. When the
// failed service recovers, dependent services that were auto-stopped are restarted.
//
// # Health Monitoring
//
// The orchestrator continuously monitors service health through:
//
//   - Periodic health checks for each service type
//   - Automatic recovery attempts for failed services
//   - Cascade failure handling for dependent services
//
// # Usage Example
//
//	cfg := orchestrator.Config{
//	    MCName: "myinstallation",
//	    WCName: "myworkload",
//	    PortForwards: portForwardConfigs,
//	    MCPServers: mcpServerConfigs,
//	}
//
//	orch := orchestrator.New(cfg)
//	if err := orch.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer orch.Stop()
//
// # Service Labels
//
// Services are identified by labels following these conventions:
//
//   - K8s connections: "k8s-mc-{cluster}" or "k8s-wc-{cluster}"
//   - Port forwards: "{name}" (from configuration)
//   - MCP servers: "mcp-{name}" (from configuration)
package orchestrator
