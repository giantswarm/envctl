# Flexible Configuration via YAML ⚙️

`envctl` offers a powerful and flexible configuration system using YAML files, allowing you to tailor its behavior, define custom MCP servers, and manage port-forwarding rules beyond the built-in defaults.

## Configuration Layers

Settings are loaded in a layered approach, with later layers overriding or extending earlier ones:

1.  **Default Configuration:** Built-in defaults for common services like Kubernetes, Prometheus, and Grafana. These are defined internally within `envctl`.
2.  **User-Specific Global Configuration:** Define your personal overrides and custom services in `~/.config/envctl/config.yaml` (or `$XDG_CONFIG_HOME/envctl/config.yaml` on Linux). This allows you to set preferences that apply every time you use `envctl`.
3.  **Project-Specific Configuration:** For settings specific to a particular project or repository, create a `.envctl/config.yaml` file in the root of your project directory. These settings will apply when `envctl` is run from within that project, overriding user and default configurations where specified.

**Merge Strategy:**

*   **`globalSettings`**: Settings from a later layer (e.g., project config) will completely override those from an earlier layer (e.g., user config or default). For example, if the default `defaultContainerRuntime` is "docker" and your user config specifies "podman", "podman" will be used. If a project config then specifies "cri-o", "cri-o" will be used for that project.
*   **`clusters`, `mcpServers` & `portForwards`**: These are lists of definitions. The merge logic is based on the `name` field of each server, port-forward, or cluster definition:
    *   If an item in a later layer has the same `name` as an item in an earlier layer, the item from the later layer completely replaces the one from the earlier layer.
    *   New items with unique names found in later layers are appended to the list.
    This allows you to override default services (like "prometheus" or "grafana") or add entirely new custom services.
*   **`activeClusters`**: The active cluster mapping from later layers extends/overrides earlier layers. You can change which cluster is active for each role.

## Configuration File Structure (`config.yaml`)

The YAML file uses the following main sections:

```yaml
globalSettings:
  defaultContainerRuntime: "docker" # Or "podman", etc.

# Define available clusters and their roles
clusters:
  - name: "mc-prod"
    context: "teleport.giantswarm.io-prod"
    role: "observability" # or "target" or "custom"
    displayName: "Production MC"
    icon: "🏢"
  
  - name: "wc-app1"
    context: "teleport.giantswarm.io-prod-app1"
    role: "target"
    displayName: "App1 Workload"
    icon: "🚀"

# Specify which cluster is active for each role
activeClusters:
  observability: "mc-prod"
  target: "wc-app1"

mcpServers: # List of MCP Server definitions
  - name: "unique-server-name"
    type: "localCommand" # or "container"
    enabledByDefault: true # or false
    icon: "✨" # Optional: Emoji or single character for TUI
    category: "My Services" # Optional: For grouping in TUI
    # Fields for type: "localCommand"
    command: ["npx", "my-mcp-server", "--some-arg"]
    env:
      MY_ENV_VAR: "value"
      ANOTHER_VAR: "another_value"
    # Fields for type: "container"
    image: "myregistry/my-mcp-image:latest"
    containerPorts: ["8080:80", "9000:9000/udp"]
    containerEnv:
      CONTAINER_SPECIFIC_VAR: "foo"
    containerVolumes: ["~/.kube/config:/root/.kube/config:ro"]
    healthCheckCmd: ["curl", "--fail", "http://localhost:80/health"]
    entrypoint: ["/custom/entrypoint.sh"]
    containerUser: "1000:1000"
    # Cluster dependencies
    requiresClusterRole: "target" # Service depends on the active target cluster
    requiresClusterName: "specific-cluster" # Or depend on a specific cluster by name
    # Port forward dependencies
    requiresPortForwards: ["name-of-required-portforward-1", "name-of-required-portforward-2"]

portForwards: # List of Port Forward definitions
  - name: "unique-portforward-name"
    enabledByDefault: true # or false
    icon: "🔗" # Optional: Emoji or single character for TUI
    category: "Databases" # Optional: For grouping in TUI
    # Cluster targeting (new flexible system)
    clusterRole: "observability" # Use the active cluster for this role
    clusterName: "specific-cluster" # Or target a specific cluster by name
    # Legacy field (deprecated but still supported)
    kubeContextTarget: "mc" # DEPRECATED: Use clusterRole or clusterName instead
    namespace: "my-namespace"
    targetType: "service" # "service", "pod", "deployment", "statefulset"
    targetName: "my-target-resource-name"
    targetLabelSelector: "app=myapp,role=db" # Optional, used if targetName is not specific enough
    localPort: "5432"
    remotePort: "5432"
    bindAddress: "127.0.0.1" # Optional, defaults to "127.0.0.1"
```

