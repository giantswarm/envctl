# envctl Architecture Overview

## Table of Contents
1. [Introduction](#introduction)
2. [System Overview](#system-overview)
3. [Core Components](#core-components)
4. [Service Types](#service-types)
5. [Dependency Management](#dependency-management)
6. [State Management](#state-management)
7. [Message Flow](#message-flow)
8. [Health Monitoring](#health-monitoring)
9. [Error Handling](#error-handling)
10. [Design Principles](#design-principles)

## Introduction

envctl is a sophisticated service orchestration tool designed to manage Kubernetes connections, port forwards, and MCP (Model Context Protocol) servers. It provides both a Terminal User Interface (TUI) and non-TUI modes for managing these services with proper dependency tracking and health monitoring.

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          User Interface                          │
│                    (TUI via Bubble Tea Framework)                │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Orchestrator                            │
│         (Dependency Management & Service Coordination)           │
└─────────────────────────────────────────────────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    ▼                             ▼
┌─────────────────────────────┐    ┌─────────────────────────────┐
│      Service Manager        │    │      State Manager          │
│  (Service Lifecycle Control)│    │   (State Persistence)       │
└─────────────────────────────┘    └─────────────────────────────┘
                    │
        ┌───────────┼───────────┬────────────────┐
        ▼           ▼           ▼                ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐
│   K8s    │ │  Port    │ │   MCP    │ │   Health     │
│ Services │ │ Forwards │ │ Servers  │ │  Checkers    │
└──────────┘ └──────────┘ └──────────┘ └──────────────┘
```

## Core Components

### 1. Orchestrator (`internal/orchestrator/`)
The central coordinator that manages all services and their dependencies.

**Key Responsibilities:**
- Building and maintaining the dependency graph
- Starting services in dependency order
- Handling cascade stops when dependencies fail
- Managing service restarts
- Tracking stop reasons (manual vs dependency failure)

**Key Features:**
- **Dependency Levels**: Services are grouped into levels (0, 1, 2, etc.) based on their dependencies
- **Cascade Operations**: When a service stops, all dependent services are automatically stopped
- **Smart Restart**: When a service is restarted, its dependencies are checked and started if needed
- **Stop Reason Tracking**: Distinguishes between manual stops and dependency-caused stops

### 2. Service Manager (`internal/managers/`)
Handles the lifecycle of individual services.

**Managed Service Types:**
- K8s Connections
- Port Forwards
- MCP Servers

**Key Operations:**
- `StartServices()`: Starts multiple services concurrently
- `StopService()`: Stops a specific service
- `IsServiceActive()`: Checks if a service is running
- `StopAllServices()`: Gracefully stops all services

### 3. State Management (`internal/state/` and `internal/reporting/`)
Manages service state and provides state persistence.

**Components:**
- **StateStore**: Central repository for service states
- **K8sStateManager**: Manages Kubernetes connection states
- **Reporter**: Handles state change notifications

**Features:**
- Atomic state updates
- State change history tracking
- Correlation ID tracking for related operations
- Cascade operation recording

### 4. TUI System (`internal/tui/`)
Provides the interactive terminal interface.

**Architecture:**
- **Model**: Contains application state and business logic
- **View**: Renders the UI components
- **Controller**: Handles user input and coordinates updates

**Key Features:**
- Real-time service status updates
- Interactive service control (start/stop/restart)
- Dependency graph visualization
- Health status monitoring
- Log viewing

## Service Types

### 1. K8s Connection Services
Manage connections to Kubernetes clusters.

**Features:**
- Health monitoring (node readiness checks)
- Automatic reconnection on failure
- Support for MC (Management Cluster) and WC (Workload Cluster)

**Configuration:**
```go
type K8sConnectionConfig struct {
    Name                string
    ContextName         string
    IsMC                bool
    HealthCheckInterval time.Duration
}
```

### 2. Port Forwards
Create kubectl port-forward tunnels to Kubernetes services.

**Features:**
- Automatic pod selection
- Connection monitoring
- Graceful shutdown
- Configurable bind addresses

**Configuration:**
```go
type PortForwardDefinition struct {
    Name              string
    Namespace         string
    TargetType        string // "service" or "pod"
    TargetName        string
    LocalPort         string
    RemotePort        string
    KubeContextTarget string
    Enabled           bool
}
```

### 3. MCP Servers
Model Context Protocol servers that provide AI model access.

**Types:**
- **Local Command**: Runs as a local process
- **Container**: Runs in a Docker container

**Features:**
- Process lifecycle management
- Health checking via JSON-RPC
- Environment variable support
- Port proxy integration

**Configuration:**
```go
type MCPServerDefinition struct {
    Name                 string
    Type                 MCPServerType
    Command              []string
    ProxyPort            int
    RequiresPortForwards []string
    Env                  map[string]string
    Enabled              bool
}
```

## Dependency Management

### Dependency Graph
Services are organized in a directed acyclic graph (DAG) where edges represent dependencies.

**Node Types:**
- K8s Connections (Level 0)
- Port Forwards (Level 1)
- MCP Servers (Level 2+)

**Example Dependency Chain:**
```
K8s MC Connection
    └── Port Forward (mc-prometheus)
            └── MCP Server (prometheus)
```

### Dependency Rules
1. **Port Forwards** depend on K8s connections
2. **MCP Servers** can depend on:
   - Port forwards (for backend access)
   - K8s connections (e.g., kubernetes MCP)
3. **Cascade Stops**: When a service stops, all dependents stop
4. **Dependency Restoration**: When a service restarts, dependent services that were stopped due to dependency failure are automatically restarted

### Stop Reasons
- **StopReasonManual**: User explicitly stopped the service
- **StopReasonDependency**: Service stopped due to dependency failure

Services with `StopReasonDependency` are automatically restarted when their dependencies are restored.

## State Management

### Service States
```go
const (
    StateUnknown   State = "unknown"
    StateStarting  State = "starting"
    StateRunning   State = "running"
    StateStopping  State = "stopping"
    StateStopped   State = "stopped"
    StateFailed    State = "failed"
)
```

### State Transitions
1. **Starting → Running**: Service successfully started
2. **Starting → Failed**: Service failed to start
3. **Running → Stopping**: Stop requested
4. **Stopping → Stopped**: Service stopped cleanly
5. **Running → Failed**: Service crashed or health check failed

### State Persistence
- States are maintained in memory via StateStore
- State changes trigger notifications to all listeners
- Correlation IDs track related state changes

## Message Flow

### 1. Service Start Flow
```
User Action → TUI Controller → Orchestrator → Service Manager
                                    ↓
                            Dependency Check
                                    ↓
                            Start in Levels
                                    ↓
                            State Updates → Reporter → TUI
```

### 2. Cascade Stop Flow
```
Stop Service A → Find Dependents → Stop B, C (mark as dependency stop)
        ↓
Record Cascade Operation
        ↓
State Updates → Reporter → TUI
```

### 3. Health Check Flow
```
Health Checker → Service Health Check → State Update
                          ↓
                    If Unhealthy
                          ↓
                  Cascade Stop Dependents
```

## Health Monitoring

### Health Check Types

1. **K8s Connection Health**
   - Checks node readiness
   - Monitors API server connectivity
   - Runs every 15 seconds by default

2. **Port Forward Health**
   - TCP connectivity check
   - Verifies tunnel is active
   - Runs every 30 seconds

3. **MCP Server Health**
   - JSON-RPC `tools/list` method
   - Verifies server is responding
   - Runs every 30 seconds

### Health Check Integration
- Health checks run in separate goroutines
- Failed health checks trigger cascade stops
- Recovery triggers automatic restart of dependent services

## Error Handling

### Error Categories

1. **Start Failures**
   - Port already in use
   - Missing dependencies
   - Configuration errors

2. **Runtime Failures**
   - Connection lost
   - Process crashed
   - Health check failures

3. **Cascade Failures**
   - Dependency failed
   - Automatic stop triggered

### Error Recovery

1. **Automatic Retry**
   - Services monitor for recovery conditions
   - Dependent services restart when dependencies recover

2. **Manual Intervention**
   - User can manually restart services
   - Clear error states via TUI

3. **Graceful Degradation**
   - Services continue running if non-critical dependencies fail
   - Partial functionality maintained

## Design Principles

### 1. Separation of Concerns
- Orchestrator handles coordination
- Service Manager handles lifecycle
- State Manager handles persistence
- TUI handles presentation

### 2. Dependency Inversion
- Core logic doesn't depend on UI
- Services don't know about orchestration
- Clean interfaces between layers

### 3. Event-Driven Architecture
- State changes trigger events
- Loose coupling via reporters
- Asynchronous updates

### 4. Fail-Safe Defaults
- Services stop on dependency failure
- Health checks prevent cascading failures
- Graceful shutdown on errors

### 5. Observability
- Comprehensive logging
- State change tracking
- Correlation ID for related operations

### 6. Testability
- Mock interfaces for testing
- Dependency injection
- Isolated component testing

## Configuration

Configuration can be provided via:
1. YAML files (`~/.config/envctl/config.yaml`)
2. Command-line arguments
3. Default configuration in code

See [configuration.md](configuration.md) for detailed configuration options.

## Future Enhancements

1. **Persistent State**
   - Save state between restarts
   - Resume previous session

2. **Advanced Health Checks**
   - Custom health check scripts
   - Configurable thresholds

3. **Service Groups**
   - Start/stop related services together
   - Named service sets

4. **External Integrations**
   - Webhook notifications
   - Metrics export
   - API endpoint

5. **Enhanced Recovery**
   - Exponential backoff
   - Circuit breakers
   - Automatic rollback 