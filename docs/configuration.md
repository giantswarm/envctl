# envctl Configuration System

This document provides comprehensive documentation for the envctl configuration system, covering all configuration file types, their purpose, structure, and usage patterns.

## Overview

The envctl configuration system uses a **layered approach** with configuration files organized in `.envctl/` directories. The system supports both project-level and user-level configurations, with user-level configurations taking precedence when present.

### Configuration Locations

- **Project Configuration**: `.envctl/` in project root (primary)
- **User Configuration**: `~/.envctl/` in user home (overrides project config when present)

### Key Features

- **Layered Loading**: User config overrides project config
- **Template Variables**: Support for `{{ .variable }}` templating throughout
- **Tool Availability Checking**: Automatic validation of required tools
- **Enhanced Error Handling**: Graceful degradation with detailed error reporting
- **Rich Metadata**: Icons, categories, and descriptive information

## Configuration File Types

### 1. Main Configuration (`config.yaml`)

The primary configuration file defining MCP servers, global settings, and aggregator configuration.

**Location**: `.envctl/config.yaml`

**Purpose**: 
- Define MCP server connections and commands
- Configure global application settings
- Set up aggregator behavior and endpoints
- Reference workflow configurations

**Structure**:
```yaml
# MCP Server configurations
mcpServers:
  - name: server-name
    type: localCommand
    enabledByDefault: true
    icon: "🔌"
    category: "Core"
    command: ["command", "args"]
    env:
      ENV_VAR: "value"
    requiresClusterRole: target  # Optional
    requiresPortForwards: ["pf-name"]  # Optional

# Global application settings
globalSettings:
  defaultContainerRuntime: docker

# Aggregator configuration
aggregator:
  port: 8090
  host: localhost
  enabled: true
  envctlPrefix: "x"  # Optional tool prefix

# Workflow references
workflows: []
```

**Key Fields**:
- `mcpServers`: Array of MCP server definitions
- `globalSettings`: Application-wide configuration
- `aggregator`: MCP aggregator settings
- `workflows`: Referenced workflow definitions

### 2. ServiceClasses (`serviceclasses/`)

ServiceClasses define dynamic service types with lifecycle management capabilities.

**Location**: `.envctl/serviceclasses/*.yaml`

**Purpose**:
- Define reusable service templates
- Specify lifecycle tool mappings (start/stop/health)
- Configure parameter validation and mapping
- Enable dynamic service instantiation

**Structure**:
```yaml
name: service-name
type: service-type
version: "1.0.0"
description: "Service description"

serviceConfig:
  serviceType: "ServiceType"
  defaultLabel: "{{ .parameter }}-{{ .other }}"
  dependencies: []
  
  lifecycleTools:
    start:
      tool: "x_tool_start"
      arguments:
        param: "{{ .input_param }}"
      responseMapping:
        serviceId: "$.id"
        status: "$.status"
    
    stop:
      tool: "x_tool_stop"
      arguments:
        id: "{{ .serviceId }}"
      responseMapping:
        status: "$.status"
  
  healthCheck:
    enabled: true
    interval: "30s"
    failureThreshold: 3
    successThreshold: 1
  
  createParameters:
    input_param:
      toolParameter: "param"
      required: true

metadata:
  provider: "kubernetes"
  category: "networking"
  icon: "🔌"
```

**Key Features**:
- Template variable support in all fields
- Response mapping for dynamic data extraction
- Health check configuration
- Parameter validation and mapping

### 3. Capabilities (`capabilities/`)

Capabilities define operation capabilities with embedded workflows and tool requirements.

**Location**: `.envctl/capabilities/*.yaml`

**Purpose**:
- Define complex operations with multiple steps
- Specify tool requirements and availability checks
- Embed workflows directly in capability definitions
- Provide parameter schemas and validation

**Structure**:
```yaml
name: capability-name
type: capability-type
version: "1.0.0"
description: "Capability description"

operations:
  operation-name:
    description: "Operation description"
    parameters:
      param-name:
        type: string
        required: true
        description: "Parameter description"
    requires:
      - x_required_tool
    workflow:
      name: workflow-name
      description: "Workflow description"
      agentModifiable: false
      inputSchema:
        type: object
        properties:
          param-name:
            type: string
        required: ["param-name"]
      steps:
        - id: step-id
          tool: x_tool_name
          args:
            arg: "{{ .param-name }}"
          store: result-var

metadata:
  provider: "provider-name"
  category: "category"
  icon: "🔐"
```

**Key Features**:
- Embedded workflow definitions
- Tool requirement validation
- Parameter schemas with type checking
- Multi-step operation support

### 4. Workflows (`workflows/`)

Workflows define multi-step automation sequences with input validation and result storage.

**Location**: `.envctl/workflows/*.yaml`

**Purpose**:
- Define automated sequences of tool calls
- Specify input parameters and validation
- Enable result storage and variable passing
- Support conditional logic and error handling

**Structure**:
```yaml
name: workflow-name
description: "Workflow description"
icon: "🔐"
agentModifiable: false
createdBy: "user"
version: 1

inputSchema:
  type: object
  properties:
    param-name:
      type: string
      description: "Parameter description"
      default: "default-value"
  required: ["param-name"]

steps:
  - id: step-id
    tool: api_tool_name
    args:
      arg: "{{ .param-name }}"
      static: "value"
    store: "result-var"
  
  - id: next-step
    tool: api_other_tool
    args:
      input: "{{ .result-var.output }}"
```

**Key Features**:
- Input schema validation
- Step-by-step execution
- Variable storage and templating
- Result chaining between steps

## Layered Configuration Loading

