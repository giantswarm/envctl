# envctl Quick Start Guide

## What is envctl?

envctl is a command-line tool that simplifies working with Giant Swarm Kubernetes clusters by managing:
- **Kubernetes connections** to Management Clusters (MC) and Workload Clusters (WC)
- **Port forwards** to access services running in the clusters
- **MCP servers** (Model Context Protocol) for AI-assisted operations

## Installation

### From Source
```bash
git clone https://github.com/giantswarm/envctl.git
cd envctl
make install
```

### Prerequisites
- Go 1.21+ (for building from source)
- kubectl
- Access to Giant Swarm clusters via Teleport
- MCP server executables (optional, for MCP features)

## Basic Usage

### 1. Connect to a Cluster

Connect to a Management Cluster (MC) only:
```bash
envctl connect <mc-name>
```

Connect to both MC and Workload Cluster (WC):
```bash
envctl connect <mc-name> <wc-name>
```

Example:
```bash
envctl connect gazelle operations
```

### 2. Understanding the TUI

When you run envctl, you'll see a Terminal User Interface (TUI) with several panels:

```
┌─ Cluster Info ─────────────────┐ ┌─ Services ─────────────────────┐
│ MC: gazelle (9/9 nodes)        │ │ ▶ K8s Connections              │
│ WC: operations (8/8 nodes)     │ │   ● k8s-mc-gazelle      [Running] │
│                                │ │   ● k8s-wc-operations   [Running] │
└────────────────────────────────┘ │ ▶ Port Forwards                │
                                   │   ● mc-prometheus       [Running] │
┌─ Logs ─────────────────────────┐ │   ● mc-grafana         [Running] │
│ [INFO] Port forward started... │ │ ▶ MCP Servers                  │
│ [INFO] Service is ready...     │ │   ● prometheus         [Running] │
└────────────────────────────────┘ └────────────────────────────────┘
```

### 3. Navigation

**Basic Controls:**
- `↑/↓` - Navigate between services
- `Tab` - Switch between panels
- `Enter` - Expand/collapse service groups
- `Space` - Select a service
- `q` - Quit envctl

**Service Controls:**
- `x` - Stop selected service
- `s` - Start selected service
- `r` - Restart selected service

**View Controls:**
- `L` - Toggle log view
- `d` - Show dependency graph
- `c` - Clear logs (when in log view)
- `h` - Show help

### 4. Service Management

#### Starting Services
Services start automatically based on their dependencies:
1. K8s connections are established first
2. Port forwards start once K8s is healthy
3. MCP servers start after their required port forwards

#### Stopping Services
When you stop a service, dependent services automatically stop:
- Stopping a K8s connection stops all its port forwards and MCPs
- Stopping a port forward stops MCPs that depend on it

#### Restarting Services
Press `r` to restart a service. envctl will:
- Stop the service
- Wait for cleanup (1 second for ports to release)
- Start the service and any required dependencies

## Common Workflows

### 1. Access Prometheus
```bash
# Start envctl
envctl connect gazelle

# Prometheus will be available at http://localhost:8080
# The mc-prometheus port forward is automatically created
```

### 2. Access Grafana
```bash
# Start envctl
envctl connect gazelle

# Grafana will be available at http://localhost:3000
# The mc-grafana port forward is automatically created
```

### 3. Use MCP Servers
MCP servers require the executables to be installed:

```bash
# Install MCP servers (example)
npm install -g @modelcontextprotocol/server-prometheus
npm install -g @modelcontextprotocol/server-kubernetes

# Start envctl - MCP servers will start automatically
envctl connect gazelle
```

### 4. Work with Workload Clusters
```bash
# Connect to both MC and WC
envctl connect gazelle operations

# This enables:
# - Alloy metrics port forward from the WC
# - Access to WC-specific services
```

## Configuration

### Default Services

envctl comes with predefined services:

**Port Forwards:**
- `mc-prometheus` - Prometheus from MC (port 8080)
- `mc-grafana` - Grafana from MC (port 3000)
- `alloy-metrics` - Alloy metrics from WC (port 12345)

**MCP Servers:**
- `kubernetes` - Kubernetes operations
- `prometheus` - Prometheus queries
- `grafana` - Grafana dashboards

### Custom Configuration

Create a configuration file at `~/.config/envctl/config.yaml`:

```yaml
portForwards:
  - name: my-service
    namespace: my-namespace
    targetType: service
    targetName: my-service-name
    localPort: "9999"
    remotePort: "80"
    kubeContextTarget: mc  # or "wc"
    enabled: true

mcpServers:
  - name: custom-mcp
    type: localCommand
    command: ["my-mcp-server"]
    proxyPort: 8100
    requiresPortForwards: []
    enabled: true
```

## Troubleshooting

### Service Won't Start
1. Check the logs panel (press `L`)
2. Verify dependencies are running
3. Check if ports are already in use

### Port Already in Use
```bash
# Find what's using the port
lsof -i :8080

# Kill the process or change the port in config
```

### K8s Connection Failed
1. Verify Teleport authentication:
   ```bash
   tsh status
   tsh kube ls
   ```
2. Check cluster name spelling
3. Ensure you have access to the cluster

### MCP Server Not Found
Install the required MCP server:
```bash
# Check if installed
which mcp-server-prometheus

# Install if missing
npm install -g @modelcontextprotocol/server-prometheus
```

## Tips and Tricks

### 1. Quick Status Check
The service colors indicate status:
- 🟢 Green = Running
- 🟡 Yellow = Starting/Stopping
- 🔴 Red = Failed
- ⚫ Gray = Stopped

### 2. Dependency Awareness
Services with `(dependency)` were stopped because their dependency failed. They'll restart automatically when the dependency recovers.

### 3. Logs are Your Friend
Always check logs when something goes wrong. The log panel shows real-time output from all services.

### 4. Clean Shutdown
Always use `q` to quit. This ensures all services stop gracefully and ports are released properly.

## Next Steps

- Read the [Architecture Overview](architecture.md) to understand how envctl works
- Check [Configuration Guide](configuration.md) for advanced configuration
- See [Troubleshooting Guide](troubleshooting.md) for detailed problem-solving
- Explore [Development Guide](development.md) to contribute to envctl 