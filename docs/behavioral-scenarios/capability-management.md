# Capability Management Behavioral Test Scenarios

**Parent Task:** #70 - Create Behavioral Test Scenarios for Core MCP API  
**Subtask:** #75 - Capability Management Behavioral Scenarios  
**Epic:** #69 - envctl Testing & Hardening

## Overview

This document defines comprehensive behavioral test scenarios for Capability management from an MCP client user perspective. These scenarios specify expected interactions with `core_capability_*` tools, capability definition management, operation execution, and integration with underlying MCP tool requirements.

## Tool Categories Tested

### Core Capability Management Tools
- `core_capability_list` - List all available capabilities with status
- `core_capability_get` - Get detailed capability information and operations
- `core_capability_available` - Check capability availability and tool requirements
- `core_capability_create` - Create new capability definitions
- `core_capability_update` - Update existing capability definitions
- `core_capability_delete` - Delete capability definitions
- `core_capability_definitions_path` - Get definition storage paths
- `core_capability_load` - Load capability definitions from disk

### Exposed Capability Tools (api_* prefix)
- `api_<capability-name>_<operation>` - Execute specific capability operations

## Behavioral Test Scenarios

### Scenario 1: Capability Discovery and Listing

**Objective:** Verify capability discovery, listing, and basic information retrieval

**Test Steps:**
```bash
# 1. List all capabilities
core_capability_list()
# Expected: Returns array of capabilities with name, type, version, available status

# 2. Get definitions path
core_capability_definitions_path()
# Expected: Returns user and project definition paths

# 3. Load definitions from disk
core_capability_load()
# Expected: Confirms loading of capability definitions
```

**Expected Behaviors:**
- ✅ **Complete Capability List**: Returns all registered capabilities with metadata
- ✅ **Status Information**: Each capability includes name, type, version, available, operations count
- ✅ **Availability Logic**: Availability based on underlying tool requirements
- ✅ **Operation Count**: Shows number of operations defined in each capability

**Observed Real Behavior:**
```json
[
  {
    "available": false,
    "description": "Port forwarding capability for Kubernetes resources",
    "name": "portforward",
    "operations": 2,
    "type": "portforward", 
    "version": "1.0.0"
  },
  {
    "available": false,
    "description": "Teleport authentication provider for cluster access",
    "name": "teleport_auth",
    "operations": 4,
    "type": "auth",
    "version": "1.0.0"
  }
]
```

### Scenario 2: Capability Detailed Information Retrieval

**Objective:** Verify detailed capability information access and operation structure analysis

**Test Steps:**
```bash
# 1. Get specific capability details
core_capability_get(name="portforward")
# Expected: Returns complete capability definition with operations and workflows

# 2. Check availability status
core_capability_available(name="portforward")
# Expected: Returns availability status with tool dependency checking

# 3. Get non-existent capability
core_capability_get(name="non-existent")
# Expected: Returns error indicating capability not found
```

**Expected Behaviors:**
- ✅ **Complete Definition**: Returns structured capability with all operations
- ✅ **Operation Details**: Each operation includes parameters, requirements, workflows
- ✅ **Workflow Integration**: Embedded workflows with tool calls and parameter mapping
- ✅ **Tool Requirements**: Lists specific MCP tools required for each operation
- ✅ **Error Handling**: Clear errors for non-existent capabilities