The envctl configuration system implements a **layered loading approach** where multiple configuration sources are merged with specific precedence rules.

### Loading Order (Highest to Lowest Precedence)

1. **Project Configuration** (`.envctl/` in project root)
2. **User Configuration** (`~/.envctl/` in user home)

### How It Works

1. **Base Loading**: User configuration is loaded first as the base
2. **Override Loading**: Project configuration overrides user configuration
3. **Merge Strategy**: Project values completely replace user values for the same keys
4. **File-by-File**: Each configuration file type is merged independently

### Example Scenario

**User Config** (`~/.envctl/serviceclasses/my-service.yaml`):
```yaml
name: my-service
type: service
healthCheck:
  enabled: false
  interval: "60s"
```

**Project Config** (`.envctl/serviceclasses/my-service.yaml`):
```yaml
name: my-service
type: service
healthCheck:
  enabled: true
```

**Result**: Project config completely replaces user config:
```yaml
name: my-service
type: service
healthCheck:
  enabled: true  # Project value wins
  # interval is lost - no merging within objects
```

## Template Variable System

All configuration files support template variables using Go template syntax: `{{ .variable }}`.

### Available Variables

#### ServiceClasses
- **Creation Parameters**: All parameters defined in `createParameters`
- **Service Metadata**: `{{ .serviceId }}`, `{{ .label }}`
- **Response Data**: Data from previous tool responses

#### Capabilities & Workflows
- **Input Parameters**: All parameters from the operation call
- **Step Results**: `{{ .step-id.field }}` from stored results
- **Built-in Variables**: Context and metadata variables

### Template Examples

```yaml
# String interpolation
defaultLabel: "pf-{{ .resource_type }}-{{ .resource_name }}"

# Conditional defaults
namespace: "{{ .namespace | default \"default\" }}"

# Nested access
context: "{{ .auth_result.context }}"

# Complex expressions
url: "http://{{ .host }}:{{ .port }}/{{ .path }}"
```

## Error Handling and Validation

The configuration system includes comprehensive error handling with graceful degradation.

### Error Categories

1. **Parse Errors**: Invalid YAML syntax
2. **Validation Errors**: Missing required fields or invalid values
3. **IO Errors**: File access issues

### Error Reporting

- **Detailed Messages**: File paths, line numbers, error types
- **Actionable Suggestions**: Specific guidance for fixing issues
- **Error Aggregation**: Multiple errors reported together
- **Graceful Degradation**: Application continues with valid configurations

### Example Error Output

```
ServiceClass loading completed with 2 errors (loaded 3 successfully)

Configuration Error Summary:
serviceclasses:
  project: 2 errors
    - invalid-service: Validation failed: service class type cannot be empty
    - broken-yaml: Invalid YAML format
```

## Common Configuration Patterns

### 1. Basic MCP Server Setup

```yaml
# config.yaml
mcpServers:
  - name: kubernetes
    type: localCommand
    enabledByDefault: true
    icon: "☸"
    category: "Core"
    command: ["mcp-kubernetes"]
```

### 2. ServiceClass with Health Checking

```yaml
# serviceclasses/monitoring.yaml
name: prometheus
type: monitoring
serviceConfig:
  lifecycleTools:
    start:
      tool: "x_prometheus_start"
      responseMapping:
        serviceId: "$.id"
        endpoint: "$.url"
  
  healthCheck:
    enabled: true
    interval: "30s"
    failureThreshold: 3
```

### 3. Multi-Step Workflow

```yaml
# workflows/auth-flow.yaml
name: authenticate
steps:
  - id: login
    tool: api_auth_login
    args:
      cluster: "{{ .cluster }}"
    store: "auth_result"
  
  - id: set_context
    tool: api_k8s_context
    args:
      context: "{{ .auth_result.context }}"
```

## Best Practices

### File Organization

- **Group Related Configs**: Keep related services/capabilities together
- **Use Descriptive Names**: File names should reflect their purpose
- **Version Your Configs**: Include version numbers in configuration files

### Template Usage

- **Use Defaults**: Provide sensible defaults with `{{ .var | default "value" }}`
- **Validate Inputs**: Define required parameters clearly
- **Document Variables**: Comment template variables and their sources

### Error Prevention

- **Validate YAML**: Use YAML validators before committing
- **Test Configurations**: Verify configurations work in development
- **Monitor Logs**: Check application logs for configuration warnings

### Security

- **No Secrets in Config**: Use environment variables for sensitive data
- **Validate Inputs**: Always validate user-provided parameters
- **Principle of Least Privilege**: Configure minimal required permissions

## Troubleshooting

### Common Issues

1. **YAML Syntax Errors**
   - Check indentation (use spaces, not tabs)
   - Verify quotes and special characters
   - Use online YAML validators

2. **Template Variable Errors**
   - Ensure variables are defined in input parameters
   - Check variable names and case sensitivity
   - Verify template syntax: `{{ .variable }}`

3. **Tool Availability Issues**
   - Check that required tools are installed and accessible
   - Verify tool names match exactly (case sensitive)
   - Review MCP server configurations

### Debugging Steps

1. **Check Logs**: Review application logs for detailed error messages
2. **Validate YAML**: Use external tools to validate YAML syntax
3. **Test Incrementally**: Start with minimal configurations and add complexity
4. **Use Examples**: Reference working example configurations

## Migration Guide

When updating configurations:

1. **Backup Current Config**: Always backup working configurations
2. **Update Incrementally**: Make small changes and test
3. **Validate Changes**: Use the built-in validation features
4. **Monitor Applications**: Watch for errors after deployment

This comprehensive configuration system provides flexibility, reliability, and ease of use for managing complex application setups in envctl.