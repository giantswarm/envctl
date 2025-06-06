# MCP Tool Workflows

## Overview

The MCP Tool Workflows feature in envctl allows you to define reusable sequences of MCP tool calls that are exposed as single tools through the aggregator. This feature enables:

- **Automation**: Complex multi-step operations become single tool calls
- **Reusability**: Common patterns can be defined once and reused
- **Self-improvement**: AI agents can create and modify their own workflows
- **Simplification**: Reduces errors in repetitive tasks

## Architecture

Workflows are integrated into the MCP aggregator and exposed as first-class MCP tools:

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Agent     │────▶│   Aggregator     │────▶│  MCP Servers    │
│  (MCP       │     │                  │     │  - Kubernetes   │
│   Client)   │     │  ┌─────────────┐ │     │  - Prometheus   │
└─────────────┘     │  │  Workflow   │ │     │  - Teleport     │
                    │  │  Manager     │ │     │  - CAPI         │
                    │  └─────────────┘ │     │  - Flux         │
                    └──────────────────┘     └─────────────────┘
```

## Configuration

Workflows are stored in YAML files in your envctl configuration directory:

- `~/.config/envctl/workflows.yaml` - User-defined workflows
- `~/.config/envctl/agent-workflows.yaml` - Agent-created workflows

### Workflow Definition Structure

```yaml
workflows:
  - name: connect_cluster
    description: "Connect to a cluster via Teleport and set up kubeconfig"
    icon: "🔗"  # Optional
    agentModifiable: false  # Whether agents can modify this workflow
    inputSchema:
      type: object
      properties:
        cluster:
          type: string
          description: "Name of the cluster to connect to"
      required:
        - cluster
    steps:
      - id: login
        description: "Login to Teleport cluster"
        tool: teleport_kube
        args:
          command: "login"
          cluster: "{{ .input.cluster }}"
      - id: get_context
        description: "Get current kube context"
        tool: kubectl_context
        args:
          operation: "get"
        store: "current_context"  # Store result for use in later steps
```

## Workflow Features

### Input Schema

Each workflow defines an input schema that specifies:
- **Properties**: Named inputs with types and descriptions
- **Required fields**: Which inputs must be provided
- **Default values**: Optional defaults for inputs

### Steps

Workflow steps are executed sequentially. Each step:
- Has a unique ID within the workflow
- Calls a specific MCP tool
- Can use template variables in arguments
- Can store results for later steps
- Fails fast on errors

### Template Variables

Workflow arguments support Go template syntax:
- `{{ .input.variableName }}` - Access workflow input variables
- `{{ .results.stepId }}` - Access stored results from previous steps

### Error Handling

In the current version, workflows fail fast on any error. The error is returned to the agent, which can then decide how to proceed.

## Using Workflows

### As an Agent

Workflows appear as regular MCP tools with the prefix `workflow_`:

```json
{
  "tool": "workflow_connect_cluster",
  "arguments": {
    "cluster": "mymc-mywc"
  }
}
```

### Managing Workflows

Agents can manage workflows using these tools:

- `workflow_list` - List all available workflows
- `workflow_get` - Get details of a specific workflow
- `workflow_create` - Create a new workflow
- `workflow_update` - Update an existing workflow (if agentModifiable)
- `workflow_delete` - Delete a workflow (only agent-created ones)
- `workflow_validate` - Validate a workflow definition

## Examples

### Example 1: Examine Cluster Workflow

```yaml
name: examine_cluster
description: "Set up full access to analyze a workload cluster"
agentModifiable: true
inputSchema:
  type: object
  properties:
    mc_name:
      type: string
      description: "Management cluster name"
    wc_name:
      type: string
      description: "Workload cluster name"
    prometheus_port:
      type: string
      description: "Local port for Prometheus"
      default: "9090"
  required:
    - mc_name
    - wc_name
