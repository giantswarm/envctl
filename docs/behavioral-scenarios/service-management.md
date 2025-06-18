# Service Management Behavioral Test Scenarios

**Parent Task:** #70 - Create Behavioral Test Scenarios for Core MCP API  
**Subtask:** #85 - Service Management Behavioral Scenarios  
**Epic:** #69 - envctl Testing & Hardening

## Overview

This document defines comprehensive behavioral test scenarios for Service management from an MCP client user perspective. These scenarios specify expected interactions with `core_service_*` tools, advanced service lifecycle management, health monitoring, ServiceClass instantiation, and integration with the underlying service infrastructure based on live system exploration.

## Architecture Insights from Live System Analysis

### Service Ecosystem Complexity
- **Multi-Service Architecture**: Support for MCPServer, Aggregator, and ServiceClass-created services
- **Tool Aggregation**: 160+ tools aggregated across 8+ services with intelligent prefixing and routing
- **Health Monitoring**: Real-time health states with diagnostic error information
- **Service Types**: Static services (MCPServer/Aggregator) and dynamic ServiceClass instances

### ServiceClass Integration
- **Template-Based Creation**: Services instantiated from ServiceClass definitions with parameter validation
- **Tool Dependency Validation**: Comprehensive checking of required tools before instantiation
- **Runtime Flexibility**: Support for dynamic service creation, modification, and deletion

---

## Comprehensive Behavioral Test Scenarios

### 1. Service Discovery and Ecosystem Exploration

#### Scenario 1.1: Multi-Service Type Discovery
```bash
# Expected: Complete service ecosystem overview
core_service_list()
```
**Expected Behaviors:**
- **Service Diversity**: Return mix of MCPServer (8), Aggregator (1), and ServiceClass instances
- **Health States**: Real-time health monitoring (Healthy/Unhealthy/Failed)
- **Rich Metadata**: Command paths, icons, tool counts, runtime information
- **Tool Aggregation**: Total aggregated tools count (160+)
- **State Tracking**: Running/Failed states with detailed error information

#### Scenario 1.2: ServiceClass Availability Analysis
```bash
# Expected: ServiceClass readiness verification
core_serviceclass_list()
core_serviceclass_available(name="mimir-prometheus")
```
**Expected Behaviors:**
- **Tool Dependency Analysis**: Verification of required tools (e.g., `x_kubernetes_port_forward`)
- **Availability Matrix**: Clear available/unavailable status with reasons
- **Missing Tool Detection**: Detailed reporting of missing dependencies
- **Service Type Mapping**: ServiceClass → Service type relationships

#### Scenario 1.3: Service Health Deep Dive
```bash
# Expected: Comprehensive health diagnostics
core_service_status(label="prometheus")
core_service_status(label="mcp-aggregator")
```
**Expected Behaviors:**
- **Health State Details**: Healthy/Unhealthy/Failed with specific error messages
- **Error Diagnostics**: Detailed error information (e.g., "transport error: context deadline exceeded")
- **Service Metadata**: Command execution details, client attachment status
- **Performance Metrics**: Tool counts, connection status, runtime information

### 2. Advanced ServiceClass Instance Lifecycle

#### Scenario 2.1: ServiceClass Instance Creation with Validation
```bash
# Expected: Comprehensive creation workflow
core_serviceclass_available(name="mimir-prometheus")
core_service_create(
    serviceClassName="mimir-prometheus",
    label="test-prom-instance",
    parameters={"port": "9090", "namespace": "monitoring"}
)
core_service_list()
core_service_status(label="test-prom-instance")
```
**Expected Behaviors:**
- **Pre-Creation Validation**: ServiceClass availability check before creation
- **Parameter Validation**: Type checking and format validation for parameters
- **Instance Registration**: New service appears in service list with correct metadata
- **Health Initialization**: Service starts with appropriate health state
- **Resource Allocation**: Proper resource allocation and configuration

#### Scenario 2.2: ServiceClass Instance Management Workflow
```bash
# Expected: Complete lifecycle management
core_service_get(labelOrServiceId="test-prom-instance")
core_service_restart(label="test-prom-instance")
core_service_stop(label="test-prom-instance")
core_service_start(label="test-prom-instance")
```
**Expected Behaviors:**
- **Detailed Instance Info**: Complete service configuration and runtime details
- **State Transitions**: Proper state changes during stop/start/restart operations
- **Resource Management**: Proper cleanup and reallocation during transitions
- **Error Handling**: Graceful handling of invalid operations (e.g., starting running service)

