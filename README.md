# envctl 🚀

Your friendly environment connector for Giant Swarm!

`envctl` is a command-line tool designed to simplify connecting your development environment, particularly [Model Context Protocol (MCP)](https://github.com/modelcontext/protocol) servers used in IDEs like Cursor, to Giant Swarm Kubernetes clusters and services like Prometheus.

It automates the process of logging into clusters via Teleport (`tsh`) and setting up necessary connections like port-forwarding for Prometheus (Mimir).

## Features ✨

*   **Simplified Connection:** Connect to management and workload clusters with a single command.
*   **Automatic Context Switching:** Sets your `kubectl` context correctly.
*   **Prometheus Port-Forwarding:** Handles `kubectl port-forward` for the Mimir query frontend automatically.
*   **Teleport Integration:** Uses your existing `tsh` setup for Kubernetes access.
*   **Shell Completion:** Provides dynamic command-line completion for cluster names (Bash & Zsh).

## Prerequisites 📋

Before using `envctl`, ensure you have the following installed and configured:

1.  **Go:** Version 1.21 or later ([Installation Guide](https://go.dev/doc/install)).
2.  **Teleport Client (`tsh`):** You need `tsh` installed and logged into your Giant Swarm Teleport proxy.
3.  **`kubectl`:** The Kubernetes command-line tool.

## Installation 🛠️

1.  Clone this repository (or ensure you are in the project directory).
2.  Build the binary:
    ```bash
    go build -o envctl .
    ```
3.  (Optional) Move the `envctl` binary to a directory in your `$PATH` (e.g., `/usr/local/bin` or `~/bin`):
    ```bash
    mv envctl /usr/local/bin/
    ```

## Usage 🎮

The primary command is `envctl connect`:

```
envctl connect <management-cluster> [workload-cluster-shortname]
```

**Arguments:**

*   `<management-cluster>`: (Required) The name of the Giant Swarm management cluster (e.g., `wallaby`, `enigma`).
*   `[workload-cluster-shortname]`: (Optional) The *short* name of the workload cluster (e.g., `plant-lille-prod` for `wallaby-plant-lille-prod`, `ve5v6` for `enigma-ve5v6`).

**Examples:**

1.  **Connect to a Management Cluster only:**

    ```bash
    envctl connect enigma
    ```

    *   Logs into `enigma` via `tsh kube login enigma`.
    *   Sets the current `kubectl` context to `enigma`.
    *   Starts port-forwarding for Prometheus (`kubectl --context enigma port-forward -n mimir service/mimir-query-frontend 8080:8080`) in the background.
    *   Prints a summary and instructions for MCP.

2.  **Connect to a Management and Workload Cluster:**

    ```bash
    envctl connect wallaby plant-cassino-prod
    ```

    *   Logs into `wallaby` via `tsh kube login wallaby`.
    *   Logs into the *full* workload cluster name (`wallaby-plant-cassino-prod`) via `tsh`.
    *   Sets the current `kubectl` context to the *full* workload cluster name (`wallaby-plant-cassino-prod`).
    *   Starts port-forwarding for Prometheus using the *management cluster* context (`wallaby`) in the background.
    *   Prints a summary and instructions for MCP.

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
envctl connect <TAB>              # Shows management clusters
envctl connect wallaby <TAB>      # Shows short names of workload clusters for wallaby
```

## MCP Integration Notes 💡

*   After running `envctl connect`, Prometheus should be available at `http://localhost:8080/prometheus`.
*   Ensure your `mcp.json` (e.g., `~/.cursor/mcp.json`) has the correct `PROMETHEUS_URL` for the Prometheus MCP server:
    ```json
    {
      "mcpServers": {
        "kubernetes": {
          "command": "npx",
          "args": ["mcp-server-kubernetes"]
        },
        "prometheus": {
          "command": "uv", // Or your specific command
          "args": [ ... ], // Your specific args
          "env": {
            "PROMETHEUS_URL": "http://localhost:8080/prometheus"
          }
        }
        // ... other servers ...
      }
    }
    ```
*   You may need to **restart your MCP servers** or your IDE after running `envctl connect` for them to pick up the new Kubernetes context and Prometheus connection.

## Future Development 🔮

*   Support for connecting to Loki.
*   Direct SSH access integration.
*   Connections for specific cloud providers (AWS, Azure, GCP).
*   More robust background process management.

--- 