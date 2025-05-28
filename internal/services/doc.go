// Package services provides the service abstraction layer for envctl.
//
// This package defines the core interfaces and types that all services in envctl
// must implement. It provides a unified way to manage different types of services
// (Kubernetes connections, port forwards, MCP servers) through a common interface.
//
// # Core Concepts
//
// Service: The fundamental unit of work in envctl. Each service represents a
// manageable component that can be started, stopped, and monitored.
//
// ServiceRegistry: A thread-safe registry that holds all active services and
// provides methods to query and manage them.
//
// ServiceState: Represents the current state of a service (stopped, starting,
// running, stopping, failed).
//
// # Service Types
//
// The package supports three main service types:
//
//   - TypeKubeConnection: Manages Kubernetes cluster connections via Teleport
//   - TypePortForward: Creates and maintains kubectl port-forward tunnels
//   - TypeMCPServer: Runs Model Context Protocol servers for AI integration
//
// # Service Interface
//
// All services must implement the Service interface:
//
//	type Service interface {
//	    GetLabel() string              // Unique identifier
//	    GetType() ServiceType          // Service type
//	    GetState() ServiceState        // Current state
//	    GetHealth() ServiceHealth      // Health status
//	    Start(ctx context.Context) error
//	    Stop(ctx context.Context) error
//	    Restart(ctx context.Context) error
//	}
//
// # Optional Interfaces
//
// Services can implement additional interfaces for extended functionality:
//
// HealthChecker: For services that support periodic health checks
//
//	type HealthChecker interface {
//	    CheckHealth(ctx context.Context) (ServiceHealth, error)
//	    GetHealthCheckInterval() time.Duration
//	}
//
// # Service Lifecycle
//
// 1. Creation: Service is created with its configuration
// 2. Registration: Service is registered with the ServiceRegistry
// 3. Starting: Service transitions through Stopped → Starting → Running
// 4. Health Monitoring: Optional periodic health checks update service health
// 5. Stopping: Service transitions through Running → Stopping → Stopped
// 6. Failure: Service can transition to Failed state from any other state
//
// # Thread Safety
//
// The ServiceRegistry is thread-safe and can be accessed concurrently.
// Individual service implementations must also be thread-safe as they may
// be accessed from multiple goroutines (orchestrator, API, health checkers).
//
// # Example Usage
//
//	// Create a registry
//	registry := services.NewRegistry()
//
//	// Create and register a service
//	service := k8s.NewK8sConnectionService("k8s-mc-prod", "prod-context", true, kubeMgr)
//	registry.Register(service)
//
//	// Start the service
//	if err := service.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Query service state
//	if service.GetState() == services.StateRunning {
//	    fmt.Println("Service is running")
//	}
//
//	// Stop the service
//	if err := service.Stop(ctx); err != nil {
//	    log.Fatal(err)
//	}
package services
