# Containerized MCP Servers

This document describes the support for running MCP servers as containers in envctl.

## Overview

envctl now supports running MCP servers as Docker containers in addition to the traditional local command approach. This provides several benefits:

- **No local dependencies**: Users don't need Node.js, Python, or other runtimes installed
- **Consistency**: All users run the same server versions in identical environments
- **Isolation**: MCP servers run in isolated containers, preventing conflicts
- **Easier deployment**: Container images can be pre-built and distributed

## Configuration

### Server Type

MCP servers can be configured with `type: container` to run as containers:

```yaml
mcpServers:
  - name: kubernetes
    type: container  # instead of localCommand
    enabled: true
    icon: ☸️
    category: Core
    image: giantswarm/mcp-server-kubernetes:latest
    proxyPort: 8001
    containerPorts:
      - "8001:3000"  # host:container
    containerVolumes:
      - "~/.kube/config:/home/mcpuser/.kube/config:ro"
    containerEnv:
      KUBECONFIG: /home/mcpuser/.kube/config
```

### Container-Specific Fields

When `type: container`, these additional fields are available:

- **`image`**: Docker image to use (required)
- **`containerPorts`**: Port mappings in "host:container" format
- **`containerEnv`**: Environment variables for the container
- **`containerVolumes`**: Volume mounts in "host:container:mode" format
- **`healthCheckCmd`**: Optional health check command
- **`entrypoint`**: Override container entrypoint
- **`containerUser`**: User to run the container as

### Dependencies

Containerized servers can still declare dependencies on port forwards:

```yaml
mcpServers:
  - name: prometheus
    type: container
    image: giantswarm/mcp-server-prometheus:latest
    requiresPortForwards:
      - mc-prometheus  # Will wait for this port forward before starting
```

## Container Runtime

By default, Docker is used as the container runtime. This can be configured globally:

```yaml
globalSettings:
  defaultContainerRuntime: docker  # or podman (future)
```

## Building Container Images

Example Dockerfiles are provided in `docker/mcp-servers/`:

```bash
cd docker/mcp-servers
./build.sh  # Build all images

# Or build individually:
docker build -t giantswarm/mcp-server-kubernetes:latest kubernetes/
```

## Network Considerations

### Accessing Host Services

Containers need to access services running on the host (like port-forwarded Kubernetes services). Use:

- **Linux**: `host.docker.internal` or the Docker bridge IP
- **macOS/Windows**: `host.docker.internal`

Example:
```yaml
containerEnv:
  PROMETHEUS_URL: http://host.docker.internal:8080/prometheus
```

### Port Detection

The container manager attempts to detect the actual port the MCP server is listening on by:

1. Parsing container logs for port announcements
2. Using Docker port inspection for mapped ports
3. Falling back to the configured `proxyPort`

## Lifecycle Management

Containerized MCP servers follow the same lifecycle as local servers:

1. **Starting**: Image is pulled (if needed), container is created and started
2. **Running**: Logs are streamed, health is monitored
3. **Stopping**: Container is stopped and removed
4. **Restarting**: Same as stop + start

## Example Configuration

Here's a complete example with both local and containerized servers:

```yaml
mcpServers:
  # Local command server (traditional)
  - name: custom-local
    type: localCommand
    enabled: true
    command: ["npx", "my-custom-mcp-server"]
    proxyPort: 9000

  # Containerized servers
  - name: kubernetes
    type: container
    enabled: true
    image: giantswarm/mcp-server-kubernetes:latest
    proxyPort: 8001
    containerPorts: ["8001:3000"]
    containerVolumes:
      - "~/.kube/config:/home/mcpuser/.kube/config:ro"

  - name: prometheus
    type: container
    enabled: true
    image: giantswarm/mcp-server-prometheus:latest
    proxyPort: 8002
    containerPorts: ["8002:3000"]
    containerEnv:
      PROMETHEUS_URL: http://host.docker.internal:8080/prometheus
    requiresPortForwards: ["mc-prometheus"]

globalSettings:
  defaultContainerRuntime: docker
```

## Troubleshooting

### Container Won't Start

1. Check Docker is running: `docker info`
2. Check image exists: `docker images | grep mcp-server`
3. Check port conflicts: `docker ps` and `netstat -an | grep <port>`

### Can't Access Host Services

1. Verify host.docker.internal resolves: `docker run --rm alpine ping host.docker.internal`
2. Check firewall rules
3. Ensure port forwards are active before dependent containers start

### Viewing Container Logs

Logs are automatically streamed to envctl's output. For direct access:

```bash
docker logs envctl-mcp-<name>-<timestamp>
``` 