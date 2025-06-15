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

### 1. Start envctl

Start the envctl aggregator server with TUI:
```bash
envctl serve
```

Start in CLI mode (no TUI):
```bash
envctl serve --no-tui
```

Example:
```bash
# Start with interactive TUI
envctl serve

# Or start in background for automation
envctl serve --no-tui &
```

### 2. Understanding the TUI

When you run `envctl serve`, you'll see a Terminal User Interface (TUI) with several panels:

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
# Start envctl with configured Prometheus port forward
envctl serve

# Prometheus will be available at http://localhost:8080
# The mc-prometheus port forward is automatically created based on config
```

### 2. Access Grafana
```bash
# Start envctl with configured Grafana port forward
envctl serve

# Grafana will be available at http://localhost:3000
# The mc-grafana port forward is automatically created based on config
```

### 3. Use MCP Servers
MCP servers require the executables to be installed:

```bash
# Install MCP servers (example)
npm install -g @modelcontextprotocol/server-kubernetes
npm install -g @modelcontextprotocol/server-prometheus
npm install -g @modelcontextprotocol/server-grafana

# Start envctl - MCP servers will start automatically based on config
envctl serve
```

### 4. Manage Services
```bash
# Start the aggregator server
envctl serve --no-tui &

# List all services and their status
envctl service list

# Start/stop specific services
envctl service start mc-prometheus
envctl service stop kubernetes-mcp

# Check service details
envctl service status prometheus
```

### 5. Debug and Test MCP
```bash
# Start the aggregator server first
envctl serve --no-tui &

# Debug MCP connections
envctl debug

# Interactive REPL for testing tools
envctl debug --repl
```

## Configuration

### Getting Started

By default, envctl starts with no services configured. To get started, you need to create a configuration file that defines:
- Your Kubernetes clusters and their roles
- Port forwards to access services
- MCP servers for AI-assisted operations

### Configuration File Location

Create your configuration at one of these locations:
- **User config**: `~/.config/envctl/config.yaml` (applies to all projects)
- **Project config**: `./.envctl/config.yaml` (project-specific, git-ignored)

### Example Configuration

Here's a typical configuration for Giant Swarm clusters:

```yaml
# Define your clusters
clusters:
  - name: mc-myinstallation
    context: teleport.giantswarm.io-myinstallation
    role: observability
    displayName: "MC: myinstallation"
    icon: "🏢"
  
  - name: wc-mycluster  
    context: teleport.giantswarm.io-myinstallation-mycluster
    role: target
    displayName: "WC: mycluster"
    icon: "⚙️"

# Set active clusters for each role
activeClusters:
  observability: mc-myinstallation
  target: wc-mycluster

# Port forwards
portForwards:
  - name: mc-prometheus
    namespace: mimir
    targetType: service
    targetName: mimir-query-frontend
    localPort: "8080"
    remotePort: "8080"
    clusterRole: observability
    enabledByDefault: true
    
  - name: mc-grafana
    namespace: monitoring
    targetType: service
    targetName: grafana
    localPort: "3000"
    remotePort: "3000"
    clusterRole: observability
    enabledByDefault: true
    
  - name: alloy-metrics
    namespace: kube-system
    targetType: service
    targetName: alloy-metrics
    localPort: "12345"
    remotePort: "12345"
    clusterRole: target
    enabledByDefault: true

# MCP servers (optional - requires MCP executables)
mcpServers:
  - name: kubernetes
    type: localCommand
    command: ["npx", "mcp-server-kubernetes"]
    requiresClusterRole: target
    enabledByDefault: true
    
  - name: prometheus
    type: localCommand
    command: ["mcp-server-prometheus"]
    env:
      PROMETHEUS_URL: "http://localhost:8080/prometheus"
      ORG_ID: "giantswarm"
    requiresPortForwards: ["mc-prometheus"]
    enabledByDefault: true
```

### Quick Start Templates

You can find example configurations in the envctl repository:
- `.envctl/config.yaml` - Complete template with all options (commented out)
- `.envctl/config-example-basic.yaml` - Minimal example for getting started

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
1. Check the logs panel (press `L` in TUI)
2. Verify dependencies are running: `envctl service list`
3. Check if ports are already in use
4. Use `envctl service status <name>` for detailed info

### Port Already in Use
```bash
# Find what's using the port
lsof -i :8080

# Kill the process or change the port in config
# Then restart the service
envctl service restart mc-prometheus
```

### K8s Connection Failed
1. Verify Teleport authentication:
   ```bash
   tsh status
   tsh kube ls
   ```
2. Check cluster configuration in `.envctl/config.yaml`
3. Ensure you have access to the cluster
4. Check service status: `envctl service status <cluster-connection>`

### MCP Server Not Found
Install the required MCP server:
```bash
# Check if installed
which mcp-server-prometheus

# Install if missing
npm install -g @modelcontextprotocol/server-prometheus

# Check MCP server status
envctl mcpserver list
envctl mcpserver available prometheus
```

### Aggregator Server Not Running
If resource commands fail, ensure the server is running:
```bash
# Check if aggregator is running
envctl service list
# If it fails, start the server
envctl serve --no-tui &
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