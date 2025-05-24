# envctl 🚀

Your friendly environment connector for Giant Swarm!

`envctl` is a command-line tool designed to simplify connecting your development environment, particularly [Model Context Protocol (MCP)](https://github.com/modelcontext/protocol) servers used in IDEs like Cursor, to Giant Swarm Kubernetes clusters and services like Prometheus.

It automates the process of logging into clusters via Teleport (`tsh`) and setting up necessary connections like port-forwarding for Prometheus (Mimir).

## Features ✨

*   **Simplified Connection:** Connect to management and workload clusters with a single command.
*   **Automatic Context Switching:** Sets your Kubernetes context correctly.
*   **Port-Forwarding Management:** 
    *   Prometheus and Grafana services (always from the Management Cluster)
    *   Alloy Metrics (from the Workload Cluster if specified, otherwise from the Management Cluster)
*   **Interactive Terminal UI:** View cluster status, manage port forwards, and monitor connections in a polished terminal interface.
*   **Teleport Integration:** Uses your existing `tsh` setup for Kubernetes access.
*   **Shell Completion:** Provides dynamic command-line completion for cluster names (Bash & Zsh).

## Releases & Changelog 📦

Releases are automatically created when pull requests are merged into the main branch. Each merged PR triggers a new release with an incremented version number.

The changelog for each release is automatically generated and included in the release notes on the [GitHub Releases page](https://github.com/giantswarm/envctl/releases).

Pre-built binaries for multiple platforms (Linux, macOS, Windows) are available for download from the Releases page.

## Prerequisites 📋

Before using `envctl`, ensure you have the following installed and configured:

1.  **Go:** Version 1.21 or later ([Installation Guide](https://go.dev/doc/install)).
2.  **Teleport Client (`tsh`):** You need `tsh` installed and logged into your Giant Swarm Teleport proxy.
3.  **`mcp-proxy`:** This tool is used by `envctl` to proxy your actual MCP servers. ([Installation Guide](https://github.com/sparfenyuk/mcp-proxy#installation)).
4.  **Underlying MCP Server Executables:** `envctl` expects specific MCP server commands to be available in your PATH, as it will invoke them via `mcp-proxy`. These are typically:
    *   For Kubernetes: `npx mcp-server-kubernetes` (requires Node.js and `npx`)
    *   For Prometheus: `uvx mcp-server-prometheus` (requires `uv` and the Python-based `mcp-server-prometheus`)
    *   For Grafana: `uvx mcp-server-grafana` (requires `uv` and the Python-based `mcp-server-grafana` - if you use a Grafana MCP).
    (Ensure `uv` is installed if you intend to use `uvx` for these servers: [uv Installation](https://github.com/astral-sh/uv#installation)).

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

The primary command is `envctl connect`:

```
envctl connect <management-cluster> [workload-cluster-shortname]
```

This command launches the interactive TUI by default, showing you real-time status of your clusters and port-forwards.

Other commands:

```
# Show current version
envctl version

# Update envctl to the latest release
envctl self-update

# Use the CLI mode without TUI (for scripts or CI environments)
# This mode will:
# - Log into the specified cluster(s) via tsh.
# - Sets the Kubernetes context.
# - Start port-forwarding for:
#   - Prometheus (MC) on localhost:8080
#   - Grafana (MC) on localhost:3000
#   - Alloy Metrics (on localhost:12345):
#     - For the Workload Cluster (WC) if specified.
#     - For the Management Cluster (MC) if only an MC is specified.
# - Print a summary and instructions, then exit. Port-forwards will run in the background.
envctl connect <management-cluster> [workload-cluster-shortname] --no-tui
```

**Arguments for `connect`:**

*   `<management-cluster>`: (Required) The name of the Giant Swarm management cluster (e.g., `myinstallation`, `mycluster`).
*   `[workload-cluster-shortname]`: (Optional) The *short* name of the workload cluster (e.g., `myworkloadcluster` for `myinstallation-myworkloadcluster`, `customerprod` for `mycluster-customerprod`).

**Examples:**

1.  **Connect to a Management Cluster only:**

    ```bash
    envctl connect myinstallation
    ```

    *   Launches an interactive terminal UI
    *   Logs into `myinstallation` via `tsh kube login myinstallation`.
    *   Sets the current Kubernetes context to `teleport.giantswarm.io-myinstallation`.
    *   Starts port-forwarding for Prometheus (MC) on `localhost:8080`, Grafana (MC) on `localhost:3000`, and Alloy Metrics (MC) on `localhost:12345`.
    *   Displays cluster health and connection status
    *   Allows management of port-forwards and contexts

2.  **Connect to a Management and Workload Cluster:**

    ```bash
    envctl connect myinstallation myworkloadcluster
    ```

    *   Logs into `myinstallation` via `tsh kube login myinstallation`.
    *   Logs into the *full* workload cluster name (`myinstallation-myworkloadcluster`) via `tsh`.
    *   Sets the current Kubernetes context to the *full* workload cluster name (`teleport.giantswarm.io-myinstallation-myworkloadcluster`).
    *   Starts port-forwarding for Prometheus using the *management cluster* context (`teleport.giantswarm.io-myinstallation`) to `localhost:8080`.
    *   Starts port-forwarding for Grafana using the *management cluster* context (`teleport.giantswarm.io-myinstallation`) to `localhost:3000`.
    *   Starts port-forwarding for Alloy metrics using the *workload cluster* context (`teleport.giantswarm.io-myinstallation-myworkloadcluster`) to `localhost:12345`.
    *   Prints a summary and instructions for MCP.

## Terminal User Interface 🖥️

When running `envctl connect`, the Terminal User Interface (TUI) provides a visual dashboard to monitor and control your connections:

![envctl TUI overview](docs/images/tui-overview.png)

### Key Features

- **Cluster Status Monitoring**: View real-time health status of both management and workload clusters
- **Port Forward Management**: Monitor active port forwards with status indicators
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
| D            | Toggle dark/light mode                   |
| z            | Toggle debug information                 |
| y            | Copy logs/config (when in overlay)       |
| Esc          | Close help/log/config overlay            |

For more details on the implementation and architecture of the TUI, see the [TUI documentation](docs/tui.md).

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

Now you can use TAB to complete cluster names:

```bash
envctl connect myinstallation <TAB>      # Shows short names of workload clusters for myinstallation
```

## Flexible Configuration via YAML ⚙️

`envctl` supports a powerful YAML-based configuration system to customize its behavior, define new MCP servers, and manage port-forwarding rules. This allows you to tailor `envctl` precisely to your development needs.

Configurations are loaded in layers (default, user-global, project-specific), with later layers overriding earlier ones. You can manage global settings, define how MCP servers are run (as local commands or containers), and specify detailed port-forwarding rules, including dynamic Kubernetes context targeting (`"mc"`, `"wc"`, or explicit contexts).

For a detailed explanation of the configuration file structure, all available options, and comprehensive examples, please see the [**Flexible Configuration Documentation (docs/configuration.md)**](docs/configuration.md).

## MCP Integration Notes 💡

*   `envctl connect` uses `mcp-proxy` to manage connections for the following predefined MCP services:
    *   **Kubernetes**: Proxied on `http://localhost:8001/sse` (underlying command: `npx mcp-server-kubernetes`)
    *   **Prometheus**: Proxied on `http://localhost:8002/sse` (underlying command: `uvx mcp-server-prometheus`, expects Prometheus port-forward on `localhost:8080` via `PROMETHEUS_URL` env var)
    *   **Grafana**: Proxied on `http://localhost:8003/sse` (underlying command: `uvx mcp-server-grafana`, expects Grafana port-forward on `localhost:3000` via `GRAFANA_URL` env var - this is started if you have a Grafana MCP server with this name and command).
*   `envctl` no longer reads `~/.cursor/mcp.json` to determine how to start these servers. The commands listed above are hardcoded.
*   You must have `mcp-proxy` installed and the respective underlying MCP server executables (e.g., `mcp-server-kubernetes`, `mcp-server-prometheus`) available in your system's PATH.
*   **IDE Configuration (Cursor/VSCode):** Update your IDE's MCP settings to point to these SSE endpoints:
    *   Kubernetes: `http://localhost:8001/sse`
    *   Prometheus: `http://localhost:8002/sse`
    *   Grafana: `http://localhost:8003/sse` (if you use a Grafana MCP server)
*   Port-forwarded services (like Prometheus on `localhost:8080` and Grafana on `localhost:3000`) are started by `envctl` as before. The `mcp-proxy` instances for Prometheus and Grafana will use these via environment variables.
*   You may need to **restart your IDE** after running `envctl connect` and configuring these `mcp-proxy` SSE endpoints for changes to take effect.

### Customizing MCP Server Configuration

If you need to customize how MCP servers are run, you can use environment variables to override the default configurations:

* Each MCP server's configuration can be customized using environment variables with the pattern:
  * `ENVCTL_MCP_<SERVER>_COMMAND`: Override the command (e.g., `ENVCTL_MCP_PROMETHEUS_COMMAND=uvx`)
  * `ENVCTL_MCP_<SERVER>_ARGS`: Override the command arguments (e.g., `ENVCTL_MCP_PROMETHEUS_ARGS="mcp-server-prometheus --debug"`)
  * `ENVCTL_MCP_<SERVER>_ENV_<KEY>`: Set an environment variable for the command (e.g., `ENVCTL_MCP_PROMETHEUS_ENV_PROMETHEUS_URL=http://localhost:9090`)

For example, to use a custom Prometheus MCP server installation:

```bash
export ENVCTL_MCP_PROMETHEUS_COMMAND="python3"
export ENVCTL_MCP_PROMETHEUS_ARGS="-m custom_prometheus_mcp_server"
export ENVCTL_MCP_PROMETHEUS_ENV_PROMETHEUS_URL="http://localhost:9090"
export ENVCTL_MCP_PROMETHEUS_ENV_DEBUG="true"

# Then run envctl as usual
envctl connect myinstallation
```

These environment variables will be detected automatically at startup, allowing you to customize the MCP server configurations without modifying the source code.

## Future Development 🔮

*   Support for connecting to Loki.
*   Direct SSH access integration.
*   Connections for specific cloud providers (AWS, Azure, GCP).
*   More robust background process management.

--- 