### `globalSettings`

*   `defaultContainerRuntime` (string, optional): Specify your preferred container runtime for MCP servers of type `container`. Supported values might include "docker", "podman", etc., depending on your system setup. Defaults to "docker".

### `clusters`

This section defines the available Kubernetes clusters that `envctl` can connect to. The flexible cluster system allows you to:
- Define clusters with specific roles (observability, target, custom)
- Switch between different clusters for the same role at runtime
- Create complex configurations where different services connect to different clusters

*   `name` (string, required): A unique identifier for this cluster (e.g., "mc-prod", "wc-staging").
*   `context` (string, required): The Kubernetes context name for this cluster (e.g., "teleport.giantswarm.io-prod").
*   `role` (string, required): The role this cluster can fulfill. Valid values:
    *   `"observability"`: Typically management clusters that host monitoring infrastructure
    *   `"target"`: Clusters being monitored or managed (workload clusters)
    *   `"custom"`: For special-purpose clusters
*   `displayName` (string, optional): Human-friendly name shown in the TUI.
*   `icon` (string, optional): Emoji or character for visual identification in the TUI.

### `activeClusters`

This mapping specifies which cluster is currently active for each role. This can be changed at runtime through the TUI.

Example:
```yaml
activeClusters:
  observability: "mc-prod"
  target: "wc-app1"
```

### `mcpServers`

This is a list of MCP (Model Context Protocol) server definitions. `envctl` can manage these servers, typically by running them via `mcp-proxy` for local commands or directly for containers.

*   `name` (string, required): A unique name for this server definition (e.g., "kubernetes", "prometheus-main", "my-custom-api"). This name is used for merging configurations and for internal references (like dependencies).
*   `type` (string, required): Specifies how the MCP server is run. Must be one of:
    *   `"localCommand"`: The server is run as a command on your local machine (often via `mcp-proxy`).
    *   `"container"`: The server is run as a container using the specified image.
*   `enabledByDefault` (boolean, optional): If `true`, this server will be started by default when `envctl connect` is run. Defaults to `false` if not specified, unless it's a core default server like "kubernetes".
*   `icon` (string, optional): An emoji or single character to represent this server in the TUI.
*   `category` (string, optional): A category name for grouping this server with others in the TUI (e.g., "Core", "Monitoring", "Development").
*   **Fields for `type: "localCommand"`:**
    *   `command` (list of strings, required for `localCommand`): The command and its arguments to execute (e.g., `["npx", "mcp-server-kubernetes"]`, `["uvx", "mcp-server-prometheus"]`).
    *   `env` (map of string to string, optional): Environment variables to set for the command.
*   **Fields for `type: "container"`:**
    *   `image` (string, required for `container`): The container image to pull and run (e.g., `"giantswarm/mcp-server-prometheus:latest"`).
    *   `containerPorts` (list of strings, optional): Port mappings from host to container, in the format `"HOST_PORT:CONTAINER_PORT"` or `"HOST_PORT:CONTAINER_PORT/PROTOCOL"` (e.g., `["8080:8080", "9090:9000/tcp"]`).
    *   `containerEnv` (map of string to string, optional): Environment variables to set inside the container.
    *   `containerVolumes` (list of strings, optional): Volume mounts in the format `"HOST_PATH:CONTAINER_PATH"` or `"HOST_PATH:CONTAINER_PATH:MODE"` (e.g., `["~/.kube/config:/root/.kube/config:ro"]`).
    *   `healthCheckCmd` (list of strings, optional): A command to run inside the container to check its health. `envctl` does not yet use this but it's planned for future enhancements.
    *   `entrypoint` (list of strings, optional): Override the default container entrypoint.
    *   `containerUser` (string, optional): User/group to run the container as (e.g., `"1000"` or `"1000:1000"`).
