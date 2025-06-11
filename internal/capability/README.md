# Service Capability Schema

This document describes the enhanced capability schema designed for dynamic service lifecycle management in envctl. This schema enables the creation of user-defined, extensible services without requiring code changes.

## Overview

The Service Capability Schema extends the existing capability system to support **dynamic service creation and management**. Instead of hardcoding service types, users can define services via YAML capability definitions that specify which aggregator tools to call for lifecycle events.

## Key Components

### ServiceCapabilityDefinition

The main structure that defines a service capability:

```yaml
name: service_portforward_provider
type: service_portforward_provider
version: "1.0.0"
description: "Dynamic service capability for managing Kubernetes port forward instances"

serviceConfig:
  # Service metadata
  serviceType: "DynamicPortForward"
  defaultLabel: "pf-{{ .resource_type }}-{{ .resource_name }}-{{ .local_port }}"
  
  # Lifecycle tool mappings
  lifecycleTools:
    create:
      tool: "x_kubernetes_port_forward"
      arguments:
        namespace: "{{ .namespace | default \"default\" }}"
        resourceType: "{{ .resource_type }}"
      responseMapping:
        serviceId: "$.id"
        status: "$.status"
    
    delete:
      tool: "x_kubernetes_stop_port_forward"
      arguments:
        id: "{{ .service_id }}"
    
    healthCheck:
      tool: "x_kubernetes_port_forward_status"
      arguments:
        id: "{{ .service_id }}"
  
  # Health check configuration
  healthCheck:
    enabled: true
    interval: "30s"
    failureThreshold: 3
    successThreshold: 1
  
  # Timeout configuration
  timeout:
    create: "60s"
    delete: "30s"
    healthCheck: "10s"
```

### LifecycleTools

Maps service lifecycle events to aggregator tools:

- **create**: Tool called when starting the service
- **delete**: Tool called when stopping the service  
- **healthCheck**: Tool called periodically to check service health
- **status**: Tool called to get detailed service information

### ResponseMapping

Defines how to extract information from tool responses:

- **serviceId**: JSON path to extract unique service identifier
- **status**: JSON path to extract status information
- **health**: JSON path to extract health status
- **metadata**: Additional data to store in service metadata

### ServiceInstance

Runtime representation of a service instance created from a capability:

```go
type ServiceInstance struct {
    ID                   string
    Label                string
    CapabilityName       string
    CapabilityType       string
    State                ServiceState
    Health               HealthStatus
    CreationParameters   map[string]interface{}
    ServiceData          map[string]interface{}
    CreatedAt            time.Time
    UpdatedAt            time.Time
    HealthCheckFailures  int
    HealthCheckSuccesses int
    Dependencies         []string
}
```

## Architecture Benefits

### 1. **Extensibility Without Recompilation**
- New service types can be added via YAML files
- No code changes required for new service patterns
- Users can define custom service behaviors

### 2. **Tool-Based Lifecycle Management**
- Leverages existing aggregator tool system
- Each lifecycle event calls specific tools
- Consistent interface across all service types

### 3. **Flexible Parameter Mapping**
- Template-based parameter substitution
- Support for default values and transformations
- Dynamic label generation

### 4. **Health Monitoring**
- Configurable health check intervals
- Automatic failure threshold handling
- Service state management

## Example Usage

### 1. Port Forward Service

Creates dynamic port forwarding services:

```yaml
# internal/capability/definitions/examples/service_portforward.yaml
name: service_portforward_provider
serviceConfig:
  serviceType: "DynamicPortForward" 
  lifecycleTools:
    create:
      tool: "x_kubernetes_port_forward"
    delete:
      tool: "x_kubernetes_stop_port_forward"
    healthCheck:
      tool: "x_kubernetes_port_forward_status"
```

### 2. Kubernetes Connection Service

Manages cluster connections with authentication:

```yaml
# internal/capability/definitions/examples/service_k8s_connection.yaml
name: service_k8s_connection_provider
serviceConfig:
  serviceType: "DynamicK8sConnection"
  lifecycleTools:
    create:
      tool: "x_kubernetes_connect"
    delete:
      tool: "x_kubernetes_disconnect"
    healthCheck:
      tool: "x_kubernetes_connection_status"
```

## Validation

The schema includes comprehensive validation:

- **Required fields**: name, type, version, serviceConfig
- **Tool name format**: Must start with 'x_' prefix
- **Timeout limits**: Reasonable bounds on operation timeouts
- **Health check configuration**: Positive intervals and thresholds

## Integration Points

### Service Orchestrator

The Service Orchestrator will:
1. Load capability definitions from YAML files
2. Create ServiceInstance objects when requested
3. Call lifecycle tools based on capability configuration
4. Manage health checking and state transitions
5. Handle service dependencies

### API Layer

Exposes service management via internal/api:
- `CreateService(capabilityName, label, parameters)`
- `DeleteService(label)`
- `GetServiceStatus(label)`
- `ListServices()`

## Files

- `service_schema.go` - Core data structures
- `service_validation.go` - Validation functions  
- `service_schema_test.go` - Comprehensive test suite
- `definitions/examples/` - Example capability definitions

## Testing

Run tests to verify schema functionality:

```bash
go test ./internal/capability/ -v
```

All tests pass with comprehensive coverage of validation, state management, and YAML parsing scenarios. 