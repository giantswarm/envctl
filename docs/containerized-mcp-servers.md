# Containerized MCP Servers

This document describes how to configure and use containerized MCP servers in envctl.

## Overview

envctl supports running MCP (Model Context Protocol) servers as Docker containers instead of requiring local installations of Node.js or Python. This provides better isolation, reproducibility, and easier dependency management.

## Configuration

To configure a containerized MCP server, use the `container` type in your configuration:

```yaml
mcpServers:
  - name: kubernetes
    enabled: true
    type: container
    image: ghcr.io/giantswarm/envctl/mcp-kubernetes:latest
    containerPorts:
      - "3000:3000"
    containerEnv:
      - MCP_ENV=production
    containerVolumes:
      - "$HOME/.kube:/home/node/.kube:ro"
    healthCheckCmd: ["node", "--version"]
    containerUser: "1000:1000"
```

### Configuration Fields

- `type`: Must be `container` for containerized servers
- `image`: Docker image to use
- `containerPorts`: Port mappings in Docker format `"host:container"`
- `containerEnv`: Environment variables to set in the container
- `containerVolumes`: Volume mounts (supports `~` expansion)
- `healthCheckCmd`: Command to verify container health (optional)
- `entrypoint`: Override the container's entrypoint (optional)
- `containerUser`: User to run the container as (optional)

## Kubernetes Dependencies and Cascading Stops

envctl models kubernetes connections as dependencies in its service dependency graph. This enables intelligent service lifecycle management:

### K8s Connection as a Dependency

- **Port forwards** depend on their kubernetes context being authenticated and healthy
- The **kubernetes MCP server** depends on the management cluster context being available
- **Other MCP servers** may depend on port forwards (configured via `requiresPortForwards`)

### Cascading Stop Behavior

When you stop a service, envctl automatically stops all dependent services:

1. **Stopping a k8s connection** (when it becomes unhealthy):
   - All port forwards using that context are stopped
   - Any MCP servers depending on those port forwards are stopped

2. **Stopping a port forward**:
   - Any MCP servers that require that port forward are stopped

3. **Manual stop with 'x' key**:
   - Uses cascading stop to cleanly shut down dependent services

### Health Monitoring

envctl continuously monitors k8s connection health:
- Authenticated state is tracked after login
- Cluster health is checked periodically (every 30 seconds)
- If a k8s connection becomes unhealthy, dependent services are automatically stopped

This ensures that services don't continue running with broken dependencies, preventing confusing error states.

## Network Considerations

### Host Network Mode (Recommended)

For MCP servers that need to access cluster services via port forwards, use host network mode:

```yaml
containerPorts:
  - "3000:3000"
# The container will use --network host automatically when accessing localhost services
```

### Bridge Network Mode

If you need network isolation, ensure proper connectivity:
- Use `host.docker.internal` to access host services from the container
- Configure the MCP server to connect to `host.docker.internal:port` instead of `localhost:port`

## Container Lifecycle

1. **Startup**: 
   - Container image is pulled if not present
   - Container is created with specified configuration
   - Health check is performed (if configured)
   - Proxy port is detected from container logs

2. **Runtime**:
   - Container logs are streamed to envctl logs
   - Container health is monitored
   - Restart on failure is handled by envctl

3. **Shutdown**:
   - Container receives SIGTERM
   - 10-second grace period for cleanup
   - Container is removed after stopping

## Example: Prometheus MCP Server

```yaml
mcpServers:
  - name: prometheus  
    enabled: true
    type: container
    image: ghcr.io/giantswarm/envctl/mcp-prometheus:latest
    requiresPortForwards: ["mc-prometheus"]
    containerEnv:
      - PROMETHEUS_URL=http://localhost:9090
    containerPorts:
      - "3001:3000"
```

This configuration:
- Runs the Prometheus MCP server in a container
- Requires the `mc-prometheus` port forward to be active
- Accesses Prometheus at `localhost:9090` (via host network)
- Exposes the MCP proxy on host port 3001

## Building Custom MCP Server Images

See the `docker/mcp-servers/` directory for Dockerfile examples. Key considerations:

1. **Base Image**: Use appropriate base for your runtime (node:20-alpine, python:3.11-slim)
2. **Dependencies**: Install all required packages in the image
3. **User**: Run as non-root user for security
4. **Entrypoint**: Set a clear entrypoint that starts the MCP server

## Troubleshooting

### Container Won't Start

Check logs with:
```bash
docker logs envctl-mcp-<server-name>
```

Common issues:
- Port already in use
- Volume mount paths don't exist
- Image pull failures

### Can't Access Port Forwards

If using bridge network:
- Ensure MCP server is configured to use `host.docker.internal`
- Check firewall rules

### Health Check Failures

- Verify the health check command is available in the container
- Check container has necessary permissions
- Review container logs for startup errors 