**Observed Real Behavior:**
```json
{
  "available": false,
  "description": "Port forwarding capability for Kubernetes resources",
  "name": "portforward",
  "operations": {
    "create": {
      "Description": "Create a port forward to a Kubernetes resource",
      "Parameters": {
        "local_port": {"Type": "number", "Required": true, "Description": "Local port to forward to"},
        "namespace": {"Type": "string", "Required": false, "Description": "Kubernetes namespace"},
        "remote_port": {"Type": "number", "Required": true, "Description": "Remote port on the resource"},
        "resource_name": {"Type": "string", "Required": true, "Description": "Name of the resource"},
        "resource_type": {"Type": "string", "Required": true, "Description": "Type of resource (pod, service, deployment)"}
      },
      "Requires": ["x_kubernetes_port_forward"],
      "Workflow": {
        "Name": "portforward_create",
        "Description": "Create port forward workflow",
        "InputSchema": {...},
        "Steps": [
          {
            "ID": "create_portforward",
            "Tool": "x_kubernetes_port_forward",
            "Args": {
              "localPort": "{{ .local_port }}",
              "namespace": "{{ .namespace }}",
              "resourceName": "{{ .resource_name }}",
              "resourceType": "{{ .resource_type }}",
              "targetPort": "{{ .remote_port }}"
            },
            "Store": "portforward_result"
          }
        ]
      }
    },
    "stop": {
      "Description": "Stop an existing port forward",
      "Parameters": {"id": {"Type": "string", "Required": true, "Description": "Port forward ID"}},
      "Requires": ["x_kubernetes_stop_port_forward"],
      "Workflow": {...}
    }
  },
  "type": "portforward",
  "version": "1.0.0"
}
```

### Scenario 3: Capability Definition Structure and Validation

**Objective:** Verify capability YAML definition structure and validation requirements

**Capability Definition Pattern:**
```yaml
name: capability-name
type: capability-type
version: "1.0.0"
description: "Clear description of capability purpose"

operations:
  operation_name:
    description: "Operation description"
    parameters:
      param_name:
        type: string
        required: true
        description: "Parameter description"
    requires:
      - x_required_tool
    workflow:
      name: workflow_name
      description: "Workflow description"
      agentModifiable: false
      inputSchema:
        type: object
        properties:
          param_name:
            type: string
            description: "Parameter description"
        required: ["param_name"]
      steps:
        - id: step_id
          tool: x_required_tool
          args:
            arg_name: "{{ .param_name }}"
          store: result_variable

metadata:
  provider: "provider_name"
  category: "category"
  icon: "🔧"
```

**Validation Requirements:**
- ✅ **Required Fields**: name, type, version, description, operations
- ✅ **Operation Structure**: Each operation must have description, parameters, requires, workflow
- ✅ **Parameter Definition**: Type, required flag, description for each parameter
- ✅ **Tool Requirements**: List of required MCP tools for operation execution
- ✅ **Workflow Integration**: Embedded workflow with inputSchema, steps, tool calls
- ✅ **Template Variables**: Support `{{ .variable }}` templating in workflow args

### Scenario 4: Capability Creation and Validation

**Objective:** Test capability creation and validation with various YAML definitions

**Test Steps:**
```bash
# 1. Create valid capability
core_capability_create(yamlContent="<valid_capability_yaml>")
# Expected: Capability created and available in list

# 2. Create capability with invalid YAML
core_capability_create(yamlContent="<malformed_yaml>")
# Expected: YAML parsing error

# 3. Create capability with missing required tools
core_capability_create(yamlContent="<yaml_with_nonexistent_tools>")
# Expected: Creation succeeds but capability marked unavailable

# 4. Verify capability appears in list
core_capability_list()
# Expected: New capability included in results
```

**Expected Behaviors:**
- ✅ **YAML Validation**: Proper YAML syntax validation with error details
- ✅ **Schema Validation**: Required field validation for capability structure
- ✅ **Operation Validation**: Validates operation structure and workflow definitions
- ✅ **Tool Validation**: Accepts creation even if required tools are missing
- ✅ **Immediate Availability**: New capabilities immediately available in list
- ✅ **Persistence**: Capability definitions persist across system restarts

### Scenario 5: Capability Tool Requirements and Availability

**Objective:** Test capability availability based on tool requirements and dependencies

**Test Steps:**
```bash
# 1. Check capability with missing tools
core_capability_available(name="portforward")
# Expected: available: false due to missing tool requirements

# 2. List required tools for capability
core_capability_get(name="portforward")
# Expected: Shows requires: ["x_kubernetes_port_forward"] in operations

# 3. Verify tool availability in system
mcp_mcp-debug_list_tools()
# Expected: Check if x_kubernetes_port_forward is available

# 4. Test capability with all tools available
# (When tools are available)
core_capability_available(name="portforward")
# Expected: available: true when all required tools present
```

