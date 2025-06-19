# EnvCtl Testing Framework

The envctl testing framework provides comprehensive behavioral and integration testing capabilities for envctl components. It creates clean, isolated instances of `envctl serve` for each test scenario, ensuring complete test isolation and repeatability.

## Key Features

- **Isolated Test Instances**: Each test scenario runs against its own clean `envctl serve` instance
- **Pre-configuration Support**: Test scenarios can specify MCP servers, workflows, capabilities, service classes, and services to be pre-loaded
- **Automatic Cleanup**: Test instances and their configuration are automatically cleaned up after test execution
- **Parallel Execution**: Multiple test scenarios can run in parallel, each with its own instance
- **Comprehensive Reporting**: Detailed test results with structured output for CI/CD integration

## Architecture

### Test Instance Management

The testing framework uses the `EnvCtlInstanceManager` to:
1. Create temporary configuration directories for each test scenario
2. Generate configuration files based on the scenario's `pre_configuration`
3. Start `envctl serve` processes with `--no-tui` and `--config-path`
4. Wait for instances to be ready to accept MCP connections
5. Clean up instances and configuration after test completion

### Pre-Configuration

Test scenarios can specify pre-configuration in the `pre_configuration` section:

```yaml
pre_configuration:
  # MCP servers to load
  mcp_servers:
    - name: "test-kubernetes"
      type: "process"
      config:
        command: "kubectl-mcp"
        args: ["--context", "test-cluster"]
        
  # Workflows to create
  workflows:
    - name: "deploy-service"
      config:
        name: "deploy-service"
        description: "Deploy a service"
        steps: [...]
        
  # Service classes to create
  service_classes:
    - name: "web-service"
      config:
        name: "web-service"
        parameters: {...}
        template: |
          apiVersion: apps/v1
          kind: Deployment
          ...
          
  # Capabilities to define
  capabilities:
    - name: "monitoring"
      config:
        name: "monitoring"
        tools: ["x_prometheus_add_target"]
        
  # Service instances to create
  services:
    - name: "test-service"
      config:
        service_class_name: "web-service"
        parameters: {...}
        
  # Main envctl configuration
  main_config:
    config:
      logging:
        level: "debug"
      server:
        timeout: "30s"
```

## Usage

### Running Tests

```bash
# Run all tests
envctl test

# Run specific category
envctl test --category behavioral

# Run specific concept tests
envctl test --concept serviceclass

# Run specific scenario
envctl test --scenario serviceclass-basic-operations

# Run with debugging
envctl test --verbose --debug

# Run with custom base port
envctl test --base-port 19000

# Run in parallel
envctl test --parallel 4
```

### Test Configuration

Tests can be configured using command-line flags or configuration files:

```bash
# Use custom scenario directory
envctl test --config /path/to/scenarios

# Save detailed reports
envctl test --report /path/to/reports

# Set timeout
envctl test --timeout 30m

# Fail fast on first error
envctl test --fail-fast
```

## Writing Test Scenarios

### Basic Scenario Structure

```yaml
name: "my-test-scenario"
category: "behavioral"  # or "integration"
concept: "serviceclass"  # serviceclass, workflow, mcpserver, capability, service
description: "Description of what this scenario tests"
tags: ["tag1", "tag2"]
timeout: "5m"

# Optional: Pre-configure the envctl instance
pre_configuration:
  mcp_servers: [...]
  workflows: [...]
  service_classes: [...]
  capabilities: [...]
  services: [...]
  main_config: {...}

steps:
  - name: "step-name"
    description: "What this step does"
    tool: "core_serviceclass_create"
    parameters:
      name: "test-serviceclass"
      # ... other parameters
    expected:
      success: true
      contains: ["created successfully"]
    timeout: "30s"
    
cleanup:
  - name: "cleanup-step"
    tool: "core_serviceclass_delete"
    parameters:
      name: "test-serviceclass"
    expected:
      success: true
```

### Expectations

Test steps can validate responses using various expectation types:

```yaml
expected:
  success: true                        # Tool call should succeed
  contains: ["text1", "text2"]         # Response should contain text
  not_contains: ["error", "failed"]    # Response should not contain text
  error_contains: ["not found"]        # Error message should contain text
  json_path:                           # Validate JSON response fields
    status: "success"
    count: 5
  status_code: 200                     # HTTP status code (if applicable)
```

### Retry Configuration

Steps can be configured to retry on failure:

```yaml
steps:
  - name: "flaky-operation"
    tool: "some_tool"
    parameters: {...}
    retry:
      count: 3                         # Retry up to 3 times
      delay: "1s"                      # Wait 1 second between retries
      backoff_multiplier: 2.0          # Double delay each retry
    expected:
      success: true
```

## Test Categories and Concepts

### Categories

- **behavioral**: BDD-style scenarios that validate expected behavior
- **integration**: End-to-end scenarios that test component interactions

### Concepts

- **serviceclass**: ServiceClass management and templating
- **workflow**: Workflow execution and parameter resolution
- **mcpserver**: MCP server registration and tool aggregation
- **capability**: Capability definitions and API operations
- **service**: Service lifecycle and dependency management

## Example Scenarios

### Simple ServiceClass Test

```yaml
name: "serviceclass-basic-crud"
category: "behavioral"
concept: "serviceclass"
description: "Basic ServiceClass CRUD operations"

steps:
  - name: "create-serviceclass"
    tool: "core_serviceclass_create"
    parameters:
      yaml: |
        name: test-serviceclass
        description: "Test ServiceClass"
        parameters:
          image:
            type: string
            required: true
        template: |
          apiVersion: v1
          kind: ConfigMap
          metadata:
            name: "{{ .name }}"
          data:
            image: "{{ .parameters.image }}"
    expected:
      success: true
      
cleanup:
  - name: "delete-serviceclass"
    tool: "core_serviceclass_delete"
    parameters:
      name: "test-serviceclass"
    expected:
      success: true
```

### Integration Test with Pre-configuration

See `internal/testing/scenarios/serviceclass_with_preconfig.yaml` for a comprehensive example that demonstrates:
- Pre-configuring MCP servers
- Creating workflows and service classes
- Testing the interaction between components
- Using the pre-configured components in test steps

## CI/CD Integration

The testing framework provides structured output suitable for CI/CD:

```bash
# JSON output for machine consumption
envctl test --format json > test-results.json

# Quiet output for CI
envctl test --quiet

# Exit codes
# 0: All tests passed
# 1: Some tests failed
```

## Development

### Adding New Test Tools

Test scenarios use the same tools exposed by the envctl aggregator. To add new test capabilities:

1. Implement the functionality in the appropriate envctl package
2. Expose it via the API service locator pattern
3. Register it with the aggregator
4. Use it in test scenarios with the `core_*` prefix

### Debugging Tests

Enable debug mode to see detailed information:

```bash
envctl test --debug --verbose
```

This will show:
- EnvCtl instance creation and configuration
- MCP protocol communication
- Tool call requests and responses
- Instance cleanup operations

### Testing the Testing Framework

The testing framework itself can be tested using standard Go testing:

```bash
go test ./internal/testing/...
```

This includes unit tests for:
- Instance management
- Configuration generation
- Scenario loading and validation
- Result reporting 