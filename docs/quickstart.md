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
envctl connect myinstallation mycluster
```

### 2. Understanding the TUI

When you run envctl, you'll see a Terminal User Interface (TUI) with several panels:

```
┌─ Cluster Info ─────────────────┐ ┌─ Cluster Info ─────────────────┐
│ ☸ MC: installation            │ │ ☸ WC: cluster (Active)         │
│ ✔ Nodes: 9/9                  │ │ ✔ Nodes: 8/8                   │
└────────────────────────────────┘ └────────────────────────────────┘

┌─ Port Forwards ─────────────────────────────────────────────────────┐
│ 🔗 mc-prometheus        Port: 8080:80         [Running]             │
│ 🔗 mc-grafana          Port: 3000:3000        [Running]             │
│ 🔗 alloy-metrics       Port: 12345:12345      [Running]             │
└─────────────────────────────────────────────────────────────────────┘

┌─ MCP Servers ───────────────────────────────────────────────────────┐
│ ☸ kubernetes           Port: 8001            [Running]             │
│ 🔥 prometheus          Port: 8002             [Running]             │
│ 📊 grafana             Port: 8003             [Running]             │
└─────────────────────────────────────────────────────────────────────┘

┌─ Logs ──────────────────────────────────────────────────────────────┐
│ [INFO] K8s connection established to myinstallation                 │
│ [INFO] Port forward mc-prometheus started on localhost:8080         │
│ [INFO] MCP server kubernetes is ready on port 8001                  │
└─────────────────────────────────────────────────────────────────────┘
```

### 3. Navigation

**Basic Controls:**
- `Tab` / `Shift+Tab` - Navigate between panels
- `↑/↓` or `j/k` - Move up/down within panels
- `Enter` - Start a stopped service
- `q` / `Ctrl+C` - Quit envctl

**Service Controls:**
- `x` - Stop selected service
- `r` - Restart selected service
- `s` - Switch to the K8s context of the selected cluster

**View Controls:**
- `h` or `?` - Show help overlay
- `L` - Toggle log overlay
- `C` - Show MCP configuration
- `M` - Show MCP tools
- `D` - Toggle dark/light mode
- `z` - Toggle debug mode
- `y` - Copy logs/config (when in overlay)
- `Esc` - Close overlays

### 4. Service Management

#### Service Dependencies
envctl manages services in a dependency hierarchy:
- **K8s Connections** - Foundation layer (no dependencies)
- **Port Forwards** - Depend on K8s connections
- **MCP Servers** - May depend on port forwards

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
envctl connect myinstallation

# Prometheus will be available at http://localhost:8080
# The mc-prometheus port forward is automatically created
```

### 2. Access Grafana
```bash
# Start envctl
envctl connect myinstallation

# Grafana will be available at http://localhost:3000
# The mc-grafana port forward is automatically created
```

### 3. Use MCP Servers
MCP servers require the executables to be installed:

```bash
# Install MCP servers (example)
npm install -g @modelcontextprotocol/server-kubernetes
npm install -g @modelcontextprotocol/server-prometheus
npm install -g @modelcontextprotocol/server-grafana

# Start envctl - MCP servers will start automatically
envctl connect myinstallation
```

### 4. Work with Workload Clusters
```bash
# Connect to both MC and WC
envctl connect myinstallation mycluster

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

envctl supports a flexible configuration system that allows you to:
- Define multiple clusters with roles (observability, target, custom)
- Switch between clusters dynamically
- Configure services to use specific clusters
- Support multi-cloud environments

Create a configuration file at `~/.config/envctl/config.yaml`:

```yaml
# Simple example - see Configuration Guide for advanced options
portForwards:
  - name: my-service
    namespace: my-namespace
    targetType: service
    targetName: my-service-name
    localPort: "9999"
    remotePort: "80"
    clusterRole: "observability"  # Uses active observability cluster
    enabledByDefault: true

mcpServers:
  - name: custom-mcp
    type: localCommand
    command: ["my-mcp-server"]
    requiresClusterRole: "target"  # Connects to active target cluster
    enabledByDefault: true
```

For advanced cluster configurations including multi-environment setups, see:
- [Configuration Guide](configuration.md) - Complete configuration reference
- [Cluster Configuration Examples](cluster-configuration-examples.md) - Practical examples

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
Services show their dependencies and will indicate when they're stopped due to a dependency failure. They'll restart automatically when the dependency recovers.

### 3. Logs are Your Friend
Always check logs when something goes wrong. The log panel shows real-time output from all services.

### 4. Clean Shutdown
Always use `q` to quit. This ensures all services stop gracefully and ports are released properly.

## Next Steps

- Read the [Architecture Overview](architecture.md) to understand how envctl works
- Check [Configuration Guide](configuration.md) for advanced configuration
- See [Troubleshooting Guide](troubleshooting.md) for detailed problem-solving
- Explore [Development Guide](development.md) to contribute to envctl 