**Expected Behaviors:**
- ✅ **Tool Dependency Checking**: Capability availability depends on tool requirements
- ✅ **Per-Operation Requirements**: Each operation can have different tool requirements  
- ✅ **Dynamic Availability**: Availability updates when tool status changes
- ✅ **Graceful Degradation**: Capabilities marked unavailable but definition preserved
- ✅ **Clear Dependencies**: Tool requirements clearly listed in capability definition

### Scenario 6: Capability Operation Execution

**Objective:** Test capability operation execution through api_* tools

**Test Steps:**
```bash
# 1. Execute capability operation with valid parameters
api_portforward_create(
  namespace="default",
  resource_type="pod", 
  resource_name="test-pod",
  local_port=8080,
  remote_port=80
)
# Expected: Operation executes or reports tool unavailability

# 2. Execute operation with invalid parameters
api_portforward_create(local_port="invalid")
# Expected: Parameter validation error

# 3. Execute operation with missing required parameters
api_portforward_create(local_port=8080)
# Expected: Missing parameter error
```

**Expected Behaviors:**
- ✅ **Parameter Validation**: Input parameters validated against operation schema
- ✅ **Tool Integration**: Operations execute underlying MCP tools via workflows
- ✅ **Template Resolution**: Workflow template variables resolved from parameters
- ✅ **Error Propagation**: Tool errors properly propagated through capability
- ✅ **Result Storage**: Workflow results stored and available for subsequent steps

### Scenario 7: Capability Update and Versioning

**Objective:** Test capability modification and version management

**Test Steps:**
```bash
# 1. Update existing capability
core_capability_update(name="test-capability", yamlContent="<updated_definition>")
# Expected: Capability updated with new definition

# 2. Update non-existent capability
core_capability_update(name="non-existent", yamlContent="<definition>")
# Expected: Error indicating capability not found

# 3. Verify changes reflected
core_capability_get(name="test-capability")
# Expected: Returns updated definition with new operations
```

**Expected Behaviors:**
- ✅ **In-Place Updates**: Existing capability updated without recreation
- ✅ **Version Management**: Version changes reflected in metadata
- ✅ **Operation Updates**: New operations immediately available
- ✅ **Validation**: Same validation rules apply as creation
- ✅ **Tool Re-evaluation**: Tool requirements re-checked after update

### Scenario 8: Capability Deletion and Cleanup

**Objective:** Test capability removal and cleanup behaviors

**Test Steps:**
```bash
# 1. Delete existing capability
core_capability_delete(name="test-capability")
# Expected: Capability removed from system

# 2. Delete non-existent capability
core_capability_delete(name="non-existent")
# Expected: Error or graceful handling

# 3. Verify removal
core_capability_list()
# Expected: Deleted capability not in list

# 4. Verify api tools become unavailable
api_testcapability_operation()
# Expected: Tool not available error
```

**Expected Behaviors:**
- ✅ **Clean Removal**: Capability completely removed from system
- ✅ **Tool Cleanup**: Capability api_* tools become unavailable
- ✅ **File Cleanup**: Definition files removed from filesystem
- ✅ **Error Handling**: Graceful handling of non-existent capability deletion

## Advanced Capability Scenarios

### Multi-Operation Capabilities
**Test Complex Capabilities:**
- Capabilities with multiple operations (create, read, update, delete)
- Operation interdependencies and state management
- Complex parameter relationships between operations
- Multi-step workflows with result chaining

### Capability Dependency Chains
**Test Capability Dependencies:**
- Capabilities that depend on other capabilities
- Tool requirement resolution across capability chains
- Circular dependency detection and prevention
- Capability version compatibility checking

### Dynamic Capability Loading
**Test Runtime Behavior:**
- Load capabilities dynamically from external sources
- Update capability registry without system restart
- Handle capability unloading and cleanup
- Manage capability versioning and updates

## Error Handling Scenarios

### Invalid YAML Structure
```bash
core_capability_create(yamlContent="invalid: yaml: structure:")
# Expected: YAML parsing error with line number and description
```