*   **Cluster Dependencies:**
    *   `requiresClusterRole` (string, optional): The cluster role that this MCP server requires. The server will connect to whichever cluster is active for this role.
    *   `requiresClusterName` (string, optional): A specific cluster name that this MCP server requires. Takes precedence over `requiresClusterRole`.
*   **Port Forward Dependencies:**
    *   `requiresPortForwards` (list of strings, optional): A list of `name`s of `PortForwardDefinition`s that this MCP server requires to be active before it starts (e.g., a Prometheus server might require a port-forward to the cluster's Prometheus instance).

### `portForwards`

This is a list of Kubernetes port-forwarding definitions.

*   `name` (string, required): A unique name for this port-forward definition (e.g., "mc-prometheus", "wc-alloy-debug"). Used for merging and references.
*   `enabledByDefault` (boolean, optional): If `true`, this port-forward will be established by default. Defaults to `false` if not specified, unless it's a core default.
*   `icon` (string, optional): An emoji or single character for the TUI.
*   `category` (string, optional): A category name for grouping in the TUI.
*   **Cluster Targeting (use one of these):**
    *   `clusterRole` (string, recommended): Target the active cluster for this role ("observability", "target", "custom").
    *   `clusterName` (string, optional): Target a specific cluster by its name.
    *   `kubeContextTarget` (string, deprecated): Legacy field. Can be:
        *   `"mc"`: Dynamically resolves to the context of the Management Cluster.
        *   `"wc"`: Dynamically resolves to the context of the Workload Cluster.
        *   An explicit Kubernetes context name.
*   `namespace` (string, required): The Kubernetes namespace where the target resource resides.
*   `targetType` (string, required): The type of Kubernetes resource to port-forward to. Common values: `"service"`, `"pod"`, `"deployment"`, `"statefulset"`.
*   `targetName` (string, required): The name of the target Kubernetes resource (e.g., `"my-service"`, `"my-pod-abcde"`).
*   `targetLabelSelector` (string, optional): A Kubernetes label selector (e.g., `"app=myapp,component=server"`). If `targetName` is for a resource like a Deployment or StatefulSet that manages multiple pods, or if you want to target a pod by labels instead of name, use this. `envctl` will attempt to find a suitable pod matching the selector.
*   `localPort` (string, required): The port on your local machine to bind to.
*   `remotePort` (string, required): The port on the target resource in the cluster.
*   `bindAddress` (string, optional): The local IP address to bind to. Defaults to `"127.0.0.1"`. Use `"0.0.0.0"` to bind to all available network interfaces.

## Example Configuration Files

Below is a more comprehensive example showcasing various features:

### Basic Giant Swarm Setup (Default Behavior)

When you run `envctl connect myinstallation myworkloadcluster`, envctl automatically creates this cluster configuration:

```yaml
# This is automatically generated from the MC/WC names
clusters:
  - name: "k8s-myinstallation"
    context: "teleport.giantswarm.io-myinstallation"
    role: "observability"
    displayName: "myinstallation"
    icon: "🏢"
  
  - name: "k8s-myinstallation-myworkloadcluster"
    context: "teleport.giantswarm.io-myinstallation-myworkloadcluster"
    role: "target"
    displayName: "myinstallation-myworkloadcluster"
    icon: "🎯"

activeClusters:
  observability: "k8s-myinstallation"
  target: "k8s-myinstallation-myworkloadcluster"
```

### Multi-Environment Setup

Configure multiple environments with easy switching between them:

```yaml
globalSettings:
  defaultContainerRuntime: "docker"

# Define all your clusters
clusters:
  # Production clusters
  - name: "mc-prod"
    context: "teleport.giantswarm.io-prod"
    role: "observability"
    displayName: "Production MC"
    icon: "🏢"
  
  - name: "wc-prod-api"
    context: "teleport.giantswarm.io-prod-api"
    role: "target"
    displayName: "Production API"
    icon: "🚀"
  
  - name: "wc-prod-web"
    context: "teleport.giantswarm.io-prod-web"
    role: "target"
    displayName: "Production Web"
    icon: "🌐"
  
  # Staging clusters
  - name: "mc-staging"
    context: "teleport.giantswarm.io-staging"
    role: "observability"
    displayName: "Staging MC"
    icon: "🧪"
  
  - name: "wc-staging-all"
    context: "teleport.giantswarm.io-staging-all"
    role: "target"
    displayName: "Staging Apps"
    icon: "🔧"

# Default to production
activeClusters:
  observability: "mc-prod"
  target: "wc-prod-api"

# Port forwards use cluster roles
portForwards:
  - name: "prometheus"
    enabledByDefault: true
    clusterRole: "observability"  # Always from the observability cluster
    namespace: "mimir"
    targetType: "service"
    targetName: "mimir-query-frontend"
    localPort: "8080"
    remotePort: "8080"
  
  - name: "app-metrics"
    enabledByDefault: true
    clusterRole: "target"  # From whichever workload cluster is active
    namespace: "default"
    targetType: "service"
    targetName: "app-metrics-service"
    localPort: "9090"
    remotePort: "9090"

# MCP servers adapt to active clusters
mcpServers:
  - name: "kubernetes"
    type: "localCommand"
    enabledByDefault: true
    command: ["mcp-server-kubernetes"]
    requiresClusterRole: "target"  # Connects to the active target cluster
  
  - name: "prometheus"
    type: "localCommand"
    enabledByDefault: true
    command: ["mcp-server-prometheus"]
    env:
      PROMETHEUS_URL: "http://localhost:8080"
    requiresPortForwards: ["prometheus"]
```

### Mixed Cluster Types

Run services across different cluster types (Giant Swarm, GKE, EKS, etc.):

```yaml
clusters:
  # Giant Swarm clusters
  - name: "gs-mc"
    context: "teleport.giantswarm.io-prod"
    role: "observability"
    displayName: "Giant Swarm MC"
    icon: "🏢"
  
  - name: "gs-wc"
    context: "teleport.giantswarm.io-prod-main"
    role: "target"
    displayName: "Giant Swarm WC"
    icon: "🎯"
  
  # Google Cloud clusters
  - name: "gke-monitoring"
    context: "gke_my-project_us-central1_monitoring"
    role: "observability"
    displayName: "GKE Monitoring"
    icon: "☁️"
  
  - name: "gke-app"
    context: "gke_my-project_us-central1_app"
    role: "target"
    displayName: "GKE Application"
    icon: "🚀"
  
  # AWS EKS cluster
  - name: "eks-data"
    context: "arn:aws:eks:us-east-1:123456789:cluster/data-platform"
    role: "custom"
    displayName: "EKS Data Platform"
    icon: "📊"

# Mix and match clusters
activeClusters:
  observability: "gke-monitoring"
  target: "gs-wc"
  custom: "eks-data"

portForwards:
  # From GKE monitoring cluster
  - name: "grafana"
    clusterRole: "observability"
    namespace: "monitoring"
    targetType: "service"
    targetName: "grafana"
    localPort: "3000"
    remotePort: "3000"
  
  # From Giant Swarm workload cluster
  - name: "app-api"
    clusterRole: "target"
    namespace: "production"
    targetType: "service"
    targetName: "api-gateway"
    localPort: "8000"
    remotePort: "80"
  
  # From EKS data platform
  - name: "jupyter"
    clusterRole: "custom"
    namespace: "notebooks"
    targetType: "service"
    targetName: "jupyter-hub"
    localPort: "8888"
    remotePort: "8888"

mcpServers:
  # Kubernetes MCP connects to Giant Swarm target
  - name: "kubernetes"
    type: "localCommand"
    command: ["mcp-server-kubernetes"]
    requiresClusterRole: "target"
  
  # Custom data platform MCP
  - name: "data-platform"
    type: "container"
    image: "myregistry/mcp-data-platform:latest"
    requiresClusterName: "eks-data"  # Specifically requires the EKS cluster
    requiresPortForwards: ["jupyter"]
```

### Service-Specific Cluster Dependencies

Different services can depend on different clusters:

```yaml
clusters:
  - name: "monitoring-cluster"
    context: "monitoring.example.com"
    role: "observability"
    displayName: "Central Monitoring"
    icon: "📊"
  
  - name: "app-cluster-1"
    context: "app1.example.com"
    role: "target"
    displayName: "App Cluster 1"
    icon: "1️⃣"
  
  - name: "app-cluster-2"
    context: "app2.example.com"
    role: "target"
    displayName: "App Cluster 2"
    icon: "2️⃣"
  
  - name: "logging-cluster"
    context: "logging.example.com"
    role: "custom"
    displayName: "Central Logging"
    icon: "📝"

activeClusters:
  observability: "monitoring-cluster"
  target: "app-cluster-1"
  custom: "logging-cluster"

mcpServers:
  # Different MCP servers connect to different clusters
  - name: "kubernetes-app1"
    type: "localCommand"
    command: ["mcp-server-kubernetes"]
    icon: "1️⃣"
    category: "App 1"
    requiresClusterName: "app-cluster-1"  # Always connects to app-cluster-1
  
  - name: "kubernetes-app2"
    type: "localCommand"
    command: ["mcp-server-kubernetes"]
    icon: "2️⃣"
    category: "App 2"
    requiresClusterName: "app-cluster-2"  # Always connects to app-cluster-2
  
  - name: "prometheus"
    type: "localCommand"
    command: ["mcp-server-prometheus"]
    requiresClusterRole: "observability"  # Connects to active monitoring cluster
    env:
      PROMETHEUS_URL: "http://localhost:9090"
  
  - name: "loki"
    type: "container"
    image: "myregistry/mcp-loki:latest"
    requiresClusterName: "logging-cluster"  # Always uses the logging cluster
    containerPorts: ["3100:3100"]

portForwards:
  # Port forwards can also target specific clusters
  - name: "prometheus-app1"
    clusterName: "app-cluster-1"  # Always from app-cluster-1
    namespace: "monitoring"
    targetType: "service"
    targetName: "prometheus"
    localPort: "9091"
    remotePort: "9090"
  
  - name: "prometheus-app2"
    clusterName: "app-cluster-2"  # Always from app-cluster-2
    namespace: "monitoring"
    targetType: "service"
    targetName: "prometheus"
    localPort: "9092"
    remotePort: "9090"
```

### Development vs Production Configuration

Use project-specific configs to override clusters for development:

```yaml
# ~/.config/envctl/config.yaml (User global config)
clusters:
  - name: "prod-mc"
    context: "teleport.giantswarm.io-prod"
    role: "observability"
    displayName: "Production MC"
  
  - name: "prod-wc"
    context: "teleport.giantswarm.io-prod-main"
    role: "target"
    displayName: "Production WC"

activeClusters:
  observability: "prod-mc"
  target: "prod-wc"

# .envctl/config.yaml (Project-specific config)
clusters:
  - name: "dev-mc"
    context: "kind-dev"
    role: "observability"
    displayName: "Local Dev MC"
    icon: "🏠"
  
  - name: "dev-wc"
    context: "kind-dev-workload"
    role: "target"
    displayName: "Local Dev WC"
    icon: "🛠️"

# Override active clusters for development
activeClusters:
  observability: "dev-mc"
  target: "dev-wc"

# Add development-specific services
mcpServers:
  - name: "dev-tools"
    type: "localCommand"
    enabledByDefault: true
    command: ["./scripts/dev-mcp-server.sh"]
    category: "Development"
    icon: "🛠️"
    requiresClusterRole: "target"
```

This flexible cluster configuration system allows you to:
- Maintain backward compatibility with Giant Swarm's MC/WC pattern
- Support multiple environments with easy switching
- Mix different cluster types (Giant Swarm, cloud providers, local)
- Have services connect to specific clusters based on their needs
- Override configurations for different contexts (dev vs prod)

## Configuration API Tools

Envctl exposes a comprehensive set of MCP tools for managing configuration dynamically at runtime. These tools are available through the aggregator when envctl is running, allowing AI agents and other MCP clients to interact with and modify the configuration programmatically.

### Available Configuration Tools

All configuration tools are prefixed with the envctl prefix (default: `x_`).

#### Reading Configuration

- **`x_config_get`**: Get the entire envctl configuration
- **`x_config_get_clusters`**: Get all configured clusters
- **`x_config_get_active_clusters`**: Get the active clusters mapping
- **`x_config_get_mcp_servers`**: Get all MCP server definitions
- **`x_config_get_port_forwards`**: Get all port forward definitions
- **`x_config_get_workflows`**: Get all workflow definitions
- **`x_config_get_aggregator`**: Get aggregator configuration
- **`x_config_get_global_settings`**: Get global settings

#### Updating Configuration

- **`x_config_update_mcp_server`**: Update or add an MCP server definition
  - Requires a `server` object parameter with the MCPServerDefinition structure
- **`x_config_update_port_forward`**: Update or add a port forward definition
  - Requires a `port_forward` object parameter with the PortForwardDefinition structure
- **`x_config_update_workflow`**: Update or add a workflow definition
  - Requires a `workflow` object parameter with the WorkflowDefinition structure
- **`x_config_update_aggregator`**: Update aggregator configuration
  - Requires an `aggregator` object parameter with the AggregatorConfig structure
- **`x_config_update_global_settings`**: Update global settings
  - Requires a `settings` object parameter with the GlobalSettings structure

#### Deleting Configuration Items

- **`x_config_delete_mcp_server`**: Delete an MCP server by name
  - Requires a `name` string parameter
- **`x_config_delete_port_forward`**: Delete a port forward by name
  - Requires a `name` string parameter
- **`x_config_delete_workflow`**: Delete a workflow by name
  - Requires a `name` string parameter
- **`x_config_delete_cluster`**: Delete a cluster by name
  - Requires a `name` string parameter

#### Persisting Changes

- **`x_config_save`**: Save the current configuration to file
  - No parameters required
  - Writes the configuration to the appropriate file based on the config loading hierarchy

### Example Usage

Here are some examples of using the configuration API tools:

```javascript
// Get all MCP servers
const servers = await callTool("x_config_get_mcp_servers");

// Add a new MCP server
await callTool("x_config_update_mcp_server", {
  server: {
    name: "custom-server",
    type: "localCommand",
    enabledByDefault: true,
    command: ["npx", "my-custom-mcp-server"],
    env: {
      API_KEY: "secret"
    },
    requiresClusterRole: "target"
  }
});

// Update port forward configuration
await callTool("x_config_update_port_forward", {
  port_forward: {
    name: "database",
    enabledByDefault: true,
    clusterRole: "target",
    namespace: "production",
    targetType: "service",
    targetName: "postgres",
    localPort: "5432",
    remotePort: "5432"
  }
});

// Delete an MCP server
await callTool("x_config_delete_mcp_server", {
  name: "obsolete-server"
});

// Save configuration changes
await callTool("x_config_save");
```

### Important Notes

1. **Runtime Updates**: Configuration changes made through these tools take effect immediately for most settings. However, some changes may require restarting affected services to fully apply.

2. **Validation**: The API validates all configuration updates to ensure they conform to the expected structure. Invalid configurations will be rejected with appropriate error messages.

3. **Persistence**: Changes are only persisted when you explicitly call `x_config_save`. Until then, changes are only in memory and will be lost if envctl is restarted.

4. **Layered Configuration**: When saving, the configuration is written to the appropriate layer based on where envctl was started (user or project configuration).

5. **Service Dependencies**: When updating MCP servers or port forwards that have dependencies, ensure the required resources exist to avoid startup failures.