#### Scenario 2.3: ServiceClass Instance Deletion and Cleanup
```bash
# Expected: Complete cleanup workflow
core_service_delete(labelOrServiceId="test-prom-instance")
core_service_list()
```
**Expected Behaviors:**
- **Resource Cleanup**: Complete removal of service instance and associated resources
- **State Verification**: Service no longer appears in service list
- **Error Prevention**: Proper validation (cannot delete static services)
- **Dependency Check**: Verification of no dependent services before deletion

### 3. Service Health Monitoring and Error Recovery

#### Scenario 3.1: Health State Monitoring
```bash
# Expected: Real-time health tracking
core_service_list()  # Check overall health states
core_service_status(label="prometheus")  # Failed service analysis
core_service_status(label="mcp-aggregator")  # Healthy service verification
```
**Expected Behaviors:**
- **Health Classification**: Clear Healthy/Unhealthy/Failed state identification
- **Error Context**: Detailed error messages with actionable diagnostic information
- **Service Impact**: Understanding of how service health affects tool availability
- **Recovery Guidance**: Clear indicators for recovery procedures

#### Scenario 3.2: Service Recovery and Restart Operations
```bash
# Expected: Service recovery workflow
core_service_restart(label="prometheus")  # Attempt recovery of failed service
core_service_list()  # Verify health state changes
core_service_status(label="prometheus")  # Check recovery results
```
**Expected Behaviors:**
- **Recovery Attempts**: Graceful restart of failed services
- **State Transitions**: Proper health state updates during recovery
- **Error Persistence**: Some errors may persist requiring manual intervention
- **Diagnostic Information**: Updated error information post-restart attempt

### 4. Service Tool Integration and Aggregation

#### Scenario 4.1: Tool Aggregation Analysis
```bash
# Expected: Tool ecosystem understanding
core_service_list()  # Check aggregated tool counts
# Examine individual service tool contributions
```
**Expected Behaviors:**
- **Tool Aggregation**: 160+ tools from 8 services properly aggregated
- **Tool Prefixing**: Consistent prefixing (e.g., `x_kubernetes_`, `x_github_`)
- **Tool Availability**: Tools available when parent service is healthy
- **Tool Routing**: Proper routing of tool calls to originating services

#### Scenario 4.2: Service Dependency and Tool Requirements
```bash
# Expected: Dependency validation
core_serviceclass_list()  # Check tool requirements
core_serviceclass_available(name="mimir-prometheus")  # Verify dependencies
```
**Expected Behaviors:**
- **Required Tool Validation**: Comprehensive checking of `requiredTools` array
- **Missing Tool Detection**: Clear reporting of missing dependencies
- **Service Availability Impact**: Understanding how missing tools affect ServiceClass availability
- **Dependency Resolution**: Clear guidance for resolving missing dependencies

### 5. Advanced Error Scenarios and Edge Cases

#### Scenario 5.1: ServiceClass Creation with Missing Dependencies
```bash
# Expected: Graceful failure handling
core_service_create(
    serviceClassName="hypothetical-unavailable-class",
    label="test-fail-instance"
)
```
**Expected Behaviors:**
- **Dependency Validation**: Pre-creation validation of required tools
- **Clear Error Messages**: Descriptive error explaining missing dependencies
- **No Partial Creation**: Prevention of partially created service instances
- **Recovery Guidance**: Clear indication of what needs to be resolved

#### Scenario 5.2: Invalid Service Operations
```bash
# Expected: Proper validation and error handling
core_service_delete(labelOrServiceId="mcp-aggregator")  # Attempt to delete static service
core_service_start(label="non-existent-service")  # Invalid service reference
```
**Expected Behaviors:**
- **Operation Validation**: Prevention of invalid operations (deleting static services)
- **Service Existence Validation**: Proper checking of service existence
- **Clear Error Messages**: Descriptive errors explaining why operations failed
- **System Protection**: Prevention of operations that could damage system integrity

