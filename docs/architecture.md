# envctl Architecture Overview

## Table of Contents
1. [Introduction](#introduction)
2. [System Overview](#system-overview)
3. [Core Components](#core-components)
4. [ServiceClass Architecture](#serviceclass-architecture)
5. [Service Types](#service-types)
6. [Dependency Management](#dependency-management)
7. [State Management](#state-management)
8. [Message Flow](#message-flow)
9. [Health Monitoring](#health-monitoring)
10. [Error Handling](#error-handling)
11. [Design Principles](#design-principles)

## Introduction

envctl is a sophisticated service orchestration tool designed to manage Kubernetes connections, port forwards, and MCP (Model Context Protocol) servers. It provides both a Terminal User Interface (TUI) and non-TUI modes for managing these services with proper dependency tracking and health monitoring.

The architecture has been redesigned around a **ServiceClass** system that enables dynamic, configuration-driven service instantiation and management. This allows for flexible service definitions through YAML configurations and programmatic service creation.

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
│                      + ServiceClass Support                      │
└─────────────────────────────────────────────────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    ▼                             ▼
┌─────────────────────────────┐    ┌─────────────────────────────┐
│     ServiceClass Manager    │    │      Service Registry       │
│  (Definition Management &   │    │   (Service Lifecycle &      │
│   Dynamic Instantiation)    │    │    State Management)        │
└─────────────────────────────┘    └─────────────────────────────┘
                    │                             │
        ┌───────────┼───────────┐                │
        ▼           ▼           ▼                ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐
│ServiceCls│ │ Generic  │ │   Tool   │ │  Health &    │
│ Definitions│ │ Service  │ │ Caller   │ │ Monitoring   │
│ (YAML)   │ │Instances │ │(Aggregatr)│ │  System      │
└──────────┘ └──────────┘ └──────────┘ └──────────────┘
```

## Core Components

### 1. Orchestrator (`internal/orchestrator/`)
The central coordinator that manages all services and their dependencies, now enhanced with ServiceClass support.

**Key Responsibilities:**
- Building and maintaining the dependency graph
- Managing ServiceClass-based service instances
- Starting services in dependency order  
- Handling cascade stops when dependencies fail
- Managing service restarts and lifecycle events
- Tracking stop reasons (manual vs dependency failure)

**ServiceClass Integration:**
- Creates ServiceClass-based service instances dynamically
- Manages lifecycle through ServiceClass tool execution
- Handles ServiceClass instance state tracking and events

### 2. ServiceClass Manager (`internal/serviceclass/`)
**New component** that manages service class definitions and enables dynamic service creation.

**Key Responsibilities:**
- Loading ServiceClass definitions from YAML files
- Validating ServiceClass configurations
- Managing tool availability and ServiceClass availability status
- Providing ServiceClass metadata and tool information
- Supporting programmatic ServiceClass registration

**Key Features:**
- **YAML-driven Configuration**: ServiceClasses defined in declarative YAML
- **Tool Availability Checking**: Validates that required tools are available
- **Dynamic Registration**: Supports runtime ServiceClass registration
- **Lifecycle Tool Management**: Manages create, delete, health check, and status tools

### 3. Generic Service Instance (`internal/services/instance.go`)
**New component** that provides a runtime-configurable service implementation.

**Key Responsibilities:**
- Implements the Service interface using ServiceClass definitions
- Executes lifecycle operations through the aggregator tool system
- Manages service state, health monitoring, and data persistence
- Handles parameter templating and response mapping

**Key Features:**
- **Tool-Driven Lifecycle**: Uses aggregator tools for create, delete, health operations
- **Template Support**: Supports parameter templating for dynamic configurations
- **Health Check Integration**: Configurable health checking through tools
- **State Management**: Comprehensive state and health status tracking

### 4. Service Registry (`internal/services/`)
Enhanced service registry with ServiceClass support and unified service management.

**Components:**
- **Service Registry**: Central repository for all services and their states
- **Registry Adapter**: API adapter following the service locator pattern
- **Service Interfaces**: Unified interfaces for all service types

**ServiceClass Integration:**
- Manages both static and ServiceClass-based services
- Provides unified service access patterns
- Handles service instance tracking and lifecycle events

### 5. API Layer (`internal/api/`)
Enhanced API layer with comprehensive ServiceClass support.

**New ServiceClass APIs:**
- ServiceClass management operations (list, get, availability)
- ServiceClass instance lifecycle management (create, delete, status)
- ServiceClass instance event streaming
- Tool provider interfaces for ServiceClass operations

**Service Locator Pattern:**
- All inter-package communication goes through the central API layer
- Prevents direct coupling between packages
- Enables independent development and testing

### 6. TUI System (`internal/tui/`)
Enhanced TUI with ServiceClass awareness and dynamic service management.

**ServiceClass Features:**
- Dynamic service creation through ServiceClass selection
- ServiceClass availability and status display
- ServiceClass instance lifecycle management
- Enhanced service status monitoring

## ServiceClass Architecture

### ServiceClass Definition Structure

ServiceClasses are defined in YAML files with the following structure:

```yaml
name: example_service
type: kubernetes_service
version: "1.0.0"
description: "Example Kubernetes service with port forwarding"

serviceConfig:
  serviceType: "PortForward"
  defaultLabel: "{{ .name }}-{{ .namespace }}"
  dependencies: ["kubernetes-connection"]
  
  lifecycleTools:
    create:
      tool: "x_kubernetes_port_forward"
      arguments:
        namespace: "{{ .namespace }}"
        service: "{{ .service_name }}"
        local_port: "{{ .local_port }}"
        remote_port: "{{ .remote_port }}"
      responseMapping:
        serviceId: "$.connection_id"
        status: "$.status"
    
    delete:
      tool: "x_kubernetes_port_forward_stop"
      arguments:
        connection_id: "{{ .serviceId }}"
      responseMapping:
        status: "$.status"
    
    healthCheck:
      tool: "x_kubernetes_port_forward_status"
      arguments:
        connection_id: "{{ .serviceId }}"
      responseMapping:
        health: "$.healthy"
        status: "$.status"

  healthCheck:
    enabled: true
    interval: 30s
    failureThreshold: 3
    successThreshold: 1

metadata:
  provider: "kubernetes"
  category: "networking"
```

### ServiceClass Lifecycle

1. **Definition Loading**: ServiceClass definitions loaded from YAML files
2. **Tool Availability Check**: Required tools validated against aggregator
3. **Availability Determination**: ServiceClass marked as available/unavailable
4. **Instance Creation**: Dynamic service instances created from ServiceClass
5. **Lifecycle Management**: Tool execution for create, delete, health operations
6. **State Tracking**: Instance state and health monitoring
7. **Event Propagation**: ServiceClass instance events for monitoring

### Tool Integration

ServiceClasses integrate with the aggregator tool system:

- **Tool Execution**: ServiceClass operations execute through aggregator tools
- **Parameter Templating**: Dynamic parameter substitution using template engine
- **Response Mapping**: Structured response handling and data extraction
- **Error Handling**: Comprehensive error handling and state management

## Service Types

### 1. ServiceClass-Based Services
**New primary service type** that provides flexible, configuration-driven services.

**Features:**
- Dynamic service creation from YAML definitions
- Tool-driven lifecycle management
- Configurable health checking and monitoring
- Template-based parameter management
- Comprehensive state and event tracking

**Lifecycle:**
```go
// Create ServiceClass instance
instance := orchestrator.CreateServiceClassInstance(ctx, CreateServiceClassRequest{
    ServiceClassName: "kubernetes_port_forward",
    Label: "my-service",
    Parameters: map[string]interface{}{
        "namespace": "default",
        "service_name": "my-app",
        "local_port": "8080",
        "remote_port": "80",
    },
})

// Lifecycle managed through tools
// - Create: Executes create tool with parameters
// - Health: Periodic health check tool execution  
// - Delete: Executes delete tool for cleanup
```

### 2. Static K8s Connection Services
Enhanced Kubernetes connection services with ServiceClass integration.

**Features:**
- Health monitoring through Kubernetes API
- ServiceClass dependency support
- Automatic reconnection on failure
- Support for MC (Management Cluster) and WC (Workload Cluster)

### 3. Static Port Forwards
Enhanced port forward services with ServiceClass dependencies.

**Features:**
- ServiceClass dependency management
- Connection monitoring through ServiceClass health checks
- Graceful shutdown and cleanup
- Configurable bind addresses

### 4. Static MCP Servers
Enhanced MCP servers with ServiceClass integration.

**Features:**
- ServiceClass dependency support
- Health checking via JSON-RPC or ServiceClass tools
- Process lifecycle management
- Environment variable support

## Dependency Management

### Enhanced Dependency Graph
Services are organized in a directed acyclic graph (DAG) with ServiceClass support:

**Node Types:**
- Static Services (K8s Connections, Port Forwards, MCP Servers)
- ServiceClass-Based Services (Dynamic instances)

**Dependency Features:**
- **ServiceClass Dependencies**: Defined in ServiceClass YAML configurations
- **Dynamic Dependencies**: ServiceClass instances can have runtime dependencies
- **Cross-Type Dependencies**: ServiceClass services can depend on static services and vice versa

**Example Dependency Chain:**
```
K8s MC Connection (Static)
    └── Port Forward Service (ServiceClass-based)
            └── Prometheus MCP Server (ServiceClass-based)
                    └── Application Service (ServiceClass-based)
```

### ServiceClass Dependency Rules
1. **ServiceClass Definition Dependencies**: Specified in YAML configuration
2. **Runtime Dependencies**: Can be added during instance creation
3. **Tool Dependencies**: ServiceClass availability depends on required tools
4. **Cascade Operations**: ServiceClass instances participate in cascade stops/starts
5. **Dependency Restoration**: ServiceClass instances restored when dependencies recover

## State Management

### Enhanced Service States
```go
const (
    StateUnknown   ServiceState = "Unknown"
    StateWaiting   ServiceState = "Waiting"    // Waiting for dependencies
    StateStarting  ServiceState = "Starting"   // Tool execution in progress
    StateRunning   ServiceState = "Running"    // Successfully running
    StateStopping  ServiceState = "Stopping"   // Stop tool execution in progress
    StateStopped   ServiceState = "Stopped"    // Cleanly stopped
    StateFailed    ServiceState = "Failed"     // Tool execution failed
    StateRetrying  ServiceState = "Retrying"   // Automatic retry in progress
)
```

### ServiceClass Instance State
ServiceClass instances maintain comprehensive state information:

```go
type ServiceInstance struct {
    ID                   string                 // Unique instance ID
    Label                string                 // Service label
    ServiceClassName     string                 // Source ServiceClass
    ServiceClassType     string                 // Type of ServiceClass
    State                ServiceState           // Current state
    Health               HealthStatus           // Health status
    CreationParameters   map[string]interface{} // Creation parameters
    ServiceData          map[string]interface{} // Runtime data from tools
    CreatedAt            time.Time              // Creation timestamp
    LastChecked          *time.Time             // Last health check
    HealthCheckFailures  int                    // Failure count
    Dependencies         []string               // Service dependencies
}
```

### State Persistence and Events
- **ServiceClass Instance Events**: Comprehensive event system for state changes
- **State Tracking**: Enhanced state tracking with ServiceClass metadata
- **Event Subscription**: Real-time event streaming for ServiceClass instances
- **Correlation IDs**: Track related operations across ServiceClass lifecycle

## Message Flow

### 1. ServiceClass Instance Creation Flow
```
User Action → TUI/API → Orchestrator → ServiceClass Manager
                                ↓
                        ServiceClass Validation
                                ↓
                        Tool Availability Check
                                ↓
                        Generic Service Instance Creation
                                ↓
                        Tool Execution (Create)
                                ↓
                        State Updates → Event System → TUI/Subscribers
```

### 2. ServiceClass Health Check Flow
```
Health Scheduler → Generic Service Instance → ServiceClass Manager
                                ↓
                        Health Check Tool Info
                                ↓
                        Tool Execution (Health Check)
                                ↓
                        Health Status Update → Event System
                                ↓
                        If Unhealthy: Cascade Stop Dependents
```

### 3. ServiceClass Tool Execution Flow
```
Service Operation → Generic Service Instance → Template Engine
                                ↓
                        Parameter Substitution
                                ↓
                        Tool Caller (Aggregator)
                                ↓
                        Tool Execution
                                ↓
                        Response Mapping
                                ↓
                        Service Data Update → Event System
```

## Health Monitoring

### Enhanced Health Check Types

1. **ServiceClass Health Checks**
   - Tool-driven health checking
   - Configurable intervals and thresholds
   - Template-based parameter substitution
   - Response mapping for health status extraction

2. **Static Service Health Checks**
   - K8s API connectivity checks
   - Port forward TCP connectivity
   - MCP server JSON-RPC health checks

### Health Check Configuration
ServiceClass health checking supports comprehensive configuration:

```yaml
healthCheck:
  enabled: true
  interval: 30s           # Check interval
  failureThreshold: 3     # Failures before unhealthy
  successThreshold: 1     # Successes to become healthy
```

### Health Check Integration
- **Unified Health Monitoring**: Both static and ServiceClass services
- **Event-Driven Updates**: Health changes trigger immediate events
- **Cascade Failure Handling**: Unhealthy services trigger dependent stops
- **Recovery Detection**: Automatic restart of dependents when health recovers

## Error Handling

### Enhanced Error Categories

1. **ServiceClass Errors**
   - ServiceClass definition validation errors
   - Tool availability errors
   - Tool execution failures
   - Parameter templating errors
   - Response mapping errors

2. **Static Service Errors**
   - Configuration errors
   - Connection failures
   - Process crashes

3. **Dependency Errors**
   - Missing dependencies
   - Circular dependency detection
   - Cascade failure propagation

### ServiceClass Error Recovery

1. **Tool Execution Retry**
   - Configurable retry policies
   - Exponential backoff support
   - Failure threshold tracking

2. **ServiceClass Availability Recovery**
   - Tool availability monitoring
   - Automatic ServiceClass re-evaluation
   - Dynamic availability updates

3. **Instance Recovery**
   - Health check recovery detection
   - Automatic dependent restart
   - State reconciliation

## Design Principles

### 1. ServiceClass-Driven Architecture
- Configuration over code for service definitions
- Tool-based lifecycle management
- Template-driven parameter handling
- Event-driven state management

### 2. Enhanced Separation of Concerns
- ServiceClass Manager handles definitions and metadata
- Generic Service Instance handles runtime behavior
- Orchestrator handles coordination and dependencies
- API Layer provides unified access patterns

### 3. Service Locator Pattern
- All inter-package communication through central API
- No direct dependencies between packages
- Clean interfaces and adapters
- Enhanced testability and modularity

### 4. Tool-Driven Operations
- ServiceClass operations executed through aggregator tools
- Consistent tool execution patterns
- Comprehensive error handling and response mapping
- Template-based parameter management

### 5. Event-Driven Architecture
- Comprehensive event system for ServiceClass instances
- Real-time state and health updates
- Loose coupling via event subscription
- Asynchronous state propagation

### 6. Fail-Safe Defaults
- ServiceClass unavailable when tools missing
- Graceful degradation for tool failures
- Comprehensive error tracking and reporting
- Safe cascade operations

## Configuration

### ServiceClass Configuration
ServiceClasses are configured through YAML files in the ServiceClass definitions directory:

- **Location**: `~/.config/envctl/serviceclass/definitions/`
- **File Pattern**: `service_*.yaml` or `provider_*.yaml`
- **Automatic Loading**: Definitions loaded at startup and refreshed dynamically

### Traditional Configuration
Traditional configuration continues to be supported:
- YAML files (`~/.config/envctl/config.yaml`)
- Command-line arguments
- Default configuration in code

See [configuration.md](configuration.md) for detailed configuration options.

## Future Enhancements

### ServiceClass Enhancements
1. **ServiceClass Templates**
   - Parameterized ServiceClass definitions
   - Multi-environment support
   - Inheritance and composition

2. **Advanced Tool Integration**
   - Tool discovery and registration
   - Tool versioning and compatibility
   - Tool execution optimization

3. **ServiceClass Marketplace**
   - Shared ServiceClass definitions
   - Community-contributed ServiceClasses
   - ServiceClass validation and certification

### Platform Enhancements
1. **Enhanced State Persistence**
   - ServiceClass instance state persistence
   - Session restoration
   - State snapshots and rollback

2. **Advanced Monitoring**
   - Metrics collection and export
   - Custom health check scripts
   - Performance monitoring

3. **External Integrations**
   - Webhook notifications for ServiceClass events
   - External tool providers
   - API gateway integration