steps:
  # Login to both clusters
  - id: login_mc
    description: "Login to management cluster"
    tool: teleport_kube
    args:
      command: "login"
      cluster: "{{ .input.mc_name }}"
      
  - id: login_wc
    description: "Login to workload cluster"
    tool: teleport_kube
    args:
      command: "login"
      cluster: "{{ .input.mc_name }}-{{ .input.wc_name }}"
      
  # Set up port forwarding for Prometheus
  - id: prometheus_forward
    description: "Create Prometheus port forward"
    tool: port_forward
    args:
      localPort: "{{ .input.prometheus_port }}"
      targetPort: "9090"
      namespace: "monitoring"
      resourceName: "prometheus-server"
      resourceType: "service"
      
  # Configure context for CAPI
  - id: set_capi_context
    description: "Set CAPI context to management cluster"
    tool: kubectl_context
    args:
      operation: "set"
      context: "{{ .input.mc_name }}"
```

### Example 2: Search Clusters Workflow

```yaml
name: search_clusters
description: "Search for available clusters matching a pattern"
agentModifiable: true
inputSchema:
  type: object
  properties:
    pattern:
      type: string
      description: "Search pattern for cluster names"
      default: "*"
steps:
  - id: list_clusters
    description: "List available Teleport clusters"
    tool: teleport_kube
    args:
      command: "ls"
    store: "cluster_list"
```

### Example 3: Deploy Application Workflow

```yaml
name: deploy_app
description: "Deploy an application using Flux"
agentModifiable: false
inputSchema:
  type: object
  properties:
    app_name:
      type: string
      description: "Name of the application"
    namespace:
      type: string
      description: "Target namespace"
    chart_version:
      type: string
      description: "Helm chart version"
  required:
    - app_name
    - namespace
steps:
  - id: create_namespace
    description: "Ensure namespace exists"
    tool: kubectl_apply
    args:
      manifest: |
        apiVersion: v1
        kind: Namespace
        metadata:
          name: {{ .input.namespace }}
          
  - id: create_helmrelease
    description: "Create Flux HelmRelease"
    tool: flux_create_helmrelease
    args:
      name: "{{ .input.app_name }}"
      namespace: "{{ .input.namespace }}"
      chart: "{{ .input.app_name }}"
      version: "{{ .input.chart_version }}"
```

## Best Practices

### 1. Keep Workflows Simple

Each workflow should have a single, clear purpose. Complex operations should be broken into multiple workflows.

### 2. Use Descriptive Names

Workflow names should clearly indicate what they do:
- ✅ `connect_and_analyze_cluster`
- ❌ `workflow1`

### 3. Document Inputs

Always provide clear descriptions for input parameters so agents understand what values to provide.

### 4. Consider Idempotency

Design workflows that can be safely run multiple times without causing issues.

### 5. Version Control

Store your workflow definitions in version control alongside your infrastructure code.

### 6. Test Workflows

Test workflows thoroughly before marking them as non-modifiable.

## Limitations

Current limitations of the workflow system:

1. **Sequential execution only** - No parallel steps or conditional logic
2. **Simple error handling** - Workflows fail fast on first error
3. **No loops or iterations** - Each step runs exactly once
4. **Limited variable interpolation** - Only basic template support

These limitations keep the system simple and predictable while still providing significant value.

## Future Enhancements

Potential future improvements:

- Parallel step execution
- Conditional steps based on previous results
- Loop constructs for repetitive operations
- More sophisticated error handling and retry logic
- Workflow composition (workflows calling other workflows)
- Version history for workflows

## Troubleshooting

### Workflow Not Appearing

If a workflow doesn't appear in the tool list:
1. Check the workflow file syntax with `workflow_validate`
2. Ensure the workflow file is in the correct directory
3. Check aggregator logs for loading errors

### Template Variable Errors

Common template issues:
- Missing `.input` prefix for input variables
- Trying to access results before they're stored
- Typos in variable names

### Permission Errors

- User workflows can only be modified manually
- Agent workflows can only be deleted if created by an agent
- Use the `agentModifiable` flag to control agent access 