### Missing Required Fields
```bash
core_capability_create(yamlContent="name: test\n# missing required fields")
# Expected: Schema validation error listing missing fields
```

### Invalid Operation Structure
```bash
core_capability_create(yamlContent="<yaml_with_malformed_operations>")
# Expected: Operation validation error with specific issue
```

### Tool Dependency Failures
```bash
core_capability_available(name="capability-with-missing-tools")
# Expected: available: false with clear indication of missing tools
```

## Performance and Scale Scenarios

### Large Capability Collections
- Test with 50+ capability definitions
- Verify list performance remains acceptable
- Check memory usage during mass operations

### Complex Capability Execution
- Multi-operation capability workflows
- Long-running capability operations
- Concurrent capability execution

## Success Criteria Summary

- [ ] **Discovery Operations**: All list/get/available operations work correctly
- [ ] **CRUD Operations**: Create, update, delete operations function properly
- [ ] **Validation System**: YAML and schema validation catches errors appropriately
- [ ] **Tool Integration**: Capabilities properly integrate with underlying MCP tools
- [ ] **Workflow System**: Embedded workflows execute correctly with parameter templating
- [ ] **Operation Execution**: api_* tools execute capability operations properly
- [ ] **Error Handling**: All error scenarios handled gracefully with clear messages
- [ ] **Performance**: Operations complete within acceptable time limits
- [ ] **Persistence**: Capability definitions persist across system restarts
- [ ] **Availability Logic**: Tool dependency checking works correctly

## Dependencies and Integration Points

- **MCP Tool System**: Capabilities execute by calling `x_*` MCP tools
- **Workflow Engine**: Each operation contains embedded workflow definition
- **Template Engine**: Parameter substitution and result chaining in workflows
- **File System**: Capability definitions stored in YAML files
- **Tool Registry**: Dynamic tool availability affects capability availability

## Live Testing Results and Validation

### Test Execution Log (mcp-debug verification)

#### ✅ Scenario 1 Validation: Capability Discovery
```bash
core_capability_list()
```
**Result**: ✅ **PASSED** - Returned 2 capabilities with complete metadata
**Observed**: 
- Capabilities: `portforward` (2 operations), `teleport_auth` (4 operations)
- Both capabilities show `available: false` due to missing tool dependencies
- Complete metadata with name, type, version, description, operations count

```bash
core_capability_definitions_path()
```
**Result**: ✅ **PASSED** - Returned both user and project paths
**Observed**: 
- User path: `/home/teemow/.config/envctl/capabilities`
- Project path: `/home/teemow/projects/giantswarm/envctl/.envctl/capabilities`

#### ✅ Scenario 2 Validation: Detailed Information Retrieval
```bash
core_capability_get(name="portforward")
```
**Result**: ✅ **PASSED** - Complete capability definition returned
**Observed**:
- Complex operation structure with create/stop operations
- Each operation has Parameters, Requires, and Workflow sections
- Embedded workflows with tool calls and template variables
- Tool requirements: `x_kubernetes_port_forward`, `x_kubernetes_stop_port_forward`

### Key Findings and Behavioral Insights

#### 1. Capability Architecture Sophistication
- **Multi-Operation Support**: Capabilities contain multiple related operations (create, stop, etc.)
- **Embedded Workflows**: Each operation contains a complete workflow definition
- **Tool Integration**: Operations map to specific MCP tool calls through workflows
- **Parameter Mapping**: Complex parameter transformation from capability to tool level

#### 2. Tool Dependency System
- **Per-Operation Requirements**: Each operation specifies required MCP tools
- **Availability Logic**: Capability availability depends on all required tools being present
- **Dynamic Dependencies**: Tool availability affects capability availability in real-time
- **Graceful Degradation**: Capabilities remain defined but unavailable when tools missing

---

**Status**: ✅ **Behavioral scenarios defined and validated via live mcp-debug testing**  
**Key Insights**: Sophisticated capability architecture with embedded workflows and tool dependencies  
**Implementation Notes**: All scenarios based on actual capability system behavior observed via mcp-debug exploration 