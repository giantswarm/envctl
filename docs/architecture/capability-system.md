# Capability System Architecture

## Overview

The envctl capability system provides a flexible, extensible mechanism for decoupling service implementations from their interfaces. This allows envctl to work with different authentication providers, port forwarding implementations, and discovery mechanisms without hardcoded dependencies.

## Key Concepts

### Capabilities

A **capability** represents a specific type of functionality that can be provided by MCP servers. Examples include:
- `auth_provider` - Authentication functionality
- `portforward_provider` - Port forwarding functionality
- `discovery_provider` - Service/resource discovery functionality

### Capability Definitions

Capabilities are defined in YAML files that specify:
- The capability type and metadata
- Available operations (e.g., login, logout, status)
- Required tools from MCP servers
- Embedded workflows that implement each operation

### Automatic Detection

Capabilities are automatically exposed when the required tools exist on connected MCP servers. This means:
- No explicit capability registration is needed
- MCP servers just provide tools without capability awareness
- envctl dynamically enables capabilities based on tool availability

## Architecture Components

### 1. Capability Loader

The `CapabilityLoader` (`internal/capability/loader.go`) handles:
- Loading capability definitions from YAML files
- Validating capability definitions
- Checking tool availability
- Tracking which capabilities can be exposed

### 2. Capability Adapter

The `CapabilityAdapter` (`internal/capability/api_adapter.go`) implements the API handler interface:
- Converts capability operations to workflow executions
- Exposes capability tools (e.g., `x_auth_login`)
- Provides management tools (capability_list, capability_info, capability_check)

### 3. Capability Registry

The `Registry` (`internal/capability/registry.go`) manages:
- Storage of capability metadata
- Capability state tracking
- Observer pattern for capability changes

### 4. Tool Availability Checker

The aggregator provides tool availability checking:
- Reports which tools are available from MCP servers
- Monitors for tool availability changes
- Triggers capability availability updates

## Capability Definition Format

```yaml
# Example: teleport_auth.yaml
name: teleport_auth
type: auth_provider
version: "1.0.0"
description: "Teleport authentication provider for cluster access"

operations:
  login:
    description: "Authenticate to a cluster using Teleport"
    parameters:
      cluster:
        type: string
        required: true
        description: "Target cluster name"
      user:
        type: string
        required: false
        description: "Username (optional)"
    requires:
      - x_teleport_kube      # Required tools from MCP servers
      - x_teleport_status
    workflow:                # Embedded workflow definition
      name: teleport_auth_login
      description: "Authenticate using Teleport"
      agentModifiable: false
      inputSchema:
        type: object
        properties:
          cluster:
            type: string
            description: "Target cluster name"
          user:
            type: string
            description: "Username (optional)"
        required:
          - cluster
      steps:
        - id: check_status
          tool: x_teleport_status
          args: {}
          store: current_status
          
        - id: perform_login
          tool: x_teleport_kube
          args:
            command: "login"
            cluster: "{{ .cluster }}"
            userParam: "{{ .user }}"
          store: login_result
```

## API Pattern

The capability system follows envctl's API pattern:

1. **Handler Interfaces** defined in `internal/api/handlers.go`
2. **Adapters** implement interfaces and call `Register()`
3. **API layer** accesses handlers via `api.GetXXX()`
4. **No direct coupling** between packages

Example usage:
```go
// Service using capability through API
handler := api.GetCapability()
if handler == nil {
    return errors.New("capability API not available")
}

result, err := handler.ExecuteCapability(ctx, "auth_provider", "login", map[string]interface{}{
    "cluster": "production",
})
```

## Workflow Integration

Capabilities leverage the existing workflow system:
- Operations are implemented as workflows
- Workflows orchestrate calls to MCP server tools
- Complex multi-step operations are supported
- Parameter templating enables dynamic arguments

## Tool Naming Convention

Capability tools follow a consistent naming pattern:
- Format: `x_<type>_<operation>`
- Examples:
  - `x_auth_provider_login`
  - `x_auth_provider_logout`
  - `x_portforward_provider_create`

Management tools:
- `capability_list` - List all available capabilities
- `capability_info` - Get information about a specific capability
- `capability_check` - Check if a capability operation is available

## Creating New Capabilities

To add a new capability:

1. **Create a YAML definition file** in `internal/capability/definitions/`
2. **Define the capability metadata** (name, type, version, description)
3. **Specify operations** with:
   - Description and parameters
   - Required tools list
   - Embedded workflow implementation
4. **Ensure MCP servers provide the required tools**

The capability will be automatically detected and exposed when its required tools are available.

## Testing Capabilities

The capability system includes comprehensive tests:

1. **Unit Tests** (`internal/capability/*_test.go`):
   - YAML parsing and validation
   - Tool availability checking
   - Operation execution
   - API adapter functionality

2. **Integration Tests** (`internal/capability/integration_test.go`):
   - End-to-end capability execution
   - Workflow integration
   - Tool aggregation

## Benefits

1. **Decoupling**: Services don't depend on specific implementations
2. **Extensibility**: New providers can be added without code changes
3. **Dynamic Configuration**: Capabilities adapt to available tools
4. **Vendor Independence**: Users can bring their own MCP servers
5. **Backward Compatibility**: Existing configurations continue to work

## Migration Path

For existing envctl users:
1. Existing hardcoded authentication continues to work
2. Capability-based auth is available alongside
3. Gradual migration as new MCP servers are added
4. No breaking changes to current workflows

## Future Enhancements

- Additional capability types (secret management, monitoring)
- Capability composition (combining multiple capabilities)
- Capability versioning and compatibility checks
- Runtime capability hot-reloading
- Capability marketplace for community contributions 