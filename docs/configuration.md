# Flexible Configuration via YAML ⚙️

`envctl` offers a powerful and flexible configuration system using YAML files, allowing you to tailor its behavior, define custom MCP servers, and manage port-forwarding rules beyond the built-in defaults.

## Configuration Layers

Settings are loaded in a layered approach, with later layers overriding or extending earlier ones:

1.  **Default Configuration:** Built-in defaults for common services like Kubernetes, Prometheus, and Grafana. These are defined internally within `envctl`.
2.  **User-Specific Global Configuration:** Define your personal overrides and custom services in `~/.config/envctl/config.yaml` (or `$XDG_CONFIG_HOME/envctl/config.yaml` on Linux). This allows you to set preferences that apply every time you use `envctl`.
3.  **Project-Specific Configuration:** For settings specific to a particular project or repository, create a `.envctl/config.yaml` file in the root of your project directory. These settings will apply when `envctl` is run from within that project, overriding user and default configurations where specified.

**Merge Strategy:**

*   **`globalSettings`**: Settings from a later layer (e.g., project config) will completely override those from an earlier layer (e.g., user config or default). For example, if the default `defaultContainerRuntime` is "docker" and your user config specifies "podman", "podman" will be used. If a project config then specifies "cri-o", "cri-o" will be used for that project.
*   **`mcpServers` & `portForwards`**: These are lists of definitions. The merge logic is based on the `name` field of each server or port-forward definition:
    *   If an item in a later layer has the same `name` as an item in an earlier layer, the item from the later layer completely replaces the one from the earlier layer.
    *   New items with unique names found in later layers are appended to the list.
    This allows you to override default services (like "prometheus" or "grafana") or add entirely new custom services.

## Configuration File Structure (`config.yaml`)

The YAML file uses the following main sections:

```yaml
globalSettings:
  defaultContainerRuntime: "docker" # Or "podman", etc.

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
    # Dependencies
    requiresPortForwards: ["name-of-required-portforward-1", "name-of-required-portforward-2"]

portForwards: # List of Port Forward definitions
  - name: "unique-portforward-name"
    enabledByDefault: true # or false
    icon: "🔗" # Optional: Emoji or single character for TUI
    category: "Databases" # Optional: For grouping in TUI
    kubeContextTarget: "mc" # "mc", "wc", or an explicit context name like "gke_my-project_us-central1_my-cluster"
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
*   **Dependencies:**
    *   `requiresPortForwards` (list of strings, optional): A list of `name`s of `PortForwardDefinition`s that this MCP server requires to be active before it starts (e.g., a Prometheus server might require a port-forward to the cluster's Prometheus instance).

### `portForwards`

This is a list of Kubernetes port-forwarding definitions.

*   `name` (string, required): A unique name for this port-forward definition (e.g., "mc-prometheus", "wc-alloy-debug"). Used for merging and references.
*   `enabledByDefault` (boolean, optional): If `true`, this port-forward will be established by default. Defaults to `false` if not specified, unless it's a core default.
*   `icon` (string, optional): An emoji or single character for the TUI.
*   `category` (string, optional): A category name for grouping in the TUI.
*   `kubeContextTarget` (string, required): Specifies which Kubernetes context to use for this port-forward. Can be:
    *   `"mc"`: Dynamically resolves to the context of the Management Cluster specified in the `envctl connect` command.
    *   `"wc"`: Dynamically resolves to the context of the Workload Cluster specified in the `envctl connect` command.
    *   An explicit Kubernetes context name (e.g., `"kind-mycluster"`, `"gke_my-project_us-central1-a_my-gke-cluster"`).
*   `namespace` (string, required): The Kubernetes namespace where the target resource resides.
*   `targetType` (string, required): The type of Kubernetes resource to port-forward to. Common values: `"service"`, `"pod"`, `"deployment"`, `"statefulset"`.
*   `targetName` (string, required): The name of the target Kubernetes resource (e.g., `"my-service"`, `"my-pod-abcde"`).
*   `targetLabelSelector` (string, optional): A Kubernetes label selector (e.g., `"app=myapp,component=server"`). If `targetName` is for a resource like a Deployment or StatefulSet that manages multiple pods, or if you want to target a pod by labels instead of name, use this. `envctl` will attempt to find a suitable pod matching the selector.
*   `localPort` (string, required): The port on your local machine to bind to.
*   `remotePort` (string, required): The port on the target resource in the cluster.
*   `bindAddress` (string, optional): The local IP address to bind to. Defaults to `"127.0.0.1"`. Use `"0.0.0.0"` to bind to all available network interfaces.

## Example Configuration File

Below is a more comprehensive example showcasing various features:

```yaml
globalSettings:
  defaultContainerRuntime: "podman"