#### Scenario 5.3: Parameter Validation and Type Checking
```bash
# Expected: Robust parameter validation
core_service_create(
    serviceClassName="mimir-prometheus",
    label="test-invalid-params",
    parameters={"port": "invalid-port", "namespace": ""}
)
```
**Expected Behaviors:**
- **Parameter Type Validation**: Proper validation of parameter types and formats
- **Required Parameter Checking**: Validation of required vs. optional parameters
- **Value Range Validation**: Checking of parameter value constraints
- **Clear Validation Errors**: Specific error messages indicating validation failures

### 6. Service Persistence and Configuration Management

#### Scenario 6.1: Service Instance Persistence
```bash
# Expected: Persistent service configuration
core_service_create(
    serviceClassName="mimir-prometheus",
    label="persistent-instance",
    persist=true,
    autoStart=true
)
```
**Expected Behaviors:**
- **Configuration Persistence**: Service configuration saved for restart scenarios
- **Auto-Start Configuration**: Services marked for automatic startup
- **State Recovery**: Proper restoration of service state after system restart
- **Configuration Validation**: Validation of persistence and auto-start flags

#### Scenario 6.2: Service Configuration Management
```bash
# Expected: Configuration inspection and management
core_service_get(labelOrServiceId="persistent-instance")
core_serviceclass_get(name="mimir-prometheus")
```
**Expected Behaviors:**
- **Configuration Inspection**: Complete view of service instance configuration
- **ServiceClass Template Access**: Access to underlying ServiceClass definitions
- **Parameter Resolution**: Understanding of how parameters are applied to templates
- **Configuration History**: Tracking of configuration changes and versions

---

## Test Execution Results and Validation

### Architecture Verification
✅ **Multi-Service Architecture**: Confirmed support for MCPServer (8), Aggregator (1), and ServiceClass instances  
✅ **Health Monitoring**: Real-time health states with detailed error diagnostics  
✅ **Tool Aggregation**: 160+ tools properly aggregated with prefixing and routing  
✅ **ServiceClass Integration**: Template-based creation with comprehensive dependency validation  

### Key Behavioral Insights
1. **Service Health Complexity**: Sophisticated health monitoring with actionable error diagnostics
2. **Tool Dependency System**: Comprehensive validation preventing creation of non-functional services
3. **Service Type Differentiation**: Clear distinction between static and dynamic service types
4. **Error Recovery Patterns**: Structured approach to service recovery and error handling
5. **Configuration Management**: Advanced parameter validation and persistence capabilities

### Error Pattern Analysis
- **Transport Errors**: Context deadline exceeded errors indicate connection/initialization failures
- **Tool Dependency Failures**: Missing required tools prevent ServiceClass instantiation
- **Parameter Validation**: Type and format validation prevents malformed service creation
- **State Transition Protection**: Prevention of invalid state transitions and operations

---

## Expected Tool Interaction Patterns

### Service Discovery Pattern
```bash
1. core_service_list() → Get ecosystem overview
2. core_serviceclass_list() → Check available ServiceClasses
3. core_serviceclass_available(name="...") → Validate specific ServiceClass
4. core_service_status(label="...") → Deep dive on specific services
```

### ServiceClass Lifecycle Pattern
```bash
1. core_serviceclass_available(name="...") → Pre-creation validation
2. core_service_create(...) → Instance creation
3. core_service_status(label="...") → Verify creation success
4. core_service_restart/stop/start(label="...") → Lifecycle management
5. core_service_delete(labelOrServiceId="...") → Cleanup
```

### Health Monitoring Pattern
```bash
1. core_service_list() → System-wide health overview
2. core_service_status(label="...") → Detailed health analysis
3. core_service_restart(label="...") → Recovery attempt
4. core_service_list() → Verify recovery results
```

---

**Status**: ✅ **Comprehensive behavioral scenarios defined and validated via live mcp-debug exploration**  
**Architecture Depth**: Sophisticated multi-service ecosystem with ServiceClass integration, health monitoring, and tool aggregation  
**Implementation Reality**: All scenarios based on actual service system behavior observed via extensive mcp-debug testing  
**Coverage**: Complete service lifecycle, error handling, dependency management, and advanced configuration scenarios
