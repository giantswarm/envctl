# envctl 🚀

Your friendly environment connector for Giant Swarm!

`envctl` is a command-line tool designed to simplify connecting your development environment, particularly [Model Context Protocol (MCP)](https://github.com/modelcontext/protocol) servers used in IDEs like Cursor, to Giant Swarm Kubernetes clusters and services like Prometheus.

It automates the process of logging into clusters via Teleport (`tsh`) and setting up necessary connections like port-forwarding for Prometheus (Mimir).

## Features ✨

*   **Simplified Connection:** Connect to management and workload clusters with a single command.
*   **Flexible Cluster Configuration:** Define clusters with roles (observability, target, custom) and switch between them dynamically.
*   **Automatic Context Switching:** Sets your Kubernetes context correctly.
*   **Port-Forwarding Management:** 
    *   Prometheus and Grafana services (always from the Management Cluster)
    *   Alloy Metrics (from the Workload Cluster if specified, otherwise from the Management Cluster)
    *   Custom port forwards via YAML configuration
*   **Interactive Terminal UI:** View cluster status, manage port forwards, and monitor connections in a polished terminal interface.
*   **Teleport Integration:** Uses your existing `tsh` setup for Kubernetes access.
*   **Shell Completion:** Provides dynamic command-line completion for cluster names (Bash & Zsh).
*   **Service Dependency Management:** Automatically handles service dependencies and cascading operations.
*   **Health Monitoring:** Continuous health checks for all services with automatic recovery.
*   **Flexible Configuration:** Layer-based configuration system (default → user → project).
*   **MCP Server Support:** Run Model Context Protocol servers as local commands or Docker containers.
*   **MCP Tool Workflows:** Define reusable sequences of MCP tool calls as higher-level operations.
*   **API Tools:** Programmatic control of envctl through MCP tools for service management, cluster switching, and monitoring.

## What's New 🎉

### Recent Improvements
- **API Tools for MCP:** New MCP tools expose envctl's API, allowing programmatic control of services, clusters, and connections through AI assistants
- **Flexible Cluster Configuration:** New cluster role system allows defining multiple clusters with roles (observability, target, custom) and dynamically switching between them
- **Service-Specific Cluster Targeting:** Services can now depend on specific clusters or cluster roles, enabling complex multi-cluster setups
- **Unified Service Architecture:** All components (K8s connections, port forwards, MCP servers) are now managed as services with consistent lifecycle management
- **Advanced Dependency Management:** Services automatically start/stop based on their dependencies with intelligent cascade handling
- **Health Monitoring:** Continuous health checks for all services with automatic recovery when dependencies are restored
- **Enhanced State Management:** Centralized state store with event-driven updates and correlation tracking
- **Improved Error Handling:** Better error messages, retry logic, and graceful degradation
- **Comprehensive Documentation:** New architecture docs, troubleshooting guide, and quick start guide

## Releases & Changelog 📦

Releases are automatically created when pull requests are merged into the main branch. Each merged PR triggers a new release with an incremented version number.

The changelog for each release is automatically generated and included in the release notes on the [GitHub Releases page](https://github.com/giantswarm/envctl/releases).

Pre-built binaries for multiple platforms (Linux, macOS, Windows) are available for download from the Releases page.

## Prerequisites 📋

Before using `envctl`, ensure you have the following installed and configured:

1.  **Go:** Version 1.21 or later ([Installation Guide](https://go.dev/doc/install)).
2.  **Teleport Client (`tsh`):** You need `tsh` installed and logged into your Giant Swarm Teleport proxy.
3.  **kubectl:** Required for managing Kubernetes connections and port forwards.
4.  **MCP Server Executables (optional):** If you want to use MCP servers, you'll need the underlying executables available in your PATH:
    *   For Kubernetes: `mcp-server-kubernetes` (can be installed via `npm install -g @modelcontextprotocol/server-kubernetes`)
    *   For Prometheus: `mcp-server-prometheus` (can be installed via `pip install mcp-server-prometheus`)
    *   For Grafana: `mcp-server-grafana` (can be installed via `pip install mcp-server-grafana`)
    
    Note: MCP servers can also be run as Docker containers if you prefer containerized deployments.

## Installation 🛠️

### Option 1: Download from GitHub Releases

Download the pre-built binary for your platform from the [Releases page](https://github.com/giantswarm/envctl/releases):

```zsh
# For macOS (Intel)
curl -L https://github.com/giantswarm/envctl/releases/latest/download/envctl_darwin_amd64 -o envctl
chmod +x envctl
mv envctl /usr/local/bin/

# For macOS (Apple Silicon)
curl -L https://github.com/giantswarm/envctl/releases/latest/download/envctl_darwin_arm64 -o envctl
chmod +x envctl
mv envctl /usr/local/bin/

# For Linux (AMD64)
curl -L https://github.com/giantswarm/envctl/releases/latest/download/envctl_linux_amd64 -o envctl
chmod +x envctl
mv envctl /usr/local/bin/
```

### Option 2: Build from Source

1.  Clone this repository (or ensure you are in the project directory).
2.  Build the binary:
    ```sh
    go build -o envctl .
    ```
3.  (Optional) Move the `envctl` binary to a directory in your `$PATH` (e.g., `/usr/local/bin` or `~/bin`):
    ```sh
    mv envctl /usr/local/bin/
    ```

## Usage 🎮

The primary command is `envctl serve`:

```
envctl serve
```

This command starts the envctl aggregator server and launches the interactive TUI by default, showing you real-time status of your clusters and port-forwards.

Other commands:

```
# Show current version
envctl version

# Update envctl to the latest release
envctl self-update

# Debug the MCP aggregator as a client
envctl debug

# Launch interactive REPL mode for MCP testing
envctl debug --repl

# Use the CLI mode without TUI (for scripts or CI environments)
# This mode will:
# - Start the aggregator server
# - Start configured services (K8s connections, port forwards, MCP servers)
# - Keep running until interrupted (Ctrl+C)
# - All services stop when envctl exits
envctl serve --no-tui

# Enable debug logging for troubleshooting
envctl serve --debug

# Disable the safety denylist for destructive MCP tool calls (use with caution!)
# By default, destructive tools like kubectl apply/delete, helm install/uninstall, etc. are blocked
# This flag allows all MCP tools to be executed without restrictions
envctl serve --yolo
```

### Resource Management Commands

Once the aggregator server is running (`envctl serve`), you can use these commands to manage resources:

```
# Service Management
envctl service list                    # List all services with status
envctl service start <service-name>    # Start a specific service
envctl service stop <service-name>     # Stop a specific service
envctl service restart <service-name>  # Restart a service
envctl service status <service-name>   # Get detailed service status

# ServiceClass Management
envctl serviceclass list               # List all ServiceClass definitions
envctl serviceclass get <name>         # Get ServiceClass details
envctl serviceclass available <name>   # Check if ServiceClass is available

# MCP Server Management
envctl mcpserver list                  # List all MCP server definitions
envctl mcpserver get <name>            # Get MCP server details
envctl mcpserver available <name>      # Check if MCP server is available

# Workflow Management
envctl workflow list                   # List all workflows
envctl workflow get <name>             # Get workflow details
envctl workflow create <file>          # Create workflow from definition
envctl workflow validate <file>        # Validate workflow definition

# Capability Management
envctl capability list                 # List all capabilities
envctl capability get <name>           # Get capability details
envctl capability available <name>     # Check if capability is available
```

### Debug Command

The `debug` command acts as an MCP (Model Context Protocol) client for debugging and testing:

```bash
# Basic mode - connects and monitors MCP servers
envctl debug

# Interactive REPL mode for exploring MCP capabilities
envctl debug --repl

# Run as MCP server for AI assistant integration
envctl debug --mcp-server
```

The debug command supports three modes:

1. **Normal Mode** (default): Connects to the aggregator, lists tools, and waits for notifications
2. **REPL Mode** (`--repl`): Provides an interactive interface to explore and execute tools
3. **MCP Server Mode** (`--mcp-server`): Runs as an MCP server exposing all REPL functionality via stdio

**REPL Mode Features:**

The interactive REPL (Read-Eval-Print Loop) mode allows you to:
- List and describe available tools, resources, and prompts
- Execute tools with JSON arguments
- Retrieve resource contents
- Get prompts with arguments
- Toggle notification display on/off
- **Tab completion** for commands, tool names, resource URIs, and prompt names
- **Command history** with arrow key navigation
- **History search** with Ctrl+R
- Persistent history across REPL sessions

**Example REPL session:**
```
MCP> list tools
Available tools (5):
  1. mcp_envctl-mcp_execute_query       - Execute a PromQL instant query
  2. mcp_envctl-mcp_execute_range_query - Execute a PromQL range query

MCP> describe tool mcp_envctl-mcp_execute_query
Tool: mcp_envctl-mcp_execute_query
Description: Execute a PromQL instant query against Prometheus
Input Schema:
{
  "type": "object",
  "properties": {
    "query": {"type": "string"},
    "time": {"type": "string"}
  },
  "required": ["query"]
}

MCP> call mcp_envctl-mcp_execute_query {"query": "up"}
Executing tool: mcp_envctl-mcp_execute_query...
Result:
{
  "status": "success",
  "data": {...}
}

MCP> exit
```

**REPL Keyboard Shortcuts:**

| Key | Action |
|-----|--------|
| TAB | Auto-complete commands, tool names, resource URIs, and prompt names |
| ↑/↓ | Navigate through command history |
| Ctrl+R | Search command history |
| Ctrl+A | Move cursor to beginning of line |
| Ctrl+E | Move cursor to end of line |
| Ctrl+W | Delete word before cursor |
| Ctrl+K | Delete from cursor to end of line |
| Ctrl+U | Delete from cursor to beginning of line |
| Ctrl+L | Clear screen |
| Ctrl+C | Cancel current line |
| Ctrl+D | Exit REPL |

**Configuration-Based Operation:**

envctl operates based on configuration files that define clusters, services, port forwards, and MCP servers. Configuration is loaded from:
- User config: `~/.config/envctl/config.yaml`
- Project config: `./.envctl/config.yaml`

**Examples:**

> **Note**: The behavior described below assumes you have configured clusters, port forwards, and MCP servers in your config file. By default, `envctl` starts with no services configured.

1.  **Start with TUI (typical usage):**

    ```bash
    envctl serve
    ```

    With a typical Giant Swarm configuration, this would:
    *   Launch an interactive terminal UI
    *   Start the aggregator server
    *   Connect to configured Kubernetes clusters via `tsh`
    *   Start any configured port forwards (e.g., Prometheus, Grafana, Alloy Metrics)
    *   Start configured MCP servers
    *   Display cluster health and connection status
    *   Allow management of services through the TUI

2.  **Start in CLI mode (for scripts/automation):**

    ```bash
    envctl serve --no-tui
    ```

    With a typical Giant Swarm configuration, this would:
    *   Start the aggregator server in background
    *   Connect to configured clusters
    *   Start configured services (port forwards, MCP servers)
    *   Print status summary and keep running until interrupted

3.  **Use resource management commands:**

    ```bash
    # Start the server first
    envctl serve --no-tui &
    
    # Then use management commands
    envctl service list
    envctl mcpserver status prometheus
    envctl workflow validate my-workflow.yaml
    ```

## Terminal User Interface 🖥️

When running `envctl serve`, the Terminal User Interface (TUI) provides a visual dashboard to monitor and control your connections:

![envctl TUI overview](docs/images/tui-overview.png)

### Key Features

- **Cluster Status Monitoring**: View real-time health status of both management and workload clusters
- **Port Forward Management**: Monitor active port forwards with status indicators
- **MCP Server Status**: Track MCP server health and available tools
- **Service Dependencies**: Visual indicators show service relationships
- **Log Viewer**: View operation logs directly in the terminal
- **Keyboard Navigation**: Easily navigate between panels with Tab/Shift+Tab
- **Dark Mode Support**: Toggle between light and dark themes with 'D' key

### Keyboard Shortcuts

| Key          | Action                                   |
|--------------|------------------------------------------|
| Tab / j / ↓  | Navigate to next panel                   |
| Shift+Tab / k / ↑ | Navigate to previous panel               |
| q / Ctrl+C   | Quit the application                     |
| r            | Restart port forwarding for focused panel|
| s            | Switch Kubernetes context                |
| N            | Start new connection                     |
| h            | Toggle help overlay                      |
| L            | Toggle log overlay                       |
| C            | Toggle MCP config overlay                |
| A            | Toggle Agent REPL overlay                |
| D            | Toggle dark/light mode                   |
| z            | Toggle debug information                 |
| y            | Copy logs/config (when in overlay)       |
| Esc          | Close help/log/config/REPL overlay       |

### Agent REPL Overlay

The Agent REPL (Read-Eval-Print Loop) overlay provides an interactive interface within the TUI for communicating with MCP servers directly:

- **Access**: Press 'A' to open the Agent REPL overlay
- **Command Execution**: Run MCP commands like `list tools`, `call <tool> {json}`, `get <resource>`
- **Tab Completion**: Smart completion for commands, tool names, and resource URIs
- **Command History**: Navigate through previous commands with up/down arrows
- **Auto-connection**: Automatically connects to the MCP aggregator when needed
- **Scrollable Output**: View long outputs with PgUp/PgDn
- **Integrated Help**: Type `help` to see all available commands

The Agent REPL overlay allows you to interact with MCP servers without leaving the TUI, making it easy to test tools, inspect resources, and debug MCP connections. For detailed usage, see the [Agent REPL TUI documentation](docs/agent-repl-tui.md).

For more details on the implementation and architecture of the TUI, see the [TUI documentation](docs/tui.md).

## Service Dependencies 🔗

`envctl` automatically manages dependencies between services to ensure everything starts and stops in the correct order:

### Dependency Hierarchy

```
┌─────────────────────┐
│  K8s Connections    │ (Foundation - no dependencies)
│  - MC Connection    │
│  - WC Connection    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│   Port Forwards     │ (Depend on K8s connections)
│  - mc-prometheus    │ → Requires MC connection
│  - mc-grafana       │ → Requires MC connection  
│  - alloy-metrics    │ → Requires WC or MC connection
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│    MCP Servers      │ (May depend on port forwards)
│  - kubernetes       │ → Requires MC connection
│  - prometheus       │ → Requires mc-prometheus port forward
│  - grafana          │ → Requires mc-grafana port forward
└─────────────────────┘
```

### Automatic Behavior

1. **Starting Services**: Services start in dependency order - K8s connections first, then port forwards, then MCP servers
2. **Cascade Stop**: Stopping a service automatically stops all services that depend on it
3. **Health Monitoring**: If a K8s connection becomes unhealthy, all dependent services are automatically stopped
4. **Auto-Recovery**: When a K8s connection recovers, services that were stopped due to the failure are automatically restarted
5. **Restart with Dependencies**: Restarting a service ensures all its dependencies are also running

### Examples

- If you stop the `mc-prometheus` port forward, the `prometheus` MCP server will also stop
- If the MC K8s connection fails, all MC port forwards and their dependent MCP servers stop
- When restarting the `grafana` MCP server, if the `mc-grafana` port forward isn't running, it will be restarted too
- Manually stopped services won't be auto-restarted when dependencies recover

## Shell Completion 🧠

`envctl` supports shell completion for cluster names.

**Setup (Zsh):**

```bash
# For Oh My Zsh
./envctl completion zsh > ~/.oh-my-zsh/completions/_envctl

# Or for standard Zsh (add to ~/.zshrc if needed: fpath=(~/.zsh/completion $fpath))
mkdir -p ~/.zsh/completion
./envctl completion zsh > ~/.zsh/completion/_envctl
exec zsh # Reload shell or run compinit
```

**Setup (Bash):**

```bash
# System-wide (requires sudo)
sudo mkdir -p /etc/bash_completion.d/
./envctl completion bash | sudo tee /etc/bash_completion.d/envctl

# Or for current user (add to ~/.bashrc)
echo "source <(./envctl completion bash)" >> ~/.bashrc
source ~/.bashrc # Reload shell
```

Now you can use TAB to complete commands and resource names:

```bash
envctl service <TAB>                     # Shows service subcommands (list, start, stop, etc.)
envctl mcpserver list <TAB>              # Shows available MCP servers
```

## Flexible Configuration via YAML ⚙️

`envctl` supports a powerful YAML-based configuration system to customize its behavior, define new MCP servers, and manage port-forwarding rules. By default, `envctl` starts with minimal configuration (no k8s connections, no MCP servers, no port forwarding) - you configure exactly what you need through YAML files.

### Configuration File Location

`envctl` looks for configuration in the following locations (later files override earlier ones):
1. **Built-in defaults** - Minimal defaults (empty services)
2. **User config** - `~/.config/envctl/config.yaml` 
3. **Project config** - `./.envctl/config.yaml` (git-ignored by default)

To get started, create your configuration file in one of these locations. You can find example configurations in:
- `.envctl/config.yaml` - Template with all available options (commented out)
- `.envctl/config-example-basic.yaml` - Basic example with one cluster and MCP server

### New: Flexible Cluster Configuration 🚀

The new cluster configuration system allows you to:
- Define multiple clusters with specific roles (observability, target, custom)
- Switch between clusters dynamically through the TUI
- Have different services connect to different clusters based on their needs
- Mix and match clusters from different providers (Giant Swarm, GKE, EKS, etc.)
- Override cluster configurations for different environments (dev vs prod)

### Configuration Examples

See the [examples directory](docs/examples/) for:
- [Basic configuration](docs/examples/basic-config.yaml) - Minimal setup for getting started
- [Advanced configuration](docs/examples/advanced-config.yaml) - Complex scenarios with custom services
- [Containerized MCP servers](docs/examples/containerized-config.yaml) - Running MCP servers in Docker
- [Cluster configuration examples](docs/cluster-configuration-examples.md) - Various cluster setup scenarios

For a detailed explanation of the configuration file structure, all available options, and comprehensive examples, please see the [**Flexible Configuration Documentation (docs/configuration.md)**](docs/configuration.md).

## Documentation 📚

### Getting Started
- **[Quick Start Guide](docs/quickstart.md)** - Get up and running with envctl in minutes
- **[Configuration Guide](docs/configuration.md)** - Detailed configuration options and examples
- **[Troubleshooting Guide](docs/troubleshooting.md)** - Common issues and solutions

### Architecture & Design
- **[Architecture Overview](docs/architecture.md)** - Comprehensive system design and component interactions
- **[Development Guide](docs/development.md)** - Contributing to envctl, testing, and architecture details
- **[Message Handling Architecture](docs/message-handling-architecture.md)** - How messages flow through the system

### Terminal UI
- **[TUI Documentation](docs/tui.md)** - Terminal User Interface features and usage
- **[TUI Implementation](docs/tui-implementation.md)** - Technical details of the TUI architecture
- **[TUI Style Guide](docs/tui-styleguide.md)** - Design principles and styling guidelines

### Advanced Topics
- **[MCP Tool Workflows](docs/workflows.md)** - Create reusable sequences of MCP tool calls
- **[Containerized MCP Servers](docs/containerized-mcp-servers.md)** - Running MCP servers in containers
- **[Benchmarking](docs/benchmarking.md)** - Performance testing and optimization

## MCP Integration Notes 💡

envctl manages MCP (Model Context Protocol) servers to provide AI assistants with access to your Kubernetes clusters and services. The MCP servers are configured through envctl's YAML configuration system and can be run as:

*   **Local Commands**: Traditional executables running as local processes
*   **Docker Containers**: For isolated, reproducible environments

### Example MCP Server Configurations

`envctl` can manage various MCP servers. Here are some commonly used configurations that you can add to your config file:

*   **Kubernetes** (port 8001): Provides Kubernetes API access
    - Example command: `npx mcp-server-kubernetes`
    - Requires: Target cluster connection
    
*   **Prometheus** (port 8002): Enables Prometheus queries
    - Example command: `mcp-server-prometheus`  
    - Requires: `mc-prometheus` port forward
    - Environment: `PROMETHEUS_URL=http://localhost:8080`
    
*   **Grafana** (port 8003): Access to Grafana dashboards
    - Example command: `mcp-server-grafana`
    - Requires: `mc-grafana` port forward
    - Environment: `GRAFANA_URL=http://localhost:3000`

See the example configuration files in `.envctl/` for complete setup examples.

### API Tools

envctl exposes its own API functionality through MCP tools, allowing AI assistants and other MCP clients to programmatically control envctl services. These tools are automatically available through the MCP aggregator server:

**Service Management Tools** (prefix: `x_service_*`):
- `x_service_list` - List all services with their current status
- `x_service_start` - Start a specific service
- `x_service_stop` - Stop a specific service
- `x_service_restart` - Restart a specific service
- `x_service_status` - Get detailed status of a service

**Cluster Management Tools** (prefix: `x_cluster_*`):
- `x_cluster_list` - List available clusters by role (talos, management, workload, observability)
- `x_cluster_switch` - Switch active cluster for a role
- `x_cluster_active` - Get currently active cluster for a role

**MCP Server Tools** (prefix: `x_mcp_server_*`):
- `x_mcp_server_list` - List all MCP servers
- `x_mcp_server_info` - Get detailed information about an MCP server
- `x_mcp_server_tools` - List tools exposed by an MCP server

**K8s Connection Tools** (prefix: `x_k8s_connection_*`):
- `x_k8s_connection_list` - List all Kubernetes connections
- `x_k8s_connection_info` - Get information about a specific connection
- `x_k8s_connection_by_context` - Find connection by context name

**Port Forward Tools** (prefix: `x_portforward_*`):
- `x_portforward_list` - List all port forwards
- `x_portforward_info` - Get information about a specific port forward

These API tools enable powerful automation scenarios, such as:
- Automatically restarting services when they fail
- Switching clusters based on workload requirements
- Monitoring service health programmatically
- Building custom workflows that orchestrate envctl operations

Example usage in the REPL:
```
MCP> call x_service_list {}
MCP> call x_cluster_switch {"role": "workload", "cluster_name": "mycluster-prod"}
MCP> call x_service_restart {"label": "mc-prometheus"}
```

### IDE Configuration

Configure your IDE (Cursor/VSCode) to connect to the MCP servers:

1. **For Cursor**: Update `.cursor/mcp_settings.json`:
   ```json
   {
     "mcpServers": {
       "kubernetes": {
         "command": "curl",
         "args": ["-N", "http://localhost:8001/sse"]
       },
       "prometheus": {
         "command": "curl", 
         "args": ["-N", "http://localhost:8002/sse"]
       },
       "grafana": {
         "command": "curl",
         "args": ["-N", "http://localhost:8003/sse"]
       }
     }
   }
   ```

2. **Restart your IDE** after starting envctl for the changes to take effect.

### Custom MCP Server Configuration

You can customize MCP servers through:

1. **YAML Configuration** (see [Configuration Guide](docs/configuration.md)):
   ```yaml
   mcpServers:
     - name: custom-mcp
       type: localCommand
       command: ["my-mcp-server", "--arg1", "value1"]
       proxyPort: 8004
       requiresPortForwards: ["my-service"]
       env:
         MY_VAR: "value"
   ```

2. **Environment Variables** (for quick overrides):
   ```bash
   export ENVCTL_MCP_PROMETHEUS_COMMAND="python3"
   export ENVCTL_MCP_PROMETHEUS_ARGS="-m custom_prometheus_mcp"
   export ENVCTL_MCP_PROMETHEUS_ENV_PROMETHEUS_URL="http://localhost:9090"
   
   envctl connect myinstallation
   ```

For more details on configuration options, see the [Configuration Guide](docs/configuration.md).

## Contributing 🤝

We welcome contributions! Please see our [Development Guide](docs/development.md) for details on:
- Setting up your development environment
- Running tests
- Understanding the architecture
- Submitting pull requests

## Future Development 🔮

*   Support for connecting to Loki.
*   Direct SSH access integration.
*   Connections for specific cloud providers (AWS, Azure, GCP).
*   Enhanced monitoring and alerting capabilities.
*   Plugin system for custom extensions.

--- 