mcpServers:
  - name: "prometheus" # Overrides the default prometheus server
    type: "localCommand"
    enabledByDefault: true
    icon: "🔔" # Different icon
    category: "Monitoring - Custom"
    command: ["my-custom-prometheus-runner", "--port", "9090"]
    env:
      PROMETHEUS_URL: "http://localhost:8081/custom-prometheus" # Custom URL
      ORG_ID: "my-org"
      CUSTOM_FLAG: "true"
    requiresPortForwards:
      - "mc-prometheus-custom" # Depends on a custom port-forward defined below

  - name: "my-dev-server"
    type: "localCommand"
    enabledByDefault: false # Not started by default
    icon: "🚀"
    category: "Development"
    command: ["npx", "some-dev-server", "--watch"]
    env:
      NODE_ENV: "development"

  - name: "jaeger-tracing"
    type: "container"
    enabledByDefault: true
    icon: "ιχ" # Greek iota chi for Jaeger (just an example)
    category: "Tracing"
    image: "jaegertracing/all-in-one:latest"
    containerPorts:
      - "16686:16686" # Jaeger UI
      - "6831:6831/udp" # Jaeger agent
    containerEnv:
      COLLECTOR_ZIPKIN_HOST_PORT: ":9411"

portForwards:
  - name: "mc-grafana" # Overrides the default mc-grafana
    enabledByDefault: true
    icon: "🎨" # Different icon
    category: "Monitoring (MC) - Custom"
    kubeContextTarget: "mc" # Use the Management Cluster context
    namespace: "custom-monitoring"
    targetType: "service"
    targetName: "my-custom-grafana"
    localPort: "3001" # Different local port
    remotePort: "3000"
    bindAddress: "0.0.0.0" # Bind to all interfaces

  - name: "mc-prometheus-custom" # Custom port-forward for the overridden prometheus
    enabledByDefault: true
    icon: "🔔"
    category: "Monitoring (MC) - Custom"
    kubeContextTarget: "mc"
    namespace: "mimir"
    targetType: "service"
    targetName: "mimir-query-frontend" # Still targets mimir, but on a different local port
    localPort: "8081" # Matches the PROMETHEUS_URL env for the custom prometheus MCP
    remotePort: "8080"

  - name: "wc-app-debug"
    enabledByDefault: false # Disabled by default
    icon: "🐞"
    category: "App Debug (WC)"
    kubeContextTarget: "wc" # Use the Workload Cluster context
    namespace: "my-app-ns"
    targetType: "pod"
    targetName: "my-app-pod-0" # Or use targetLabelSelector
    # targetLabelSelector: "app=my-app,role=main"
    localPort: "9999"
    remotePort: "8080"

  - name: "external-service-via-specific-context"
    enabledByDefault: false
    icon: "🌐"
    category: "External Services"
    kubeContextTarget: "gke_my-gcp-project_us-central1-a_my-specific-cluster" # Explicit context
    namespace: "external-tools"
    targetType: "service"
    targetName: "some-external-facing-svc"
    localPort: "7000"
    remotePort: "80"
```

This system allows for fine-grained control over `envctl`'s behavior, making it adaptable to diverse development environments and requirements. 