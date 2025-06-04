# Cluster Configuration Examples

This document provides practical examples of the flexible cluster configuration system in envctl.

## Table of Contents

1. [Simple Giant Swarm Setup](#simple-giant-swarm-setup)
2. [Multiple Workload Clusters](#multiple-workload-clusters)
3. [Multi-Cloud Setup](#multi-cloud-setup)
4. [Development vs Production](#development-vs-production)
5. [Role-Based Service Configuration](#role-based-service-configuration)
6. [Service-Specific Cluster Targeting](#service-specific-cluster-targeting)

## Simple Giant Swarm Setup

This is the default behavior when you run:
```bash
envctl connect production workload-api
```

The following configuration is automatically generated:

```yaml
clusters:
  - name: "k8s-production"
    context: "teleport.giantswarm.io-production"
    role: "observability"
    displayName: "production"
    icon: "🏢"
  
  - name: "k8s-production-workload-api"
    context: "teleport.giantswarm.io-production-workload-api"
    role: "target"
    displayName: "production-workload-api"
    icon: "🎯"

activeClusters:
  observability: "k8s-production"
  target: "k8s-production-workload-api"
```

## Multiple Workload Clusters

When you have multiple workload clusters and want to switch between them while keeping the same management cluster:

```yaml
clusters:
  - name: "mc-prod"
    context: "teleport.giantswarm.io-production"
    role: "observability"
    displayName: "Production MC"
    icon: "🏢"
  
  - name: "wc-api"
    context: "teleport.giantswarm.io-production-api"
    role: "target"
    displayName: "API Services"
    icon: "🔌"
  
  - name: "wc-web"
    context: "teleport.giantswarm.io-production-web"
    role: "target"
    displayName: "Web Frontend"
    icon: "🌐"
  
  - name: "wc-batch"
    context: "teleport.giantswarm.io-production-batch"
    role: "target"
    displayName: "Batch Processing"
    icon: "⚙️"

# You can switch between workload clusters in the TUI
activeClusters:
  observability: "mc-prod"
  target: "wc-api"  # Change this via TUI to switch workloads
```

## Multi-Cloud Setup

Mix Giant Swarm with cloud provider clusters:

```yaml
clusters:
  # Giant Swarm for core monitoring
  - name: "gs-monitoring"
    context: "teleport.giantswarm.io-monitoring"
    role: "observability"
    displayName: "Giant Swarm Monitoring"
    icon: "🏢"
  
  # AWS EKS for production workloads
  - name: "eks-prod"
    context: "arn:aws:eks:us-east-1:123456:cluster/production"
    role: "target"
    displayName: "AWS Production"
    icon: "☁️"
  
  # GKE for data processing
  - name: "gke-data"
    context: "gke_my-project_us-central1_data-cluster"
    role: "custom"
    displayName: "GKE Data Platform"
    icon: "📊"
  
  # Azure AKS for EU workloads
  - name: "aks-eu"
    context: "aks-eu-west-1-production"
    role: "target"
    displayName: "Azure EU Production"
    icon: "🌍"

activeClusters:
  observability: "gs-monitoring"
  target: "eks-prod"
  custom: "gke-data"

# Services can now connect to different cloud providers
portForwards:
  - name: "grafana"
    clusterRole: "observability"  # From Giant Swarm
    namespace: "monitoring"
    targetType: "service"
    targetName: "grafana"
    localPort: "3000"
    remotePort: "3000"
  
  - name: "app-api"
    clusterRole: "target"  # From AWS EKS
    namespace: "production"
    targetType: "service"
    targetName: "api"
    localPort: "8000"
    remotePort: "80"
```

## Development vs Production

Use project-specific configs to override clusters for local development:

### User Global Config (`~/.config/envctl/config.yaml`)

```yaml
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
```

### Project-Specific Config (`.envctl/config.yaml`)

This overrides the global config when in the project directory:

```yaml
clusters:
  - name: "kind-mc"
    context: "kind-local-mc"
    role: "observability"
    displayName: "Local Kind MC"
    icon: "🐳"
  
  - name: "kind-wc"
    context: "kind-local-wc"
    role: "target"
    displayName: "Local Kind WC"
    icon: "🏠"
  
  - name: "minikube"
    context: "minikube"
    role: "target"
    displayName: "Minikube Test"
    icon: "🧪"

# Override active clusters for development
activeClusters:
  observability: "kind-mc"
  target: "kind-wc"

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

## Role-Based Service Configuration

Services automatically adapt based on active cluster roles:

```yaml
clusters:
  - name: "monitoring-us"
    context: "monitoring.us.example.com"
    role: "observability"
    displayName: "US Monitoring"
    icon: "🇺🇸"
  
  - name: "monitoring-eu"
    context: "monitoring.eu.example.com"
    role: "observability"
    displayName: "EU Monitoring"
    icon: "🇪🇺"
  
  - name: "app-us-east"
    context: "app.us-east.example.com"
    role: "target"
    displayName: "US East Apps"
  
  - name: "app-eu-west"
    context: "app.eu-west.example.com"
    role: "target"
    displayName: "EU West Apps"

# Active clusters determine which services connect where
activeClusters:
  observability: "monitoring-us"  # Change to "monitoring-eu" for EU monitoring
  target: "app-us-east"          # Change to "app-eu-west" for EU apps

# Services automatically use the active clusters
portForwards:
  - name: "prometheus"
    clusterRole: "observability"  # Uses whichever monitoring cluster is active
    namespace: "monitoring"
    targetType: "service"
    targetName: "prometheus"
    localPort: "9090"
    remotePort: "9090"
  
  - name: "app-metrics"
    clusterRole: "target"  # Uses whichever app cluster is active
    namespace: "default"
    targetType: "service"
    targetName: "app-metrics"
    localPort: "8080"
    remotePort: "8080"

mcpServers:
  - name: "kubernetes"
    type: "localCommand"
    command: ["mcp-server-kubernetes"]
    requiresClusterRole: "target"  # Connects to active target cluster
  
  - name: "monitoring"
    type: "localCommand"
    command: ["mcp-server-prometheus"]
    requiresClusterRole: "observability"  # Connects to active monitoring cluster
```

## Service-Specific Cluster Targeting

Different services can target specific clusters regardless of the active cluster settings:

```yaml
clusters:
  - name: "monitoring"
    context: "monitoring.example.com"
    role: "observability"
  
  - name: "app-team-a"
    context: "team-a.example.com"
    role: "target"
  
  - name: "app-team-b"
    context: "team-b.example.com"
    role: "target"
  
  - name: "shared-services"
    context: "shared.example.com"
    role: "custom"

activeClusters:
  observability: "monitoring"
  target: "app-team-a"  # Default target
  custom: "shared-services"

mcpServers:
  # Team-specific Kubernetes MCPs
  - name: "k8s-team-a"
    type: "localCommand"
    command: ["mcp-server-kubernetes"]
    requiresClusterName: "app-team-a"  # Always connects to Team A cluster
    category: "Team A"
  
  - name: "k8s-team-b"
    type: "localCommand"
    command: ["mcp-server-kubernetes"]
    requiresClusterName: "app-team-b"  # Always connects to Team B cluster
    category: "Team B"
  
  # Shared service MCP
  - name: "shared-apis"
    type: "localCommand"
    command: ["mcp-shared-services"]
    requiresClusterName: "shared-services"
    category: "Shared"

portForwards:
  # Team-specific forwards
  - name: "team-a-api"
    clusterName: "app-team-a"  # Always from Team A
    namespace: "production"
    targetType: "service"
    targetName: "api"
    localPort: "8001"
    remotePort: "80"
  
  - name: "team-b-api"
    clusterName: "app-team-b"  # Always from Team B
    namespace: "production"
    targetType: "service"
    targetName: "api"
    localPort: "8002"
    remotePort: "80"
```

## Key Concepts

### Cluster Roles

- **`observability`**: Typically management clusters that host monitoring infrastructure (Prometheus, Grafana, etc.)
- **`target`**: Clusters being monitored or managed (where your applications run)
- **`custom`**: Special-purpose clusters (logging, ML platforms, shared services, etc.)

### Cluster Targeting Options

1. **`clusterRole`**: Service uses whichever cluster is active for that role
   - Flexible: Changes when you switch active clusters
   - Good for: Services that should adapt to your current focus

2. **`clusterName`**: Service always uses a specific cluster
   - Fixed: Doesn't change with active cluster switches
   - Good for: Team-specific or specialized services

3. **`kubeContextTarget`** (deprecated): Legacy option for backward compatibility
   - Use `clusterRole` or `clusterName` instead

### Benefits

1. **Flexibility**: Switch between environments without changing configuration
2. **Team Isolation**: Different teams can work on their clusters simultaneously
3. **Multi-Cloud**: Mix and match clusters from different providers
4. **Development**: Easy local development with production-like configuration
5. **Backward Compatible**: Works seamlessly with existing Giant Swarm setups 